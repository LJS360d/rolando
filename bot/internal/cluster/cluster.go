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

var (
	peerClient = &http.Client{Timeout: 5 * time.Second}
	// streamClient has no overall timeout: streamed responses last as long as ctx
	streamClient = &http.Client{}
)

// Do sends an authenticated request to a peer's node api.
func Do(ctx context.Context, node int, method, path string, body io.Reader) (*http.Response, error) {
	return do(ctx, peerClient, node, method, path, body)
}

// Stream is Do for long-running streamed responses, bounded only by ctx.
func Stream(ctx context.Context, node int, method, path string, body io.Reader) (*http.Response, error) {
	return do(ctx, streamClient, node, method, path, body)
}

func do(ctx context.Context, c *http.Client, node int, method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, PeerURL(node)+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Node-Secret", config.NodeSecret)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.Do(req)
	if err == nil && resp.StatusCode >= 300 {
		CloseBody(resp)
		return nil, fmt.Errorf("node %d: %s %s: %s", node, method, path, resp.Status)
	}
	return resp, err
}

// CloseBody drains what's left of a peer response (a JSON decoder stops
// before EOF) so the keep-alive connection can be reused.
func CloseBody(resp *http.Response) {
	io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	resp.Body.Close()
}

// GetJSON fetches path from every other node in parallel and decodes each
// response into a fresh T. Unreachable nodes are reported in errs.
func GetJSON[T any](ctx context.Context, path string) (results map[int]T, errs map[int]error) {
	paths := map[int]string{}
	for node := range config.NodeCount {
		if node != config.NodeIndex {
			paths[node] = path
		}
	}
	return GetJSONEach[T](ctx, paths)
}

// GetJSONEach is GetJSON with a path per node, querying only those nodes.
func GetJSONEach[T any](ctx context.Context, paths map[int]string) (results map[int]T, errs map[int]error) {
	results, errs = map[int]T{}, map[int]error{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for node, path := range paths {
		wg.Go(func() {
			var out T
			resp, err := Do(ctx, node, http.MethodGet, path, nil)
			if err == nil {
				err = json.NewDecoder(resp.Body).Decode(&out)
				CloseBody(resp)
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
