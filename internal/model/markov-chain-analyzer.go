package model

import (
	"fmt"
	"math"
	"rolando/internal/utils"
)

type ChainAnalytics struct {
	ComplexityScore string
	Gifs            string
	Images          string
	Videos          string
	ReplyRate       string
	Words           string
	Messages        string
	Size            string
}

type NumericChainAnalytics struct {
	ComplexityScore int
	Gifs            int
	Images          int
	Videos          int
	ReplyRate       int
	Words           int
	Messages        uint32
	Size            uint64
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
		Gifs:            fmt.Sprintf("%d", len(mca.chain.MediaStorage.gifs)),
		Images:          fmt.Sprintf("%d", len(mca.chain.MediaStorage.images)),
		Videos:          fmt.Sprintf("%d", len(mca.chain.MediaStorage.videos)),
		ReplyRate:       fmt.Sprintf("%d", mca.chain.ReplyRate),
		Words:           fmt.Sprintf("%d", len(mca.chain.State)),
		Messages:        fmt.Sprintf("%d", mca.chain.MessageCounter),
		Size:            utils.FormatBytes(uint64(utils.MeasureSize(mca.chain.State))),
	}
}

func (mca *MarkovChainAnalyzer) GetRawAnalytics() NumericChainAnalytics {
	return NumericChainAnalytics{
		ComplexityScore: mca.GetComplexity(),
		Gifs:            len(mca.chain.MediaStorage.gifs),
		Images:          len(mca.chain.MediaStorage.images),
		Videos:          len(mca.chain.MediaStorage.videos),
		ReplyRate:       mca.chain.ReplyRate,
		Words:           len(mca.chain.State),
		Messages:        mca.chain.MessageCounter,
		Size:            uint64(utils.MeasureSize(mca.chain.State)),
	}
}
