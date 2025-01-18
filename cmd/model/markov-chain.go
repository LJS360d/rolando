package model

import (
	"math/rand"
	"regexp"
	"rolando/cmd/repositories"
	"strings"
	"sync"
)

type MarkovChain struct {
	ID             string
	ReplyRate      int
	Pings          bool
	State          map[string]map[string]int
	MessageCounter uint32
	MediaStorage   *MediaStorage
	mu             sync.RWMutex
}

func NewMarkovChain(id string, replyRate int, pings bool, messages []string, messagesRepo repositories.MessagesRepository) *MarkovChain {
	mc := &MarkovChain{
		ID:           id,
		ReplyRate:    replyRate,
		Pings:        pings,
		State:        make(map[string]map[string]int),
		MediaStorage: NewMediaStorage(id, nil, nil, nil, messagesRepo),
	}
	mc.ProvideData(messages)
	return mc
}

func (mc *MarkovChain) ProvideData(messages []string) {
	for _, message := range messages {
		mc.UpdateState(message)
	}
}

func (mc *MarkovChain) UpdateState(message string) {
	if strings.HasPrefix(message, "https://") {
		mc.MediaStorage.AddMedia(message)
		return
	}

	mc.MessageCounter++
	tokens := mc.Tokenize(message)
	for i := 0; i < len(tokens)-1; i++ {
		currentWord := tokens[i]
		nextWord := tokens[i+1]

		mc.mu.Lock()
		if mc.State[currentWord] == nil {
			mc.State[currentWord] = make(map[string]int)
		}
		mc.State[currentWord][nextWord]++
		mc.mu.Unlock()
	}
}

func (mc *MarkovChain) GenerateText(startWord string, length int) string {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	currentWord := startWord
	var generatedText strings.Builder
	generatedText.WriteString(currentWord)

	for i := 0; i < length; i++ {
		nextWords, exists := mc.State[currentWord]
		if !exists {
			break
		}

		var nextWordArray []string
		var nextWordWeights []int
		for word, weight := range nextWords {
			nextWordArray = append(nextWordArray, word)
			nextWordWeights = append(nextWordWeights, weight)
		}

		smoothedWeights := make([]float64, len(nextWordWeights))
		for i, weight := range nextWordWeights {
			smoothedWeights[i] = float64(weight+1) / float64(int(mc.MessageCounter)+len(nextWordArray))
		}

		nextWord := mc.StochasticChoice(nextWordArray, smoothedWeights)
		currentWord = nextWord
		generatedText.WriteString(" " + currentWord)
	}

	if !mc.Pings {
		// Replace @mentions, add your regex logic here
		generatedTextStr := generatedText.String()
		return strings.ReplaceAll(generatedTextStr, "<@&?\\w+>", "$1")
	}
	return generatedText.String()
}

func (mc *MarkovChain) Delete(message string) {
	if strings.HasPrefix(message, "https://") {
		mc.MediaStorage.RemoveMedia(message)
		return
	}

	tokens := mc.Tokenize(message)
	for i := 0; i < len(tokens)-1; i++ {
		currentWord := tokens[i]
		nextWord := tokens[i+1]

		if nextWordMap, exists := mc.State[currentWord]; exists {
			if _, exists := nextWordMap[nextWord]; exists {
				delete(nextWordMap, nextWord)
				// Clean up the map if it's empty
				if len(nextWordMap) == 0 {
					delete(mc.State, currentWord)
				}
			}
		}
	}
}

func (mc *MarkovChain) StochasticChoice(options []string, weights []float64) string {
	totalWeight := 0.0
	for _, weight := range weights {
		totalWeight += weight
	}
	randomWeight := rand.Float64() * totalWeight
	var weightSum float64
	for i, option := range options {
		weightSum += weights[i]
		if randomWeight <= weightSum {
			return option
		}
	}
	return options[len(options)-1]
}

func (mc *MarkovChain) Tokenize(text string) []string {
	tokens := strings.Fields(text)
	var filteredTokens []string
	for _, token := range tokens {
		if len(token) > 0 {
			filteredTokens = append(filteredTokens, token)
		}
	}
	return filteredTokens
}

func (mc *MarkovChain) Talk(length int) string {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	keys := make([]string, 0, len(mc.State))
	for key := range mc.State {
		keys = append(keys, key)
	}

	randomIndex := rand.Intn(len(keys))
	startingWord := keys[randomIndex]

	return mc.GenerateText(startingWord, length)
}

func (mc *MarkovChain) TalkOnlyText(length int) string {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	keys := make([]string, 0, len(mc.State))
	for key := range mc.State {
		keys = append(keys, key)
	}

	randomIndex := rand.Intn(len(keys))
	startingWord := keys[randomIndex]

	gt := mc.GenerateText(startingWord, length)

	// Remove URLs
	reURL := regexp.MustCompile(`(?:https?|ftp|file|mailto):\/\/[^\s]+|www\.[^\s]+`)
	gt = reURL.ReplaceAllString(gt, "")

	// Remove special characters
	reBadChars := regexp.MustCompile(`[^a-zA-Z0-9\s.,!?\*=` + "`]")
	gt = reBadChars.ReplaceAllString(gt, "")

	// Normalize spacing
	gt = strings.TrimSpace(gt)
	gt = regexp.MustCompile(`\s+`).ReplaceAllString(gt, " ")

	return gt
}
