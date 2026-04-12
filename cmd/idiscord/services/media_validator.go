package services

import (
	"context"
	"net/http"
	"rolando/internal/logger"
	"rolando/internal/repositories"
	"time"
)

const (
	mediaValidatorConcurrency = 10
	mediaValidatorTimeout     = 4 * time.Second
)

type MediaValidator struct {
	markovRepo   *repositories.RedisRepository
	messagesRepo *repositories.MessagesRepository
	sem          chan struct{}
	httpClient   *http.Client
}

func NewMediaValidator(markovRepo *repositories.RedisRepository, messagesRepo *repositories.MessagesRepository) *MediaValidator {
	return &MediaValidator{
		markovRepo:   markovRepo,
		messagesRepo: messagesRepo,
		sem:          make(chan struct{}, mediaValidatorConcurrency),
		httpClient:   &http.Client{Timeout: mediaValidatorTimeout},
	}
}

// GetValidMedia returns a valid URL of the given kind for the guild.
// If the returned URL is dead it is purged and the next one is tried.
// At most maxRetries attempts are made before giving up.
func (mv *MediaValidator) GetValidMedia(ctx context.Context, guildID, kind string, maxRetries int) string {
	for i := 0; i < maxRetries; i++ {
		url, err := mv.markovRepo.GetRandomMedia(ctx, guildID, kind)
		if err != nil || url == "" {
			return ""
		}
		if mv.isAlive(url) {
			return url
		}
		mv.purgeAsync(ctx, guildID, kind, url)
	}
	return ""
}

// purgeAsync removes the URL from Redis and SQLite without blocking the caller.
// Respects the concurrency semaphore so we never flood with goroutines.
func (mv *MediaValidator) purgeAsync(ctx context.Context, guildID, kind, url string) {
	select {
	case mv.sem <- struct{}{}:
	default:
		// Semaphore full — skip purge this time rather than block.
		return
	}
	go func() {
		defer func() { <-mv.sem }()
		if err := mv.markovRepo.RemoveMedia(ctx, guildID, kind, url); err != nil {
			logger.Errorf("purgeAsync: redis remove failed for %s: %v", url, err)
		}
		if err := mv.messagesRepo.DeleteGuildMessage(guildID, url); err != nil {
			logger.Errorf("purgeAsync: db remove failed for %s: %v", url, err)
		}
	}()
}

func (mv *MediaValidator) isAlive(url string) bool {
	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return false
	}
	resp, err := mv.httpClient.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode < 400
}
