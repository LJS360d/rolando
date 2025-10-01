package model

import (
	"fmt"
	"math"
	"rolando/internal/utils"
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
	Gifs            int    `json:"gifs"`
	Images          int    `json:"images"`
	Videos          int    `json:"videos"`
	ReplyRate       int    `json:"reply_rate"`
	NGramSize       int    `json:"n_gram_size"`
	Words           int    `json:"words"`
	Messages        uint32 `json:"messages"`
	Size            uint64 `json:"bytes"`
}

type MarkovChainAnalyzer struct {
	chain *MarkovChain
}

func NewMarkovChainAnalyzer(chain *MarkovChain) *MarkovChainAnalyzer {
	return &MarkovChainAnalyzer{chain: chain}
}

func (mca *MarkovChainAnalyzer) GetComplexity() int {
	stateSize := len(mca.chain.State)
	highValueWords := 0
	for _, nextWords := range mca.chain.State {
		for _, wordValue := range nextWords {
			if wordValue > 15 {
				highValueWords++
			}
		}
	}
	return int(math.Ceil(math.Log2(float64(10*stateSize*highValueWords + 1))))
}

func (mca *MarkovChainAnalyzer) GetAnalytics() ChainAnalytics {
	return ChainAnalytics{
		ComplexityScore: fmt.Sprintf("%d", mca.GetComplexity()),
		Gifs:            fmt.Sprintf("%d", len(mca.chain.MediaStore.Gifs)),
		Images:          fmt.Sprintf("%d", len(mca.chain.MediaStore.Images)),
		Videos:          fmt.Sprintf("%d", len(mca.chain.MediaStore.Videos)),
		ReplyRate:       fmt.Sprintf("%d", mca.chain.ReplyRate),
		NGramSize:       fmt.Sprintf("%d", mca.chain.NGramSize),
		Words:           fmt.Sprintf("%d", len(mca.chain.State)),
		Messages:        fmt.Sprintf("%d", mca.chain.MessageCounter),
		Size:            utils.FormatBytes(uint64(utils.MeasureSize(mca.chain.State))),
	}
}

func (mca *MarkovChainAnalyzer) GetRawAnalytics() NumericChainAnalytics {
	return NumericChainAnalytics{
		ComplexityScore: mca.GetComplexity(),
		Gifs:            len(mca.chain.MediaStore.Gifs),
		Images:          len(mca.chain.MediaStore.Images),
		Videos:          len(mca.chain.MediaStore.Videos),
		ReplyRate:       mca.chain.ReplyRate,
		NGramSize:       mca.chain.NGramSize,
		Words:           len(mca.chain.State),
		Messages:        mca.chain.MessageCounter,
		Size:            uint64(utils.MeasureSize(mca.chain.State)),
	}
}
