package games

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"strings"
	"sync"
	"sync/atomic"

	"rolando/internal/jackbox/common"

	"github.com/gorilla/websocket"
)

func init() {
	common.RegisterFactory("apply-yourself", newJobjob)
}

var jobjobAvatars = []string{
	"heart", "duck", "owl", "shirt", "dode", "poop", "bomb", "skull",
	"faker", "jack", "robot", "wampus", "screw", "moon",
}

type jobjob struct {
	sess           common.Session
	jackboxService common.Jackbox
	seq            atomic.Uint64
	selfID         atomic.Int64
	avatarMu       sync.Mutex
	avatarPicked   bool
	writeMu        sync.Mutex
	lastWriteKey   string
	compMu         sync.Mutex
	lastCompKey    string
	voteMu         sync.Mutex
	lastVoteKey    string
	skipMu         sync.Mutex
	logoSkipped    bool
	lastStash      [][]string
}

func newJobjob(ctx context.Context, s common.Session, jackboxService common.Jackbox) (common.GameModule, error) {
	_ = ctx
	return &jobjob{
		sess:           s,
		jackboxService: jackboxService,
	}, nil
}

func (m *jobjob) nextSeq() uint64 {
	return m.seq.Add(1)
}

func (m *jobjob) HandleWSMessage(ctx context.Context, messageType int, payload []byte) error {
	if messageType != websocket.TextMessage {
		return nil
	}
	if jobjobFastIgnore(payload) {
		return nil
	}
	var top map[string]any
	if err := json.Unmarshal(payload, &top); err != nil {
		return nil
	}
	op, _ := top["opcode"].(string)
	switch op {
	case "client/welcome":
		if r, ok := top["result"].(map[string]any); ok {
			if id, ok := common.AsInt(r["id"]); ok && id > 0 {
				m.selfID.Store(int64(id))
			}
		}
	case "object":
		r, ok := top["result"].(map[string]any)
		if !ok {
			return nil
		}
		key, _ := r["key"].(string)
		id := int(m.selfID.Load())
		if id <= 0 {
			return nil
		}
		if !jobjobRelevantObjectKey(key, id) {
			return nil
		}
		if val, ok := r["val"].(map[string]any); ok {
			m.tryPickAvatar(val)
			m.trySkipLogo(val)
			m.tryWriting(ctx, val, r)
			m.tryMagnets(val, r)
			m.tryVoting(val, r)
		}
	}
	return nil
}

func jobjobFastIgnore(payload []byte) bool {
	if len(payload) < 12 {
		return false
	}
	return bytes.Contains(payload, []byte("timerLeft"))
}

func jobjobRelevantObjectKey(key string, selfID int) bool {
	prefix := fmt.Sprintf("player:%d", selfID)
	return key == prefix
}

func (m *jobjob) tryPickAvatar(val map[string]any) {
	kind, _ := val["kind"].(string)
	if kind != "lobby" {
		return
	}
	m.avatarMu.Lock()
	defer m.avatarMu.Unlock()
	if m.avatarPicked {
		return
	}
	id := int(m.selfID.Load())
	if id <= 0 {
		return
	}
	rk, _ := val["responseKey"].(string)
	if rk == "" {
		return
	}
	choice := jobjobAvatars[rand.IntN(len(jobjobAvatars))]
	msg := map[string]any{
		"seq":    m.nextSeq(),
		"opcode": "object/update",
		"params": map[string]any{
			"key": rk,
			"val": map[string]any{
				"action": "choose-avatar",
				"choice": choice,
			},
		},
	}
	if err := common.WriteTextJSON(m.sess.Conn, msg); err != nil {
		return
	}
	m.avatarPicked = true
}

func (m *jobjob) trySkipLogo(val map[string]any) {
	kind, _ := val["kind"].(string)
	if kind != "Logo" {
		return
	}
	rk, _ := val["responseKey"].(string)
	if rk == "" {
		return
	}
	m.skipMu.Lock()
	defer m.skipMu.Unlock()
	if m.logoSkipped {
		return
	}
	msg := map[string]any{
		"seq":    m.nextSeq(),
		"opcode": "object/update",
		"params": map[string]any{
			"key": rk,
			"val": map[string]any{
				"action": "skip",
			},
		},
	}
	if err := common.WriteTextJSON(m.sess.Conn, msg); err != nil {
		return
	}
	m.logoSkipped = true
}

func (m *jobjob) tryWriting(ctx context.Context, val map[string]any, result map[string]any) {
	kind, _ := val["kind"].(string)
	if kind != "writing" {
		return
	}
	entryID, _ := val["entryId"].(string)
	if entryID == "" {
		return
	}
	prog, _ := val["progress"].(map[string]any)
	at := common.IntFromAny(prog["at"], 0)
	of := common.IntFromAny(prog["of"], 0)
	ver := versionFromResult(result)
	dedupe := fmt.Sprintf("%s|%d|%d|%d", entryID, at, of, ver)
	m.writeMu.Lock()
	defer m.writeMu.Unlock()
	if m.lastWriteKey == dedupe {
		return
	}
	id := int(m.selfID.Load())
	if id <= 0 {
		return
	}
	textKey, _ := val["textKey"].(string)
	if textKey == "" {
		textKey = fmt.Sprintf("entertext:%d", id)
	}
	doneKey, _ := val["doneKey"].(string)
	if doneKey == "" {
		doneKey = fmt.Sprintf("done:%d", id)
	}
	maxLen := common.IntFromAny(val["maxLength"], 128)
	minWords := common.IntFromAny(val["minWords"], 5)
	text := m.genWriting(ctx, maxLen, minWords)
	if err := common.WriteTextJSON(m.sess.Conn, map[string]any{
		"seq":    m.nextSeq(),
		"opcode": "text/update",
		"params": map[string]any{"key": textKey, "val": text},
	}); err != nil {
		return
	}
	if err := common.WriteTextJSON(m.sess.Conn, map[string]any{
		"seq":    m.nextSeq(),
		"opcode": "object/update",
		"params": map[string]any{
			"key": doneKey,
			"val": map[string]any{"done": true},
		},
	}); err != nil {
		return
	}
	m.lastWriteKey = dedupe
}

func (m *jobjob) genWriting(ctx context.Context, maxLen, minWords int) string {
	if maxLen <= 0 {
		maxLen = 128
	}
	if minWords <= 0 {
		minWords = 5
	}
	var base string
	if m.jackboxService != nil && m.sess.GuildID != "" {
		if t, err := m.jackboxService.GenerateLine(ctx, m.sess.GuildID, minWords+rand.IntN(12)+4); err == nil {
			base = strings.TrimSpace(t)
		}
	}
	if strings.TrimSpace(base) == "" {
		base = "one two three four five six seven eight nine ten eleven twelve"
	}
	for wordCount(base) < minWords {
		base += " fillerword"
	}
	base = clipToWordsMin(common.ClipRunes(strings.TrimSpace(base), maxLen), maxLen, minWords)
	return base
}

func wordCount(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	return len(strings.Fields(s))
}

func clipToWordsMin(s string, maxRunes, minWords int) string {
	const pad = "word"
	for i := 0; i < 64 && wordCount(s) < minWords; i++ {
		s = strings.TrimSpace(s) + " " + pad
		s = common.ClipRunes(s, maxRunes)
	}
	return s
}

func (m *jobjob) tryMagnets(val map[string]any, result map[string]any) {
	kind, _ := val["kind"].(string)
	isResu := kind == "resumagnets"
	isMag := kind == "magnets"
	if !isMag && !isResu {
		ak, _ := val["answerKey"].(string)
		if ak == "" || !strings.HasPrefix(ak, "composition:") {
			return
		}
		if _, has := val["stash"]; !has && len(m.lastStash) == 0 {
			return
		}
		isMag = true
	}
	stash := parseStash(val["stash"])
	if len(stash) > 0 {
		m.lastStash = stash
	} else if len(m.lastStash) > 0 {
		stash = m.lastStash
	}
	if len(stash) == 0 {
		return
	}
	answerKey, _ := val["answerKey"].(string)
	if answerKey == "" {
		return
	}
	maxWords := common.IntFromAny(val["maxWords"], 12)
	entryID, _ := val["entryId"].(string)
	ver := versionFromResult(result)
	dedupe := fmt.Sprintf("%s|%s|%s|%d", val["kind"], answerKey, entryID, ver)
	m.compMu.Lock()
	defer m.compMu.Unlock()
	if m.lastCompKey == dedupe {
		return
	}
	var text any
	if isResu {
		per := common.IntFromAny(val["maxWordsPerAnswer"], 8)
		nAns := max(common.IntFromAny(val["numAnswers"], 2), 2)
		answers := make([]any, 0, nAns)
		for range nAns {
			answers = append(answers, randomWordPicks1Based(stash, per))
		}
		text = answers
	} else {
		text = randomWordPicks1Based(stash, maxWords)
	}
	msg := map[string]any{
		"seq":    m.nextSeq(),
		"opcode": "object/update",
		"params": map[string]any{
			"key": answerKey,
			"val": map[string]any{
				"text":  text,
				"final": true,
			},
		},
	}
	if err := common.WriteTextJSON(m.sess.Conn, msg); err != nil {
		return
	}
	m.lastCompKey = dedupe
}

func versionFromResult(result map[string]any) int {
	if result == nil {
		return 0
	}
	return common.IntFromAny(result["version"], 0)
}

func parseStash(v any) [][]string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	var out [][]string
	for _, row := range arr {
		words, ok := row.([]any)
		if !ok {
			continue
		}
		var line []string
		for _, w := range words {
			s, _ := w.(string)
			line = append(line, s)
		}
		if len(line) > 0 {
			out = append(out, line)
		}
	}
	return out
}

func randomWordPicks1Based(stash [][]string, maxWords int) []any {
	if maxWords <= 0 {
		maxWords = 12
	}
	if len(stash) == 0 {
		return nil
	}
	picks := make([]any, 0, maxWords)
	seen := make(map[string]struct{})
	for len(picks) < maxWords {
		ri := rand.IntN(len(stash))
		row := stash[ri]
		if len(row) == 0 {
			continue
		}
		wi := rand.IntN(len(row))
		key := fmt.Sprintf("%d:%d", ri, wi)
		if _, ok := seen[key]; ok {
			if len(seen) >= countStashTokens(stash) {
				break
			}
			continue
		}
		seen[key] = struct{}{}
		picks = append(picks, map[string]any{
			"index": ri,
			"word":  wi + 1,
		})
	}
	return picks
}

func countStashTokens(stash [][]string) int {
	n := 0
	for _, row := range stash {
		n += len(row)
	}
	return n
}

func (m *jobjob) tryVoting(val map[string]any, result map[string]any) {
	kind, _ := val["kind"].(string)
	if kind != "voting" {
		return
	}
	rk, _ := val["responseKey"].(string)
	if rk == "" {
		rk = fmt.Sprintf("choose:%d", int(m.selfID.Load()))
	}
	choices, ok := val["choices"].([]any)
	if !ok || len(choices) == 0 {
		return
	}
	ver := versionFromResult(result)
	dedupe := fmt.Sprintf("%s|%d", rk, ver)
	m.voteMu.Lock()
	defer m.voteMu.Unlock()
	if m.lastVoteKey == dedupe {
		return
	}
	choice := rand.IntN(len(choices))
	msg := map[string]any{
		"seq":    m.nextSeq(),
		"opcode": "object/update",
		"params": map[string]any{
			"key": rk,
			"val": map[string]any{
				"action": "choose",
				"choice": choice,
			},
		},
	}
	if err := common.WriteTextJSON(m.sess.Conn, msg); err != nil {
		return
	}
	m.lastVoteKey = dedupe
}

func (m *jobjob) Close() error {
	return nil
}
