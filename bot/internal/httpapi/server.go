package httpapi

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"rolando/internal/analytics"
	"rolando/internal/cluster"
	"rolando/internal/config"
	"rolando/internal/discord/services"
	"rolando/internal/guilds"
	"rolando/internal/logger"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
)

// Server is the JSON API behind the standalone web UI (web/), which proxies /api here.
type Server struct {
	client *bot.Client
	chains *services.ChainsService
}

func NewServer(client *bot.Client, chains *services.ChainsService) *Server {
	return &Server{client: client, chains: chains}
}

func (s *Server) Start() {
	if config.NodeCount > 1 {
		if config.SessionSecret == "" {
			logger.Fatalf("SESSION_SECRET is required when NODE_COUNT > 1")
		}
		go func() {
			logger.Infof("Node API listening at %v", config.NodeAPIAddress)
			logger.Fatalf("node api: %v", http.ListenAndServe(config.NodeAPIAddress, s.nodeAPI()))
		}()
	}
	logger.Infof("Server listening at %v", config.ServerAddress)
	logger.Fatalf("http server: %v", http.ListenAndServe(config.ServerAddress, s.public()))
}

func (s *Server) public() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/bot", s.bot)
	mux.HandleFunc("GET /api/session", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, getSession(r)) })
	mux.HandleFunc("GET /api/auth/login", s.login)
	mux.HandleFunc("GET /api/auth/callback", s.callback)
	mux.HandleFunc("POST /api/auth/logout", s.logout)
	mux.HandleFunc("GET /api/me/guilds", s.myGuilds)
	mux.HandleFunc("GET /api/admin/guilds", s.admin)
	mux.HandleFunc("POST /api/admin/broadcast", s.broadcast)
	mux.Handle("/api/guilds/{id}/", s.routeGuild(s.guildRoutes()))
	return jsonOnly(mux)
}

// jsonOnly rejects state-changing requests without a JSON body type, so a
// cross-site form can't trigger them (it would need a CORS preflight).
func jsonOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, "content-type must be application/json", http.StatusUnsupportedMediaType)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) guildRoutes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/guilds/{id}/summary", s.guild)
	mux.HandleFunc("GET /api/guilds/{id}/messages", s.messages)
	mux.HandleFunc("GET /api/guilds/{id}/graph", s.graph)
	mux.HandleFunc("PUT /api/guilds/{id}/config", s.updateConfig)
	mux.HandleFunc("POST /api/guilds/{id}/invite", s.invite)
	mux.HandleFunc("POST /api/guilds/{id}/leave", s.leave)
	return mux
}

// routeGuild serves guilds owned by this node and proxies the rest to the
// owner's node API; the session cookie travels along and is checked there.
func (s *Server) routeGuild(local http.Handler) http.Handler {
	var proxies sync.Map
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := snowflake.Parse(r.PathValue("id"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if cluster.IsLocal(id) {
			local.ServeHTTP(w, r)
			return
		}
		node := cluster.OwnerNode(id)
		p, ok := proxies.Load(node)
		if !ok {
			target, _ := url.Parse(cluster.PeerURL(node))
			rp := httputil.NewSingleHostReverseProxy(target)
			rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
				http.Error(w, fmt.Sprintf("node %d unreachable", node), http.StatusBadGateway)
			}
			p, _ = proxies.LoadOrStore(node, rp)
		}
		r.Header.Set("X-Node-Secret", config.NodeSecret)
		p.(*httputil.ReverseProxy).ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		logger.Errorf("write json: %v", err)
	}
}

type Command struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (s *Server) bot(w http.ResponseWriter, r *http.Request) {
	self, _ := s.client.Caches.SelfUser()
	cmds, _ := s.commands()
	out := struct {
		Name     string    `json:"name"`
		Avatar   string    `json:"avatar"`
		Invite   string    `json:"invite"`
		Support  string    `json:"support"`
		Guilds   int       `json:"guilds"`
		Commands []Command `json:"commands"`
	}{self.Username, self.EffectiveAvatarURL(), config.InviteUrl, config.SupportURL, s.totalGuilds(r.Context()), []Command{}}
	for _, c := range cmds {
		if sc, ok := c.(discord.SlashCommand); ok {
			out.Commands = append(out.Commands, Command{sc.Name(), sc.Description})
		}
	}
	writeJSON(w, out)
}

var commandsCache struct {
	sync.Mutex
	cmds []discord.ApplicationCommand
	at   time.Time
}

func (s *Server) commands() ([]discord.ApplicationCommand, error) {
	commandsCache.Lock()
	defer commandsCache.Unlock()
	if time.Since(commandsCache.at) > time.Hour {
		cmds, err := s.client.Rest.GetGlobalCommands(s.client.ApplicationID, false)
		if err != nil {
			return commandsCache.cmds, err
		}
		commandsCache.cmds, commandsCache.at = cmds, time.Now()
	}
	return commandsCache.cmds, nil
}

var guildCount struct {
	sync.Mutex
	n  int
	at time.Time
}

// TotalGuilds sums guild counts across the cluster, cached for 30s since
// /api/bot is public; unreachable nodes count 0.
func (s *Server) totalGuilds(ctx context.Context) int {
	guildCount.Lock()
	defer guildCount.Unlock()
	if time.Since(guildCount.at) > 30*time.Second {
		n := s.client.Caches.GuildsLen()
		counts, _ := cluster.GetJSON[int](ctx, "/node/count")
		for _, c := range counts {
			n += c
		}
		guildCount.n, guildCount.at = n, time.Now()
	}
	return guildCount.n
}

func (s *Server) TotalGuilds() int { return s.totalGuilds(context.Background()) }

type GuildRow struct {
	ID        string                   `json:"id"`
	Name      string                   `json:"name"`
	Icon      string                   `json:"icon"`
	Members   int                      `json:"members"`
	State     string                   `json:"state"`
	Config    guilds.Config            `json:"config"`
	Analytics analytics.ChainAnalytics `json:"analytics"`
}

// localGuilds lists this node's guilds, only those in ids when given.
func (s *Server) localGuilds(ids []string) []GuildRow {
	rows := []GuildRow{}
	for g := range s.client.Caches.Guilds() {
		if ids != nil && !slices.Contains(ids, g.ID.String()) {
			continue
		}
		row := GuildRow{ID: g.ID.String(), Name: g.Name, Members: g.MemberCount, State: s.chains.State(g.ID.String())}
		if g.Icon != nil {
			row.Icon = *g.IconURL()
		}
		if sum, err := s.chains.Guilds.Summary(row.ID); err == nil {
			row.Config, row.Analytics = sum.Config, analytics.FromSummary(sum)
		}
		rows = append(rows, row)
	}
	return rows
}

var sortKeys = map[string]func(GuildRow) float64{
	"bytes":            func(g GuildRow) float64 { return float64(g.Analytics.Size) },
	"messages":         func(g GuildRow) float64 { return float64(g.Analytics.Messages) },
	"words":            func(g GuildRow) float64 { return float64(g.Analytics.Words) },
	"complexity_score": func(g GuildRow) float64 { return float64(g.Analytics.ComplexityScore) },
	"gifs":             func(g GuildRow) float64 { return float64(g.Analytics.Gifs) },
	"images":           func(g GuildRow) float64 { return float64(g.Analytics.Images) },
	"videos":           func(g GuildRow) float64 { return float64(g.Analytics.Videos) },
	"reply_rate":       func(g GuildRow) float64 { return float64(g.Config.ReplyRate) },
	"members":          func(g GuildRow) float64 { return float64(g.Members) },
}

const adminPageSize = 50

func (s *Server) admin(w http.ResponseWriter, r *http.Request) {
	if !requireOwner(w, r) {
		return
	}
	rows := s.localGuilds(nil)
	remote, errs := cluster.GetJSON[[]GuildRow](r.Context(), "/node/guilds")
	for _, rs := range remote {
		rows = append(rows, rs...)
	}
	if q := strings.ToLower(r.URL.Query().Get("q")); q != "" {
		rows = slices.DeleteFunc(rows, func(g GuildRow) bool { return !strings.Contains(strings.ToLower(g.Name), q) && g.ID != q })
	}
	size := adminPageSize
	if r.URL.Query().Has("all") {
		size = max(len(rows), 1)
	}
	sortBy := r.URL.Query().Get("sort")
	key, ok := sortKeys[sortBy]
	if !ok {
		sortBy, key = "bytes", sortKeys["bytes"]
	}
	asc := r.URL.Query().Get("dir") == "asc"
	slices.SortFunc(rows, func(a, b GuildRow) int {
		if asc {
			return cmp.Compare(key(a), key(b))
		}
		return cmp.Compare(key(b), key(a))
	})
	pg := atoiOr(r.URL.Query().Get("page"), 1)
	start := min((pg-1)*size, len(rows))
	nodeErrs := map[int]string{}
	for n, err := range errs {
		nodeErrs[n] = err.Error()
	}
	writeJSON(w, map[string]any{
		"rows": rows[start:min(start+size, len(rows))], "total": len(rows), "errors": nodeErrs,
		"sort": sortBy, "page": pg, "pages": (len(rows) + size - 1) / size,
		"started": config.StartupTime.Unix(), "loaded": s.chains.Guilds.Loaded(),
	})
}

type broadcastReq struct {
	Content string   `json:"content"`
	Guilds  []string `json:"guilds"`
}

func (s *Server) broadcast(w http.ResponseWriter, r *http.Request) {
	if !requireOwner(w, r) {
		return
	}
	var req broadcastReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	body, _ := json.Marshal(req)
	sent := s.broadcastLocal(req.Content, req.Guilds)
	failed := []int{}
	for node := range config.NodeCount {
		if node == config.NodeIndex {
			continue
		}
		resp, err := cluster.Do(r.Context(), node, http.MethodPost, "/node/broadcast", strings.NewReader(string(body)))
		if err != nil {
			failed = append(failed, node)
			continue
		}
		var n int
		json.NewDecoder(resp.Body).Decode(&n)
		resp.Body.Close()
		sent += n
	}
	writeJSON(w, map[string]any{"sent": sent, "failed": failed})
}

// broadcastLocal posts content to the system channel of the given local
// guilds, or of all local guilds when ids is empty.
func (s *Server) broadcastLocal(content string, ids []string) int {
	if strings.TrimSpace(content) == "" {
		return 0
	}
	sent := 0
	for g := range s.client.Caches.Guilds() {
		if g.SystemChannelID == nil || (len(ids) > 0 && !slices.Contains(ids, g.ID.String())) {
			continue
		}
		if _, err := s.client.Rest.CreateMessage(*g.SystemChannelID, discord.NewMessageCreate().WithContent(content)); err != nil {
			logger.Warnf("broadcast %s: %v", g.ID, err)
			continue
		}
		sent++
	}
	return sent
}

func requireOwner(w http.ResponseWriter, r *http.Request) bool {
	sess := getSession(r)
	if sess == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return false
	}
	if !sess.Owner {
		http.Error(w, "forbidden", http.StatusForbidden)
		return false
	}
	return true
}

func requireView(w http.ResponseWriter, r *http.Request) bool {
	sess := getSession(r)
	if sess == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return false
	}
	if !sess.CanView(r.PathValue("id")) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return false
	}
	return true
}

func (s *Server) guild(w http.ResponseWriter, r *http.Request) {
	if !requireView(w, r) {
		return
	}
	id := r.PathValue("id")
	g, ok := s.client.Caches.Guild(snowflake.MustParse(id))
	if !ok || !s.chains.Guilds.Exists(id) {
		http.NotFound(w, r)
		return
	}
	sum, err := s.chains.Summary(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	row := GuildRow{ID: id, Name: g.Name, Members: g.MemberCount, State: s.chains.State(id),
		Config: sum.Config, Analytics: analytics.FromSummary(sum)}
	if g.Icon != nil {
		row.Icon = *g.IconURL()
	}
	writeJSON(w, row)
}

// myGuilds lists the session's manageable guilds the bot is in, across the cluster.
func (s *Server) myGuilds(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)
	if sess == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if len(sess.Guilds) == 0 {
		writeJSON(w, []GuildRow{})
		return
	}
	rows := s.localGuilds(sess.Guilds)
	remote, _ := cluster.GetJSON[[]GuildRow](r.Context(), "/node/guilds?"+url.Values{"id": sess.Guilds}.Encode())
	for _, rs := range remote {
		rows = append(rows, rs...)
	}
	slices.SortFunc(rows, func(a, b GuildRow) int { return cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)) })
	writeJSON(w, rows)
}

func (s *Server) graph(w http.ResponseWriter, r *http.Request) {
	if !requireView(w, r) {
		return
	}
	n, err := s.chains.Neighbors(r.PathValue("id"), r.URL.Query().Get("word"), 16)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, n)
}

const messagesPageSize = 100

func (s *Server) messages(w http.ResponseWriter, r *http.Request) {
	if !requireView(w, r) {
		return
	}
	pg := atoiOr(r.URL.Query().Get("page"), 1)
	msgs, total, err := s.chains.MessagesPage(r.PathValue("id"), messagesPageSize, (pg-1)*messagesPageSize)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	contents := make([]string, len(msgs))
	for i, m := range msgs {
		contents[i] = m.Content
	}
	writeJSON(w, map[string]any{"messages": contents, "page": pg, "pages": (int(total) + messagesPageSize - 1) / messagesPageSize})
}

var configRanges = map[string][2]int{
	"reply_rate": {0, 1 << 20}, "reaction_rate": {0, 1 << 20}, "vc_join_rate": {0, 1 << 20},
	"n_gram_size": {2, 8}, "max_size_mb": {1, 1 << 14}, "markov_max_branches": {0, 1 << 16},
}

func (s *Server) updateConfig(w http.ResponseWriter, r *http.Request) {
	if !requireOwner(w, r) {
		return
	}
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	fields := map[string]any{}
	for _, k := range []string{"pings", "premium"} {
		var b bool
		json.Unmarshal(raw[k], &b)
		fields[k] = b
	}
	for k, rg := range configRanges {
		v := -1
		if err := json.Unmarshal(raw[k], &v); err != nil || v < rg[0] || v > rg[1] {
			http.Error(w, fmt.Sprintf("%s must be between %d and %d", k, rg[0], rg[1]), http.StatusBadRequest)
			return
		}
		fields[k] = v
	}
	cfg, err := s.chains.UpdateChainMeta(r.Context(), r.PathValue("id"), fields)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, cfg)
}

func (s *Server) invite(w http.ResponseWriter, r *http.Request) {
	if !requireOwner(w, r) {
		return
	}
	for ch := range s.client.Caches.ChannelsForGuild(snowflake.MustParse(r.PathValue("id"))) {
		if ch.Type() != discord.ChannelTypeGuildText {
			continue
		}
		inv, err := s.client.Rest.CreateInvite(ch.ID(), discord.InviteCreate{MaxAge: new(86400), MaxUses: new(1)})
		if err != nil {
			continue
		}
		writeJSON(w, map[string]string{"url": "https://discord.gg/" + inv.Code})
		return
	}
	http.Error(w, "no channel to create an invite in", http.StatusBadRequest)
}

func (s *Server) leave(w http.ResponseWriter, r *http.Request) {
	if !requireOwner(w, r) {
		return
	}
	id := snowflake.MustParse(r.PathValue("id"))
	if err := s.client.Rest.LeaveGuild(id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.chains.DeleteChain(r.Context(), id.String()); err != nil {
		logger.Errorf("delete chain after leaving %s: %v", id, err)
	}
	w.WriteHeader(http.StatusNoContent)
}
