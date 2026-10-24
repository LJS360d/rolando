package guilds

import (
	"testing"
	"time"
)

func TestGuildLifecycle(t *testing.T) {
	m := NewManager(t.TempDir(), time.Hour, time.Hour)
	defer m.Close()
	use := func(fn func(*Guild) error) {
		t.Helper()
		if err := m.Use("1", fn); err != nil {
			t.Fatal(err)
		}
	}
	use(func(g *Guild) error {
		if _, err := g.CreateConfig("test"); err != nil {
			return err
		}
		if err := g.AddMessages([]string{"a b c", "look https://tenor.com/x.gif"}); err != nil {
			return err
		}
		c, err := g.Chain()
		if err != nil {
			return err
		}
		if s := c.Stats(); s.Messages != 2 || c.Watermark() != 2 {
			t.Fatalf("after load %+v wm=%d", s, c.Watermark())
		}
		if err := g.Snapshot(); err != nil {
			return err
		}
		if err := g.AddMessages([]string{"c d e"}); err != nil {
			return err
		}
		if u, _ := g.RandomMedia("gif"); u != "https://tenor.com/x.gif" {
			t.Fatalf("media %q", u)
		}
		return nil
	})

	m.Close()
	use(func(g *Guild) error {
		c, err := g.Chain()
		if err != nil {
			return err
		}
		if s := c.Stats(); s.Messages != 3 || c.Watermark() != 3 {
			t.Fatalf("snapshot + replay %+v wm=%d", s, c.Watermark())
		}
		if _, err := g.UpdateConfig(map[string]any{"n_gram_size": 3}); err != nil {
			return err
		}
		if err := g.Rebuild(); err != nil {
			return err
		}
		c, _ = g.Chain()
		if c.N() != 3 || c.Stats().Messages != 2 {
			t.Fatalf("rebuild n=%d %+v", c.N(), c.Stats())
		}
		if err := g.DeleteMessage("c d e"); err != nil {
			return err
		}
		if c.Stats().Messages != 1 {
			t.Fatalf("delete %+v", c.Stats())
		}
		return g.Reset()
	})
	sum, err := m.Summary("1")
	if err != nil || sum.Stats.Messages != 0 || sum.Media.Gifs != 0 {
		t.Fatalf("after reset %+v %v", sum, err)
	}
}

func TestConfigCache(t *testing.T) {
	g, err := Open(t.TempDir(), "1")
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	if _, err := g.CreateConfig("test"); err != nil {
		t.Fatal(err)
	}
	c, _ := g.Config()
	c.ReplyRate = 99
	if c, _ := g.Config(); c.ReplyRate != 10 {
		t.Fatalf("cached config mutated through a copy: %d", c.ReplyRate)
	}
	if _, err := g.UpdateConfig(map[string]any{"reply_rate": 5}); err != nil {
		t.Fatal(err)
	}
	if c, _ := g.Config(); c.ReplyRate != 5 {
		t.Fatalf("stale config after update: %d", c.ReplyRate)
	}
}

func TestMessageCount(t *testing.T) {
	g, err := Open(t.TempDir(), "1")
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	if _, err := g.CreateConfig("test"); err != nil {
		t.Fatal(err)
	}
	g.AddMessages([]string{"a b", "c d", "a b"})
	total := func() int64 {
		t.Helper()
		_, n, err := g.MessagesPage(10, 0)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	if n := total(); n != 3 {
		t.Fatalf("count %d", n)
	}
	g.AddMessages([]string{"e f"})
	g.DeleteMessage("a b")
	if n := total(); n != 2 {
		t.Fatalf("count after add+delete %d", n)
	}
	g.Reset()
	if n := total(); n != 0 {
		t.Fatalf("count after reset %d", n)
	}
}

func TestReopenAfterRemoveMigrates(t *testing.T) {
	m := NewManager(t.TempDir(), time.Hour, time.Hour)
	defer m.Close()
	create := func() error {
		return m.Use("1", func(g *Guild) error { _, err := g.CreateConfig("test"); return err })
	}
	if err := create(); err != nil {
		t.Fatal(err)
	}
	if err := m.Remove("1"); err != nil {
		t.Fatal(err)
	}
	if err := create(); err != nil {
		t.Fatalf("recreated guild not migrated: %v", err)
	}
}

func TestPeekConfigWithoutSummary(t *testing.T) {
	m := NewManager(t.TempDir(), time.Hour, time.Hour)
	defer m.Close()
	if err := m.Use("1", func(g *Guild) error {
		_, err := g.CreateConfig("test")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	m.Close()
	if c, err := m.PeekConfig("1"); err != nil || c.Name != "test" {
		t.Fatalf("%+v %v", c, err)
	}
	if _, err := m.PeekConfig("2"); err == nil || m.Exists("2") {
		t.Fatal("unknown guild created")
	}
}
