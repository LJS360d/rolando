package jackbox

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"rolando/internal/jackbox/common"
	"rolando/internal/logger"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type RoomSession struct {
	Room     *RoomBody
	RoomCode string
	UserID   string
	Name     string
	Conn     *websocket.Conn
}

func DialPlayWebSocket(ctx context.Context, room *RoomBody, roomCode, displayName, userID string) (*websocket.Conn, error) {
	host := strings.TrimSpace(room.Host)
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimPrefix(host, "http://")
	roomCode = strings.ToUpper(strings.TrimSpace(roomCode))
	if userID == "" {
		userID = uuid.NewString()
	}
	q := url.Values{}
	q.Set("role", "player")
	q.Set("name", displayName)
	q.Set("format", "json")
	q.Set("user-id", userID)
	u := url.URL{
		Scheme:   "wss",
		Host:     host,
		Path:     "/api/v2/rooms/" + roomCode + "/play",
		RawQuery: q.Encode(),
	}
	hdr := http.Header{}
	hdr.Set("User-Agent", jackboxUserAgent)
	hdr.Set("Origin", "https://jackbox.tv")
	hdr.Set("Referer", "https://jackbox.tv/")
	d := websocket.Dialer{
		HandshakeTimeout: 20 * time.Second,
		Subprotocols:     []string{"ecast-v0"},
	}
	conn, httpResp, err := d.DialContext(ctx, u.String(), hdr)
	if httpResp != nil {
		_ = httpResp.Body.Close()
	}
	if err != nil {
		return nil, fmt.Errorf("ecast play ws: %w", err)
	}
	return conn, nil
}

func DialRoomSession(ctx context.Context, room *RoomBody, roomCode, displayName string) (*RoomSession, error) {
	userID := uuid.NewString()
	conn, err := DialPlayWebSocket(ctx, room, roomCode, displayName, userID)
	if err != nil {
		return nil, err
	}
	return &RoomSession{
		Room:     room,
		RoomCode: strings.ToUpper(strings.TrimSpace(roomCode)),
		UserID:   userID,
		Name:     displayName,
		Conn:     conn,
	}, nil
}

const (
	readIdleTimeout = 15 * time.Minute
	pingInterval    = time.Minute
)

func RunPlayReadLoop(ctx context.Context, conn *websocket.Conn, mod common.GameModule) error {
	if mod == nil {
		return fmt.Errorf("RunPlayReadLoop: nil GameModule")
	}
	defer func() {
		err := mod.Close()
		if err != nil {
			logger.Errorf("failed to close jackbox game module: %v", err)
		}
	}()

	// a half-open connection never errors on its own; give up on silence.
	// Our pings keep an idle lobby talking, so only a dead peer goes quiet.
	conn.SetPingHandler(func(data string) error {
		conn.SetReadDeadline(time.Now().Add(readIdleTimeout))
		return ignoreTransient(conn.WriteControl(websocket.PongMessage, []byte(data), time.Now().Add(5*time.Second)))
	})
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(readIdleTimeout))
	})
	stopPings := make(chan struct{})
	defer close(stopPings)
	go func() {
		t := time.NewTicker(pingInterval)
		defer t.Stop()
		for {
			select {
			case <-stopPings:
				return
			case <-t.C:
				conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second))
			}
		}
	}()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		conn.SetReadDeadline(time.Now().Add(readIdleTimeout))
		mt, data, err := conn.ReadMessage()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseNoStatusReceived) {
				return nil
			}
			return err
		}
		if err := mod.HandleWSMessage(ctx, mt, data); err != nil {
			return err
		}
	}
}

// ignoreTransient mirrors gorilla's default ping handler: a pong that can't
// be written now is not a reason to drop the connection.
func ignoreTransient(err error) error {
	var ne net.Error
	if errors.Is(err, websocket.ErrCloseSent) || errors.As(err, &ne) && ne.Timeout() {
		return nil
	}
	return err
}
