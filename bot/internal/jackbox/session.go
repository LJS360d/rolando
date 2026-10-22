package jackbox

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

func RunEcastRoomConnection(ctx context.Context, httpClient *http.Client, roomCode, displayName string) error {
	room, err := FetchEcastRoom(ctx, httpClient, roomCode)
	if err != nil {
		return err
	}
	sess, err := DialRoomSession(ctx, room, roomCode, displayName)
	if err != nil {
		return err
	}
	defer sess.Conn.Close()
	if !IsGameSupported(room.AppTag) {
		_ = sess.Conn.Close()
		return fmt.Errorf("unsupported jackbox game %q", room.AppTag)
	}
	mod, err := NewGameModuleForSession(ctx, room.AppTag, "standalone", "", sess, nil)
	if err != nil {
		_ = sess.Conn.Close()
		return err
	}
	return RunPlayReadLoop(ctx, sess.Conn, mod)
}

func DefaultHTTPClient() *http.Client {
	return &http.Client{Timeout: 45 * time.Second}
}
