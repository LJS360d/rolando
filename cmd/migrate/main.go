// cmd/migrate/main.go
//
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
     --db    ./data/rolando.db \
     --redis redis://localhost:6379
*/
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"rolando/internal/config"
	"rolando/internal/repositories"

	"github.com/redis/go-redis/v9"
)

func main() {
	dbPath := flag.String("db", config.DatabasePath, "path to SQLite messages database")
	redisAddr := flag.String("redis", config.RedisUrl, "Redis URL")
	flag.Parse()

	// --- SQLite ---
	messagesRepo, err := repositories.NewMessagesRepository(*dbPath)
	if err != nil {
		log.Fatalf("open sqlite (messages): %v", err)
	}

	// --- Redis ---
	opt, err := redis.ParseURL(*redisAddr)
	if err != nil {
		log.Fatalf("parse redis url: %v", err)
	}
	rdb := redis.NewClient(opt)
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("redis ping: %v", err)
	}

	// ChainsRepository now requires the Redis client for cache warming.
	chainsRepo, err := repositories.NewChainsRepository(*dbPath, rdb)
	if err != nil {
		log.Fatalf("open sqlite (chains): %v", err)
	}

	markovRepo := repositories.NewRedisRepository(rdb)

	// --- Load all guilds ---
	chains, err := chainsRepo.GetAll()
	if err != nil {
		log.Fatalf("list chains: %v", err)
	}

	logger := log.New(os.Stdout, "", log.LstdFlags)
	logger.Printf("Migrating %d guilds...", len(chains))

	for _, chain := range chains {
		start := time.Now()

		// 1. Load messages from SQLite.
		messages, err := messagesRepo.GetAllGuildMessages(chain.ID)
		if err != nil {
			logger.Printf("  [SKIP] %s (%s): load messages failed: %v", chain.Name, chain.ID, err)
			continue
		}

		texts := make([]string, 0, len(messages))
		for _, m := range messages {
			texts = append(texts, m.Content)
		}

		// 2. Train the chain (size limit intentionally bypassed during migration
		//    so we faithfully restore existing data; the limit will be enforced
		//    from the first live write after migration).
		if err := markovRepo.TrainBatch(ctx, chain.ID, texts, chain.NGramSize, 0); err != nil {
			logger.Printf("  [ERR]  %s (%s): train failed: %v", chain.Name, chain.ID, err)
			continue
		}

		// 3. Reconcile the byte counter against the true MEMORY USAGE values so
		//    the size-limit check is accurate from day one post-migration.
		trueBytes, err := markovRepo.ReconcileBytes(ctx, chain.ID)
		if err != nil {
			logger.Printf("  [WARN] %s (%s): reconcile_bytes failed: %v", chain.Name, chain.ID, err)
		}

		// 4. Warm the Redis config cache for this guild.
		//    GetChainByID writes through to the cache internally.
		if _, err := chainsRepo.GetChainByID(chain.ID); err != nil {
			logger.Printf("  [WARN] %s (%s): cache warm failed: %v", chain.Name, chain.ID, err)
		}

		elapsed := time.Since(start).Round(time.Millisecond)
		logger.Printf("  [OK]   %-30s  %6d messages  n_gram=%d  ~%s  %s",
			chain.Name, len(texts), chain.NGramSize,
			formatBytes(trueBytes), elapsed)
	}

	fmt.Println("Migration complete.")
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
