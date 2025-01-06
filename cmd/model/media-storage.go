package model

import (
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"rolando/cmd/log"
	"rolando/cmd/repositories"
	"rolando/cmd/utils"
	"sync"
	"time"
)

type MediaSet map[string]bool

type MediaStorage struct {
	chainID      string
	gifs         MediaSet
	images       MediaSet
	videos       MediaSet
	messagesRepo repositories.MessagesRepository
	mu           sync.RWMutex
	client       *http.Client
}

func NewMediaStorage(chainID string, gifs, images, videos []string, messagesRepo repositories.MessagesRepository) *MediaStorage {
	gifsMap := make(MediaSet, len(gifs))
	imagesMap := make(MediaSet, len(images))
	videosMap := make(MediaSet, len(videos))

	for _, gif := range gifs {
		gifsMap[gif] = true
	}
	for _, image := range images {
		imagesMap[image] = true
	}
	for _, video := range videos {
		videosMap[video] = true
	}

	return &MediaStorage{
		chainID:      chainID,
		gifs:         gifsMap,
		images:       imagesMap,
		videos:       videosMap,
		messagesRepo: messagesRepo,
		client: &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:       100,
				IdleConnTimeout:    90 * time.Second,
				DisableCompression: true, // only need headers
			},
		},
	}
}

func (ms *MediaStorage) AddMedia(url string) {
	ms.mu.Lock()
	if utils.IsGif(url) {
		ms.gifs[url] = true
	} else if utils.IsVideo(url) {
		ms.videos[url] = true
	} else if utils.IsImage(url) {
		ms.images[url] = true
	}
	ms.mu.Unlock()
}

func (ms *MediaStorage) RemoveMedia(url string) {
	ms.mu.Lock()
	delete(ms.gifs, url)
	delete(ms.videos, url)
	delete(ms.images, url)
	ms.mu.Unlock()
}

func (ms *MediaStorage) GetMedia(mediaType string) (string, error) {
	ms.mu.RLock()
	var urls []string
	var set MediaSet

	switch mediaType {
	case "gif":
		set = ms.gifs
	case "image":
		set = ms.images
	case "video":
		set = ms.videos
	default:
		ms.mu.RUnlock()
		return "", errors.New("invalid media type")
	}

	if len(set) == 0 {
		ms.mu.RUnlock()
		return "", fmt.Errorf("no media found for type '%s'", mediaType)
	}

	// Pre-allocate slice with exact size needed
	urls = make([]string, 0, len(set))
	for url := range set {
		urls = append(urls, url)
	}
	ms.mu.RUnlock()

	return ms.getValidUrlFromSet(urls)
}

func (ms *MediaStorage) getValidUrlFromSet(urls []string) (string, error) {
	if len(urls) == 0 {
		return "", errors.New("no URLs available")
	}

	// Create a copy of indices and shuffle them
	indices := make([]int, len(urls))
	for i := range indices {
		indices[i] = i
	}
	rand.Shuffle(len(indices), func(i, j int) {
		indices[i], indices[j] = indices[j], indices[i]
	})

	// Try URLs in random order
	for _, idx := range indices {
		url := urls[idx]
		valid, err := ms.validateUrl(url)
		if err != nil {
			continue
		}
		if valid {
			return url, nil
		}
	}

	return "", errors.New("no valid media URLs found")
}

func (ms *MediaStorage) validateUrl(url string) (bool, error) {
	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return false, err
	}

	resp, err := ms.client.Do(req)
	if err != nil {
		ms.handleInvalidUrl(url)
		return false, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		ms.handleInvalidUrl(url)
		return false, nil
	}

	return true, nil
}

func (ms *MediaStorage) handleInvalidUrl(url string) {
	ms.RemoveMedia(url)
	if err := ms.messagesRepo.DeleteGuildMessagesContaining(ms.chainID, url); err != nil {
		log.Log.Errorf("Error removing invalid URL: %v", err)
	}
}
