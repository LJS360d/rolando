package model

import (
	"math/rand"
	"regexp"
	"rolando/internal/repositories"
	"strings"
	"sync"
)

type MarkovChain struct {
	ID             string
	NGramSize      int
	ReplyRate      int
	Pings          bool
	State          map[string]map[string]int
	MessageCounter uint32
	MediaStorage   *MediaStorage
	mu             sync.RWMutex
}

func NewMarkovChain(id string, replyRate int, nGramSize int, pings bool,
	messages []string, messagesRepo repositories.MessagesRepository) *MarkovChain {
	if nGramSize < 2 {
		nGramSize = 2
	}

	mc := &MarkovChain{
		ID:           id,
		NGramSize:    nGramSize,
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

// The UpdateState method is now updated to handle N-grams.
// It creates a prefix of N-1 words and maps it to the N-th word.
func (mc *MarkovChain) UpdateState(message string) {
	if strings.HasPrefix(message, "https://") {
		mc.MediaStorage.AddMedia(message)
		return
	}

	mc.MessageCounter++
	tokens := mc.Tokenize(message)

	// We need at least NGramSize tokens to form a prefix and a next word.
	if len(tokens) < mc.NGramSize {
		return
	}

	for i := 0; i < len(tokens)-mc.NGramSize+1; i++ {
		// The prefix is a slice of N-1 words
		prefixTokens := tokens[i : i+mc.NGramSize-1]
		// The next word is the Nth word
		nextWord := tokens[i+mc.NGramSize-1]

		// Join the prefix tokens to create a single string key for the map
		prefixKey := strings.Join(prefixTokens, " ")

		mc.mu.Lock()
		if mc.State[prefixKey] == nil {
			mc.State[prefixKey] = make(map[string]int)
		}
		mc.State[prefixKey][nextWord]++
		mc.mu.Unlock()
	}
}

// ChangeNGramSize requires the full list of messages to be passed in.
// It is responsible for clearing the old state and rebuilding it with the new size.
func (mc *MarkovChain) ChangeNGramSize(newSize int, messages []string) {
	if newSize < 2 {
		// Do not allow invalid sizes
		return
	}

	mc.NGramSize = newSize

	// Clear the existing state and counter
	mc.State = make(map[string]map[string]int)
	mc.MessageCounter = 0

	mc.ProvideData(messages)
}

func (mc *MarkovChain) GenerateText(startPrefix string, length int) string {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	var generatedTokens []string
	currentPrefixTokens := strings.Fields(startPrefix)
	generatedTokens = append(generatedTokens, currentPrefixTokens...)

	for i := 0; i < length; i++ {
		// Backoff loop: if the current N-gram prefix doesn't exist,
		// we shorten the prefix and try again.
		var nextWords map[string]int
		var exists bool
		backoffPrefixTokens := currentPrefixTokens
		for len(backoffPrefixTokens) > 0 {
			backoffPrefixKey := strings.Join(backoffPrefixTokens, " ")
			nextWords, exists = mc.State[backoffPrefixKey]
			if exists {
				break
			}
			// If not found, back off by removing the first token
			backoffPrefixTokens = backoffPrefixTokens[1:]
		}

		// If no prefix at all is found (even a single word), we stop.
		if !exists || len(nextWords) == 0 {
			break
		}

		// Prepare for stochastic choice
		var nextWordArray []string
		var nextWordWeights []int
		for word, weight := range nextWords {
			nextWordArray = append(nextWordArray, word)
			nextWordWeights = append(nextWordWeights, weight)
		}

		nextWord := mc.StochasticChoice(nextWordArray, nextWordWeights)

		generatedTokens = append(generatedTokens, nextWord)

		// Update the current prefix for the next iteration by
		// taking the last N-1 tokens from the generated sequence.
		if len(generatedTokens) >= mc.NGramSize-1 {
			currentPrefixTokens = generatedTokens[len(generatedTokens)-(mc.NGramSize-1):]
		} else {
			// If we don't have enough tokens for a full prefix yet, use the whole sequence.
			currentPrefixTokens = generatedTokens
		}
	}

	generatedText := strings.Join(generatedTokens, " ")

	// to remove Discord pings
	if !mc.Pings {
		re := regexp.MustCompile(`<\@\S+>`)
		generatedText = re.ReplaceAllString(generatedText, "")
	}
	return generatedText
}

func (mc *MarkovChain) Delete(message string) {
	if strings.HasPrefix(message, "https://") {
		mc.MediaStorage.RemoveMedia(message)
		return
	}

	tokens := mc.Tokenize(message)
	if len(tokens) < mc.NGramSize {
		return
	}

	for i := 0; i < len(tokens)-mc.NGramSize+1; i++ {
		prefixTokens := tokens[i : i+mc.NGramSize-1]
		nextWord := tokens[i+mc.NGramSize-1]
		prefixKey := strings.Join(prefixTokens, " ")

		if nextWordMap, exists := mc.State[prefixKey]; exists {
			if _, exists := nextWordMap[nextWord]; exists {
				delete(nextWordMap, nextWord)
				// Clean up the map if it's empty
				if len(nextWordMap) == 0 {
					delete(mc.State, prefixKey)
				}
			}
		}
	}
}

func (mc *MarkovChain) StochasticChoice(options []string, weights []int) string {
	totalWeight := 0
	for _, weight := range weights {
		totalWeight += weight
	}
	if totalWeight == 0 {
		// should never happen, and if it does i dont care, it would just not generate anything
		return ""
	}
	randomWeight := rand.Intn(totalWeight)
	var weightSum int
	for i, option := range options {
		weightSum += weights[i]
		if randomWeight < weightSum {
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
	if len(keys) == 0 {
		return ""
	}
	randomIndex := rand.Intn(len(keys))
	startingPrefix := keys[randomIndex]

	return mc.GenerateText(startingPrefix, length)
}

func (mc *MarkovChain) TalkOnlyText(length int) string {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	keys := make([]string, 0, len(mc.State))
	for key := range mc.State {
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return ""
	}
	randomIndex := rand.Intn(len(keys))
	startingPrefix := keys[randomIndex]

	gt := mc.GenerateText(startingPrefix, length)

	// Remove URLs
	reURL := regexp.MustCompile(`(?:https?|ftp|file|mailto):\/\/[^\s]+|(?:www\.)[^\s]+`)
	gt = reURL.ReplaceAllString(gt, "")

	// Remove special characters
	reBadChars := regexp.MustCompile(`[\*_~|\[\]\(\)\{\}#\+\-!<>=\\` + "`" + `]`)
	gt = reBadChars.ReplaceAllString(gt, "")

	// Normalize spacing
	gt = strings.TrimSpace(gt)
	gt = regexp.MustCompile(`\s+`).ReplaceAllString(gt, " ")

	// Truncate numbers longer than 5 digits
	reLongNumbers := regexp.MustCompile(`\b\d{6,}\b`)
	gt = reLongNumbers.ReplaceAllStringFunc(gt, func(match string) string {
		return match[:5] // Truncate to 5 digits
	})

	return gt
}
