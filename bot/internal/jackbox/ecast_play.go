package jackbox

import (
	"context"
	"fmt"
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

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
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
