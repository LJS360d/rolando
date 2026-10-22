package httpapi

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"rolando/internal/config"

	"github.com/disgoorg/disgo/discord"
)

const (
	sessionCookie = "rolando_session"
	stateCookie   = "rolando_oauth_state"
	sessionTTL    = 7 * 24 * time.Hour
	maxGuilds     = 150
)

// Session lives in an HMAC-signed cookie so any node can verify it.
type Session struct {
	UserID string   `json:"u"`
	Name   string   `json:"n"`
	Avatar string   `json:"a"`
	Color  int      `json:"c"`
	Owner  bool     `json:"o"`
	Guilds []string `json:"g"`
	Exp    int64    `json:"e"`
}

func (s *Session) CanView(guildID string) bool {
	return s != nil && (s.Owner || slices.Contains(s.Guilds, guildID))
}

func sign(payload string) string {
	m := hmac.New(sha256.New, sessionSecret)
	m.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

var sessionSecret = func() []byte {
	if config.SessionSecret != "" {
		return []byte(config.SessionSecret)
	}
	b := make([]byte, 32)
	rand.Read(b)
	return b
}()

func setSession(w http.ResponseWriter, s *Session) {
	s.Exp = time.Now().Add(sessionTTL).Unix()
	b, _ := json.Marshal(s)
	payload := base64.RawURLEncoding.EncodeToString(b)
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: payload + "." + sign(payload), Path: "/",
		HttpOnly: true, Secure: strings.HasPrefix(config.SiteURL, "https"), SameSite: http.SameSiteLaxMode,
		MaxAge: int(sessionTTL.Seconds())})
}

func getSession(r *http.Request) *Session {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return nil
	}
	payload, sig, ok := strings.Cut(c.Value, ".")
	if !ok || !hmac.Equal([]byte(sig), []byte(sign(payload))) {
		return nil
	}
	b, err := base64.RawURLEncoding.DecodeString(payload)
	var s Session
	if err != nil || json.Unmarshal(b, &s) != nil || time.Now().Unix() > s.Exp {
		return nil
	}
	return &s
}

func (s *Server) redirectURI() string { return config.SiteURL + "/api/auth/callback" }

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	b := make([]byte, 16)
	rand.Read(b)
	state := base64.RawURLEncoding.EncodeToString(b)
	http.SetCookie(w, &http.Cookie{Name: stateCookie, Value: state, Path: "/api/auth", HttpOnly: true, MaxAge: 600, SameSite: http.SameSiteLaxMode})
	q := url.Values{
		"client_id":     {s.client.ApplicationID.String()},
		"response_type": {"code"},
		"scope":         {"identify guilds"},
		"redirect_uri":  {s.redirectURI()},
		"state":         {state},
		"prompt":        {"none"},
	}
	http.Redirect(w, r, "https://discord.com/oauth2/authorize?"+q.Encode(), http.StatusFound)
}

func (s *Server) callback(w http.ResponseWriter, r *http.Request) {
	st, err := r.Cookie(stateCookie)
	if err != nil || st.Value == "" || st.Value != r.URL.Query().Get("state") {
		http.Error(w, "invalid oauth state", http.StatusBadRequest)
		return
	}
	token, err := s.exchange(r.URL.Query().Get("code"))
	if err != nil {
		http.Error(w, "login failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	var user discord.OAuth2User
	var guilds []discord.OAuth2Guild
	if err := discordGet(token, "/users/@me", &user); err != nil {
		http.Error(w, "login failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	if err := discordGet(token, "/users/@me/guilds", &guilds); err != nil {
		http.Error(w, "login failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	sess := &Session{UserID: user.ID.String(), Name: user.EffectiveName(), Avatar: user.EffectiveAvatarURL(),
		Owner: slices.Contains(config.OwnerIDs, user.ID.String())}
	if user.AccentColor != nil {
		sess.Color = *user.AccentColor
	}
	for _, g := range guilds {
		if (g.Owner || g.Permissions.Has(discord.PermissionManageGuild)) && len(sess.Guilds) < maxGuilds {
			sess.Guilds = append(sess.Guilds, g.ID.String())
		}
	}
	setSession(w, sess)
	if sess.Owner {
		http.Redirect(w, r, "/admin", http.StatusFound)
		return
	}
	http.Redirect(w, r, "/me", http.StatusFound)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Path: "/", MaxAge: -1})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) exchange(code string) (string, error) {
	resp, err := http.PostForm("https://discord.com/api/v10/oauth2/token", url.Values{
		"client_id":     {s.client.ApplicationID.String()},
		"client_secret": {config.OAuthClientSecret},
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {s.redirectURI()},
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.AccessToken == "" {
		return "", fmt.Errorf("token exchange: %s %s", resp.Status, out.Error)
	}
	return out.AccessToken, nil
}

func discordGet(token, path string, out any) error {
	req, _ := http.NewRequest(http.MethodGet, "https://discord.com/api/v10"+path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", path, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func atoiOr(s string, def int) int {
	if v, err := strconv.Atoi(s); err == nil && v > 0 {
		return v
	}
	return def
}
