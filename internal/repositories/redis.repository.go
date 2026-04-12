package repositories

import (
	"context"
	"regexp"
	"strings"

	"github.com/redis/go-redis/v9"
)

var (
	reURL         = regexp.MustCompile(`[a-zA-Z][a-zA-Z0-9+.-]*://[^\s]+`)
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
func (r *RedisRepository) Train(ctx context.Context, guildID, message string, nGramSize, maxSizeBytes int) error {
	if strings.HasPrefix(message, "https://") {
		kind := classifyURL(message)
		return r.rdb.FCall(ctx, "add_media", []string{guildID}, kind, message).Err()
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

	args := make([]any, 0, 2+len(pairs))
	args = append(args, int64(maxSizeBytes), int64(1)) // max_size_bytes, message_count
	for _, p := range pairs {
		args = append(args, p)
	}

	return r.rdb.FCall(ctx, "train_batch", []string{guildID}, args...).Err()
}

// TrainBatch ingests multiple messages using a single FCall per flush window.
// This replaces the old per-n-gram pipeline, cutting round-trips from O(tokens)
// to O(messages/flushEvery).
func (r *RedisRepository) TrainBatch(ctx context.Context, guildID string, messages []string, nGramSize, maxSizeBytes int) error {
	const maxPairsPerCall = 4096 // keeps individual ARGV lists sane

	pairs := make([]string, 0, 512)
	msgCount := 0

	flush := func() error {
		if len(pairs) == 0 {
			return nil
		}
		args := make([]any, 0, 2+len(pairs))
		args = append(args, int64(maxSizeBytes), int64(msgCount))
		for _, p := range pairs {
			args = append(args, p)
		}
		err := r.rdb.FCall(ctx, "train_batch", []string{guildID}, args...).Err()
		pairs = pairs[:0]
		msgCount = 0
		return err
	}

	for _, msg := range messages {
		if strings.HasPrefix(msg, "https://") {
			kind := classifyURL(msg)
			// Media additions are cheap; send immediately via pipeline or
			// accumulate into a separate mini-pipeline if needed.
			if err := r.rdb.FCall(ctx, "add_media", []string{guildID}, kind, msg).Err(); err != nil {
				return err
			}
			continue
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
	if strings.HasPrefix(message, "https://") {
		kind := classifyURL(message)
		return r.rdb.FCall(ctx, "remove_media", []string{guildID}, kind, message).Err()
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

// ReconcileBytes runs the slow MEMORY USAGE walk and resets the estimated-bytes
// counter to the true value. Call from admin tooling or after a migration,
// never on the hot path.
func (r *RedisRepository) ReconcileBytes(ctx context.Context, guildID string) (uint64, error) {
	return r.rdb.FCall(ctx, "reconcile_bytes", []string{guildID}).Uint64()
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
	text = reURL.ReplaceAllString(text, "")
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
	lower := strings.ToLower(url)
	switch {
	case strings.Contains(lower, "tenor.com") ||
		strings.Contains(lower, "giphy.com") ||
		strings.HasSuffix(lower, ".gif"):
		return "gif"
	case strings.HasSuffix(lower, ".mp4") ||
		strings.HasSuffix(lower, ".webm") ||
		strings.HasSuffix(lower, ".mov") ||
		strings.Contains(lower, "youtube.com") ||
		strings.Contains(lower, "youtu.be"):
		return "video"
	default:
		return "image"
	}
}
