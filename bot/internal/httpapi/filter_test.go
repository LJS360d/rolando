package httpapi

import (
	"net/url"
	"testing"

	"rolando/internal/analytics"
	"rolando/internal/guilds"
)

func TestFilterRows(t *testing.T) {
	rows := []GuildRow{
		{ID: "1", Name: "Alpha", State: "loaded", Members: 10, Config: guilds.Config{ReplyRate: 150, Premium: true}},
		{ID: "2", Name: "Beta", State: "unloaded", Members: 500, Config: guilds.Config{ReplyRate: 10}},
		{ID: "3", Name: "Gamma", State: "fetching", Members: 50, Analytics: analytics.ChainAnalytics{Size: 3 << 20}},
	}
	for _, c := range []struct {
		query string
		want  []string
	}{
		{"", []string{"1", "2", "3"}},
		{"f=state:eq:loaded", []string{"1"}},
		{"f=state:in:loaded,fetching", []string{"1", "3"}},
		{"f=state:nin:loaded,fetching", []string{"2"}},
		{"f=config.reply_rate:gt:100", []string{"1"}},
		{"f=members:gte:50&f=members:lt:500", []string{"3"}},
		{"f=config.premium:eq:true", []string{"1"}},
		{"f=config.premium:ne:true", []string{"2", "3"}},
		{"f=analytics.bytes:gt:1048576", []string{"3"}},
		{"f=name:contains:ET", []string{"2"}},
		{"q=gamma&f=members:gt:1", []string{"3"}},
	} {
		q, _ := url.ParseQuery(c.query)
		got, err := filterRows(append([]GuildRow(nil), rows...), q)
		if err != nil {
			t.Fatalf("%s: %v", c.query, err)
		}
		ids := []string{}
		for _, g := range got {
			ids = append(ids, g.ID)
		}
		if len(ids) != len(c.want) {
			t.Fatalf("%s: got %v want %v", c.query, ids, c.want)
		}
		for i := range ids {
			if ids[i] != c.want[i] {
				t.Fatalf("%s: got %v want %v", c.query, ids, c.want)
			}
		}
	}
	for _, bad := range []string{"f=nope:eq:1", "f=members:like:1", "f=members", "f=config:eq:1"} {
		q, _ := url.ParseQuery(bad)
		if _, err := filterRows(rows, q); err == nil {
			t.Fatalf("%s: want error", bad)
		}
	}
}
