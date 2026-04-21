package jackbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type RoomBody struct {
	AppID             string `json:"appId"`
	AppTag            string `json:"appTag"`
	AudienceEnabled   bool   `json:"audienceEnabled"`
	Code              string `json:"code"`
	Host              string `json:"host"`
	AudienceHost      string `json:"audienceHost"`
	Locked            bool   `json:"locked"`
	Full              bool   `json:"full"`
	MaxPlayers        int   `json:"maxPlayers"`
	MinPlayers        int   `json:"minPlayers"`
	ModerationEnabled bool  `json:"moderationEnabled"`
	PasswordRequired  bool  `json:"passwordRequired"`
	TwitchLocked      bool  `json:"twitchLocked"`
	Locale            string `json:"locale"`
	Keepalive         bool  `json:"keepalive"`
	ControllerBranch  string `json:"controllerBranch"`
}

type ecastRoomEnvelope struct {
	Ok    bool      `json:"ok"`
	Error string    `json:"error"`
	Body  *RoomBody `json:"body"`
}

func FetchEcastRoom(ctx context.Context, client *http.Client, roomCode string) (*RoomBody, error) {
	if client == nil {
		client = http.DefaultClient
	}
	roomCode = strings.ToUpper(strings.TrimSpace(roomCode))
	if len(roomCode) < 4 || len(roomCode) > 8 {
		return nil, fmt.Errorf("invalid room code length")
	}
	url := "https://ecast.jackboxgames.com/api/v2/rooms/" + roomCode
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	applyJackboxHTTPHeaders(req)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var env ecastRoomEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		if resp.StatusCode == http.StatusNotFound {
			return nil, errors.New("no such room")
		}
		if looksLikeHTML(body) {
			return nil, fmt.Errorf("ecast room: non-json response (http %d)", resp.StatusCode)
		}
		return nil, fmt.Errorf("ecast room: decode: %w", err)
	}
	if !env.Ok {
		if env.Error != "" {
			return nil, errors.New(env.Error)
		}
		return nil, errors.New("ecast room: not ok")
	}
	if env.Body == nil {
		return nil, fmt.Errorf("ecast room: empty body")
	}
	if strings.TrimSpace(env.Body.Host) == "" {
		return nil, fmt.Errorf("ecast room: missing host")
	}
	return env.Body, nil
}

func looksLikeHTML(b []byte) bool {
	s := strings.ToLower(string(b))
	return strings.Contains(s, "<html") || strings.Contains(s, "<!doctype")
}
