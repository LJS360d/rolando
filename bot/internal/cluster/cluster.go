package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"rolando/internal/config"

	"github.com/disgoorg/disgo/sharding"
	"github.com/disgoorg/snowflake/v2"
)

func ShardOf(guildID snowflake.ID) int {
	return sharding.ShardIDByGuild(guildID, config.ShardCount)
}

func OwnerNode(guildID snowflake.ID) int { return ShardOf(guildID) % config.NodeCount }

func IsLocal(guildID snowflake.ID) bool { return OwnerNode(guildID) == config.NodeIndex }

func LocalShards() []int {
	var ids []int
	for s := config.NodeIndex; s < config.ShardCount; s += config.NodeCount {
		ids = append(ids, s)
	}
	return ids
}

func PeerURL(node int) string { return fmt.Sprintf(config.NodeAddr, node) }

// SlotLimiter spaces identifies 5.5s apart cluster-wide without coordination:
// wall-clock time is cut into 5.5s slots and node i only identifies in slots
// where slot % NODE_COUNT == i. Needs synced clocks
type SlotLimiter struct {
	mu   sync.Mutex
	last time.Time
}

const identifyInterval = 5500 * time.Millisecond

func (l *SlotLimiter) Wait(ctx context.Context, _ int) error {
	l.mu.Lock()
	start := time.Now()
	if next := l.last.Add(identifyInterval); next.After(start) {
		start = next
	}
	slot := (start.UnixMilli() + identifyInterval.Milliseconds() - 1) / identifyInterval.Milliseconds()
	for slot%int64(config.NodeCount) != int64(config.NodeIndex) {
		slot++
	}
	at := time.UnixMilli(slot * identifyInterval.Milliseconds())
	t := time.NewTimer(time.Until(at))
	defer t.Stop()
	select {
	case <-ctx.Done():
		l.mu.Unlock()
		return ctx.Err()
	case <-t.C:
		l.last = at
		return nil
	}
}

func (l *SlotLimiter) Unlock(int) { l.mu.Unlock() }

func (l *SlotLimiter) Close(context.Context) {}

var peerClient = &http.Client{Timeout: 5 * time.Second}

// Do sends an authenticated request to a peer's node api.
func Do(ctx context.Context, node int, method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, PeerURL(node)+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Node-Secret", config.NodeSecret)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := peerClient.Do(req)
	if err == nil && resp.StatusCode >= 300 {
		resp.Body.Close()
		return nil, fmt.Errorf("node %d: %s %s: %s", node, method, path, resp.Status)
	}
	return resp, err
}

// GetJSON fetches path from every other node in parallel and decodes each
// response into a fresh T. Unreachable nodes are reported in errs.
func GetJSON[T any](ctx context.Context, path string) (results map[int]T, errs map[int]error) {
	results, errs = map[int]T{}, map[int]error{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for node := range config.NodeCount {
		if node == config.NodeIndex {
			continue
		}
		wg.Go(func() {
			var out T
			resp, err := Do(ctx, node, http.MethodGet, path, nil)
			if err == nil {
				err = json.NewDecoder(resp.Body).Decode(&out)
				resp.Body.Close()
			}
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs[node] = err
			} else {
				results[node] = out
			}
		})
	}
	wg.Wait()
	return
}
