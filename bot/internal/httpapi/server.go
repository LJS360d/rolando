package httpapi

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"rolando/internal/analytics"
	"rolando/internal/chain"
	"rolando/internal/cluster"
	"rolando/internal/config"
	"rolando/internal/discord/services"
	"rolando/internal/guilds"
	"rolando/internal/logger"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/rest"
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
			logger.Fatalf("node api: %v", serve(config.NodeAPIAddress, s.nodeAPI()))
		}()
	}
	logger.Infof("Server listening at %v", config.ServerAddress)
	logger.Fatalf("http server: %v", serve(config.ServerAddress, s.public()))
}

// serve bounds how long a client may hold a connection without making progress.
func serve(addr string, h http.Handler) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    64 << 10,
	}
	return srv.ListenAndServe()
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
	mux.HandleFunc("POST /api/admin/batch", s.batch)
	mux.HandleFunc("POST /api/admin/batch/cancel", s.cancelBatch)
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

// requireGuild 404s guilds without data here; opening one would create it.
func (s *Server) requireGuild(w http.ResponseWriter, r *http.Request) bool {
	if !s.chains.Guilds.Exists(r.PathValue("id")) {
		http.NotFound(w, r)
		return false
	}
	return true
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
		if r.Method != http.MethodGet {
			// a leave or config change shows in the admin listing right away
			adminCache.clear()
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
	cmds := s.commands()
	out := struct {
		Name     string    `json:"name"`
		Avatar   string    `json:"avatar"`
		Invite   string    `json:"invite"`
		Support  string    `json:"support"`
		Guilds   int       `json:"guilds"`
		Commands []Command `json:"commands"`
	}{self.Username, self.EffectiveAvatarURL(), config.InviteUrl, config.SupportURL, s.TotalGuilds(), []Command{}}
	for _, c := range cmds {
		if sc, ok := c.(discord.SlashCommand); ok {
			out.Commands = append(out.Commands, Command{sc.Name(), sc.Description})
		}
	}
	writeJSON(w, out)
}

// commandsCache holds the global commands, which only change on a deploy:
// fetched until the first success, then kept for the process lifetime.
var commandsCache struct {
	sync.Mutex
	cmds []discord.ApplicationCommand
}

func (s *Server) commands() []discord.ApplicationCommand {
	commandsCache.Lock()
	defer commandsCache.Unlock()
	if commandsCache.cmds == nil {
		if cmds, err := s.client.Rest.GetGlobalCommands(s.client.ApplicationID, false); err == nil {
			commandsCache.cmds = cmds
		}
	}
	return commandsCache.cmds
}

// guildCount is the cluster guild total, refreshed every 30s since
// /api/bot is public; unreachable nodes count 0.
var guildCount = refresher[int]{every: 30 * time.Second}

func (s *Server) TotalGuilds() int { return guildCount.get(s.countGuilds) }

// FreshTotalGuilds counts now instead of serving the cached total.
func (s *Server) FreshTotalGuilds() int { return guildCount.fresh(s.countGuilds) }

func (s *Server) countGuilds(ctx context.Context) (int, error) {
	n := s.client.Caches.GuildsLen()
	counts, _ := cluster.GetJSON[int](ctx, "/node/count")
	for _, c := range counts {
		n += c
	}
	return n, nil
}

// refresher serves the last fetched value and refreshes it in the
// background once stale, so callers never wait on (or pile up behind) a
// slow fetch. Only the very first call fetches inline.
type refresher[T any] struct {
	every time.Duration

	mu     sync.Mutex
	v      T
	at     time.Time
	busy   bool
	inline sync.Mutex
}

func (r *refresher[T]) get(fetch func(context.Context) (T, error)) T {
	r.mu.Lock()
	fetched := !r.at.IsZero()
	if fetched && time.Since(r.at) > r.every && !r.busy {
		r.busy = true
		go r.refresh(fetch)
	}
	v := r.v
	r.mu.Unlock()
	if fetched {
		return v
	}
	r.inline.Lock()
	defer r.inline.Unlock()
	r.mu.Lock()
	fetched = !r.at.IsZero()
	r.mu.Unlock()
	if !fetched {
		r.refresh(fetch)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.v
}

func (r *refresher[T]) fresh(fetch func(context.Context) (T, error)) T {
	r.refresh(fetch)
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.v
}

func (r *refresher[T]) refresh(fetch func(context.Context) (T, error)) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	v, err := fetch(ctx)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.at, r.busy = time.Now(), false
	if err != nil {
		// retry in 30s rather than after a whole interval
		r.at = r.at.Add(min(30*time.Second, r.every) - r.every)
		return
	}
	r.v = v
}

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
	add := func(g discord.Guild) {
		row := GuildRow{ID: g.ID.String(), Name: g.Name, Members: g.MemberCount, State: s.chains.State(g.ID.String())}
		if g.Icon != nil {
			row.Icon = *g.IconURL()
		}
		if sum, err := s.chains.Guilds.Summary(row.ID); err == nil {
			row.Config, row.Analytics = sum.Config, analytics.FromSummary(sum)
		}
		rows = append(rows, row)
	}
	if ids == nil {
		for g := range s.client.Caches.Guilds() {
			add(g)
		}
		return rows
	}
	for _, id := range ids {
		if sf, err := snowflake.Parse(id); err == nil {
			if g, ok := s.client.Caches.Guild(sf); ok {
				add(g)
			}
		}
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
	all := s.allRows(r.Context())
	rows, err := filterRows(slices.Clone(all.rows), r.URL.Query())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
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
	for n, err := range all.errs {
		nodeErrs[n] = err.Error()
	}
	writeJSON(w, map[string]any{
		"rows": rows[start:min(start+size, len(rows))], "total": len(rows), "errors": nodeErrs,
		"sort": sortBy, "page": pg, "pages": (len(rows) + size - 1) / size,
		"started": config.StartupTime.Unix(), "loaded": s.chains.Guilds.Loaded(),
		"batch": currentStatus(),
	})
}

// allRows is every guild in the cluster, cached in adminCache.
func (s *Server) allRows(ctx context.Context) adminRows {
	if all, ok := adminCache.get(""); ok {
		return all
	}
	all := adminRows{rows: s.localGuilds(nil)}
	// shared by later requests: don't let this one's cancellation cut it short
	remote, errs := cluster.GetJSON[[]GuildRow](context.WithoutCancel(ctx), "/node/guilds")
	for _, rs := range remote {
		all.rows = append(all.rows, rs...)
	}
	all.errs = errs
	if len(errs) == 0 {
		adminCache.put("", all)
	}
	return all
}

// adminCache keeps the cluster-wide listing between page, sort and search
// requests: building it reads every guild's summary on every node.
var adminCache = ttlCache[adminRows]{ttl: 10 * time.Second, max: 1}

type adminRows struct {
	rows []GuildRow
	errs map[int]error
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
		cluster.CloseBody(resp)
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
	local := []string{}
	byNode := map[int][]string{}
	for _, id := range sess.Guilds {
		sf, err := snowflake.Parse(id)
		if err != nil {
			continue
		}
		if cluster.IsLocal(sf) {
			local = append(local, id)
		} else {
			node := cluster.OwnerNode(sf)
			byNode[node] = append(byNode[node], id)
		}
	}
	rows := s.localGuilds(local)
	paths := map[int]string{}
	for node, ids := range byNode {
		paths[node] = "/node/guilds?" + url.Values{"id": ids}.Encode()
	}
	remote, _ := cluster.GetJSONEach[[]GuildRow](r.Context(), paths)
	for _, rs := range remote {
		rows = append(rows, rs...)
	}
	slices.SortFunc(rows, func(a, b GuildRow) int { return cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)) })
	writeJSON(w, rows)
}

func (s *Server) graph(w http.ResponseWriter, r *http.Request) {
	if !requireView(w, r) || !s.requireGuild(w, r) {
		return
	}
	word := r.URL.Query().Get("word")
	var n chain.Neighborhood
	err := s.chains.WithChain(r.PathValue("id"), func(c *chain.Chain) {
		key := strconv.FormatUint(c.ID(), 10) + "/" + word
		if word != "" {
			var ok bool
			if n, ok = graphCache.get(key); ok {
				return
			}
		}
		n = c.Neighbors(word, 16)
		if word != "" && n.Word != "" {
			// unknown words return before the scan; don't let them fill the cache
			graphCache.put(key, n)
		}
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, n)
}

// graphCache holds recent neighbor lookups: each one scans every prefix of
// the chain under its read lock. Keyed by chain ID so a wipe or rebuild
// misses; random lookups (no word) are not cached.
var graphCache = ttlCache[chain.Neighborhood]{ttl: time.Minute, max: 512}

type ttlCache[V any] struct {
	mu      sync.Mutex
	ttl     time.Duration
	max     int
	entries map[string]ttlEntry[V]
}

type ttlEntry[V any] struct {
	v  V
	at time.Time
}

func (c *ttlCache[V]) get(k string) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[k]
	if !ok || time.Since(e.at) > c.ttl {
		var zero V
		return zero, false
	}
	return e.v, true
}

func (c *ttlCache[V]) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	clear(c.entries)
}

func (c *ttlCache[V]) put(k string, v V) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = map[string]ttlEntry[V]{}
	}
	if len(c.entries) >= c.max {
		for k, e := range c.entries {
			if time.Since(e.at) > c.ttl {
				delete(c.entries, k)
			}
		}
		if len(c.entries) >= c.max {
			clear(c.entries)
		}
	}
	c.entries[k] = ttlEntry[V]{v, time.Now()}
}

const messagesPageSize = 100

func (s *Server) messages(w http.ResponseWriter, r *http.Request) {
	if !requireView(w, r) || !s.requireGuild(w, r) {
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
	writeJSON(w, map[string]any{
		"messages": contents,
		"mentions": s.mentions(r.PathValue("id"), contents),
		"page":     pg,
		"pages":    (int(total) + messagesPageSize - 1) / messagesPageSize,
	})
}

var mentionRe = regexp.MustCompile(`<(@!?|@&|#)(\d+)>`)

// mentions maps each raw mention token (e.g. "<@123>") to its display text.
func (s *Server) mentions(guild string, contents []string) map[string]string {
	gid, err := snowflake.Parse(guild)
	if err != nil {
		return nil
	}
	out := map[string]string{}
	for _, c := range contents {
		for _, m := range mentionRe.FindAllStringSubmatch(c, -1) {
			if _, ok := out[m[0]]; ok {
				continue
			}
			id, _ := snowflake.Parse(m[2])
			switch m[1] {
			case "#":
				if ch, ok := s.client.Caches.Channel(id); ok {
					out[m[0]] = "#" + ch.Name()
				}
			case "@&":
				if role, ok := s.client.Caches.Role(gid, id); ok {
					out[m[0]] = "@" + role.Name
				}
			default:
				if name := s.memberName(gid, id); name != "" {
					out[m[0]] = "@" + name
				}
			}
		}
	}
	return out
}

// memberNames caches mentioned members' names: the member cache holds only
// the bot, so each would otherwise cost a REST call per page view. "" means
// the user is not a member.
var memberNames = ttlCache[string]{ttl: 10 * time.Minute, max: 4096}

func (s *Server) memberName(gid, id snowflake.ID) string {
	key := gid.String() + "/" + id.String()
	if name, ok := memberNames.get(key); ok {
		return name
	}
	var name string
	if m, err := s.client.Rest.GetMember(gid, id); err == nil {
		name = m.EffectiveName()
	} else if !rest.IsJSONErrorCode(err, rest.JSONErrorCodeUnknownMember, rest.JSONErrorCodeUnknownUser) {
		return ""
	}
	memberNames.put(key, name)
	return name
}

var configRanges = map[string][2]int{
	"reply_rate": {0, 1 << 20}, "reaction_rate": {0, 1 << 20}, "vc_join_rate": {0, 1 << 20},
	"n_gram_size": {2, 8}, "max_size_mb": {1, 1 << 14}, "markov_max_branches": {0, 1 << 16},
}

var configBools = []string{"pings", "premium"}

// configFields validates config edits; partial only takes the fields present.
func configFields(raw map[string]json.RawMessage, partial bool) (map[string]any, error) {
	fields := map[string]any{}
	for _, k := range configBools {
		v, ok := raw[k]
		if !ok {
			if !partial {
				fields[k] = false
			}
			continue
		}
		var b *bool
		if err := json.Unmarshal(v, &b); err != nil || b == nil {
			return nil, fmt.Errorf("%s must be true or false", k)
		}
		fields[k] = *b
	}
	for k, rg := range configRanges {
		if _, ok := raw[k]; partial && !ok {
			continue
		}
		v := -1
		if err := json.Unmarshal(raw[k], &v); err != nil || v < rg[0] || v > rg[1] {
			return nil, fmt.Errorf("%s must be between %d and %d", k, rg[0], rg[1])
		}
		fields[k] = v
	}
	return fields, nil
}

func (s *Server) updateConfig(w http.ResponseWriter, r *http.Request) {
	if !requireOwner(w, r) || !s.requireGuild(w, r) {
		return
	}
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	fields, err := configFields(raw, false)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
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
	if err := s.leaveGuild(r.Context(), r.PathValue("id")); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) leaveGuild(ctx context.Context, id string) error {
	sf, err := snowflake.Parse(id)
	if err != nil {
		return err
	}
	if err := s.client.Rest.LeaveGuild(sf); err != nil {
		return err
	}
	if err := s.chains.DeleteChain(ctx, id); err != nil {
		logger.Errorf("delete chain after leaving %s: %v", id, err)
	}
	return nil
}
