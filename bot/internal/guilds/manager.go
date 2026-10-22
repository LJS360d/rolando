package guilds

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"time"

	"rolando/internal/logger"
)

// Manager owns the open guilds of this node. A guild directory is opened on
// first use and closed again after IdleTTL without users; dirty chains are
// snapshotted every SnapshotInterval, on close and on shutdown.
type Manager struct {
	Dir              string
	IdleTTL          time.Duration
	SnapshotInterval time.Duration

	mu     sync.Mutex
	guilds map[string]*Guild
}

func NewManager(dir string, idleTTL, snapshotInterval time.Duration) *Manager {
	return &Manager{Dir: dir, IdleTTL: idleTTL, SnapshotInterval: snapshotInterval, guilds: map[string]*Guild{}}
}

func (m *Manager) guildDir(id string) string { return filepath.Join(m.Dir, id) }

// Use runs fn with the guild open; the guild is not closed while fn runs.
func (m *Manager) Use(id string, fn func(*Guild) error) error {
	g, err := m.acquire(id)
	if err != nil {
		return err
	}
	defer m.release(g)
	return fn(g)
}

func (m *Manager) acquire(id string) (*Guild, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	g, ok := m.guilds[id]
	if !ok {
		var err error
		if g, err = Open(m.guildDir(id), id); err != nil {
			return nil, err
		}
		m.guilds[id] = g
	}
	g.refs++
	return g, nil
}

func (m *Manager) release(g *Guild) {
	m.mu.Lock()
	g.refs--
	g.lastUsed = time.Now()
	m.mu.Unlock()
}

// Exists reports whether the guild has a directory on this node.
func (m *Manager) Exists(id string) bool {
	_, err := os.Stat(filepath.Join(m.guildDir(id), "guild.db"))
	return err == nil
}

// Remove closes the guild and deletes its directory.
func (m *Manager) Remove(id string) error {
	m.mu.Lock()
	if g, ok := m.guilds[id]; ok {
		delete(m.guilds, id)
		closeDB(g.DB)
	}
	m.mu.Unlock()
	return os.RemoveAll(m.guildDir(id))
}

// IDs lists every guild directory on this node.
func (m *Manager) IDs() ([]string, error) {
	entries, err := os.ReadDir(m.Dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	ids := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			ids = append(ids, e.Name())
		}
	}
	return ids, err
}

// Summary returns live stats for open guilds and the persisted summary for
// the others, without opening them.
func (m *Manager) Summary(id string) (Summary, error) {
	m.mu.Lock()
	g, open := m.guilds[id]
	if open {
		g.refs++
	}
	m.mu.Unlock()
	if !open {
		return readSummary(filepath.Join(m.guildDir(id), "summary.json"))
	}
	defer m.release(g)
	return g.Summary()
}

func (m *Manager) Loaded() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, g := range m.guilds {
		if g.loaded() != nil {
			n++
		}
	}
	return n
}

func (m *Manager) IsLoaded(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	g, ok := m.guilds[id]
	return ok && g.loaded() != nil
}

// Run snapshots and evicts until ctx is done, then flushes everything.
func (m *Manager) Run(ctx context.Context) {
	snap := time.NewTicker(m.SnapshotInterval)
	evict := time.NewTicker(time.Minute)
	defer snap.Stop()
	defer evict.Stop()
	for {
		select {
		case <-ctx.Done():
			m.Close()
			return
		case <-snap.C:
			m.snapshotAll()
		case <-evict.C:
			m.evictIdle()
		}
	}
}

func (m *Manager) open() []*Guild {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*Guild, 0, len(m.guilds))
	for _, g := range m.guilds {
		out = append(out, g)
	}
	return out
}

func (m *Manager) snapshotAll() {
	for _, g := range m.open() {
		if err := g.Snapshot(); err != nil {
			logger.Errorf("snapshot %s: %v", g.ID, err)
		}
	}
}

func (m *Manager) evictIdle() {
	cutoff := time.Now().Add(-m.IdleTTL)
	m.mu.Lock()
	var idle []*Guild
	for id, g := range m.guilds {
		if g.refs == 0 && g.lastUsed.Before(cutoff) {
			delete(m.guilds, id)
			idle = append(idle, g)
		}
	}
	m.mu.Unlock()
	for _, g := range idle {
		logger.Debugf("guild %s: evicting, idle since %s", g.label(), g.lastUsed.Format(time.TimeOnly))
		m.closeGuild(g)
	}
}

func (m *Manager) closeGuild(g *Guild) {
	if err := g.Snapshot(); err != nil {
		logger.Errorf("snapshot %s: %v", g.ID, err)
	}
	closeDB(g.DB)
}

func (m *Manager) Close() {
	m.mu.Lock()
	all := m.guilds
	m.guilds = map[string]*Guild{}
	m.mu.Unlock()
	for _, g := range all {
		start := time.Now()
		name := g.label()
		logger.Debugf("guild %s: snapshot+close", name)
		m.closeGuild(g)
		logger.Debugf("guild %s: closed in %s", name, time.Since(start).Round(time.Millisecond))
	}
}
