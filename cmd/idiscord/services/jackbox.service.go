package services

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"rolando/internal/jackbox"
	"rolando/internal/logger"
	"rolando/internal/repositories"

	"github.com/disgoorg/disgo/bot"
	"github.com/gorilla/websocket"
)

type guildJackbox struct {
	mu        sync.Mutex
	cancel    context.CancelFunc
	conn      *websocket.Conn
	closeOnce sync.Once
}

func (g *guildJackbox) setConn(c *websocket.Conn) {
	g.mu.Lock()
	g.conn = c
	g.mu.Unlock()
}

func (g *guildJackbox) shutdownConn() {
	g.closeOnce.Do(func() {
		g.mu.Lock()
		c := g.conn
		g.conn = nil
		g.mu.Unlock()
		if c != nil {
			_ = c.Close()
		}
	})
}

type JackboxService struct {
	client   *bot.Client
	redis    *repositories.RedisRepository
	chains   *ChainsService
	http     *http.Client
	sessions sync.Map
	rateMu   sync.Mutex
	rateFail map[string][]time.Time
}

var jackboxRateWindow = time.Minute

const jackboxMaxFails = 6

func NewJackboxService(client *bot.Client, redis *repositories.RedisRepository, chains *ChainsService) *JackboxService {
	return &JackboxService{
		client:   client,
		redis:    redis,
		chains:   chains,
		http:     jackbox.DefaultHTTPClient(),
		rateFail: make(map[string][]time.Time),
	}
}

func (s *JackboxService) GenerateLine(ctx context.Context, guildID string, maxWords int) (string, error) {
	if s.chains == nil {
		return "", fmt.Errorf("chains service unavailable")
	}
	return s.chains.GenerateLine(ctx, guildID, maxWords)
}

func (s *JackboxService) rateLimited(guildID string) bool {
	s.rateMu.Lock()
	defer s.rateMu.Unlock()
	now := time.Now()
	cutoff := now.Add(-jackboxRateWindow)
	list := s.rateFail[guildID]
	kept := list[:0]
	for _, t := range list {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	s.rateFail[guildID] = kept
	return len(kept) >= jackboxMaxFails
}

func (s *JackboxService) noteFailure(guildID string) {
	s.rateMu.Lock()
	defer s.rateMu.Unlock()
	s.rateFail[guildID] = append(s.rateFail[guildID], time.Now())
}

func (s *JackboxService) Stop(guildID string) {
	if v, ok := s.sessions.LoadAndDelete(guildID); ok {
		g := v.(*guildJackbox)
		g.cancel()
		g.shutdownConn()
	}
	if err := s.redis.ClearJackboxState(context.Background(), guildID); err != nil {
		logger.Errorf("jackbox redis clear %s: %v", guildID, err)
	}
}

func (s *JackboxService) Start(ctx context.Context, guildID string, roomCode string) (activeAppTag string, err error) {
	if s.rateLimited(guildID) {
		return "", fmt.Errorf("rate limited: too many failed attempts for this server")
	}
	s.Stop(guildID)

	self, ok := s.client.Caches.SelfUser()
	if !ok {
		return "", fmt.Errorf("bot user not in cache")
	}
	room, err := jackbox.FetchEcastRoom(ctx, s.http, roomCode)
	if err != nil {
		s.noteFailure(guildID)
		return "", err
	}
	if !jackbox.IsGameSupported(room.AppTag) {
		s.noteFailure(guildID)
		return "", fmt.Errorf("unsupported jackbox game %q (supported: %s)", room.AppTag, strings.Join(jackbox.SupportedGameTags(), ", "))
	}

	sess, err := jackbox.DialRoomSession(ctx, room, roomCode, self.Username)
	if err != nil {
		s.noteFailure(guildID)
		return "", err
	}
	mod, err := jackbox.NewGameModuleForSession(ctx, room.AppTag, "guild="+guildID, guildID, sess, s)
	if err != nil {
		_ = sess.Conn.Close()
		s.noteFailure(guildID)
		return "", err
	}
	runCtx, cancel := context.WithCancel(context.Background())
	g := &guildJackbox{cancel: cancel}
	g.setConn(sess.Conn)
	s.sessions.Store(guildID, g)

	if err := s.redis.SetJackboxState(ctx, guildID, room.AppTag); err != nil {
		cancel()
		g.shutdownConn()
		s.sessions.CompareAndDelete(guildID, g)
		return "", err
	}

	go func(bg *guildJackbox, conn *websocket.Conn, mod jackbox.GameModule) {
		defer bg.shutdownConn()
		defer func() {
			if s.sessions.CompareAndDelete(guildID, bg) {
				if err := s.redis.ClearJackboxState(context.Background(), guildID); err != nil {
					logger.Errorf("jackbox redis clear %s: %v", guildID, err)
				}
			}
		}()
		err := jackbox.RunPlayReadLoop(runCtx, conn, mod)
		if err != nil && err != context.Canceled {
			logger.Errorf("jackbox session guild=%s: %v", guildID, err)
		}
	}(g, sess.Conn, mod)

	return room.AppTag, nil
}
