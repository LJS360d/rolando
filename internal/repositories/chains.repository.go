package repositories

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// ChainConfig is the canonical config record, stored durably in SQLite and
// cached in Redis as a hash at config:<guild_id>.
type ChainConfig struct {
	ID                string     `gorm:"primaryKey"      json:"id"`
	Name              string     `gorm:"not null"        json:"name"`
	ReplyRate         int        `gorm:"default:10"      json:"reply_rate"`
	ReactionRate      int        `gorm:"default:30"      json:"reaction_rate"`
	VcJoinRate        int        `gorm:"default:100"     json:"vc_join_rate"`
	MaxSizeMb         int        `gorm:"default:25" json:"max_size_mb"`
	NGramSize         int        `gorm:"default:2"  json:"n_gram_size"`
	MarkovMaxBranches int        `gorm:"default:0"  json:"markov_max_branches"`
	TTSLanguage       string     `gorm:"default:'en'"    json:"tts_language"`
	Pings             bool       `gorm:"default:true"    json:"pings"`
	TrainedAt         *time.Time `gorm:"default:null"    json:"trained_at"`
	UpdatedAt         time.Time  `gorm:"autoUpdateTime"  json:"updated_at"`
	Premium           bool       `gorm:"default:false"   json:"premium"`
}

// MaxSizeBytes returns the configured size limit in bytes (0 = unlimited).
func (c *ChainConfig) MaxSizeBytes() int {
	return c.MaxSizeMb * 1024 * 1024
}

// ChainsRepository persists ChainConfig in SQLite and caches it in Redis.
// Redis is always tried first; SQLite is the source of truth for durability.
type ChainsRepository struct {
	DB  *gorm.DB
	rdb *redis.Client
}

func NewChainsRepository(dbPath string, rdb *redis.Client) (*ChainsRepository, error) {
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&ChainConfig{}); err != nil {
		return nil, err
	}
	return &ChainsRepository{DB: db, rdb: rdb}, nil
}

// ---------- public API ----------

// CreateChain inserts a new chain into SQLite and warms the Redis cache.
func (repo *ChainsRepository) CreateChain(id, name string) (*ChainConfig, error) {
	chain := &ChainConfig{
		ID:          id,
		Name:        name,
		ReplyRate:   10,
		NGramSize:   2,
		Pings:       true,
		MaxSizeMb:   25,
		TTSLanguage: "en",
	}
	if err := repo.DB.Create(chain).Error; err != nil {
		return nil, err
	}
	repo.warmCache(context.Background(), chain)
	return chain, nil
}

// GetChainByID fetches from Redis first, falling back to SQLite on a cache miss.
func (repo *ChainsRepository) GetChainByID(id string) (*ChainConfig, error) {
	if chain, err := repo.getFromCache(context.Background(), id); err == nil {
		return chain, nil
	}
	// Cache miss — load from SQLite and warm cache.
	var chain ChainConfig
	if err := repo.DB.First(&chain, "id = ?", id).Error; err != nil {
		return nil, err
	}
	repo.warmCache(context.Background(), &chain)
	return &chain, nil
}

// GetAll returns all chains from DB (admin/bulk path; not cache-critical).
func (repo *ChainsRepository) GetAll() ([]*ChainConfig, error) {
	var chains []*ChainConfig
	return chains, repo.DB.Find(&chains).Error
}

// GetChainsPage returns a paginated slice and the total count.
func (repo *ChainsRepository) GetChainsPage(limit, offset int) ([]*ChainConfig, int64, error) {
	var (
		elements []*ChainConfig
		total    int64
	)
	if err := repo.DB.Model(&ChainConfig{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := repo.DB.Limit(limit).Offset(offset).Find(&elements).Error; err != nil {
		return nil, 0, err
	}
	return elements, total, nil
}

// UpdateChain applies field-level updates to SQLite and refreshes the Redis cache.
func (repo *ChainsRepository) UpdateChain(id string, fields map[string]any) (*ChainConfig, error) {
	var chain ChainConfig
	if err := repo.DB.First(&chain, "id = ?", id).Error; err != nil {
		return nil, err
	}
	if err := repo.DB.Model(&chain).Updates(fields).Error; err != nil {
		return nil, err
	}
	// Re-read the full updated record so the cache is always coherent.
	if err := repo.DB.First(&chain, "id = ?", id).Error; err != nil {
		return nil, err
	}
	repo.warmCache(context.Background(), &chain)
	return &chain, nil
}

// DeleteChain removes the chain from SQLite and evicts the Redis cache entry.
func (repo *ChainsRepository) DeleteChain(id string) error {
	if err := repo.DB.Delete(&ChainConfig{}, "id = ?", id).Error; err != nil {
		return err
	}
	repo.evictCache(context.Background(), id)
	return nil
}

// WarmCacheForAll pre-loads all chains into Redis at startup / after migration.
func (repo *ChainsRepository) WarmCacheForAll(ctx context.Context) error {
	chains, err := repo.GetAll()
	if err != nil {
		return err
	}
	for _, c := range chains {
		repo.warmCache(ctx, c)
	}
	return nil
}

// CountChains returns the total number of chain records in SQLite.
func (repo *ChainsRepository) CountChains() (int64, error) {
	var count int64
	return count, repo.DB.Model(&ChainConfig{}).Count(&count).Error
}

// ---------- cache internals ----------

const configCacheTTL = 0 // no TTL — cache is invalidated explicitly on writes

func (repo *ChainsRepository) warmCache(ctx context.Context, c *ChainConfig) {
	args := chainConfigToArgs(c)
	if len(args) == 0 {
		return
	}
	// HSET config:<id> field value [field value ...]
	key := "config:" + c.ID
	_ = repo.rdb.HSet(ctx, key, args...).Err()
}

func (repo *ChainsRepository) evictCache(ctx context.Context, id string) {
	_ = repo.rdb.Del(ctx, "config:"+id).Err()
}

func (repo *ChainsRepository) getFromCache(ctx context.Context, id string) (*ChainConfig, error) {
	vals, err := repo.rdb.HGetAll(ctx, "config:"+id).Result()
	if err != nil || len(vals) == 0 {
		return nil, fmt.Errorf("cache miss")
	}
	return chainConfigFromMap(vals)
}

// ---------- serialisation helpers ----------

// chainConfigToArgs returns a flat []any{field, value, ...} slice suitable
// for redis.HSet.
func chainConfigToArgs(c *ChainConfig) []any {
	trainedAt := ""
	if c.TrainedAt != nil {
		trainedAt = c.TrainedAt.UTC().Format(time.RFC3339)
	}
	pings := "0"
	if c.Pings {
		pings = "1"
	}
	premium := "0"
	if c.Premium {
		premium = "1"
	}
	return []any{
		"id", c.ID,
		"name", c.Name,
		"reply_rate", strconv.Itoa(c.ReplyRate),
		"reaction_rate", strconv.Itoa(c.ReactionRate),
		"vc_join_rate", strconv.Itoa(c.VcJoinRate),
		"max_size_mb", strconv.Itoa(c.MaxSizeMb),
		"n_gram_size", strconv.Itoa(c.NGramSize),
		"markov_max_branches", strconv.Itoa(c.MarkovMaxBranches),
		"tts_language", c.TTSLanguage,
		"pings", pings,
		"trained_at", trainedAt,
		"updated_at", c.UpdatedAt.UTC().Format(time.RFC3339),
		"premium", premium,
	}
}

func chainConfigFromMap(m map[string]string) (*ChainConfig, error) {
	c := &ChainConfig{}
	var err error

	c.ID = m["id"]
	c.Name = m["name"]
	c.TTSLanguage = m["tts_language"]

	if c.ReplyRate, err = strconv.Atoi(m["reply_rate"]); err != nil {
		return nil, fmt.Errorf("reply_rate: %w", err)
	}
	if c.ReactionRate, err = strconv.Atoi(m["reaction_rate"]); err != nil {
		return nil, fmt.Errorf("reaction_rate: %w", err)
	}
	if c.VcJoinRate, err = strconv.Atoi(m["vc_join_rate"]); err != nil {
		return nil, fmt.Errorf("vc_join_rate: %w", err)
	}
	if c.MaxSizeMb, err = strconv.Atoi(m["max_size_mb"]); err != nil {
		return nil, fmt.Errorf("max_size_mb: %w", err)
	}
	if c.NGramSize, err = strconv.Atoi(m["n_gram_size"]); err != nil {
		return nil, fmt.Errorf("n_gram_size: %w", err)
	}
	if s := m["markov_max_branches"]; s != "" {
		if c.MarkovMaxBranches, err = strconv.Atoi(s); err != nil {
			return nil, fmt.Errorf("markov_max_branches: %w", err)
		}
	}

	c.Pings = m["pings"] == "1"
	c.Premium = m["premium"] == "1"

	if s := m["trained_at"]; s != "" {
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			return nil, fmt.Errorf("trained_at: %w", err)
		}
		c.TrainedAt = &t
	}
	if s := m["updated_at"]; s != "" {
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			return nil, fmt.Errorf("updated_at: %w", err)
		}
		c.UpdatedAt = t
	}

	return c, nil
}
