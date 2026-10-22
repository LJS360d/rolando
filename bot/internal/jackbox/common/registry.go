package common

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

type Session struct {
	GuildLabel  string
	GuildID     string
	AppTag      string
	RoomCode    string
	UserID      string
	DisplayName string
	Conn        *websocket.Conn
}

type GameModule interface {
	HandleWSMessage(ctx context.Context, messageType int, payload []byte) error
	Close() error
}

type Factory func(ctx context.Context, s Session, jackboxService Jackbox) (GameModule, error)

type Registry struct {
	mu sync.RWMutex
	by map[string]Factory
}

func NewRegistry() *Registry {
	return &Registry{by: make(map[string]Factory)}
}

var defaultRegistry = NewRegistry()

func Default() *Registry {
	return defaultRegistry
}

func RegisterFactory(appTag string, f Factory) {
	defaultRegistry.MustRegister(appTag, f)
}

func (r *Registry) MustRegister(appTag string, f Factory) {
	tag := strings.ToLower(strings.TrimSpace(appTag))
	if tag == "" {
		panic("common: RegisterFactory empty app tag")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.by[tag]; exists {
		panic(fmt.Sprintf("common: duplicate game %q", tag))
	}
	r.by[tag] = f
}

func IsSupported(appTag string) bool {
	tag := strings.ToLower(strings.TrimSpace(appTag))
	defaultRegistry.mu.RLock()
	defer defaultRegistry.mu.RUnlock()
	_, ok := defaultRegistry.by[tag]
	return ok
}

func SupportedTags() []string {
	defaultRegistry.mu.RLock()
	defer defaultRegistry.mu.RUnlock()
	out := make([]string, 0, len(defaultRegistry.by))
	for k := range defaultRegistry.by {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

func (r *Registry) NewModule(ctx context.Context, appTag string, s Session, jackboxService Jackbox) (GameModule, error) {
	tag := strings.ToLower(strings.TrimSpace(appTag))
	r.mu.RLock()
	f, ok := r.by[tag]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unsupported jackbox game %q (supported: %s)", appTag, strings.Join(SupportedTags(), ", "))
	}
	return f(ctx, s, jackboxService)
}
