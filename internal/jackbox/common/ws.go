package common

import (
	"encoding/json"

	"github.com/gorilla/websocket"
)

func WriteTextJSON(conn *websocket.Conn, v any) error {
	if conn == nil {
		return nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.TextMessage, raw)
}
