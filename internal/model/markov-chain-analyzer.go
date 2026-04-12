package model

import (
	"context"
	"fmt"
	"math"
	"rolando/internal/repositories"
)

type ChainAnalytics struct {
	ComplexityScore string `json:"complexity_score"`
	Gifs            string `json:"gifs"`
	Images          string `json:"images"`
	Videos          string `json:"videos"`
	ReplyRate       string `json:"reply_rate"`
	NGramSize       string `json:"n_gram_size"`
	Words           string `json:"words"`
	Messages        string `json:"messages"`
	Size            string `json:"bytes"`
}

type NumericChainAnalytics struct {
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

type MarkovChainAnalyzer struct {
	chain     *repositories.ChainConfig
	redisRepo *repositories.RedisRepository
}

func NewMarkovChainAnalyzer(chain *repositories.ChainConfig, redisRepo *repositories.RedisRepository) *MarkovChainAnalyzer {
	return &MarkovChainAnalyzer{chain: chain, redisRepo: redisRepo}
}

// complexityScore computes a cheap, stable metric:
//
//	log2( prefixes * log2(messages + 2) + 1 )
//
// Grows with vocabulary breadth (prefixes) and depth (messages trained).
// All inputs come from O(1) Redis GET calls.
func complexityScore(prefixes, messages int64) int {
	if prefixes <= 0 {
		return 0
	}
	depth := math.Log2(float64(messages) + 2)
	return int(math.Ceil(math.Log2(float64(prefixes)*depth + 1)))
}

func (mca *MarkovChainAnalyzer) GetAnalytics(ctx context.Context) (ChainAnalytics, error) {
	raw, err := mca.getRaw(ctx)
	if err != nil {
		return ChainAnalytics{}, err
	}
	return ChainAnalytics{
		ComplexityScore: fmt.Sprintf("%d", raw.ComplexityScore),
		Gifs:            fmt.Sprintf("%d", raw.Gifs),
		Images:          fmt.Sprintf("%d", raw.Images),
		Videos:          fmt.Sprintf("%d", raw.Videos),
		ReplyRate:       fmt.Sprintf("%d", raw.ReplyRate),
		NGramSize:       fmt.Sprintf("%d", raw.NGramSize),
		Words:           fmt.Sprintf("%d", raw.Words),
		Messages:        fmt.Sprintf("%d", raw.Messages),
		Size:            fmt.Sprintf("%d", raw.Size),
	}, nil
}

func (mca *MarkovChainAnalyzer) GetRawAnalytics(ctx context.Context) (NumericChainAnalytics, error) {
	return mca.getRaw(ctx)
}

func (mca *MarkovChainAnalyzer) getRaw(ctx context.Context) (NumericChainAnalytics, error) {
	prefixes, messages, size, err := mca.redisRepo.GetStats(ctx, mca.chain.ID)
	if err != nil {
		return NumericChainAnalytics{}, err
	}
	gifs, images, videos, err := mca.redisRepo.GetMediaCounts(ctx, mca.chain.ID)
	if err != nil {
		return NumericChainAnalytics{}, err
	}
	return NumericChainAnalytics{
		ComplexityScore: complexityScore(prefixes, messages),
		Gifs:            gifs,
		Images:          images,
		Videos:          videos,
		ReplyRate:       mca.chain.ReplyRate,
		NGramSize:       mca.chain.NGramSize,
		Words:           prefixes,
		Messages:        messages,
		Size:            size,
	}, nil
}
