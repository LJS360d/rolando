package repositories

import (
	"context"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"rolando/internal/logger"
	"rolando/internal/utils"

	"github.com/valkey-io/valkey-go"
)

var (
	reDiscordPing = regexp.MustCompile(`<@\S+>`)
	reBadChars    = regexp.MustCompile(`[\*_~|\[\]\(\)\{\}#\+\-!<>=\\` + "`" + `]`)
	reLongNums    = regexp.MustCompile(`\b\d{6,}\b`)
	reSpaces      = regexp.MustCompile(`\s+`)
)

type CacheRepository struct {
	rdb valkey.Client
}

func NewCacheRepository(rdb valkey.Client) *CacheRepository {
	return &CacheRepository{rdb: rdb}
}

// Train ingests a single message into the chain for the given guild.
// URLs are classified and stored in media sets instead.
// maxSizeBytes = 0 means unlimited.
func (r *CacheRepository) Train(ctx context.Context, guildID, message string, nGramSize, maxSizeBytes, maxBranches int) error {
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

	args := make([]string, 0, 3+len(pairs))
	args = append(args, strconv.Itoa(maxSizeBytes), strconv.Itoa(maxBranches), "1")
	args = append(args, pairs...)

	return r.runWriteFCall(ctx, guildID, "train_batch", func(c context.Context) error {
		return r.doFCall(c, "train_batch", []string{guildID}, args).Error()
	})
}

// TrainBatch ingests multiple messages using a single FCall per flush window.
// This replaces the old per-n-gram pipeline, cutting round-trips from O(tokens)
// to O(messages/flushEvery).
func (r *CacheRepository) TrainBatch(ctx context.Context, guildID string, messages []string, nGramSize, maxSizeBytes, maxBranches int) error {
	const maxPairsPerCall = 4096 // keeps individual ARGV lists sane

	pairs := make([]string, 0, 512)
	msgCount := 0

	flush := func() error {
		if len(pairs) == 0 {
			return nil
		}
		args := make([]string, 0, 3+len(pairs))
		args = append(args, strconv.Itoa(maxSizeBytes), strconv.Itoa(maxBranches), strconv.Itoa(msgCount))
		args = append(args, pairs...)
		err := r.runWriteFCall(ctx, guildID, "train_batch", func(c context.Context) error {
			return r.doFCall(c, "train_batch", []string{guildID}, args).Error()
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
func (r *CacheRepository) Delete(ctx context.Context, guildID, message string, nGramSize int) error {
	for _, url := range utils.ExtractUrls(message) {
		if err := r.RemoveMedia(ctx, guildID, classifyURL(url), url); err != nil {
			return err
		}
	}

	tokens := tokenize(message)
	if len(tokens) < nGramSize {
		return nil
	}

	last := len(tokens) - nGramSize
	cmds := make([]valkey.Completed, 0, last+1)
	var prefixB strings.Builder
	for i := 0; i <= last; i++ {
		prefixB.Reset()
		writeJoinedTokens(&prefixB, tokens, i, nGramSize-1)
		prefix := prefixB.String()
		next := tokens[i+nGramSize-1]
		cmds = append(cmds, r.buildFCall("delete_markov", []string{guildID}, prefix, next))
	}
	return r.runWriteFCall(ctx, guildID, "delete_markov_pipeline", func(c context.Context) error {
		resps := r.rdb.DoMulti(c, cmds...)
		for _, resp := range resps {
			if err := resp.Error(); err != nil {
				return err
			}
		}
		return nil
	})
}

// ClearGuild wipes all Markov state, media sets, and the byte counter for a guild.
func (r *CacheRepository) ClearGuild(ctx context.Context, guildID string) error {
	return r.runWriteFCall(ctx, guildID, "clear_guild", func(c context.Context) error {
		return r.fcallErr(c, "clear_guild", []string{guildID})
	})
}

// SetFetching sets a flag indicating that the guild is currently fetching messages.
func (r *CacheRepository) SetFetching(ctx context.Context, guildID string) error {
	return r.fcallErr(ctx, "set_fetching", []string{guildID})
}

// ClearFetching removes the fetching flag for the guild.
func (r *CacheRepository) ClearFetching(ctx context.Context, guildID string) error {
	return r.fcallErr(ctx, "clear_fetching", []string{guildID})
}

// IsFetching returns whether the guild is currently fetching messages.
func (r *CacheRepository) IsFetching(ctx context.Context, guildID string) (bool, error) {
	res, err := r.fcallString(ctx, "is_fetching", []string{guildID})
	if err != nil {
		return false, err
	}
	return res == "1", nil
}

// Generate produces text of up to maxLength tokens from a random starting prefix.
func (r *CacheRepository) Generate(ctx context.Context, guildID string, maxLength int) (string, error) {
	var prefix string
	err := r.runWithCacheReadRetry(ctx, guildID, "find_prefix", func(c context.Context) error {
		var e error
		prefix, e = r.fcallString(c, "find_prefix", []string{guildID}, "")
		return e
	})
	if err != nil || prefix == "" {
		return "", err
	}
	return r.generateFrom(ctx, guildID, prefix, maxLength)
}

// GenerateFromSeed produces text seeded by the given string.
func (r *CacheRepository) GenerateFromSeed(ctx context.Context, guildID, seed string, maxLength int) (string, error) {
	var prefix string
	err := r.runWithCacheReadRetry(ctx, guildID, "find_prefix", func(c context.Context) error {
		var e error
		prefix, e = r.fcallString(c, "find_prefix", []string{guildID}, seed)
		return e
	})
	if err != nil || prefix == "" {
		return "", err
	}
	return r.generateFrom(ctx, guildID, prefix, maxLength)
}

func (r *CacheRepository) generateFrom(ctx context.Context, guildID, prefix string, maxLength int) (string, error) {
	var out string
	err := r.runWithCacheReadRetry(ctx, guildID, "generate_markov", func(c context.Context) error {
		var e error
		out, e = r.fcallString(c, "generate_markov", []string{guildID}, prefix, maxLength)
		return e
	})
	return out, err
}

// GenerateRhyme generates text whose last token rhymes with rhymeWord.
// The retry loop runs in Go so each FCALL is one short generation attempt,
// never a long blocking script. Falls back to the last attempt's text if no
// rhyming result is found within maxAttempts calls.
func (r *CacheRepository) GenerateRhyme(ctx context.Context, guildID, rhymeWord string, maxLength int) (string, error) {
	suffix := extractRhymeSuffix(rhymeWord)

	var prefix string
	err := r.runWithCacheReadRetry(ctx, guildID, "find_prefix", func(c context.Context) error {
		var e error
		prefix, e = r.fcallString(c, "find_prefix", []string{guildID}, "")
		return e
	})
	if err != nil || prefix == "" {
		return "", err
	}

	if suffix == "" {
		return r.generateFrom(ctx, guildID, prefix, maxLength)
	}

	const maxAttempts = 10
	var last string
	for range maxAttempts {
		var out string
		callErr := r.runWithCacheReadRetry(ctx, guildID, "generate_rhyme", func(c context.Context) error {
			var e error
			out, e = r.fcallString(c, "generate_rhyme", []string{guildID}, prefix, maxLength, suffix)
			return e
		})
		if callErr != nil {
			break
		}
		if out != "" {
			last = out
		}
		if hasRhymeSuffix(out, suffix) {
			return out, nil
		}
	}
	return last, nil
}

// GenerateRhymeFiltered is the filtered counterpart of GenerateRhyme.
func (r *CacheRepository) GenerateRhymeFiltered(ctx context.Context, guildID, rhymeWord string, maxLength int) (string, error) {
	raw, err := r.GenerateRhyme(ctx, guildID, rhymeWord, maxLength)
	if err != nil {
		return "", err
	}
	return FilterText(raw, false), nil
}

func hasRhymeSuffix(text, suffix string) bool {
	words := strings.Fields(text)
	if len(words) == 0 {
		return false
	}
	return strings.HasSuffix(strings.ToLower(words[len(words)-1]), suffix)
}

func extractRhymeSuffix(word string) string {
	w := strings.ToLower(strings.TrimSpace(word))
	if w == "" {
		return ""
	}
	runes := []rune(w)
	n := min(len(runes), 3)
	return string(runes[len(runes)-n:])
}

// GetStats returns (uniquePrefixes, messageCount, estimatedBytes) for a guild.
func (r *CacheRepository) GetStats(ctx context.Context, guildID string) (uniquePrefixes, messageCount int64, estimatedBytes uint64, err error) {
	var res []int64
	err = r.runWithCacheReadRetry(ctx, guildID, "get_stats_markov", func(c context.Context) error {
		var e error
		res, e = r.fcallInt64Slice(c, "get_stats_markov", []string{guildID})
		return e
	})
	if err != nil || len(res) < 3 {
		return 0, 0, 0, err
	}
	return res[0], res[1], uint64(res[2]), nil
}

// ReconcileBytes drives the paginated reconcile_bytes_batch Lua function in a
// loop so no individual FCall blocks the server for more than ~batchSize key lookups.
// It commits the final total atomically at the end.
func (r *CacheRepository) ReconcileBytes(ctx context.Context, guildID string) (uint64, error) {
	const batchSize = 200

	cursor := "0"
	var total uint64

	for {
		var raw []valkey.ValkeyMessage
		err := r.runWriteFCall(ctx, guildID, "reconcile_bytes_batch", func(c context.Context) error {
			var e error
			raw, e = r.fcallArray(c, "reconcile_bytes_batch", []string{guildID}, cursor, batchSize)
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
		_, e := r.fcallArray(c, "reconcile_bytes_batch", []string{guildID}, "COMMIT", 0, total)
		return e
	}); err != nil {
		return 0, fmt.Errorf("reconcile_bytes_batch COMMIT: %w", err)
	}

	return total, nil
}

// CapBranching drives the paginated cap_branching_batch Lua function in a loop.
// maxBranches <= 0 is a no-op.
func (r *CacheRepository) CapBranching(ctx context.Context, guildID string, maxBranches int) (removed int64, err error) {
	if maxBranches <= 0 {
		return 0, nil
	}

	const batchSize = 200
	cursor := "0"

	for {
		var raw []valkey.ValkeyMessage
		err := r.runWriteFCall(ctx, guildID, "cap_branching_batch", func(c context.Context) error {
			var e error
			raw, e = r.fcallArray(c, "cap_branching_batch", []string{guildID}, maxBranches, cursor, batchSize)
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
func (r *CacheRepository) GetGuildSize(ctx context.Context, guildID string) (uint64, error) {
	var res []int64
	err := r.runWithCacheReadRetry(ctx, guildID, "get_stats_markov", func(c context.Context) error {
		var e error
		res, e = r.fcallInt64Slice(c, "get_stats_markov", []string{guildID})
		return e
	})
	if err != nil || len(res) < 3 {
		return 0, err
	}
	return uint64(res[2]), nil
}

// GetMediaCounts returns (gifs, images, videos) counts for a guild.
func (r *CacheRepository) GetMediaCounts(ctx context.Context, guildID string) (gifs, images, videos int64, err error) {
	var res []int64
	err = r.runWithCacheReadRetry(ctx, guildID, "get_media_counts", func(c context.Context) error {
		var e error
		res, e = r.fcallInt64Slice(c, "get_media_counts", []string{guildID})
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

func (r *CacheRepository) SetJackboxState(ctx context.Context, guildID, appTag string) error {
	return r.runWriteFCall(ctx, guildID, "jackbox_set", func(c context.Context) error {
		return r.rdb.Do(c, r.rdb.B().Set().Key(jackboxGuildKey(guildID)).Value(appTag).Build()).Error()
	})
}

func (r *CacheRepository) ClearJackboxState(ctx context.Context, guildID string) error {
	return r.runWriteFCall(ctx, guildID, "jackbox_del", func(c context.Context) error {
		return r.rdb.Do(c, r.rdb.B().Del().Key(jackboxGuildKey(guildID)).Build()).Error()
	})
}

func (r *CacheRepository) GetJackboxState(ctx context.Context, guildID string) (string, error) {
	var out string
	err := r.runWithCacheReadRetry(ctx, guildID, "jackbox_get", func(c context.Context) error {
		s, e := r.rdb.Do(c, r.rdb.B().Get().Key(jackboxGuildKey(guildID)).Build()).ToString()
		if valkey.IsValkeyNil(e) {
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
func (r *CacheRepository) GetRandomMedia(ctx context.Context, guildID, kind string) (string, error) {
	var out string
	err := r.runWithCacheReadRetry(ctx, guildID, "get_random_media", func(c context.Context) error {
		var e error
		out, e = r.fcallString(c, "get_random_media", []string{guildID}, kind)
		return e
	})
	return out, err
}

// RemoveMedia removes a specific URL from a media set.
func (r *CacheRepository) RemoveMedia(ctx context.Context, guildID, kind, url string) error {
	return r.runWriteFCall(ctx, guildID, "remove_media", func(c context.Context) error {
		return r.fcallErr(c, "remove_media", []string{guildID}, kind, url)
	})
}

// AddMedia adds a URL to a media set.
func (r *CacheRepository) AddMedia(ctx context.Context, guildID, url string) error {
	kind := classifyURL(url)
	return r.runWriteFCall(ctx, guildID, "add_media", func(c context.Context) error {
		return r.fcallErr(c, "add_media", []string{guildID}, kind, url)
	})
}

// GenerateFiltered generates text and strips URLs, pings, and noisy characters.
func (r *CacheRepository) GenerateFiltered(ctx context.Context, guildID string, maxLength int) (string, error) {
	unfiltered, err := r.Generate(ctx, guildID, maxLength)
	if err != nil {
		return "", err
	}
	return FilterText(unfiltered, false), nil
}

func (r *CacheRepository) buildFCall(function string, keys []string, args ...string) valkey.Completed {
	arb := r.rdb.B().Arbitrary("FCALL", function, strconv.Itoa(len(keys)))
	if len(keys) > 0 {
		arb = arb.Keys(keys...)
	}
	if len(args) > 0 {
		arb = arb.Args(args...)
	}
	return arb.Build()
}

func (r *CacheRepository) doFCall(ctx context.Context, function string, keys, argv []string) valkey.ValkeyResult {
	return r.rdb.Do(ctx, r.buildFCall(function, keys, argv...))
}

func (r *CacheRepository) fcallErr(ctx context.Context, function string, keys []string, args ...any) error {
	return r.doFCall(ctx, function, keys, stringifyArgs(args...)).Error()
}

func (r *CacheRepository) fcallString(ctx context.Context, function string, keys []string, args ...any) (string, error) {
	return r.doFCall(ctx, function, keys, stringifyArgs(args...)).ToString()
}

func (r *CacheRepository) fcallArray(ctx context.Context, function string, keys []string, args ...any) ([]valkey.ValkeyMessage, error) {
	return r.doFCall(ctx, function, keys, stringifyArgs(args...)).ToArray()
}

func (r *CacheRepository) fcallInt64Slice(ctx context.Context, function string, keys []string, args ...any) ([]int64, error) {
	arr, err := r.fcallArray(ctx, function, keys, args...)
	if err != nil {
		return nil, err
	}
	out := make([]int64, len(arr))
	for i, msg := range arr {
		v, e := msg.AsInt64()
		if e != nil {
			return nil, e
		}
		out[i] = v
	}
	return out, nil
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

func (r *CacheRepository) runWriteFCall(ctx context.Context, guildID, opName string, fn func(context.Context) error) error {
	start := time.Now()
	err := fn(ctx)
	d := time.Since(start)
	if err != nil || d > 200*time.Millisecond {
		logger.Debugf("cache %s guild=%s dur=%s err=%v", opName, guildID, d, err)
	}
	return err
}

func (r *CacheRepository) runWithCacheReadRetry(ctx context.Context, guildID, opName string, fn func(context.Context) error) error {
	const maxAttempts = 5
	var lastErr error
	for attempt := range maxAttempts {
		start := time.Now()
		lastErr = fn(ctx)
		d := time.Since(start)
		if lastErr != nil || d > 200*time.Millisecond {
			logger.Debugf("cache %s guild=%s dur=%s attempt=%d err=%v", opName, guildID, d, attempt+1, lastErr)
		}
		if lastErr == nil || !isCacheReadRetryable(lastErr) {
			return lastErr
		}
		if err := sleepCtx(ctx, cacheReadBackoff(attempt)); err != nil {
			return err
		}
	}
	return lastErr
}

func isCacheReadRetryable(err error) bool {
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

func cacheReadBackoff(attempt int) time.Duration {
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

func writeJoinedTokens(b *strings.Builder, tokens []string, start, count int) {
	for j := 0; j < count; j++ {
		if j > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(tokens[start+j])
	}
}

// buildPairs converts a token slice into NUL-delimited "prefix\0next_word" strings
// ready to be sent as ARGV to train_batch.
func buildPairs(tokens []string, nGramSize int) []string {
	last := len(tokens) - nGramSize
	if last < 0 {
		return nil
	}
	pairs := make([]string, 0, last+1)
	var b strings.Builder
	for i := 0; i <= last; i++ {
		b.Reset()
		writeJoinedTokens(&b, tokens, i, nGramSize-1)
		pairs = append(pairs, b.String()+"\x00"+tokens[i+nGramSize-1])
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
// Lua returns the cursor as a bulk string and the count as an integer,
// so the slice elements have mixed types — string and int64.
func parseCursorCount(raw []valkey.ValkeyMessage) (cursor string, count int64, err error) {
	if len(raw) < 2 {
		return "", 0, fmt.Errorf("unexpected response len %d", len(raw))
	}
	cursor, err = raw[0].ToString()
	if err != nil {
		return "", 0, fmt.Errorf("cursor: %w", err)
	}
	count, err = raw[1].AsInt64()
	if err != nil {
		return "", 0, fmt.Errorf("count: %w", err)
	}
	return cursor, count, nil
}

func stringifyArgs(args ...any) []string {
	if len(args) == 0 {
		return nil
	}
	out := make([]string, len(args))
	for i, v := range args {
		switch t := v.(type) {
		case string:
			out[i] = t
		case int:
			out[i] = strconv.Itoa(t)
		case int64:
			out[i] = strconv.FormatInt(t, 10)
		case uint:
			out[i] = strconv.FormatUint(uint64(t), 10)
		case uint64:
			out[i] = strconv.FormatUint(t, 10)
		default:
			out[i] = fmt.Sprint(v)
		}
	}
	return out
}
