package cluster

import (
	"context"
	"slices"
	"testing"
	"time"

	"rolando/internal/config"

	"github.com/disgoorg/snowflake/v2"
)

func TestOwnershipAndSlots(t *testing.T) {
	config.ShardCount, config.NodeCount, config.NodeIndex = 16, 4, 2
	if got := LocalShards(); !slices.Equal(got, []int{2, 6, 10, 14}) {
		t.Fatalf("shards %v", got)
	}
	for id := snowflake.ID(1 << 22); id < 64<<22; id += 1 << 22 {
		if IsLocal(id) != slices.Contains(LocalShards(), ShardOf(id)) {
			t.Fatalf("guild %d shard %d", id, ShardOf(id))
		}
	}

	config.NodeCount, config.NodeIndex = 2, 0
	var l SlotLimiter
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := l.Wait(ctx, 0); err != nil {
		t.Fatal(err)
	}
	slot := time.Now().UnixMilli() / identifyInterval.Milliseconds()
	l.Unlock(0)
	if slot%2 != 0 {
		t.Fatalf("identified in slot %d owned by another node", slot)
	}

	config.NodeCount = 1
	time.Sleep(300 * time.Millisecond)
	first := time.Now()
	if err := l.Wait(ctx, 1); err != nil {
		t.Fatal(err)
	}
	l.Unlock(1)
	if gap := time.Since(first); gap > identifyInterval {
		t.Fatalf("next identify waited %s, want at most one slot", gap)
	}
}
