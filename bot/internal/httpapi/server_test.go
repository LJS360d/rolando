package httpapi

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

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
