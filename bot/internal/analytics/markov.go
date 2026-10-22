package analytics

import (
	"math"

	"rolando/internal/guilds"
)

type ChainAnalytics struct {
	ComplexityScore int    `json:"complexity_score"`
	Gifs            int64  `json:"gifs"`
	Images          int64  `json:"images"`
	Videos          int64  `json:"videos"`
	ReplyRate       int    `json:"reply_rate"`
	NGramSize       int    `json:"n_gram_size"`
	Words           int64  `json:"words"`
	Messages        int64  `json:"messages"`
	Size            uint64 `json:"bytes"`
}

func complexityScore(prefixes, messages int64) int {
	if prefixes <= 0 {
		return 0
	}
	depth := math.Log2(float64(messages) + 2)
	return int(math.Ceil(math.Log2(float64(prefixes)*depth + 1)))
}

func FromSummary(s guilds.Summary) ChainAnalytics {
	return ChainAnalytics{
		ComplexityScore: complexityScore(s.Stats.Prefixes, s.Stats.Messages),
		Gifs:            s.Media.Gifs,
		Images:          s.Media.Images,
		Videos:          s.Media.Videos,
		ReplyRate:       s.Config.ReplyRate,
		NGramSize:       s.Config.NGramSize,
		Words:           s.Stats.Prefixes,
		Messages:        s.Stats.Messages,
		Size:            s.Stats.Bytes,
	}
}
