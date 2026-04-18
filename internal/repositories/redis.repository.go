package repositories

import (
	"context"
	"fmt"
	"regexp"
	"strings"

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

	return r.rdb.FCall(ctx, "train_batch", []string{guildID}, args...).Err()
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
		err := r.rdb.FCall(ctx, "train_batch", []string{guildID}, args...).Err()
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
	_, err := pipe.Exec(ctx)
	return err
}

// ClearGuild wipes all Markov state, media sets, and the byte counter for a guild.
func (r *RedisRepository) ClearGuild(ctx context.Context, guildID string) error {
	return r.rdb.FCall(ctx, "clear_guild", []string{guildID}).Err()
}

// Generate produces text of up to maxLength tokens from a random starting prefix.
func (r *RedisRepository) Generate(ctx context.Context, guildID string, maxLength, nGramSize int) (string, error) {
	prefix, err := r.rdb.FCall(ctx, "find_prefix", []string{guildID}, "").Text()
	if err != nil || prefix == "" {
		return "", err
	}
	return r.generateFrom(ctx, guildID, prefix, maxLength, nGramSize)
}

// GenerateFromSeed produces text seeded by the given string.
func (r *RedisRepository) GenerateFromSeed(ctx context.Context, guildID, seed string, maxLength, nGramSize int) (string, error) {
	prefix, err := r.rdb.FCall(ctx, "find_prefix", []string{guildID}, seed).Text()
	if err != nil || prefix == "" {
		return "", err
	}
	return r.generateFrom(ctx, guildID, prefix, maxLength, nGramSize)
}

func (r *RedisRepository) generateFrom(ctx context.Context, guildID, prefix string, maxLength, nGramSize int) (string, error) {
	return r.rdb.FCall(ctx, "generate_markov", []string{guildID}, prefix, maxLength, nGramSize).Text()
}

// GetStats returns (uniquePrefixes, messageCount, estimatedBytes) for a guild.
func (r *RedisRepository) GetStats(ctx context.Context, guildID string) (uniquePrefixes, messageCount int64, estimatedBytes uint64, err error) {
	res, err := r.rdb.FCall(ctx, "get_stats_markov", []string{guildID}).Int64Slice()
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
		raw, err := r.rdb.FCall(ctx, "reconcile_bytes_batch", []string{guildID}, cursor, batchSize).Slice()
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

	// Commit the accumulated total back to Redis.
	if _, err := r.rdb.FCall(ctx, "reconcile_bytes_batch", []string{guildID}, "COMMIT", 0, total).Slice(); err != nil {
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
		raw, err := r.rdb.FCall(ctx, "cap_branching_batch", []string{guildID}, maxBranches, cursor, batchSize).Slice()
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
	res, err := r.rdb.FCall(ctx, "get_stats_markov", []string{guildID}).Int64Slice()
	if err != nil || len(res) < 3 {
		return 0, err
	}
	return uint64(res[2]), nil
}

// GetMediaCounts returns (gifs, images, videos) counts for a guild.
func (r *RedisRepository) GetMediaCounts(ctx context.Context, guildID string) (gifs, images, videos int64, err error) {
	res, err := r.rdb.FCall(ctx, "get_media_counts", []string{guildID}).Int64Slice()
	// TODO get_media_counts actually returns a 4th value for generic links
	if err != nil || len(res) < 3 {
		return 0, 0, 0, err
	}
	return res[0], res[1], res[2], nil
}

// GetRandomMedia returns a random URL of the given kind ("gif", "image", "video").
func (r *RedisRepository) GetRandomMedia(ctx context.Context, guildID, kind string) (string, error) {
	return r.rdb.FCall(ctx, "get_random_media", []string{guildID}, kind).Text()
}

// RemoveMedia removes a specific URL from a media set.
func (r *RedisRepository) RemoveMedia(ctx context.Context, guildID, kind, url string) error {
	return r.rdb.FCall(ctx, "remove_media", []string{guildID}, kind, url).Err()
}

// AddMedia adds a URL to a media set.
func (r *RedisRepository) AddMedia(ctx context.Context, guildID, url string) error {
	kind := classifyURL(url)
	return r.rdb.FCall(ctx, "add_media", []string{guildID}, kind, url).Err()
}

// GenerateFiltered generates text and strips URLs, pings, and noisy characters.
func (r *RedisRepository) GenerateFiltered(ctx context.Context, guildID string, maxLength, nGramSize int) (string, error) {
	unfiltered, err := r.Generate(ctx, guildID, maxLength, nGramSize)
	if err != nil {
		return "", err
	}
	return FilterText(unfiltered, false), nil
}

// FilterText strips URLs, special chars, and normalises spacing.
// Set pings=true to preserve Discord mention strings.
func FilterText(text string, pings bool) string {
	if !pings {
		text = reDiscordPing.ReplaceAllString(text, "")
	}
	text = utils.ReURL.ReplaceAllString(text, "")
	text = reBadChars.ReplaceAllString(text, "")
	text = strings.TrimSpace(text)
	text = reSpaces.ReplaceAllString(text, " ")
	text = reLongNums.ReplaceAllStringFunc(text, func(m string) string { return m[:5] })
	return text
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
