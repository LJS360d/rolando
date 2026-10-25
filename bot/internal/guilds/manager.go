package guilds

import (
	"context"
	"errors"
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
	// guilds taken out of guilds that are still snapshotting and closing;
	// the channel closes when they are done
	closing map[string]chan struct{}
}

func NewManager(dir string, idleTTL, snapshotInterval time.Duration) *Manager {
	return &Manager{Dir: dir, IdleTTL: idleTTL, SnapshotInterval: snapshotInterval, guilds: map[string]*Guild{}, closing: map[string]chan struct{}{}}
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
	m.waitClosing(id)
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
	m.waitClosing(id)
	if g, ok := m.guilds[id]; ok {
		delete(m.guilds, id)
		closeDB(g.DB)
	}
	m.mu.Unlock()
	return os.RemoveAll(m.guildDir(id))
}

var ErrBusy = errors.New("guild is in use")

// Unload snapshots and closes an open guild now instead of waiting for
// IdleTTL; ErrBusy while something holds it.
func (m *Manager) Unload(id string) error {
	m.mu.Lock()
	g, ok := m.guilds[id]
	switch {
	case !ok:
		m.mu.Unlock()
		return nil
	case g.refs > 0:
		m.mu.Unlock()
		return ErrBusy
	}
	m.evict(g)
	m.mu.Unlock()
	m.closeEvicted(g)
	return nil
}

// evict takes g out of the open guilds, with mu held. Until closeEvicted
// finishes, acquire waits instead of opening the guild a second time while
// its snapshot is still being written.
func (m *Manager) evict(g *Guild) {
	delete(m.guilds, g.ID)
	m.closing[g.ID] = make(chan struct{})
}

func (m *Manager) closeEvicted(g *Guild) {
	m.closeGuild(g)
	m.mu.Lock()
	close(m.closing[g.ID])
	delete(m.closing, g.ID)
	m.mu.Unlock()
}

// waitClosing returns, with mu held, once id is not closing.
func (m *Manager) waitClosing(id string) {
	for {
		done, ok := m.closing[id]
		if !ok {
			return
		}
		m.mu.Unlock()
		<-done
		m.mu.Lock()
	}
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
	g := m.peek(id)
	if g == nil {
		return m.closedSummary(id)
	}
	defer m.unpeek(g)
	return g.Summary()
}

// PeekConfig returns the config without opening the guild, from the
// persisted summary when it is closed.
func (m *Manager) PeekConfig(id string) (*Config, error) {
	g := m.peek(id)
	if g == nil {
		s, err := m.closedSummary(id)
		return &s.Config, err
	}
	defer m.unpeek(g)
	return g.Config()
}

// closedSummary reads the persisted summary, opening the guild once to
// write it if it was never written (e.g. closed before its chain loaded).
func (m *Manager) closedSummary(id string) (Summary, error) {
	s, err := readSummary(filepath.Join(m.guildDir(id), "summary.json"))
	if !os.IsNotExist(err) || !m.Exists(id) {
		return s, err
	}
	err = m.Use(id, func(g *Guild) error {
		if s, err = g.Summary(); err != nil {
			return err
		}
		return g.WriteSummary()
	})
	return s, err
}

// peek holds an open guild without counting as a use: readers polling it
// must not keep it from evicting. nil if not open.
func (m *Manager) peek(id string) *Guild {
	m.mu.Lock()
	defer m.mu.Unlock()
	g, open := m.guilds[id]
	if !open {
		return nil
	}
	g.refs++
	return g
}

func (m *Manager) unpeek(g *Guild) {
	m.mu.Lock()
	g.refs--
	m.mu.Unlock()
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
	for _, g := range m.guilds {
		if g.refs == 0 && g.lastUsed.Before(cutoff) {
			m.evict(g)
			idle = append(idle, g)
		}
	}
	m.mu.Unlock()
	for _, g := range idle {
		logger.Debugf("guild %s: evicting, idle since %s", g.label(), g.lastUsed.Format(time.TimeOnly))
		m.closeEvicted(g)
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
	all := make([]*Guild, 0, len(m.guilds))
	for _, g := range m.guilds {
		all = append(all, g)
		m.evict(g)
	}
	m.mu.Unlock()
	for _, g := range all {
		start := time.Now()
		name := g.label()
		logger.Debugf("guild %s: snapshot+close", name)
		m.closeEvicted(g)
		logger.Debugf("guild %s: closed in %s", name, time.Since(start).Round(time.Millisecond))
	}
}
