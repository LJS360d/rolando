package httpapi

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"rolando/internal/discord/services"
	"rolando/internal/guilds"

	"github.com/disgoorg/disgo"
)

func TestSessionAndGuards(t *testing.T) {
	s := NewServer(nil, nil)
	w := httptest.NewRecorder()
	setSession(w, &Session{UserID: "1", Name: "me", Guilds: []string{"42"}})
	cookie := w.Result().Cookies()[0]

	r := httptest.NewRequest("GET", "/api/session", nil)
	r.AddCookie(cookie)
	sess := getSession(r)
	if !sess.CanView("42") || sess.CanView("43") {
		t.Fatalf("session %+v", sess)
	}
	out := httptest.NewRecorder()
	s.public().ServeHTTP(out, r)
	if !strings.Contains(out.Body.String(), `"n":"me"`) {
		t.Fatal(out.Body.String())
	}

	for _, c := range []struct {
		method, path, ctype string
		code                int
	}{
		{"GET", "/api/admin/guilds", "", 403},
		{"POST", "/api/admin/broadcast", "application/x-www-form-urlencoded", 415},
		{"POST", "/api/auth/logout", "", 415},
	} {
		r := httptest.NewRequest(c.method, c.path, nil)
		r.AddCookie(cookie)
		if c.ctype != "" {
			r.Header.Set("Content-Type", c.ctype)
		}
		out := httptest.NewRecorder()
		s.public().ServeHTTP(out, r)
		if out.Code != c.code {
			t.Fatalf("%s %s: got %d want %d", c.method, c.path, out.Code, c.code)
		}
	}

	r.Header.Set("Cookie", "rolando_session="+strings.Replace(cookie.Value, "e", "f", 1))
	if getSession(r) != nil {
		t.Fatal("tampered session accepted")
	}
}

func TestNoGuildsMarshalAsEmptyArray(t *testing.T) {
	client, err := disgo.New("MTA.x.y")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(NewServer(client, nil).localGuilds(nil))
	if string(b) != "[]" {
		t.Fatal(string(b))
	}
}

func TestManagerConfigKeepsOwnerFields(t *testing.T) {
	m := guilds.NewManager(t.TempDir(), time.Hour, time.Hour)
	defer m.Close()
	if err := m.Use("42", func(g *guilds.Guild) error { _, err := g.CreateConfig("test"); return err }); err != nil {
		t.Fatal(err)
	}
	s := NewServer(nil, services.NewChainsService(nil, m))
	put := func(sess *Session, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		setSession(w, sess)
		r := httptest.NewRequest("PUT", "/api/guilds/42/config", strings.NewReader(body))
		r.AddCookie(w.Result().Cookies()[0])
		out := httptest.NewRecorder()
		s.guildRoutes().ServeHTTP(out, r)
		return out
	}
	body := `{"reply_rate":5,"reaction_rate":30,"vc_join_rate":1,"n_gram_size":2,"max_size_mb":999,"markov_max_branches":0,"pings":false,"premium":true}`
	if out := put(&Session{UserID: "1", Guilds: []string{"43"}}, body); out.Code != 403 {
		t.Fatalf("stranger: %d", out.Code)
	}
	out := put(&Session{UserID: "1", Guilds: []string{"42"}}, body)
	var cfg guilds.Config
	json.Unmarshal(out.Body.Bytes(), &cfg)
	if out.Code != 200 || cfg.ReplyRate != 5 || cfg.Pings || cfg.Premium || cfg.MaxSizeMb != 25 || cfg.MarkovMaxBranches != 256 || cfg.VcJoinRate != 100 {
		t.Fatalf("manager: %d %+v", out.Code, cfg)
	}
	out = put(&Session{UserID: "1", Owner: true}, body)
	json.Unmarshal(out.Body.Bytes(), &cfg)
	if out.Code != 200 || !cfg.Premium || cfg.MaxSizeMb != 999 || cfg.VcJoinRate != 1 {
		t.Fatalf("owner: %d %+v", out.Code, cfg)
	}
}
