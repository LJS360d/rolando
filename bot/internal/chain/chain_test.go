package chain

import (
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	_ "github.com/glebarez/go-sqlite"
)

func TestTrainGenerateDeleteSnapshot(t *testing.T) {
	c := New(3)
	c.Train([]string{"the cat sat on the mat", "the cat ran away fast", "hi"}, 2, 0, 0)
	st := c.Stats()
	if st.Messages != 2 || st.Prefixes != 6 {
		t.Fatalf("stats %+v", st)
	}
	out := c.GenerateFromSeed("the cat", 10)
	if !strings.HasPrefix(out, "the cat ") {
		t.Fatalf("seeded generate: %q", out)
	}
	if r := c.GenerateRhyme("hat", 10); !strings.HasSuffix(r, "at") {
		t.Fatalf("rhyme: %q", r)
	}
	if n := c.Neighbors("sat", 10); n.Word != "sat" || len(n.In) != 1 || n.In[0] != (Link{"cat", 1}) || len(n.Out) != 1 || n.Out[0].Word != "on" {
		t.Fatalf("neighbors: %+v", n)
	}
	if n := c.Neighbors("cat", 1); len(n.In) != 0 || len(n.Out) != 1 || n.OutTotal != 2 {
		t.Fatalf("neighbors cut: %+v", n)
	}

	path := filepath.Join(t.TempDir(), "chain.snap")
	if err := c.WriteFile(path); err != nil {
		t.Fatal(err)
	}
	if c.Dirty() {
		t.Fatal("dirty after snapshot")
	}
	l, err := ReadFile(path, 3)
	if err != nil {
		t.Fatal(err)
	}
	if l.Stats() != st || l.Watermark() != 2 {
		t.Fatalf("loaded %+v wm=%d, want %+v", l.Stats(), l.Watermark(), st)
	}
	if _, err := ReadFile(path, 2); err != ErrSnapshot {
		t.Fatalf("ngram mismatch accepted: %v", err)
	}

	l.Delete("the cat ran away fast")
	l.Delete("the cat sat on the mat")
	if s := l.Stats(); s.Prefixes != 0 || s.Messages != 0 || len(l.keys) != 0 {
		t.Fatalf("after delete %+v", s)
	}

	data, _ := os.ReadFile(path)
	data[10] ^= 1
	os.WriteFile(path, data, 0o644)
	if _, err := ReadFile(path, 3); err != ErrSnapshot {
		t.Fatalf("corrupt snapshot accepted: %v", err)
	}
}

func TestCapAndSizeLimit(t *testing.T) {
	c := New(2)
	c.Train([]string{"a b", "a b", "a c", "a d"}, 0, 0, 2)
	if n := len(c.next[packKey([]uint32{c.ids["a"]})].edges); n != 2 {
		t.Fatalf("edges %d", n)
	}
	c.Train([]string{"x y", "x z", "x w"}, 0, 0, 0)
	if removed := c.Cap(1); removed != 3 {
		t.Fatalf("removed %d", removed)
	}
	full := New(2)
	full.Train([]string{"a b", "c d", "e f"}, 0, 1, 0)
	if full.Stats().Messages != 1 {
		t.Fatalf("size limit ignored: %+v", full.Stats())
	}
}

// TestBenchmarkLargestGuild is the phase 0 go/no-go measurement. It needs a
// copy of the legacy single-file database:
//
//	ROLANDO_BENCH_DB=data/data-backup/rolando.db go test -run Benchmark -v ./internal/chain
func TestBenchmarkLargestGuild(t *testing.T) {
	path := os.Getenv("ROLANDO_BENCH_DB")
	if path == "" {
		t.Skip("ROLANDO_BENCH_DB not set")
	}
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var guild string
	var n int
	if err := db.QueryRow(`SELECT guild_id, COUNT(*) c FROM messages GROUP BY guild_id ORDER BY c DESC LIMIT 1`).Scan(&guild, &n); err != nil {
		t.Fatal(err)
	}
	rows, err := db.Query(`SELECT content FROM messages WHERE guild_id = ? ORDER BY id`, guild)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var msgs []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		msgs = append(msgs, s)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	start := time.Now()
	c := New(2)
	c.Train(msgs, int64(len(msgs)), 0, 0)
	rebuild := time.Since(start)
	msgs = nil
	runtime.GC()
	runtime.ReadMemStats(&after)

	snap := filepath.Join(t.TempDir(), "chain.snap")
	start = time.Now()
	if err := c.WriteFile(snap); err != nil {
		t.Fatal(err)
	}
	write := time.Since(start)
	fi, _ := os.Stat(snap)
	start = time.Now()
	if _, err := ReadFile(snap, 2); err != nil {
		t.Fatal(err)
	}
	load := time.Since(start)
	st := c.Stats()
	t.Logf("guild=%s messages=%d prefixes=%d estimated=%dMB heap=%dMB rebuild=%s snapshot=%dMB write=%s load=%s",
		guild, n, st.Prefixes, st.Bytes>>20, (after.HeapAlloc-before.HeapAlloc)>>20, rebuild, fi.Size()>>20, write, load)
	runtime.KeepAlive(c)
}
