// One-shot migration: reads all guild messages from SQLite, trains the Markov
// chains in Redis, warms the config cache for every guild, and reconciles the
// estimated-bytes counter so size-limit enforcement is accurate from the first
// live message after migration.
//
// Safe to re-run after clearing Redis with FCALL clear_guild <guild_id>.
// Re-running without clearing will double-count existing n-grams.
//
/* Usage:
   go run ./cmd/migrate \
     --db      ./data/rolando.db \
     --redis  redis://:change_me@localhost:6379 \
     --workers 8
*/
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"rolando/internal/config"
	"rolando/internal/repositories"

	"github.com/redis/go-redis/v9"
)

func main() {
	dbPath := flag.String("db", config.DatabasePath, "path to SQLite messages database")
	redisURL := flag.String("redis", config.RedisUrl, "Redis URL")
	workers := flag.Int("workers", 8, "number of concurrent workers")
	flag.Parse()

	// --- SQLite ---
	messagesRepo, err := repositories.NewMessagesRepository(*dbPath)
	if err != nil {
		log.Fatalf("open sqlite (messages): %v", err)
	}

	// --- Redis ---
	opt, err := redis.ParseURL(*redisURL)
	if err != nil {
		log.Fatalf("parse redis url: %v", err)
	}
	// Give the pool enough connections so workers don't queue behind each other.
	opt.PoolSize = *workers + 2
	rdb := redis.NewClient(opt)
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("redis ping: %v", err)
	}

	chainsRepo, err := repositories.NewChainsRepository(*dbPath, rdb)
	if err != nil {
		log.Fatalf("open sqlite (chains): %v", err)
	}

	markovRepo := repositories.NewRedisRepository(rdb)

	chains, err := chainsRepo.GetAll()
	if err != nil {
		log.Fatalf("list chains: %v", err)
	}

	logger := log.New(os.Stdout, "", log.LstdFlags)
	logger.Printf("Migrating %d guilds with %d workers...", len(chains), *workers)

	// Counters for the final summary line.
	var (
		nOK      atomic.Int64
		nSkipped atomic.Int64
		nErr     atomic.Int64
	)

	// Feed guilds through a buffered channel so workers can pull at their own pace.
	type job struct {
		index int
		chain *repositories.ChainConfig
	}
	jobs := make(chan job, len(chains))
	for i, c := range chains {
		jobs <- job{i + 1, c}
	}
	close(jobs)

	var wg sync.WaitGroup
	for range *workers {
		wg.Go(func() {
			for j := range jobs {
				chain := j.chain

				// Fast path: if the guild has no messages at all in SQLite,
				// all we need to do is warm the config cache. Skip the Redis
				// train + reconcile round-trips entirely.
				count, err := messagesRepo.CountGuildMessages(chain.ID)
				if err != nil {
					logger.Printf("  [ERR]  [%d] %s: count failed: %v", j.index, chain.Name, err)
					nErr.Add(1)
					continue
				}
				if count == 0 {
					if _, err := chainsRepo.GetChainByID(chain.ID); err != nil {
						logger.Printf("  [WARN] [%d] %s: cache warm failed: %v", j.index, chain.Name, err)
					}
					logger.Printf("  [SKIP] [%d] %-30s  (0 messages)", j.index, chain.Name)
					nSkipped.Add(1)
					continue
				}

				start := time.Now()

				// Train.
				var totalRows int
				trainErr := messagesRepo.ScanGuildMessageContents(chain.ID, 2000, func(texts []string) error {
					totalRows += len(texts)
					return markovRepo.TrainBatch(ctx, chain.ID, texts, chain.NGramSize, 0, chain.MarkovMaxBranches)
				})
				if trainErr != nil {
					logger.Printf("  [ERR]  [%d] %s (%s): train failed: %v", j.index, chain.Name, chain.ID, trainErr)
					nErr.Add(1)
					continue
				}

				// Reconcile bytes (paginated, non-blocking per-call).
				// Only run for guilds that actually have data — saves many
				// round-trips for the long tail of tiny guilds.
				trueBytes, err := markovRepo.ReconcileBytes(ctx, chain.ID)
				if err != nil {
					logger.Printf("  [WARN] [%d] %s (%s): reconcile_bytes failed: %v", j.index, chain.Name, chain.ID, err)
				}

				// Warm the config cache.
				if _, err := chainsRepo.GetChainByID(chain.ID); err != nil {
					logger.Printf("  [WARN] [%d] %s (%s): cache warm failed: %v", j.index, chain.Name, chain.ID, err)
				}

				elapsed := time.Since(start).Round(time.Millisecond)
				logger.Printf("  [OK]   [%d] %-30s  %6d messages  n_gram=%d  ~%s  %s",
					j.index, chain.Name, totalRows, chain.NGramSize,
					formatBytes(trueBytes), elapsed)
				nOK.Add(1)
			}
		})
	}

	wg.Wait()

	fmt.Printf("\nMigration complete. ok=%d  skipped=%d  errors=%d\n",
		nOK.Load(), nSkipped.Load(), nErr.Load())
}

func formatBytes(b uint64) string {
	switch {
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(b)/(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(b)/(1<<10))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
