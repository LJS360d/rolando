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
