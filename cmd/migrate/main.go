// One-shot migration: reads all guild messages from SQLite, trains the Markov
// chains in the cache service, warms the config cache for every guild, and reconciles the
// estimated-bytes counter so size-limit enforcement is accurate from the first
// live message after migration.
//
// Safe to re-run after clearing the cache data with FCALL clear_guild <guild_id>.
// Re-running without clearing will double-count existing n-grams.
//
/* Usage:
   go run ./cmd/migrate \
     --db      ./data/rolando.db \
     --cache  valkey://:change_me@localhost:6379 \
     --workers 16 \
     --clear
*/
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"rolando/internal/config"
	"rolando/internal/repositories"

	"github.com/valkey-io/valkey-go"
)

func main() {
	dbPath := flag.String("db", config.DatabasePath, "path to SQLite messages database")
	cacheURL := flag.String("cache", config.CacheURL, "cache service URL")
	workers := flag.Int("workers", 8, "number of concurrent workers")
	clearCache := flag.Bool("clear", true, "clear each guild's cache data before training")
	flag.Parse()

	// --- SQLite ---
	messagesRepo, err := repositories.NewMessagesRepository(*dbPath)
	if err != nil {
		log.Fatalf("open sqlite (messages): %v", err)
	}

	// --- Cache service ---
	opt, err := valkey.ParseURL(*cacheURL)
	if err != nil {
		log.Fatalf("parse cache url: %v", err)
	}
	config.ApplyValkeyClientTuning(&opt)
	rdb, err := valkey.NewClient(opt)
	if err != nil {
		log.Fatalf("create cache client: %v", err)
	}
	ctx := context.Background()
	if err := rdb.Do(ctx, rdb.B().Ping().Build()).Error(); err != nil {
		log.Fatalf("cache ping: %v", err)
	}

	chainsRepo, err := repositories.NewChainsRepository(*dbPath, rdb)
	if err != nil {
		log.Fatalf("open sqlite (chains): %v", err)
	}

	chains, err := chainsRepo.GetAll()
	if err != nil {
		log.Fatalf("list chains: %v", err)
	}

	// Pre-load message counts for all guilds in a single query to avoid N round-trips.
	type guildCount struct {
		GuildID string
		Count   int64
	}
	var counts []guildCount
	if err := messagesRepo.DB.Raw("SELECT guild_id, COUNT(*) as count FROM messages GROUP BY guild_id").Scan(&counts).Error; err != nil {
		log.Fatalf("count messages: %v", err)
	}
	countMap := make(map[string]int64, len(counts))
	for _, c := range counts {
		countMap[c.GuildID] = c.Count
	}

	// Pre-warm config cache for all guilds using a pipeline to avoid N round-trips.
	cmds := make([]valkey.Completed, 0, len(chains))
	for _, c := range chains {
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
		hargs := []string{
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
		cmds = append(cmds, rdb.B().Arbitrary("HSET").Keys("config:"+c.ID).Args(hargs...).Build())
	}
	resps := rdb.DoMulti(ctx, cmds...)
	for _, resp := range resps {
		if err := resp.Error(); err != nil {
			log.Printf("WARN: cache warm pipeline failed: %v", err)
			break
		}
	}

	markovRepo := repositories.NewCacheRepository(rdb)

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
				count := countMap[chain.ID]

				// Fast path: if the guild has no messages at all in SQLite,
				// skip train + reconcile round-trips entirely.
				if count == 0 {
					logger.Printf("  [SKIP] [%d] %-30s  (0 messages)", j.index, chain.Name)
					nSkipped.Add(1)
					continue
				}

				start := time.Now()

				// Clear guild data if requested (per-guild, not global).
				if *clearCache {
					if err := markovRepo.ClearGuild(ctx, chain.ID); err != nil {
						logger.Printf("  [WARN] [%d] %s (%s): clear failed: %v", j.index, chain.Name, chain.ID, err)
					}
				}

				// Train.
				var totalRows int
				trainErr := messagesRepo.ScanGuildMessageContents(chain.ID, 5000, func(texts []string) error {
					totalRows += len(texts)
					return markovRepo.TrainBatch(ctx, chain.ID, texts, chain.NGramSize, chain.MaxSizeBytes(), chain.MarkovMaxBranches)
				})
				if trainErr != nil {
					logger.Printf("  [ERR]  [%d] %s (%s): train failed: %v", j.index, chain.Name, chain.ID, trainErr)
					nErr.Add(1)
					continue
				}

				var trueBytes uint64
				// Reconcile bytes only when NOT doing a fresh train (train_batch already tracks bytes accurately).
				if !*clearCache {
					var err error
					trueBytes, err = markovRepo.ReconcileBytes(ctx, chain.ID)
					if err != nil {
						logger.Printf("  [WARN] [%d] %s (%s): reconcile_bytes failed: %v", j.index, chain.Name, chain.ID, err)
					}
				} else {
					// When clearing, we can just read the counter since train_batch updated it accurately.
					trueBytes, _ = markovRepo.GetGuildSize(ctx, chain.ID)
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
