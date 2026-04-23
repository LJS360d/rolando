package repositories

import (
	"context"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"rolando/internal/logger"
	"rolando/internal/utils"

	"github.com/redis/go-redis/v9"
)

var (
	reDiscordPing = regexp.MustCompile(`<@\S+>`)
	reBadChars    = regexp.MustCompile(`[\*_~|\[\]\(\)\{\}#\+\-!<>=\\` + "`" + `]`)
	reLongNums    = regexp.MustCompile(`\b\d{6,}\b`)
	reSpaces      = regexp.MustCompile(`\s+`)
)

type RedisRepository struct {
	rdb *redis.Client

	// scriptWriteMu serializes FCALL-based writes in this process. Redis already
	// runs commands one at a time on its event loop; this only bounds how many
	// long-running train_batch (etc.) calls we overlap from concurrent goroutines.
	scriptWriteMu sync.Mutex
}

func NewRedisRepository(rdb *redis.Client) *RedisRepository {
	return &RedisRepository{rdb: rdb}
}

// Train ingests a single message into the chain for the given guild.
// URLs are classified and stored in media sets instead.
// maxSizeBytes = 0 means unlimited.
func (r *RedisRepository) Train(ctx context.Context, guildID, message string, nGramSize, maxSizeBytes, maxBranches int) error {
	for _, url := range utils.ExtractUrls(message) {
		if err := r.AddMedia(ctx, guildID, url); err != nil {
			return err
		}
	}

	tokens := tokenize(message)
	if len(tokens) < nGramSize {
		return nil
	}

	// Build the flat pair list for train_batch (one message = trivially a batch of 1).
	pairs := buildPairs(tokens, nGramSize)
	if len(pairs) == 0 {
		return nil
	}

	args := make([]any, 0, 3+len(pairs))
	args = append(args, int64(maxSizeBytes), int64(maxBranches), int64(1)) // max_size, max_branching, message_count
	for _, p := range pairs {
		args = append(args, p)
	}

	return r.runWriteFCall(ctx, guildID, "train_batch", func(c context.Context) error {
		return r.rdb.FCall(c, "train_batch", []string{guildID}, args...).Err()
	})
}

// TrainBatch ingests multiple messages using a single FCall per flush window.
// This replaces the old per-n-gram pipeline, cutting round-trips from O(tokens)
// to O(messages/flushEvery).
func (r *RedisRepository) TrainBatch(ctx context.Context, guildID string, messages []string, nGramSize, maxSizeBytes, maxBranches int) error {
	const maxPairsPerCall = 4096 // keeps individual ARGV lists sane

	pairs := make([]string, 0, 512)
	msgCount := 0

	flush := func() error {
		if len(pairs) == 0 {
			return nil
		}
		args := make([]any, 0, 3+len(pairs))
		args = append(args, int64(maxSizeBytes), int64(maxBranches), int64(msgCount))
		for _, p := range pairs {
			args = append(args, p)
		}
		err := r.runWriteFCall(ctx, guildID, "train_batch", func(c context.Context) error {
			return r.rdb.FCall(c, "train_batch", []string{guildID}, args...).Err()
		})
		pairs = pairs[:0]
		msgCount = 0
		return err
	}

	for _, msg := range messages {
		for _, url := range utils.ExtractUrls(msg) {
			if err := r.AddMedia(ctx, guildID, url); err != nil {
				return err
			}
		}

		tokens := tokenize(msg)
		if len(tokens) < nGramSize {
			continue
		}

		msgPairs := buildPairs(tokens, nGramSize)
		pairs = append(pairs, msgPairs...)
		msgCount++

		if len(pairs) >= maxPairsPerCall {
			if err := flush(); err != nil {
				return err
			}
		}
	}

	return flush()
}

// Delete removes a message's contribution from the chain.
func (r *RedisRepository) Delete(ctx context.Context, guildID, message string, nGramSize int) error {
	for _, url := range utils.ExtractUrls(message) {
		if err := r.RemoveMedia(ctx, guildID, classifyURL(url), url); err != nil {
			return err
		}
	}

	tokens := tokenize(message)
	if len(tokens) < nGramSize {
		return nil
	}

	pipe := r.rdb.Pipeline()
	for i := 0; i <= len(tokens)-nGramSize; i++ {
		prefix := strings.Join(tokens[i:i+nGramSize-1], " ")
		next := tokens[i+nGramSize-1]
		pipe.FCall(ctx, "delete_markov", []string{guildID}, prefix, next)
	}
	err := r.runWriteFCall(ctx, guildID, "delete_markov_pipeline", func(c context.Context) error {
		_, e := pipe.Exec(c)
		return e
	})
	return err
}

// ClearGuild wipes all Markov state, media sets, and the byte counter for a guild.
func (r *RedisRepository) ClearGuild(ctx context.Context, guildID string) error {
	return r.runWriteFCall(ctx, guildID, "clear_guild", func(c context.Context) error {
		return r.rdb.FCall(c, "clear_guild", []string{guildID}).Err()
	})
}

// SetFetching sets a flag indicating that the guild is currently fetching messages.
func (r *RedisRepository) SetFetching(ctx context.Context, guildID string) error {
	return r.runWriteFCall(ctx, guildID, "set_fetching", func(c context.Context) error {
		return r.rdb.FCall(c, "set_fetching", []string{guildID}).Err()
	})
}

// ClearFetching removes the fetching flag for the guild.
func (r *RedisRepository) ClearFetching(ctx context.Context, guildID string) error {
	return r.runWriteFCall(ctx, guildID, "clear_fetching", func(c context.Context) error {
		return r.rdb.FCall(c, "clear_fetching", []string{guildID}).Err()
	})
}

// IsFetching returns whether the guild is currently fetching messages.
func (r *RedisRepository) IsFetching(ctx context.Context, guildID string) (bool, error) {
	res, err := r.rdb.FCall(ctx, "is_fetching", []string{guildID}).Text()
	if err != nil {
		return false, err
	}
	return res == "1", nil
}

// Generate produces text of up to maxLength tokens from a random starting prefix.
func (r *RedisRepository) Generate(ctx context.Context, guildID string, maxLength int) (string, error) {
	return r.generateWithSeed(ctx, guildID, "", maxLength)
}

// GenerateFromSeed produces text seeded by the given string.
func (r *RedisRepository) GenerateFromSeed(ctx context.Context, guildID, seed string, maxLength int) (string, error) {
	return r.generateWithSeed(ctx, guildID, seed, maxLength)
}

func (r *RedisRepository) generateWithSeed(ctx context.Context, guildID, seed string, maxLength int) (string, error) {
	prefix, err := r.findPrefix(ctx, guildID, seed)
	if err != nil || prefix == "" {
		return "", err
	}
	return r.generateFrom(ctx, guildID, prefix, maxLength)
}

func (r *RedisRepository) findPrefix(ctx context.Context, guildID, seed string) (string, error) {
	var prefix string
	err := r.runWithRedisReadRetry(ctx, guildID, "find_prefix", func(c context.Context) error {
		var e error
		prefix, e = r.rdb.FCall(c, "find_prefix", []string{guildID}, seed).Text()
		return e
	})
	return prefix, err
}

func (r *RedisRepository) runGenerateMarkov(ctx context.Context, guildID, prefix string, maxLength, nGramSize int) (string, error) {
	var out string
	err := r.runWithRedisReadRetry(ctx, guildID, "generate_markov", func(c context.Context) error {
		var e error
		out, e = r.rdb.FCall(c, "generate_markov", []string{guildID}, prefix, maxLength, nGramSize).Text()
		return e
	})
	return out, err
}

func (r *RedisRepository) generateFrom(ctx context.Context, guildID, prefix string, maxLength int) (string, error) {
	preserve, nGram, err := r.guildMarkovTextSettings(ctx, guildID)
	if err != nil {
		return "", err
	}
	raw, err := r.runGenerateMarkov(ctx, guildID, prefix, maxLength, nGram)
	if err != nil {
		return "", err
	}
	return filterDiscordMentions(raw, preserve), nil
}

func (r *RedisRepository) guildMarkovTextSettings(ctx context.Context, guildID string) (preservePings bool, nGramSize int, err error) {
	vals, err := r.rdb.HMGet(ctx, chainConfigRedisKey(guildID), "pings", "n_gram_size").Result()
	if err != nil {
		return false, 0, err
	}
	return parseGuildMarkovTextSettings(vals)
}

func parseGuildMarkovTextSettings(vals []any) (preservePings bool, nGramSize int, err error) {
	preservePings = true
	nGramSize = 2
	if len(vals) < 2 {
		return preservePings, nGramSize, nil
	}
	if vals[0] != nil {
		if s, _ := vals[0].(string); s == "0" {
			preservePings = false
		}
	}
	if vals[1] != nil {
		s, ok := vals[1].(string)
		if ok && s != "" {
			n, e := strconv.Atoi(s)
			if e != nil {
				return false, 0, fmt.Errorf("n_gram_size: %w", e)
			}
			if n > 0 {
				nGramSize = n
			}
		}
	}
	return preservePings, nGramSize, nil
}

func chainConfigRedisKey(guildID string) string {
	return "config:" + guildID
}

// GetStats returns (uniquePrefixes, messageCount, estimatedBytes) for a guild.
func (r *RedisRepository) GetStats(ctx context.Context, guildID string) (uniquePrefixes, messageCount int64, estimatedBytes uint64, err error) {
	var res []int64
	err = r.runWithRedisReadRetry(ctx, guildID, "get_stats_markov", func(c context.Context) error {
		var e error
		res, e = r.rdb.FCall(c, "get_stats_markov", []string{guildID}).Int64Slice()
		return e
	})
	if err != nil || len(res) < 3 {
		return 0, 0, 0, err
	}
	return res[0], res[1], uint64(res[2]), nil
}

// ReconcileBytes drives the paginated reconcile_bytes_batch Lua function in a
// loop so no individual FCall blocks Redis for more than ~batchSize key lookups.
// It commits the final total back to Redis atomically at the end.
func (r *RedisRepository) ReconcileBytes(ctx context.Context, guildID string) (uint64, error) {
	const batchSize = 200

	cursor := "0"
	var total uint64

	for {
		var raw []any
		err := r.runWriteFCall(ctx, guildID, "reconcile_bytes_batch", func(c context.Context) error {
			var e error
			raw, e = r.rdb.FCall(c, "reconcile_bytes_batch", []string{guildID}, cursor, batchSize).Slice()
			return e
		})
		if err != nil {
			return 0, fmt.Errorf("reconcile_bytes_batch: %w", err)
		}
		nextCursor, partial, err := parseCursorCount(raw)
		if err != nil {
			return 0, fmt.Errorf("reconcile_bytes_batch: %w", err)
		}
		total += uint64(partial)
		cursor = nextCursor
		if cursor == "0" {
			break
		}
	}

	if err := r.runWriteFCall(ctx, guildID, "reconcile_bytes_batch_commit", func(c context.Context) error {
		_, e := r.rdb.FCall(c, "reconcile_bytes_batch", []string{guildID}, "COMMIT", 0, total).Slice()
		return e
	}); err != nil {
		return 0, fmt.Errorf("reconcile_bytes_batch COMMIT: %w", err)
	}

	return total, nil
}

// CapBranching drives the paginated cap_branching_batch Lua function in a loop.
// maxBranches <= 0 is a no-op.
func (r *RedisRepository) CapBranching(ctx context.Context, guildID string, maxBranches int) (removed int64, err error) {
	if maxBranches <= 0 {
		return 0, nil
	}

	const batchSize = 200
	cursor := "0"

	for {
		var raw []any
		err := r.runWriteFCall(ctx, guildID, "cap_branching_batch", func(c context.Context) error {
			var e error
			raw, e = r.rdb.FCall(c, "cap_branching_batch", []string{guildID}, maxBranches, cursor, batchSize).Slice()
			return e
		})
		if err != nil {
			return removed, fmt.Errorf("cap_branching_batch: %w", err)
		}
		nextCursor, n, err := parseCursorCount(raw)
		if err != nil {
			return removed, fmt.Errorf("cap_branching_batch: %w", err)
		}
		removed += n
		cursor = nextCursor
		if cursor == "0" {
			break
		}
	}

	return removed, nil
}

// GetGuildSize returns the current estimated byte count (cheap counter read).
// For an exact figure use ReconcileBytes.
func (r *RedisRepository) GetGuildSize(ctx context.Context, guildID string) (uint64, error) {
	var res []int64
	err := r.runWithRedisReadRetry(ctx, guildID, "get_stats_markov", func(c context.Context) error {
		var e error
		res, e = r.rdb.FCall(c, "get_stats_markov", []string{guildID}).Int64Slice()
		return e
	})
	if err != nil || len(res) < 3 {
		return 0, err
	}
	return uint64(res[2]), nil
}

// GetMediaCounts returns (gifs, images, videos) counts for a guild.
func (r *RedisRepository) GetMediaCounts(ctx context.Context, guildID string) (gifs, images, videos int64, err error) {
	var res []int64
	err = r.runWithRedisReadRetry(ctx, guildID, "get_media_counts", func(c context.Context) error {
		var e error
		res, e = r.rdb.FCall(c, "get_media_counts", []string{guildID}).Int64Slice()
		return e
	})
	if err != nil || len(res) < 3 {
		return 0, 0, 0, err
	}
	return res[0], res[1], res[2], nil
}

func jackboxGuildKey(guildID string) string {
	return "guild:" + guildID + ":jackbox"
}

func (r *RedisRepository) SetJackboxState(ctx context.Context, guildID, appTag string) error {
	return r.runWriteFCall(ctx, guildID, "jackbox_set", func(c context.Context) error {
		return r.rdb.Set(c, jackboxGuildKey(guildID), appTag, 0).Err()
	})
}

func (r *RedisRepository) ClearJackboxState(ctx context.Context, guildID string) error {
	return r.runWriteFCall(ctx, guildID, "jackbox_del", func(c context.Context) error {
		return r.rdb.Del(c, jackboxGuildKey(guildID)).Err()
	})
}

func (r *RedisRepository) GetJackboxState(ctx context.Context, guildID string) (string, error) {
	var out string
	err := r.runWithRedisReadRetry(ctx, guildID, "jackbox_get", func(c context.Context) error {
		s, e := r.rdb.Get(c, jackboxGuildKey(guildID)).Result()
		if e == redis.Nil {
			out = ""
			return nil
		}
		if e != nil {
			return e
		}
		out = s
		return nil
	})
	return out, err
}

// GetRandomMedia returns a random URL of the given kind ("gif", "image", "video").
func (r *RedisRepository) GetRandomMedia(ctx context.Context, guildID, kind string) (string, error) {
	var out string
	err := r.runWithRedisReadRetry(ctx, guildID, "get_random_media", func(c context.Context) error {
		var e error
		out, e = r.rdb.FCall(c, "get_random_media", []string{guildID}, kind).Text()
		return e
	})
	return out, err
}

// RemoveMedia removes a specific URL from a media set.
func (r *RedisRepository) RemoveMedia(ctx context.Context, guildID, kind, url string) error {
	return r.runWriteFCall(ctx, guildID, "remove_media", func(c context.Context) error {
		return r.rdb.FCall(c, "remove_media", []string{guildID}, kind, url).Err()
	})
}

// AddMedia adds a URL to a media set.
func (r *RedisRepository) AddMedia(ctx context.Context, guildID, url string) error {
	kind := classifyURL(url)
	return r.runWriteFCall(ctx, guildID, "add_media", func(c context.Context) error {
		return r.rdb.FCall(c, "add_media", []string{guildID}, kind, url).Err()
	})
}

// GenerateFiltered generates text and strips URLs, pings, and noisy characters.
func (r *RedisRepository) GenerateFiltered(ctx context.Context, guildID string, maxLength int) (string, error) {
	preserve, nGram, err := r.guildMarkovTextSettings(ctx, guildID)
	if err != nil {
		return "", err
	}
	prefix, err := r.findPrefix(ctx, guildID, "")
	if err != nil || prefix == "" {
		return "", err
	}
	raw, err := r.runGenerateMarkov(ctx, guildID, prefix, maxLength, nGram)
	if err != nil {
		return "", err
	}
	return FilterText(raw, preserve), nil
}

func filterDiscordMentions(text string, preservePings bool) string {
	if preservePings {
		return text
	}
	return reDiscordPing.ReplaceAllString(text, "")
}

// FilterText strips URLs, special chars, and normalises spacing.
// Set pings=true to preserve Discord mention strings.
func FilterText(text string, pings bool) string {
	text = filterDiscordMentions(text, pings)
	text = utils.ReURL.ReplaceAllString(text, "")
	text = reBadChars.ReplaceAllString(text, "")
	text = strings.TrimSpace(text)
	text = reSpaces.ReplaceAllString(text, " ")
	text = reLongNums.ReplaceAllStringFunc(text, func(m string) string { return m[:5] })
	return text
}

func (r *RedisRepository) runWriteFCall(ctx context.Context, guildID, opName string, fn func(context.Context) error) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	r.scriptWriteMu.Lock()
	defer r.scriptWriteMu.Unlock()
	start := time.Now()
	err := fn(ctx)
	d := time.Since(start)
	if err != nil || d > 200*time.Millisecond {
		logger.Debugf("redis %s guild=%s dur=%s err=%v", opName, guildID, d, err)
	}
	return err
}

func (r *RedisRepository) runWithRedisReadRetry(ctx context.Context, guildID, opName string, fn func(context.Context) error) error {
	const maxAttempts = 5
	var lastErr error
	for attempt := range maxAttempts {
		start := time.Now()
		lastErr = fn(ctx)
		d := time.Since(start)
		if lastErr != nil || d > 200*time.Millisecond {
			logger.Debugf("redis %s guild=%s dur=%s attempt=%d err=%v", opName, guildID, d, attempt+1, lastErr)
		}
		if lastErr == nil || !isRedisReadRetryable(lastErr) {
			return lastErr
		}
		if err := sleepCtx(ctx, redisReadBackoff(attempt)); err != nil {
			return err
		}
	}
	return lastErr
}

func isRedisReadRetryable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return true
	}
	s := err.Error()
	return strings.Contains(s, "BUSY") || strings.Contains(s, "LOADING") || strings.Contains(s, "i/o timeout")
}

func redisReadBackoff(attempt int) time.Duration {
	d := (25 * time.Millisecond) << attempt
	if d > 750*time.Millisecond {
		return 750 * time.Millisecond
	}
	return d
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// ---------- helpers ----------

// buildPairs converts a token slice into NUL-delimited "prefix\0next_word" strings
// ready to be sent as ARGV to train_batch.
func buildPairs(tokens []string, nGramSize int) []string {
	pairs := make([]string, 0, len(tokens)-nGramSize+1)
	for i := 0; i <= len(tokens)-nGramSize; i++ {
		prefix := strings.Join(tokens[i:i+nGramSize-1], " ")
		next := tokens[i+nGramSize-1]
		pairs = append(pairs, prefix+"\x00"+next)
	}
	return pairs
}

func tokenize(text string) []string {
	fields := strings.Fields(text)
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if len(f) > 0 {
			out = append(out, f)
		}
	}
	return out
}

func classifyURL(url string) string {
	if utils.IsGif(url) {
		return "gif"
	}
	if utils.IsImage(url) {
		return "image"
	}
	if utils.IsVideo(url) {
		return "video"
	}
	return "generic"
}

// parseCursorCount decodes the {cursor_string, integer_count} pair that
// reconcile_bytes_batch and cap_branching_batch return.
// Redis Lua returns the cursor as a bulk string and the count as a native
// integer, so the slice elements have mixed types — string and int64.
func parseCursorCount(raw []any) (cursor string, count int64, err error) {
	if len(raw) < 2 {
		return "", 0, fmt.Errorf("unexpected response len %d", len(raw))
	}
	switch v := raw[0].(type) {
	case string:
		cursor = v
	case []byte:
		cursor = string(v)
	default:
		return "", 0, fmt.Errorf("cursor: unexpected type %T", raw[0])
	}
	switch v := raw[1].(type) {
	case int64:
		count = v
	case string:
		if _, e := fmt.Sscanf(v, "%d", &count); e != nil {
			return "", 0, fmt.Errorf("count: %w", e)
		}
	default:
		return "", 0, fmt.Errorf("count: unexpected type %T", raw[1])
	}
	return cursor, count, nil
}
