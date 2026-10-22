package logger

import (
	"bytes"
	"log/slog"
	"strings"
	"sync"
	"testing"
)

func TestHandler(t *testing.T) {
	var buf bytes.Buffer
	l := slog.New(&handler{w: &buf, mu: &sync.Mutex{}, level: slog.LevelInfo, color: true}).With("shard", 2)
	l.Debug("hidden")
	l.Warn("hello", "n", 1)
	out := buf.String()
	if strings.Contains(out, "hidden") || !strings.Contains(out, "\t"+yellow+"WARN"+reset+"\thello shard=2 n=1\n") || !strings.HasPrefix(out, gray+"[") {
		t.Fatalf("%q", out)
	}
}
