package guilds

import (
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"rolando/internal/chain"
	"rolando/internal/logger"
	"rolando/internal/utils"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	gormlogger "gorm.io/gorm/logger"
)

type Config struct {
	ID                string     `gorm:"primaryKey"     json:"id"`
	Name              string     `gorm:"not null"       json:"name"`
	ReplyRate         int        `gorm:"default:10"     json:"reply_rate"`
	ReactionRate      int        `gorm:"default:30"     json:"reaction_rate"`
	VcJoinRate        int        `gorm:"default:100"    json:"vc_join_rate"`
	MaxSizeMb         int        `gorm:"default:25"     json:"max_size_mb"`
	NGramSize         int        `gorm:"default:2"      json:"n_gram_size"`
	MarkovMaxBranches int        `gorm:"default:256"    json:"markov_max_branches"`
	TTSLanguage       string     `gorm:"default:'en'"   json:"tts_language"`
	Pings             bool       `gorm:"default:true"   json:"pings"`
	TrainedAt         *time.Time `gorm:"default:null"   json:"trained_at"`
	UpdatedAt         time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
	Premium           bool       `gorm:"default:false"  json:"premium"`
}

func (Config) TableName() string { return "config" }

func (c *Config) MaxSizeBytes() uint64 { return uint64(max(c.MaxSizeMb, 0)) << 20 }

type Message struct {
	ID        int64     `gorm:"primaryKey"`
	Content   string    `gorm:"type:text"`
	CreatedAt time.Time `gorm:"index"`
}

type Media struct {
	URL  string `gorm:"primaryKey"`
	Kind string `gorm:"index"`
}

type MediaCounts struct {
	Gifs   int64 `json:"gifs"`
	Images int64 `json:"images"`
	Videos int64 `json:"videos"`
}

// Summary is what admin listings need without loading the chain. It is
// persisted next to the snapshot and refreshed on snapshot/config writes.
type Summary struct {
	Config Config      `json:"config"`
	Stats  chain.Stats `json:"stats"`
	Media  MediaCounts `json:"media"`
}

type Guild struct {
	ID  string
	dir string
	DB  *gorm.DB

	mu    sync.Mutex
	chain atomic.Pointer[chain.Chain]
	// cfg caches the config row, read on every message; nil until read.
	// Writes hold cfgMu so a concurrent miss can't cache the old row.
	cfgMu sync.Mutex
	cfg   *Config
	// snapMu serializes snapshot writers, which share chain.snap.tmp
	snapMu sync.Mutex
	// stored message count, kept under mu once counted
	msgCount   int64
	msgCounted bool

	refs     int
	lastUsed time.Time
}

// migrated holds the guild dirs migrated by this process.
var migrated sync.Map

// Open opens (creating if needed) the guild directory dir.
func Open(dir, id string) (*Guild, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	_, statErr := os.Stat(filepath.Join(dir, "guild.db"))
	fresh := statErr != nil
	db, err := gorm.Open(sqlite.Open(filepath.Join(dir, "guild.db")+"?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(10000)&_pragma=cache_size(-1024)"),
		// single statements are atomic in SQLite; multi-statement writes use explicit transactions
		&gorm.Config{Logger: dbLogger, SkipDefaultTransaction: true})
	if err != nil {
		return nil, err
	}
	// pure-Go SQLite: each connection's page cache sits on the Go heap
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.SetMaxOpenConns(2)
		sqlDB.SetMaxIdleConns(1)
	}
	// the models can't change while the process runs, so a guild reopened
	// after idle eviction skips the schema introspection; a new file never does
	if _, done := migrated.Load(dir); !done || fresh {
		if err := db.AutoMigrate(&Config{}, &Message{}, &Media{}); err != nil {
			closeDB(db)
			return nil, err
		}
		migrated.Store(dir, struct{}{})
	}
	return &Guild{ID: id, dir: dir, DB: db}, nil
}

var dbLogger = gormlogger.NewSlogLogger(slog.Default(), gormlogger.Config{
	LogLevel: gormlogger.Warn, SlowThreshold: time.Second, IgnoreRecordNotFoundError: true})

func (g *Guild) Close() { closeDB(g.DB) }

func closeDB(db *gorm.DB) {
	if s, err := db.DB(); err == nil {
		s.Close()
	}
}

func (g *Guild) snapPath() string    { return filepath.Join(g.dir, "chain.snap") }
func (g *Guild) summaryPath() string { return filepath.Join(g.dir, "summary.json") }

// Config returns a copy of the guild config, cached until it changes.
func (g *Guild) Config() (*Config, error) {
	g.cfgMu.Lock()
	defer g.cfgMu.Unlock()
	if g.cfg == nil {
		var c Config
		if err := g.DB.First(&c).Error; err != nil {
			return &c, err
		}
		g.cfg = &c
	}
	cp := *g.cfg
	return &cp, nil
}

// writeConfig runs a write touching the config row and drops the cache.
func (g *Guild) writeConfig(write func() error) error {
	g.cfgMu.Lock()
	defer g.cfgMu.Unlock()
	g.cfg = nil
	return write()
}

func (g *Guild) CreateConfig(name string) (*Config, error) {
	c := &Config{ID: g.ID, Name: name, ReplyRate: 10, ReactionRate: 30, VcJoinRate: 100, MaxSizeMb: 25,
		NGramSize: 2, MarkovMaxBranches: 256, TTSLanguage: "en", Pings: true}
	if err := g.writeConfig(func() error { return g.DB.Create(c).Error }); err != nil {
		return nil, err
	}
	return c, nil
}

func (g *Guild) UpdateConfig(fields map[string]any) (*Config, error) {
	err := g.writeConfig(func() error { return g.DB.Model(&Config{}).Where("id = ?", g.ID).Updates(fields).Error })
	if err != nil {
		return nil, err
	}
	c, err := g.Config()
	if err == nil {
		g.WriteSummary()
	}
	return c, err
}

// Chain returns the loaded chain, loading the snapshot and replaying newer
// messages (or rebuilding from scratch) on first use.
func (g *Guild) Chain() (*chain.Chain, error) {
	if c := g.chain.Load(); c != nil {
		return c, nil
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if c := g.chain.Load(); c != nil {
		return c, nil
	}
	cfg, err := g.Config()
	if err != nil {
		return nil, err
	}
	start := time.Now()
	logger.Debugf("guild %s: loading chain", g.label())
	src := "snapshot"
	c, err := chain.ReadFile(g.snapPath(), cfg.NGramSize)
	if err != nil {
		c = chain.New(cfg.NGramSize)
		src = "rebuild"
	}
	if err := g.replay(c, cfg); err != nil {
		return nil, err
	}
	g.chain.Store(c)
	logger.Debugf("guild %s: chain loaded (%s) in %s", g.label(), src, time.Since(start).Round(time.Millisecond))
	return c, nil
}

func (g *Guild) label() string {
	if c, err := g.Config(); err == nil && c.Name != "" {
		return c.Name + " (" + g.ID + ")"
	}
	return g.ID
}

func (g *Guild) loaded() *chain.Chain { return g.chain.Load() }

func (g *Guild) replay(c *chain.Chain, cfg *Config) error {
	const batch = 2000
	last := c.Watermark()
	// scanned directly: gorm's per-row reflection into structs dominates a rebuild
	texts := make([]string, 0, batch)
	for {
		rows, err := g.DB.Model(&Message{}).Select("id", "content").Where("id > ?", last).Order("id").Limit(batch).Rows()
		if err != nil {
			return err
		}
		texts = texts[:0]
		for rows.Next() {
			var text string
			if err := rows.Scan(&last, &text); err != nil {
				rows.Close()
				return err
			}
			texts = append(texts, text)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		if len(texts) == 0 {
			return nil
		}
		c.Train(texts, last, cfg.MaxSizeBytes(), cfg.MarkovMaxBranches)
	}
}

// AddMessages persists messages and their media, then trains the chain if
// it is loaded. An unloaded chain picks them up on its next load.
func (g *Guild) AddMessages(texts []string) error {
	if len(texts) == 0 {
		return nil
	}
	cfg, err := g.Config()
	if err != nil {
		return err
	}
	rows := make([]Message, len(texts))
	var media []Media
	for i, t := range texts {
		rows[i].Content = t
		for _, u := range utils.ExtractUrls(t) {
			if k := utils.ClassifyURL(u); k != "" {
				media = append(media, Media{URL: u, Kind: k})
			}
		}
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	err = g.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.CreateInBatches(rows, 500).Error; err != nil {
			return err
		}
		if len(media) == 0 {
			return nil
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(media, 500).Error
	})
	if err != nil {
		return err
	}
	g.msgCount += int64(len(rows))
	if c := g.loaded(); c != nil {
		c.Train(texts, rows[len(rows)-1].ID, cfg.MaxSizeBytes(), cfg.MarkovMaxBranches)
	}
	return nil
}

// DeleteMessage removes every stored copy of content, its media and one
// occurrence of its transitions.
func (g *Guild) DeleteMessage(content string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	var deleted int64
	err := g.DB.Transaction(func(tx *gorm.DB) error {
		res := tx.Where("content = ?", content).Delete(&Message{})
		if res.Error != nil {
			return res.Error
		}
		deleted = res.RowsAffected
		if urls := utils.ExtractUrls(content); len(urls) > 0 {
			return tx.Where("url IN ?", urls).Delete(&Media{}).Error
		}
		return nil
	})
	if err != nil {
		return err
	}
	g.msgCount -= deleted
	if c := g.loaded(); c != nil {
		c.Delete(content)
	}
	return nil
}

func (g *Guild) Reset() error {
	cfg, err := g.Config()
	if err != nil {
		return err
	}
	// a snapshot in flight would rename the old chain back over the removal
	g.snapMu.Lock()
	defer g.snapMu.Unlock()
	g.mu.Lock()
	defer g.mu.Unlock()
	err = g.writeConfig(func() error {
		return g.DB.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("1 = 1").Delete(&Message{}).Error; err != nil {
				return err
			}
			if err := tx.Where("1 = 1").Delete(&Media{}).Error; err != nil {
				return err
			}
			return tx.Model(&Config{}).Where("id = ?", g.ID).Update("trained_at", nil).Error
		})
	})
	g.msgCounted = false
	if err != nil {
		return err
	}
	g.chain.Store(chain.New(cfg.NGramSize))
	os.Remove(g.snapPath())
	return nil
}

// Rebuild retrains the chain from all stored messages with the current
// config, then swaps it in. Live writes keep landing on the old chain and are
// replayed from SQLite before the swap.
func (g *Guild) Rebuild() error {
	cfg, err := g.Config()
	if err != nil {
		return err
	}
	c := chain.New(cfg.NGramSize)
	if err := g.replay(c, cfg); err != nil {
		return err
	}
	g.mu.Lock()
	err = g.replay(c, cfg)
	if err == nil {
		g.chain.Store(c)
	}
	g.mu.Unlock()
	if err != nil {
		return err
	}
	// the old chain is garbage now; hand it back to the OS instead of keeping RSS at the peak
	debug.FreeOSMemory()
	return g.Snapshot()
}

// Snapshot writes chain.snap if the chain is loaded and dirty.
func (g *Guild) Snapshot() error {
	g.snapMu.Lock()
	defer g.snapMu.Unlock()
	c := g.loaded()
	if c == nil || !c.Dirty() {
		return nil
	}
	if err := c.WriteFile(g.snapPath()); err != nil {
		return err
	}
	return g.WriteSummary()
}

func (g *Guild) RandomMedia(kind string) (string, error) {
	var urls []string
	err := g.DB.Model(&Media{}).Where("kind = ?", kind).Order("RANDOM()").Limit(1).Pluck("url", &urls).Error
	if err != nil || len(urls) == 0 {
		return "", err
	}
	return urls[0], nil
}

func (g *Guild) MediaCounts() (MediaCounts, error) {
	var rows []struct {
		Kind string
		N    int64
	}
	var mc MediaCounts
	if err := g.DB.Model(&Media{}).Select("kind, COUNT(*) AS n").Group("kind").Scan(&rows).Error; err != nil {
		return mc, err
	}
	for _, r := range rows {
		switch r.Kind {
		case "gif":
			mc.Gifs = r.N
		case "image":
			mc.Images = r.N
		case "video":
			mc.Videos = r.N
		}
	}
	return mc, nil
}

func (g *Guild) MessagesPage(limit, offset int) ([]Message, int64, error) {
	total, err := g.messageCount()
	if err != nil {
		return nil, 0, err
	}
	var rows []Message
	err = g.DB.Order("id").Limit(limit).Offset(offset).Find(&rows).Error
	return rows, total, err
}

// messageCount counts stored messages once; writers keep it current.
func (g *Guild) messageCount() (int64, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.msgCounted {
		if err := g.DB.Model(&Message{}).Count(&g.msgCount).Error; err != nil {
			return 0, err
		}
		g.msgCounted = true
	}
	return g.msgCount, nil
}

// Summary computes stats from the loaded chain, falling back to the last
// persisted summary for chain stats when the chain is not loaded.
func (g *Guild) Summary() (Summary, error) {
	var s Summary
	cfg, err := g.Config()
	if err != nil {
		return s, err
	}
	if c := g.loaded(); c != nil {
		s.Stats = c.Stats()
	} else if old, err := readSummary(g.summaryPath()); err == nil {
		s.Stats = old.Stats
	}
	s.Config = *cfg
	s.Media, err = g.MediaCounts()
	return s, err
}

func (g *Guild) WriteSummary() error {
	s, err := g.Summary()
	if err != nil {
		return err
	}
	b, _ := json.Marshal(s)
	tmp := g.summaryPath() + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, g.summaryPath())
}

func readSummary(path string) (Summary, error) {
	var s Summary
	b, err := os.ReadFile(path)
	if err != nil {
		return s, err
	}
	return s, json.Unmarshal(b, &s)
}

func IsNotFound(err error) bool { return errors.Is(err, gorm.ErrRecordNotFound) }
