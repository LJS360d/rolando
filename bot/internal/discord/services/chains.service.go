package services

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"rolando/internal/chain"
	"rolando/internal/guilds"
	"rolando/internal/logger"
	"rolando/internal/utils"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/snowflake/v2"
)

type ChainsService struct {
	session  *bot.Client
	Guilds   *guilds.Manager
	fetching sync.Map
	// guilds with a post-delete snapshot queued
	snapping sync.Map
}

func NewChainsService(client *bot.Client, manager *guilds.Manager) *ChainsService {
	return &ChainsService{session: client, Guilds: manager}
}

// GetChainConf returns the guild config, creating it on first contact.
func (cs *ChainsService) GetChainConf(_ context.Context, id string) (*guilds.Config, error) {
	var cfg *guilds.Config
	err := cs.Guilds.Use(id, func(g *guilds.Guild) error {
		var err error
		cfg, err = g.Config()
		if !guilds.IsNotFound(err) {
			return err
		}
		gid, err := snowflake.Parse(id)
		if err != nil {
			return err
		}
		guild, ok := cs.session.Caches.Guild(gid)
		if !ok {
			return fmt.Errorf("guild %s not found in cache", id)
		}
		logger.Infof("Creating chain: %s", guild.Name)
		cfg, err = g.CreateConfig(guild.Name)
		return err
	})
	return cfg, err
}

func (cs *ChainsService) CreateChain(ctx context.Context, id, _ string) (*guilds.Config, error) {
	return cs.GetChainConf(ctx, id)
}

func (cs *ChainsService) WithChain(id string, fn func(*chain.Chain)) error {
	return cs.Guilds.Use(id, func(g *guilds.Guild) error {
		c, err := g.Chain()
		if err == nil {
			fn(c)
		}
		return err
	})
}

// StartFetch marks a guild as fetching its history; false if it already is.
func (cs *ChainsService) StartFetch(id string) bool {
	_, busy := cs.fetching.LoadOrStore(id, struct{}{})
	return !busy
}

func (cs *ChainsService) EndFetch(id string) { cs.fetching.Delete(id) }

// "fetching", "empty", "loaded", "unloaded"
func (cs *ChainsService) State(id string) string {
	if _, ok := cs.fetching.Load(id); ok {
		return "fetching"
	}
	if !cs.Guilds.Exists(id) {
		return "empty"
	}
	if cs.Guilds.IsLoaded(id) {
		return "loaded"
	}
	return "unloaded"
}

// Load builds the chain now; it is still evicted after IdleTTL
func (cs *ChainsService) Load(id string) error {
	return cs.WithChain(id, func(*chain.Chain) {})
}

func (cs *ChainsService) Unload(id string) error {
	if cs.State(id) == "fetching" {
		return guilds.ErrBusy
	}
	return cs.Guilds.Unload(id)
}

func (cs *ChainsService) UpdateChainState(_ context.Context, id string, texts []string) error {
	return cs.Guilds.Use(id, func(g *guilds.Guild) error { return g.AddMessages(texts) })
}

// DeleteTextData removes a message from storage and the chain, then
// snapshots in the background so the delete survives a crash. A burst of
// deletes shares one snapshot.
func (cs *ChainsService) DeleteTextData(_ context.Context, id, data string) error {
	if err := cs.Guilds.Use(id, func(g *guilds.Guild) error { return g.DeleteMessage(data) }); err != nil {
		return err
	}
	if _, queued := cs.snapping.LoadOrStore(id, struct{}{}); queued {
		return nil
	}
	go func() {
		// deletes landing after this point queue the next snapshot
		cs.snapping.Delete(id)
		if err := cs.Guilds.Use(id, (*guilds.Guild).Snapshot); err != nil {
			logger.Errorf("snapshot after delete %s: %v", id, err)
		}
	}()
	return nil
}

func (cs *ChainsService) Generate(_ context.Context, id string, maxLength int) (out string, err error) {
	err = cs.WithChain(id, func(c *chain.Chain) { out = c.Generate(maxLength) })
	return
}

func (cs *ChainsService) GenerateFromSeed(_ context.Context, id, seed string, maxLength int) (out string, err error) {
	err = cs.WithChain(id, func(c *chain.Chain) { out = c.GenerateFromSeed(seed, maxLength) })
	return
}

func (cs *ChainsService) GenerateRhyme(_ context.Context, id, word string, maxLength int) (out string, err error) {
	err = cs.WithChain(id, func(c *chain.Chain) { out = c.GenerateRhyme(word, maxLength) })
	return
}

func (cs *ChainsService) GenerateFiltered(ctx context.Context, id string, maxLength int) (string, error) {
	out, err := cs.Generate(ctx, id, maxLength)
	return FilterText(out, false), err
}

func (cs *ChainsService) GetRandomMedia(_ context.Context, id, kind string) (url string, err error) {
	err = cs.Guilds.Use(id, func(g *guilds.Guild) error {
		url, err = g.RandomMedia(kind)
		return err
	})
	return
}

func (cs *ChainsService) Persist(id string) error {
	return cs.Guilds.Use(id, func(g *guilds.Guild) error {
		if _, err := g.Chain(); err != nil {
			return err
		}
		if err := g.Snapshot(); err != nil {
			return err
		}
		return g.WriteSummary()
	})
}

// Summary does not load the chain; unloaded chains report persisted stats.
func (cs *ChainsService) Summary(id string) (s guilds.Summary, err error) {
	err = cs.Guilds.Use(id, func(g *guilds.Guild) error {
		s, err = g.Summary()
		return err
	})
	return
}

func (cs *ChainsService) MessagesPage(id string, limit int, q guilds.PageQuery) (rows []guilds.Message, total int64, err error) {
	err = cs.Guilds.Use(id, func(g *guilds.Guild) error {
		rows, total, err = g.MessagesPage(limit, q)
		return err
	})
	return
}

// UpdateChainMeta applies config updates. An n-gram change rebuilds the chain
// and a branch cap change prunes it, both in the background.
func (cs *ChainsService) UpdateChainMeta(ctx context.Context, id string, fields map[string]any) (*guilds.Config, error) {
	old, updated, err := cs.updateConfig(ctx, id, fields)
	if err != nil {
		return nil, err
	}
	switch {
	case updated.NGramSize != old.NGramSize:
		go cs.background(id, "rebuild", func(g *guilds.Guild) error { return g.Rebuild() })
	case branchesChanged(old, updated):
		go cs.background(id, "cap branches", func(g *guilds.Guild) error { return capBranches(g, updated.MarkovMaxBranches) })
	}
	return updated, nil
}

// UpdateChainMetaSync is UpdateChainMeta for batches: the branch cap runs
// before it returns, and a chain loaded only to be capped is unloaded again,
// so a batch holds at most one chain per worker. No n-gram changes.
func (cs *ChainsService) UpdateChainMetaSync(ctx context.Context, id string, fields map[string]any) (*guilds.Config, error) {
	if _, ok := fields["n_gram_size"]; ok {
		return nil, errors.New("n_gram_size changes rebuild the chain")
	}
	wasLoaded := cs.Guilds.IsLoaded(id)
	old, updated, err := cs.updateConfig(ctx, id, fields)
	if err != nil || !branchesChanged(old, updated) {
		return updated, err
	}
	if err := cs.Guilds.Use(id, func(g *guilds.Guild) error { return capBranches(g, updated.MarkovMaxBranches) }); err != nil {
		return nil, err
	}
	if !wasLoaded {
		// ErrBusy: someone started using it meanwhile, it evicts after IdleTTL
		cs.Unload(id)
	}
	return updated, nil
}

func (cs *ChainsService) updateConfig(ctx context.Context, id string, fields map[string]any) (old, updated *guilds.Config, err error) {
	if _, ok := fields["id"]; ok {
		return nil, nil, errors.New("cannot change field 'id'")
	}
	if old, err = cs.GetChainConf(ctx, id); err != nil {
		return nil, nil, err
	}
	err = cs.Guilds.Use(id, func(g *guilds.Guild) error {
		updated, err = g.UpdateConfig(fields)
		return err
	})
	return old, updated, err
}

func branchesChanged(old, updated *guilds.Config) bool {
	return updated.MarkovMaxBranches > 0 && updated.MarkovMaxBranches != old.MarkovMaxBranches
}

func capBranches(g *guilds.Guild, n int) error {
	c, err := g.Chain()
	if err == nil {
		c.Cap(n)
	}
	return err
}

func (cs *ChainsService) background(id, what string, fn func(*guilds.Guild) error) {
	if err := cs.Guilds.Use(id, fn); err != nil {
		logger.Errorf("%s %s: %v", what, id, err)
	}
}

func (cs *ChainsService) DeleteChain(_ context.Context, id string) error {
	logger.Warnf("Deleting chain: %s", id)
	return cs.Guilds.Remove(id)
}

// ResetChain clears messages, media and the chain but keeps the config.
func (cs *ChainsService) ResetChain(_ context.Context, id string) error {
	logger.Warnf("Resetting chain: %s", id)
	return cs.Guilds.Use(id, func(g *guilds.Guild) error { return g.Reset() })
}

var (
	reDiscordPing = regexp.MustCompile(`<@\S+>`)
	reBadChars    = regexp.MustCompile(`[\*_~|\[\]\(\)\{\}#\+\-!<>=\\` + "`" + `]`)
	reLongNums    = regexp.MustCompile(`\b\d{6,}\b`)
	reSpaces      = regexp.MustCompile(`\s+`)
)

func FilterText(text string, pings bool) string {
	if !pings {
		text = reDiscordPing.ReplaceAllString(text, "")
	}
	text = utils.ReURL.ReplaceAllString(text, "")
	text = reBadChars.ReplaceAllString(text, "")
	text = strings.TrimSpace(text)
	text = reSpaces.ReplaceAllString(text, " ")
	text = reLongNums.ReplaceAllStringFunc(text, func(m string) string { return m[:5] })
	return text
}
