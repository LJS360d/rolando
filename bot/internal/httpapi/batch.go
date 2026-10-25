package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"sync"
	"time"

	"rolando/internal/cluster"
	"rolando/internal/config"
	"rolando/internal/logger"

	"github.com/disgoorg/snowflake/v2"
)

// batchWorkers is how many guilds of an op run at once on each node.
var batchWorkers = map[string]int{"load": 2, "unload": 4, "reset": 2, "leave": 2, "config": 4}

// maxLoadTargets bounds force-loads: every loaded chain sits in memory.
const maxLoadTargets = 50

type batchReq struct {
	Op     string                     `json:"op"`
	IDs    []string                   `json:"ids"`
	Query  string                     `json:"query"`
	Params map[string]json.RawMessage `json:"params"`
	Dry    bool                       `json:"dry"`
	// Job names a coordinator's batch on peers, for /node/batch/cancel
	Job string `json:"job,omitempty"`
}

// batchEvent is one NDJSON line of a batch stream: start, item, ping or done.
type batchEvent struct {
	T         string `json:"t"`
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	Error     string `json:"error,omitempty"`
	Total     int    `json:"total,omitempty"`
	OK        int    `json:"ok,omitempty"`
	Failed    int    `json:"failed,omitempty"`
	Cancelled bool   `json:"cancelled,omitempty"`
}

func itemEvent(t batchTarget, err error) batchEvent {
	e := batchEvent{T: "item", ID: t.ID, Name: t.Name}
	if err != nil {
		e.Error = err.Error()
	}
	return e
}

func (e batchEvent) err() error {
	if e.Error == "" {
		return nil
	}
	return errors.New(e.Error)
}

type batchTarget struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type batchStatus struct {
	Op      string `json:"op"`
	Total   int    `json:"total"`
	OK      int    `json:"ok"`
	Failed  int    `json:"failed"`
	Started int64  `json:"started"`
}

// batchJob is the one batch running on this (coordinating) node. Its events
// go to the client that started it while that client is connected; after
// that the job keeps going and only its counters are visible, through the
// admin listing.
type batchJob struct {
	mu     sync.Mutex
	status batchStatus
	cancel context.CancelFunc
	out    chan batchEvent
}

// send never blocks: out is sized for every event, and one past that is
// dropped rather than stalling the job while it holds mu.
func (j *batchJob) send(e batchEvent) {
	select {
	case j.out <- e:
	default:
	}
}

func (j *batchJob) item(t batchTarget, err error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if err != nil {
		j.status.Failed++
		logger.Warnf("batch %s %s: %v", j.status.Op, t.ID, err)
	} else {
		j.status.OK++
	}
	j.send(itemEvent(t, err))
}

func (j *batchJob) detach() {
	j.mu.Lock()
	j.out = nil
	j.mu.Unlock()
}

var current struct {
	sync.Mutex
	job *batchJob
}

func currentStatus() *batchStatus {
	current.Lock()
	j := current.job
	current.Unlock()
	if j == nil {
		return nil
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	st := j.status
	return &st
}

func (s *Server) batch(w http.ResponseWriter, r *http.Request) {
	if !requireOwner(w, r) {
		return
	}
	var req batchReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	fields, err := batchFields(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	targets, err := s.batchTargets(r.Context(), req)
	if err == nil && req.Op == "load" && len(targets) > maxLoadTargets {
		err = fmt.Errorf("load at most %d guilds at once", maxLoadTargets)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Dry {
		writeJSON(w, map[string]any{"targets": targets})
		return
	}

	current.Lock()
	if current.job != nil {
		current.Unlock()
		http.Error(w, "another batch is running", http.StatusConflict)
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	// buffered for every event, so the job never waits on a slow client
	job := &batchJob{cancel: cancel, out: make(chan batchEvent, len(targets)+2),
		status: batchStatus{Op: req.Op, Total: len(targets), Started: time.Now().Unix()}}
	current.job = job
	current.Unlock()
	job.send(batchEvent{T: "start", Total: len(targets)})
	logger.Infof("batch %s: %d guilds", req.Op, len(targets))
	req.Job = fmt.Sprintf("%d-%d", config.NodeIndex, time.Now().UnixNano())
	go s.runBatch(ctx, job, req, fields, targets)
	streamEvents(w, r, job.out, job.detach)
}

// streamEvents writes events as NDJSON until out closes or the client goes
// away, pinging so idle proxies (nginx, Cloudflare) keep the connection.
func streamEvents(w http.ResponseWriter, r *http.Request, out <-chan batchEvent, gone func()) {
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-cache")
	// nginx would otherwise buffer the stream until it ends
	w.Header().Set("X-Accel-Buffering", "no")
	rc := http.NewResponseController(w)
	// the server's ReadTimeout would otherwise cancel r.Context() mid-stream
	rc.SetReadDeadline(time.Time{})
	enc := json.NewEncoder(w)
	ping := time.NewTicker(15 * time.Second)
	defer ping.Stop()
	for {
		var e batchEvent
		select {
		case <-r.Context().Done():
			gone()
			return
		case <-ping.C:
			e = batchEvent{T: "ping"}
		case ev, ok := <-out:
			if !ok {
				return
			}
			e = ev
		}
		if enc.Encode(e) != nil || rc.Flush() != nil {
			gone()
			return
		}
	}
}

func (s *Server) runBatch(ctx context.Context, job *batchJob, req batchReq, fields map[string]any, targets []batchTarget) {
	byNode := map[int][]batchTarget{}
	for _, t := range targets {
		sf, err := snowflake.Parse(t.ID)
		if err != nil {
			job.item(t, errors.New("invalid guild id"))
			continue
		}
		n := cluster.OwnerNode(sf)
		byNode[n] = append(byNode[n], t)
	}
	var wg sync.WaitGroup
	for node, ts := range byNode {
		wg.Go(func() {
			if node == config.NodeIndex {
				s.batchLocal(ctx, req.Op, fields, ts, job.item)
			} else {
				batchRemote(ctx, node, req, ts, job.item)
			}
		})
	}
	wg.Wait()
	adminCache.clear()

	job.mu.Lock()
	st := job.status
	job.send(batchEvent{T: "done", OK: st.OK, Failed: st.Failed, Cancelled: ctx.Err() != nil})
	if job.out != nil {
		close(job.out)
	}
	job.mu.Unlock()
	current.Lock()
	current.job = nil
	current.Unlock()
	job.cancel()
	logger.Infof("batch %s done: %d ok, %d failed", st.Op, st.OK, st.Failed)
}

var errCancelled = errors.New("cancelled")

// batchLocal runs op on this node's guilds and reports each one; once ctx
// is cancelled it stops starting guilds and reports the rest as cancelled.
func (s *Server) batchLocal(ctx context.Context, op string, fields map[string]any, targets []batchTarget, report func(batchTarget, error)) {
	sem := make(chan struct{}, batchWorkers[op])
	var wg sync.WaitGroup
	for _, t := range targets {
		select {
		case <-ctx.Done():
		case sem <- struct{}{}:
		}
		if ctx.Err() != nil {
			report(t, errCancelled)
			continue
		}
		wg.Go(func() {
			defer func() { <-sem }()
			// a started guild finishes even if the batch is cancelled
			report(t, s.batchOne(context.WithoutCancel(ctx), op, fields, t.ID))
		})
	}
	wg.Wait()
}

func (s *Server) batchOne(ctx context.Context, op string, fields map[string]any, id string) error {
	if op == "leave" {
		return s.leaveGuild(ctx, id)
	}
	if !s.chains.Guilds.Exists(id) {
		return errors.New("no data")
	}
	switch op {
	case "load":
		return s.chains.Load(id)
	case "unload":
		return s.chains.Unload(id)
	case "reset":
		return s.chains.ResetChain(ctx, id)
	case "config":
		_, err := s.chains.UpdateChainMetaSync(ctx, id, fields)
		return err
	}
	return fmt.Errorf("unknown op %s", op)
}

// batchRemote runs the node's share through its node API; guilds it never
// reported (node down, stream cut) are reported failed here. Cancelling
// asks the node to stop instead of cutting the stream, so guilds it already
// started still report how they ended.
func batchRemote(ctx context.Context, node int, req batchReq, targets []batchTarget, report func(batchTarget, error)) {
	ids := make([]string, len(targets))
	pending := make(map[string]batchTarget, len(targets))
	for i, t := range targets {
		ids[i] = t.ID
		pending[t.ID] = t
	}
	body, _ := json.Marshal(batchReq{Op: req.Op, IDs: ids, Params: req.Params, Job: req.Job})
	streamed := make(chan struct{})
	defer close(streamed)
	go func() {
		select {
		case <-streamed:
		case <-ctx.Done():
			cancelBody, _ := json.Marshal(map[string]string{"job": req.Job})
			resp, err := cluster.Do(context.Background(), node, http.MethodPost, "/node/batch/cancel", bytes.NewReader(cancelBody))
			if err != nil {
				logger.Warnf("batch %s: cancel on node %d: %v", req.Job, node, err)
				return
			}
			cluster.CloseBody(resp)
		}
	}()
	resp, err := cluster.Stream(context.WithoutCancel(ctx), node, http.MethodPost, "/node/batch", bytes.NewReader(body))
	if err == nil {
		dec := json.NewDecoder(resp.Body)
		for {
			var e batchEvent
			if err = dec.Decode(&e); err != nil {
				break
			}
			t, ok := pending[e.ID]
			if e.T != "item" || !ok {
				continue
			}
			delete(pending, e.ID)
			report(t, e.err())
		}
		resp.Body.Close()
	}
	for _, t := range targets {
		if _, ok := pending[t.ID]; ok {
			report(t, fmt.Errorf("node %d: %v", node, err))
		}
	}
}

// peerJobs cancels the batch shares this node runs, by coordinator job.
var peerJobs = struct {
	sync.Mutex
	m map[string]context.CancelFunc
}{m: map[string]context.CancelFunc{}}

// nodeBatch runs a coordinator's batch share on this node, streaming one
// item per guild; the coordinator closing the request cancels the rest.
func (s *Server) nodeBatch(w http.ResponseWriter, r *http.Request) {
	var req batchReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	fields, err := batchFields(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	targets := make([]batchTarget, len(req.IDs))
	for i, id := range req.IDs {
		targets[i] = batchTarget{ID: id}
	}
	// a lost coordinator connection still cancels the rest
	ctx, cancel := context.WithCancel(r.Context())
	peerJobs.Lock()
	peerJobs.m[req.Job] = cancel
	peerJobs.Unlock()
	out := make(chan batchEvent, len(targets))
	go func() {
		defer func() {
			peerJobs.Lock()
			delete(peerJobs.m, req.Job)
			peerJobs.Unlock()
			cancel()
		}()
		s.batchLocal(ctx, req.Op, fields, targets, func(t batchTarget, err error) { out <- itemEvent(t, err) })
		close(out)
	}()
	streamEvents(w, r, out, func() {})
}

func nodeCancelBatch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Job string `json:"job"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	peerJobs.Lock()
	cancel, ok := peerJobs.m[req.Job]
	peerJobs.Unlock()
	if ok {
		cancel()
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) cancelBatch(w http.ResponseWriter, r *http.Request) {
	if !requireOwner(w, r) {
		return
	}
	current.Lock()
	j := current.job
	current.Unlock()
	if j == nil {
		http.Error(w, "no batch running", http.StatusNotFound)
		return
	}
	j.cancel()
	w.WriteHeader(http.StatusNoContent)
}

// batchFields validates the op and, for config, its partial edit.
func batchFields(req batchReq) (map[string]any, error) {
	if _, ok := batchWorkers[req.Op]; !ok {
		return nil, fmt.Errorf("unknown op %q", req.Op)
	}
	if req.Op != "config" || req.Dry && len(req.Params) == 0 {
		return nil, nil
	}
	if _, ok := req.Params["n_gram_size"]; ok {
		// would rebuild every targeted chain at once
		return nil, errors.New("n_gram_size can't be batch edited")
	}
	fields, err := configFields(req.Params, true)
	if err == nil && len(fields) == 0 {
		err = errors.New("no config fields to change")
	}
	return fields, err
}

// batchTargets resolves explicit ids, or every guild matching query (the
// listing's q and f params), against the cluster listing.
func (s *Server) batchTargets(ctx context.Context, req batchReq) ([]batchTarget, error) {
	all := s.allRows(ctx)
	var targets []batchTarget
	switch {
	case req.IDs != nil:
		names := make(map[string]string, len(all.rows))
		for _, g := range all.rows {
			names[g.ID] = g.Name
		}
		for _, id := range slices.Compact(slices.Sorted(slices.Values(req.IDs))) {
			targets = append(targets, batchTarget{ID: id, Name: names[id]})
		}
	case req.Query != "":
		if len(all.errs) > 0 {
			return nil, fmt.Errorf("%d node(s) unreachable: can't resolve every matching guild", len(all.errs))
		}
		q, err := url.ParseQuery(req.Query)
		if err != nil {
			return nil, err
		}
		rows, err := filterRows(slices.Clone(all.rows), q)
		if err != nil {
			return nil, err
		}
		for _, g := range rows {
			targets = append(targets, batchTarget{ID: g.ID, Name: g.Name})
		}
	}
	if len(targets) == 0 {
		return nil, errors.New("no guilds selected")
	}
	return targets, nil
}
