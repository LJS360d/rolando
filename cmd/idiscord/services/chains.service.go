package services

import (
	"context"
	"errors"
	"fmt"
	"rolando/internal/analytics"
	"rolando/internal/logger"
	"rolando/internal/repositories"
	"strconv"
	"sync"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/snowflake/v2"
)

type contextKey string

const skipFetchingCheckKey contextKey = "skipFetchingCheck"

type ChainsService struct {
	session      *bot.Client
	chainsRepo   *repositories.ChainsRepository
	redisRepo    *repositories.RedisRepository
	messagesRepo *repositories.MessagesRepository

	// rebuildMu prevents concurrent rebuilds of the same guild.
	rebuildMu sync.Map // map[string]*sync.Mutex

	// bulkTrainingMu ensures at most one bulk Redis train (history import or
	// n-gram rebuild) runs at a time so long train_batch scripts do not stack
	// against live per-message TrainBatch traffic from other guilds.
	bulkTrainingMu sync.Mutex
}

func NewChainsService(
	client *bot.Client,
	chainsRepo *repositories.ChainsRepository,
	redisRepo *repositories.RedisRepository,
	messagesRepo *repositories.MessagesRepository,
) *ChainsService {
	return &ChainsService{
		session:      client,
		chainsRepo:   chainsRepo,
		redisRepo:    redisRepo,
		messagesRepo: messagesRepo,
	}
}

func (cs *ChainsService) NewMarkovAnalyzer(chain *repositories.ChainConfig) *analytics.MarkovChainAnalyzer {
	return analytics.NewMarkovChainAnalyzer(chain, cs.redisRepo)
}

func (cs *ChainsService) Train(ctx context.Context, guildID, message string, nGramSize, maxSizeBytes, maxBranches int) error {
	if err := cs.CheckFetchingAndBlock(ctx, guildID); err != nil {
		return err
	}
	return cs.redisRepo.Train(ctx, guildID, message, nGramSize, maxSizeBytes, maxBranches)
}

func (cs *ChainsService) Generate(ctx context.Context, guildID string, maxLength, nGramSize int) (string, error) {
	if err := cs.CheckFetchingAndBlock(ctx, guildID); err != nil {
		return "", err
	}
	return cs.redisRepo.Generate(ctx, guildID, maxLength, nGramSize)
}

func (cs *ChainsService) GenerateFromSeed(ctx context.Context, guildID, seed string, maxLength, nGramSize int) (string, error) {
	if err := cs.CheckFetchingAndBlock(ctx, guildID); err != nil {
		return "", err
	}
	return cs.redisRepo.GenerateFromSeed(ctx, guildID, seed, maxLength, nGramSize)
}

func (cs *ChainsService) GenerateFiltered(ctx context.Context, guildID string, maxLength, nGramSize int) (string, error) {
	if err := cs.CheckFetchingAndBlock(ctx, guildID); err != nil {
		return "", err
	}
	return cs.redisRepo.GenerateFiltered(ctx, guildID, maxLength, nGramSize)
}

func (cs *ChainsService) GenerateLine(ctx context.Context, guildID string, maxWords int) (string, error) {
	if guildID == "" {
		return "", fmt.Errorf("empty guild id")
	}
	conf, err := cs.GetChainConf(ctx, guildID)
	if err != nil {
		return "", err
	}
	return cs.GenerateFiltered(ctx, guildID, maxWords, conf.NGramSize)
}

func (cs *ChainsService) GetRandomMedia(ctx context.Context, guildID, kind string) (string, error) {
	if err := cs.CheckFetchingAndBlock(ctx, guildID); err != nil {
		return "", err
	}
	return cs.redisRepo.GetRandomMedia(ctx, guildID, kind)
}

// IsFetching returns whether the guild is currently fetching messages.
func (cs *ChainsService) IsFetching(ctx context.Context, guildID string) (bool, error) {
	return cs.redisRepo.IsFetching(ctx, guildID)
}

// CheckFetchingAndBlock returns an error if the guild is currently fetching messages.
func (cs *ChainsService) CheckFetchingAndBlock(ctx context.Context, guildID string) error {
	isFetching, err := cs.IsFetching(ctx, guildID)
	if err != nil {
		return fmt.Errorf("failed to check fetching status: %w", err)
	}
	if isFetching {
		return errors.New("guild is currently fetching messages, please try again later")
	}
	return nil
}

// GetChainConf returns the config for a guild, creating it if unknown.
// Reads from the Redis cache first; SQLite is only hit on a true miss.
func (cs *ChainsService) GetChainConf(ctx context.Context, id string) (*repositories.ChainConfig, error) {
	doc, err := cs.chainsRepo.GetChainByID(id)
	if err != nil {
		gid, parseErr := snowflake.Parse(id)
		if parseErr != nil {
			return nil, parseErr
		}
		guild, ok := cs.session.Caches.Guild(gid)
		if !ok {
			return nil, fmt.Errorf("guild %s not found in cache", id)
		}
		return cs.CreateChain(ctx, id, guild.Name)
	}
	return doc, nil
}

func (cs *ChainsService) GetAllChains(_ context.Context) ([]*repositories.ChainConfig, error) {
	return cs.chainsRepo.GetAll()
}

func (cs *ChainsService) GetChainsPage(_ context.Context, limit, offset int) ([]*repositories.ChainConfig, int64, error) {
	return cs.chainsRepo.GetChainsPage(limit, offset)
}

func (cs *ChainsService) CreateChain(_ context.Context, id, name string) (*repositories.ChainConfig, error) {
	logger.Infof("Creating chain: %s", name)
	return cs.chainsRepo.CreateChain(id, name)
}

// UpdateChainState ingests a batch of raw messages into Redis.
// The guild's configured size limit is enforced inside the Lua function.
// RunBulkRedisTraining runs fn while holding a process-wide lock for heavy
// Redis ingestion (full history fetch or chain rebuild). Live UpdateChainState
// calls from message events do not use this lock.
func (cs *ChainsService) RunBulkRedisTraining(fn func()) {
	cs.bulkTrainingMu.Lock()
	defer cs.bulkTrainingMu.Unlock()
	fn()
}

func (cs *ChainsService) UpdateChainState(ctx context.Context, id string, texts []string) error {
	// Block if the guild is currently fetching messages
	if err := cs.CheckFetchingAndBlock(ctx, id); err != nil {
		return err
	}

	chain, err := cs.GetChainConf(ctx, id)
	if err != nil {
		return err
	}
	if err := cs.redisRepo.TrainBatch(ctx, id, texts, chain.NGramSize, chain.MaxSizeBytes(), chain.MarkovMaxBranches); err != nil {
		logger.Errorf("UpdateChainState train error for %s: %v", id, err)
	}
	return nil
}

// DeleteTextData removes a message from both Redis and the SQLite message store.
func (cs *ChainsService) DeleteTextData(ctx context.Context, id, data string) error {
	if err := cs.CheckFetchingAndBlock(ctx, id); err != nil {
		return err
	}

	chain, err := cs.GetChainConf(ctx, id)
	if err != nil {
		return err
	}
	if err := cs.redisRepo.Delete(ctx, id, data, chain.NGramSize); err != nil {
		logger.Errorf("DeleteTextData redis error for %s: %v", id, err)
	}
	return cs.messagesRepo.DeleteGuildMessage(id, data)
}

// UpdateChainMeta applies field-level updates to SQLite and refreshes the
// Redis config cache.  If n_gram_size changes, a rebuild is triggered in the
// background.
func (cs *ChainsService) UpdateChainMeta(ctx context.Context, id string, fields map[string]any) (*repositories.ChainConfig, error) {
	// Skip fetching check if the context indicates to do so
	if skip, ok := ctx.Value(skipFetchingCheckKey).(bool); !ok || !skip {
		if err := cs.CheckFetchingAndBlock(ctx, id); err != nil {
			return nil, err
		}
	}

	if _, ok := fields["id"]; ok {
		return nil, errors.New("cannot change field 'id'")
	}

	oldChain, err := cs.GetChainConf(ctx, id)
	if err != nil {
		return nil, err
	}

	updated, err := cs.chainsRepo.UpdateChain(id, fields)
	if err != nil {
		return nil, err
	}

	// If the n-gram order changed the entire chain must be rebuilt.
	if updated.NGramSize != oldChain.NGramSize {
		go cs.rebuildChain(id, updated.NGramSize)
	}

	if _, touched := fields["markov_max_branches"]; touched && updated.MarkovMaxBranches > 0 &&
		updated.MarkovMaxBranches != oldChain.MarkovMaxBranches {
		go func(guildID string, cap int) {
			n, err := cs.redisRepo.CapBranching(context.Background(), guildID, cap)
			if err != nil {
				logger.Errorf("CapBranching for %s: %v", guildID, err)
				return
			}
			if n > 0 {
				logger.Infof("CapBranching %s: removed %d low-weight transitions (cap=%d)", guildID, n, cap)
			}
		}(id, updated.MarkovMaxBranches)
	}

	return updated, nil
}

// DeleteChain removes all data for a guild: Redis state, SQLite config, and
// stored messages.
func (cs *ChainsService) DeleteChain(ctx context.Context, id string) error {
	if err := cs.CheckFetchingAndBlock(ctx, id); err != nil {
		return err
	}
	doc, err := cs.chainsRepo.GetChainByID(id)
	if err != nil {
		return err
	}
	logger.Warnf("Deleting chain: %s", id)

	if err := cs.redisRepo.ClearGuild(ctx, id); err != nil {
		logger.Errorf("DeleteChain: ClearGuild failed for %s: %v", id, err)
	}
	if err := cs.chainsRepo.DeleteChain(doc.ID); err != nil {
		return err
	}
	if err := cs.messagesRepo.DeleteAllGuildMessages(id); err != nil {
		return err
	}
	logger.Infof("Chain %s deleted", doc.Name)
	return nil
}

// ResetChain clears the Markov state, media, stats, and stored messages for a guild,
// and resets the trained_at timestamp. The chain config is preserved.
func (cs *ChainsService) ResetChain(ctx context.Context, id string) error {
	logger.Warnf("Resetting chain: %s", id)

	// Check if already fetching
	isFetching, err := cs.redisRepo.IsFetching(ctx, id)
	if err != nil {
		logger.Errorf("ResetChain: failed to check fetching flag for %s: %v", id, err)
		return err
	}
	if isFetching {
		return errors.New("guild is already fetching messages")
	}

	// Set fetching flag to indicate the guild is in a fetching state
	if err := cs.redisRepo.SetFetching(ctx, id); err != nil {
		logger.Errorf("ResetChain: SetFetching failed for %s: %v", id, err)
		return err
	}

	success := false
	defer func() {
		if !success {
			// If reset fails, clear the fetching flag
			if err := cs.redisRepo.ClearFetching(ctx, id); err != nil {
				logger.Errorf("ResetChain: ClearFetching failed for %s: %v", id, err)
			}
		}
	}()

	// 1. Clear Redis state (Markov, media, stats)
	if err := cs.redisRepo.ClearGuild(ctx, id); err != nil {
		logger.Errorf("ResetChain: ClearGuild failed for %s: %v", id, err)
		return err
	}

	// 2. Delete stored messages in SQLite
	if err := cs.messagesRepo.DeleteAllGuildMessages(id); err != nil {
		logger.Errorf("ResetChain: DeleteAllGuildMessages failed for %s: %v", id, err)
		return err
	}

	// 3. Update the chain config to set trained_at to nil
	// Use a context that skips the fetching check because we are in the middle of resetting
	skipCtx := context.WithValue(ctx, skipFetchingCheckKey, true)
	if _, err := cs.UpdateChainMeta(skipCtx, id, map[string]any{"trained_at": nil}); err != nil {
		logger.Errorf("ResetChain: UpdateChainMeta failed for %s: %v", id, err)
		return err
	}

	success = true
	// Note: The fetching flag is left set for the DataFetchService to clear after fetching
	logger.Infof("Chain %s reset", id)
	return nil
}

func (cs *ChainsService) GetChainMessages(id string) ([]string, error) {
	messages, err := cs.messagesRepo.GetAllGuildMessages(id)
	if err != nil {
		return nil, err
	}
	texts := make([]string, 0, len(messages))
	for _, m := range messages {
		texts = append(texts, m.Content)
	}
	return texts, nil
}

// ---------- rebuild ----------

// rebuildChain clears and re-trains a guild's chain with a new n-gram size.
// A per-guild mutex prevents duplicate concurrent rebuilds (e.g. if the user
// hammers the config endpoint).
func (cs *ChainsService) rebuildChain(id string, newNGramSize int) {
	// Per-guild mutex — load-or-store a *sync.Mutex for this id.
	mu, _ := cs.rebuildMu.LoadOrStore(id, &sync.Mutex{})
	guildMu := mu.(*sync.Mutex)

	if !guildMu.TryLock() {
		logger.Warnf("rebuildChain: rebuild already in progress for %s, skipping", id)
		return
	}
	defer guildMu.Unlock()

	ctx := context.Background()

	// Check if guild is currently fetching
	isFetching, err := cs.redisRepo.IsFetching(ctx, id)
	if err != nil {
		logger.Errorf("rebuildChain: failed to check fetching flag for %s: %v", id, err)
		return
	}
	if isFetching {
		logger.Warnf("rebuildChain: guild %s is currently fetching, skipping rebuild", id)
		return
	}

	doc, err := cs.chainsRepo.GetChainByID(id)
	if err != nil {
		logger.Errorf("rebuildChain: failed to load chain %s: %v", id, err)
		return
	}
	logger.Infof("Rebuilding chain %s with n_gram_size=%d", doc.Name, newNGramSize)

	cs.RunBulkRedisTraining(func() {
		if err := cs.redisRepo.ClearGuild(ctx, id); err != nil {
			logger.Errorf("rebuildChain: ClearGuild failed for %s: %v", id, err)
			return
		}

		messages, err := cs.GetChainMessages(id)
		if err != nil {
			logger.Errorf("rebuildChain: GetChainMessages failed for %s: %v", id, err)
			return
		}

		if err := cs.redisRepo.TrainBatch(ctx, id, messages, newNGramSize, doc.MaxSizeBytes(), doc.MarkovMaxBranches); err != nil {
			logger.Errorf("rebuildChain: TrainBatch failed for %s: %v", id, err)
			return
		}

		if _, err := cs.redisRepo.ReconcileBytes(ctx, id); err != nil {
			logger.Warnf("rebuildChain: ReconcileBytes failed for %s: %v", id, err)
		}

		logger.Infof("Rebuild complete for %s", doc.Name)
	})
}

// ---------- helpers ----------

func parseToInt(v any) (int, error) {
	switch val := v.(type) {
	case int:
		return val, nil
	case float64:
		return int(val), nil
	case string:
		return strconv.Atoi(val)
	default:
		return 0, fmt.Errorf("unsupported type %T", val)
	}
}
