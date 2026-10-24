package httpapi

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestRefresher(t *testing.T) {
	var calls atomic.Int32
	var fail atomic.Bool
	fetch := func(context.Context) (int, error) {
		n := calls.Add(1)
		if fail.Load() {
			return 0, errors.New("down")
		}
		return int(n), nil
	}
	r := refresher[int]{every: 20 * time.Millisecond}
	if v := r.get(fetch); v != 1 {
		t.Fatalf("first get %d", v)
	}
	if v := r.get(fetch); v != 1 || calls.Load() != 1 {
		t.Fatalf("fresh get %d after %d calls", v, calls.Load())
	}
	time.Sleep(30 * time.Millisecond)
	fail.Store(true)
	if v := r.get(fetch); v != 1 {
		t.Fatalf("stale get should serve the old value, got %d", v)
	}
	deadline := time.Now().Add(time.Second)
	for calls.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if v := r.get(fetch); v != 1 {
		t.Fatalf("failed refresh replaced the value: %d", v)
	}
}
