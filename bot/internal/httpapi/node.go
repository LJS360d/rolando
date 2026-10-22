package httpapi

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"

	"rolando/internal/config"
)

// nodeAPI is the internal API peers call. It is never exposed through the
// load balancer and every request must carry NODE_SECRET.
func (s *Server) nodeAPI() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /node/count", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(s.client.Caches.GuildsLen())
	})
	mux.HandleFunc("GET /node/guilds", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(s.localGuilds(r.URL.Query()["id"]))
	})
	mux.HandleFunc("POST /node/broadcast", func(w http.ResponseWriter, r *http.Request) {
		var req broadcastReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		json.NewEncoder(w).Encode(s.broadcastLocal(req.Content, req.Guilds))
	})
	mux.Handle("/api/guilds/{id}/", s.guildRoutes())
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Node-Secret")), []byte(config.NodeSecret)) != 1 {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		mux.ServeHTTP(w, r)
	})
}
