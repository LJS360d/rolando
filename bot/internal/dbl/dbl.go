// Package dbl posts commands and per-shard stats to discordbotlist.com.
package dbl

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"rolando/internal/cluster"
	"rolando/internal/config"
	"rolando/internal/logger"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
)

const (
	baseURL       = "https://discordbotlist.com/api/v1"
	statsInterval = 30 * time.Minute
)

var httpClient = &http.Client{Timeout: 15 * time.Second}

type Stats struct {
	Guilds           int `json:"guilds"`
	Users            int `json:"users"`
	VoiceConnections int `json:"voice_connections"`
	ShardID          int `json:"shard_id"`
}

// Run is a no-op without DBL_TOKEN. Node 0 posts the global commands once;
// every node posts stats for its own shards until ctx is done.
func Run(ctx context.Context, client *bot.Client, commands []discord.ApplicationCommandCreate) {
	if config.DBLToken == "" {
		return
	}
	appID := client.ApplicationID.String()

	if config.NodeIndex == 0 && len(commands) > 0 {
		if err := post(ctx, "/bots/"+appID+"/commands", commands); err != nil {
			logger.Errorf("dbl: posting commands: %v", err)
		} else {
			logger.Infof("dbl: posted %d commands", len(commands))
		}
	}

	t := time.NewTimer(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			postStats(ctx, client, appID)
			t.Reset(statsInterval)
		}
	}
}

func postStats(ctx context.Context, client *bot.Client, appID string) {
	stats := make(map[int]*Stats)
	for _, s := range cluster.LocalShards() {
		stats[s] = &Stats{ShardID: s}
	}
	for g := range client.Caches.Guilds() {
		if s, ok := stats[cluster.ShardOf(g.ID)]; ok {
			s.Guilds++
			s.Users += g.MemberCount
		}
	}
	for c := range client.VoiceManager.Conns() {
		if s, ok := stats[cluster.ShardOf(c.GuildID())]; ok {
			s.VoiceConnections++
		}
	}
	for _, s := range stats {
		if err := post(ctx, "/bots/"+appID+"/stats", s); err != nil {
			logger.Warnf("dbl: posting stats for shard %d: %v", s.ShardID, err)
		}
	}
	logger.Debugf("dbl: posted stats for %d shards", len(stats))
}

func post(ctx context.Context, path string, body any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bot "+config.DBLToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("%s: %s", resp.Status, msg)
	}
	return nil
}
