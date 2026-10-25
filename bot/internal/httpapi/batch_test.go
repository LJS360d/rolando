package httpapi

import (
	"bufio"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"rolando/internal/discord/services"
	"rolando/internal/guilds"

	"github.com/disgoorg/disgo"
)

func TestBatchStream(t *testing.T) {
	client, err := disgo.New("MTA.x.y")
	if err != nil {
		t.Fatal(err)
	}
	m := guilds.NewManager(t.TempDir(), time.Hour, time.Hour)
	defer m.Close()
	const id, missing = "123456789012345678", "223456789012345678"
	if err := m.Use(id, func(g *guilds.Guild) error {
		if _, err := g.CreateConfig("test"); err != nil {
			return err
		}
		_, err := g.Chain()
		return err
	}); err != nil {
		t.Fatal(err)
	}
	s := NewServer(client, services.NewChainsService(client, m))
	w := httptest.NewRecorder()
	setSession(w, &Session{UserID: "1", Owner: true})
	cookie := w.Result().Cookies()[0]

	post := func(body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/api/admin/batch", strings.NewReader(body))
		r.AddCookie(cookie)
		r.Header.Set("Content-Type", "application/json")
		out := httptest.NewRecorder()
		s.public().ServeHTTP(out, r)
		return out
	}
	events := func(out *httptest.ResponseRecorder) []batchEvent {
		var es []batchEvent
		sc := bufio.NewScanner(out.Body)
		for sc.Scan() {
			var e batchEvent
			if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
				t.Fatal(sc.Text())
			}
			es = append(es, e)
		}
		return es
	}

	for body, code := range map[string]int{
		`{"op":"nope","ids":["1"]}`: 400,
		`{"op":"unload"}`:           400,
		`{"op":"config","ids":["1"],"params":{"n_gram_size":3}}`: 400,
		`{"op":"config","ids":["1"],"params":{}}`:                400,
		`{"op":"config","ids":["1"],"params":{"reply_rate":-1}}`: 400,
		`{"op":"unload","query":"f=nope:eq:1"}`:                  400,
		`{"op":"unload","ids":["` + id + `"],"dry":true}`:        200,
		`{"op":"config","ids":["` + id + `"],"dry":true}`:        200,
		`{"op":"config","ids":["1"],"params":{"pings":null}}`:    400,
		`{"op":"config","ids":["1"],"params":{"pings":"yes"}}`:   400,
	} {
		if out := post(body); out.Code != code {
			t.Fatalf("%s: got %d want %d: %s", body, out.Code, code, out.Body)
		}
	}

	es := events(post(`{"op":"unload","ids":["` + id + `","` + missing + `"]}`))
	if len(es) != 4 || es[0].T != "start" || es[0].Total != 2 || es[3].T != "done" || es[3].OK != 1 || es[3].Failed != 1 {
		t.Fatalf("unload events %+v", es)
	}
	if m.IsLoaded(id) {
		t.Fatal("still loaded")
	}
	if currentStatus() != nil {
		t.Fatal("batch still current")
	}

	es = events(post(`{"op":"config","ids":["` + id + `"],"params":{"premium":true,"reply_rate":5}}`))
	if last := es[len(es)-1]; last.OK != 1 {
		t.Fatalf("config events %+v", es)
	}
	if c, err := m.PeekConfig(id); err != nil || !c.Premium || c.ReplyRate != 5 || c.ReactionRate != 30 {
		t.Fatalf("config %+v %v", c, err)
	}

	es = events(post(`{"op":"config","ids":["` + id + `"],"params":{"markov_max_branches":1}}`))
	if last := es[len(es)-1]; last.OK != 1 {
		t.Fatalf("cap events %+v", es)
	}
	if m.IsLoaded(id) {
		t.Fatal("chain loaded only to be capped stayed loaded")
	}
}
