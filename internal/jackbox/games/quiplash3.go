package games

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math/rand/v2"
	"strings"
	"sync"
	"sync/atomic"

	"rolando/internal/jackbox/common"

	"github.com/gorilla/websocket"
)

func init() {
	common.RegisterFactory("quiplash3", newQuiplash3)
}

const safetyQuipVal = "\u2047"

type quiplash3 struct {
	sess            common.Session
	jackboxService  common.Jackbox
	seq             atomic.Uint64
	selfID          atomic.Int64
	pickMu          sync.Mutex
	picked          bool
	startMu         sync.Mutex
	vipStarted      bool
	textMu          sync.Mutex
	lastEntry       string
	choiceMu        sync.Mutex
	lastChoice      string
}

func newQuiplash3(ctx context.Context, s common.Session, jackboxService common.Jackbox) (common.GameModule, error) {
	_ = ctx
	return &quiplash3{
		sess:           s,
		jackboxService: jackboxService,
	}, nil
}

func (m *quiplash3) HandleWSMessage(ctx context.Context, messageType int, payload []byte) error {
	if messageType != websocket.TextMessage {
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
			if names := walkCharacters(r); len(names) > 0 {
				m.tryPickAvatar(names)
			}
		}
	case "object":
		if r, ok := top["result"].(map[string]any); ok {
			key, _ := r["key"].(string)
			if key == "room" {
				if val, ok := r["val"]; ok {
					if names := walkCharacters(val); len(names) > 0 {
						m.tryPickAvatar(names)
					}
				}
			}
			if id := int(m.selfID.Load()); id > 0 && key == fmt.Sprintf("player:%d", id) {
				if val, ok := r["val"].(map[string]any); ok {
					m.tryVIPStart(val)
					m.tryEnterText(ctx, val)
					m.tryMakeChoice(val, r)
				}
			}
			if names := walkCharacters(r); len(names) > 0 {
				m.tryPickAvatar(names)
			}
		}
	}
	return nil
}

func (m *quiplash3) tryPickAvatar(available []string) {
	m.pickMu.Lock()
	defer m.pickMu.Unlock()
	if m.picked {
		return
	}
	id := int(m.selfID.Load())
	if id <= 0 || len(available) == 0 {
		return
	}
	name := available[rand.IntN(len(available))]
	msg := map[string]any{
		"seq":    m.nextSeq(),
		"opcode": "client/send",
		"params": map[string]any{
			"from": id,
			"to":   1,
			"body": map[string]any{
				"action": "avatar",
				"name":   name,
			},
		},
	}
	if err := common.WriteTextJSON(m.sess.Conn, msg); err != nil {
		return
	}
	m.picked = true
}

func (m *quiplash3) nextSeq() uint64 {
	return m.seq.Add(1)
}

func (m *quiplash3) tryVIPStart(val map[string]any) {
	vip, _ := val["playerIsVIP"].(bool)
	can, _ := val["playerCanStartGame"].(bool)
	if !vip || !can {
		return
	}
	m.startMu.Lock()
	defer m.startMu.Unlock()
	if m.vipStarted {
		return
	}
	id := int(m.selfID.Load())
	if id <= 0 {
		return
	}
	msg := map[string]any{
		"seq":    m.nextSeq(),
		"opcode": "client/send",
		"params": map[string]any{
			"from": id,
			"to":   1,
			"body": map[string]any{
				"action": "start",
			},
		},
	}
	if err := common.WriteTextJSON(m.sess.Conn, msg); err != nil {
		return
	}
	m.vipStarted = true
}

func (m *quiplash3) tryEnterText(ctx context.Context, val map[string]any) {
	entryID, _ := val["entryId"].(string)
	if entryID == "" {
		return
	}
	state, _ := val["state"].(string)
	var textVal string
	switch state {
	case "EnterTextList":
		maxLen := common.IntFromAny(val["maxLength"], 30)
		fc := max(common.IntFromAny(val["fieldCount"], 3), 1)
		lines := make([]string, 0, fc)
		for range fc {
			lines = append(lines, m.oneLine(ctx, maxLen))
		}
		textVal = strings.Join(lines, "\n")
	default:
		if state != "EnterSingleText" && !strings.Contains(state, "EnterText") {
			return
		}
		maxLen := common.IntFromAny(val["maxLength"], 45)
		textVal = m.oneLine(ctx, maxLen)
		if rand.IntN(100) == 0 {
			textVal = safetyQuipVal
		}
	}
	m.textMu.Lock()
	defer m.textMu.Unlock()
	if m.lastEntry == entryID {
		return
	}
	id := int(m.selfID.Load())
	if id <= 0 {
		return
	}
	paramKey := fmt.Sprintf("entertext:%d", id)
	if tk, ok := val["textKey"].(string); ok && strings.TrimSpace(tk) != "" {
		paramKey = tk
	}
	msg := map[string]any{
		"seq":    m.nextSeq(),
		"opcode": "text/update",
		"params": map[string]any{
			"key": paramKey,
			"val": textVal,
		},
	}
	if err := common.WriteTextJSON(m.sess.Conn, msg); err != nil {
		return
	}
	m.lastEntry = entryID
}

func (m *quiplash3) tryMakeChoice(val map[string]any, result map[string]any) {
	state, _ := val["state"].(string)
	if state != "MakeSingleChoice" {
		return
	}
	choiceID, _ := val["choiceId"].(string)
	if choiceID == "" {
		return
	}
	keys := choiceKeysFromVal(val["choices"])
	if len(keys) == 0 {
		return
	}
	voteKey := choiceVoteKey(val, result)
	if voteKey == "" {
		return
	}
	m.choiceMu.Lock()
	defer m.choiceMu.Unlock()
	if m.lastChoice == voteKey {
		return
	}
	id := int(m.selfID.Load())
	if id <= 0 {
		return
	}
	choice := keys[rand.IntN(len(keys))]
	msg := map[string]any{
		"seq":    m.nextSeq(),
		"opcode": "client/send",
		"params": map[string]any{
			"from": id,
			"to":   1,
			"body": map[string]any{
				"action": "choose",
				"choice": choice,
			},
		},
	}
	if err := common.WriteTextJSON(m.sess.Conn, msg); err != nil {
		return
	}
	m.lastChoice = voteKey
}

func choiceVoteKey(val map[string]any, result map[string]any) string {
	cid, _ := val["choiceId"].(string)
	if cid == "" {
		return ""
	}
	if result != nil {
		if v, ok := result["version"]; ok {
			return fmt.Sprintf("%s|%v", cid, v)
		}
	}
	if p, ok := val["prompt"].(map[string]any); ok {
		if pid, ok := common.AsInt(p["id"]); ok {
			return fmt.Sprintf("%s|p%d", cid, pid)
		}
	}
	raw, _ := json.Marshal(val["choices"])
	h := fnv.New32a()
	h.Write(raw)
	return fmt.Sprintf("%s|c%x", cid, h.Sum32())
}

func choiceKeysFromVal(choices any) []int {
	list, ok := choices.([]any)
	if !ok {
		return nil
	}
	var keys []int
	for i, el := range list {
		m, ok := el.(map[string]any)
		if !ok {
			continue
		}
		if dis, ok := m["disabled"].(bool); ok && dis {
			continue
		}
		if k, ok := common.AsInt(m["key"]); ok {
			keys = append(keys, k)
			continue
		}
		keys = append(keys, i)
	}
	return keys
}

func (m *quiplash3) oneLine(ctx context.Context, maxLen int) string {
	if maxLen <= 0 {
		maxLen = 45
	}
	guildID := m.sess.GuildID
	if m.jackboxService == nil || guildID == "" {
		return ""
	}
	maxWords := 1 + rand.IntN(8)
	t, err := m.jackboxService.GenerateLine(ctx, guildID, maxWords)
	if err != nil {
		return ""
	}
	return common.ClipRunes(t, maxLen)
}

func (m *quiplash3) Close() error {
	return nil
}

func walkCharacters(v any) []string {
	switch t := v.(type) {
	case map[string]any:
		if arr, ok := t["characters"]; ok {
			if names := parseCharacterArray(arr); len(names) > 0 {
				return names
			}
		}
		for _, vv := range t {
			if names := walkCharacters(vv); len(names) > 0 {
				return names
			}
		}
	case []any:
		for _, el := range t {
			if names := walkCharacters(el); len(names) > 0 {
				return names
			}
		}
	}
	return nil
}

func parseCharacterArray(arr any) []string {
	list, ok := arr.([]any)
	if !ok {
		return nil
	}
	var names []string
	for _, el := range list {
		m, ok := el.(map[string]any)
		if !ok {
			continue
		}
		avail, _ := m["available"].(bool)
		if !avail {
			continue
		}
		name, _ := m["name"].(string)
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}
