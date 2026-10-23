package chain

import (
	"encoding/binary"
	"math/rand/v2"
	"slices"
	"strings"
	"sync"
)

type edge struct{ tok, cnt uint32 }

type state struct {
	edges []edge
	idx   int
}

type Stats struct {
	Prefixes int64  `json:"prefixes"`
	Messages int64  `json:"messages"`
	Bytes    uint64 `json:"bytes"`
}

// Chain is one guild Markov chain. Tokens are interned; a prefix key is the
// n-1 token ids packed as little-endian uint32 bytes
type Chain struct {
	mu        sync.RWMutex
	n         int
	tokens    []string
	ids       map[string]uint32
	next      map[string]*state
	keys      []string
	messages  int64
	bytes     uint64
	watermark int64
	dirty     bool
}

const (
	tokenOverhead  = 48
	prefixOverhead = 96
	edgeSize       = 8
)

func New(n int) *Chain {
	return &Chain{n: max(n, 2), ids: map[string]uint32{}, next: map[string]*state{}}
}

func (c *Chain) N() int { return c.n }

func (c *Chain) Stats() Stats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return Stats{Prefixes: int64(len(c.next)), Messages: c.messages, Bytes: c.bytes}
}

func (c *Chain) Watermark() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.watermark
}

func (c *Chain) Dirty() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.dirty
}

// Train adds messages and advances the watermark to lastID. Messages are
// skipped once the chain reaches maxBytes (0 = unlimited). maxBranches caps
// distinct successors per prefix by dropping the lowest counts (0 = unlimited)
func (c *Chain) Train(texts []string, lastID int64, maxBytes uint64, maxBranches int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, t := range texts {
		if maxBytes > 0 && c.bytes >= maxBytes {
			break
		}
		c.trainOne(t, maxBranches)
	}
	if lastID > c.watermark {
		c.watermark = lastID
	}
	c.dirty = true
}

func (c *Chain) trainOne(text string, maxBranches int) {
	toks := strings.Fields(text)
	if len(toks) < c.n {
		return
	}
	ids := make([]uint32, len(toks))
	for i, t := range toks {
		ids[i] = c.intern(t)
	}
	for i := 0; i+c.n <= len(ids); i++ {
		c.add(packKey(ids[i:i+c.n-1]), ids[i+c.n-1], maxBranches)
	}
	c.messages++
}

func (c *Chain) intern(t string) uint32 {
	if id, ok := c.ids[t]; ok {
		return id
	}
	id := uint32(len(c.tokens))
	c.tokens = append(c.tokens, t)
	c.ids[t] = id
	c.bytes += uint64(len(t) + tokenOverhead)
	return id
}

func (c *Chain) add(key string, tok uint32, maxBranches int) {
	s, ok := c.next[key]
	if !ok {
		s = &state{idx: len(c.keys)}
		c.next[key] = s
		c.keys = append(c.keys, key)
		c.bytes += uint64(len(key) + prefixOverhead)
	}
	for i := range s.edges {
		if s.edges[i].tok == tok {
			s.edges[i].cnt++
			return
		}
	}
	s.edges = append(s.edges, edge{tok, 1})
	c.bytes += edgeSize
	if maxBranches > 0 && len(s.edges) > maxBranches {
		c.bytes -= uint64(edgeSize * c.prune(s, maxBranches))
	}
}

func (c *Chain) prune(s *state, maxBranches int) int {
	if len(s.edges) <= maxBranches {
		return 0
	}
	slices.SortStableFunc(s.edges, func(a, b edge) int { return int(b.cnt) - int(a.cnt) })
	drop := len(s.edges) - maxBranches
	s.edges = s.edges[:maxBranches]
	return drop
}

// Cap applies maxBranches to every prefix and returns the removed transitions.
func (c *Chain) Cap(maxBranches int) int {
	if maxBranches <= 0 {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	removed := 0
	for _, s := range c.next {
		removed += c.prune(s, maxBranches)
	}
	if removed > 0 {
		c.bytes -= uint64(edgeSize * removed)
		c.dirty = true
	}
	return removed
}

// Delete removes one occurrence of each transition of text.
func (c *Chain) Delete(text string) {
	toks := strings.Fields(text)
	if len(toks) < c.n {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	ids := make([]uint32, len(toks))
	for i, t := range toks {
		id, ok := c.ids[t]
		if !ok {
			return
		}
		ids[i] = id
	}
	for i := 0; i+c.n <= len(ids); i++ {
		c.remove(packKey(ids[i:i+c.n-1]), ids[i+c.n-1])
	}
	c.messages = max(0, c.messages-1)
	c.dirty = true
}

func (c *Chain) remove(key string, tok uint32) {
	s, ok := c.next[key]
	if !ok {
		return
	}
	i := slices.IndexFunc(s.edges, func(e edge) bool { return e.tok == tok })
	if i < 0 {
		return
	}
	if s.edges[i].cnt > 1 {
		s.edges[i].cnt--
		return
	}
	s.edges = slices.Delete(s.edges, i, i+1)
	c.bytes -= edgeSize
	if len(s.edges) > 0 {
		return
	}
	last := c.keys[len(c.keys)-1]
	c.keys[s.idx] = last
	c.next[last].idx = s.idx
	c.keys = c.keys[:len(c.keys)-1]
	delete(c.next, key)
	c.bytes -= uint64(len(key) + prefixOverhead)
}

// Generate walks the chain from a random prefix.
func (c *Chain) Generate(maxLength int) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(c.keys) == 0 {
		return ""
	}
	toks, _ := c.walk(c.keys[rand.IntN(len(c.keys))], maxLength, "")
	return c.join(toks)
}

// GenerateFromSeed starts from a prefix equal to seed, else a random prefix
// containing seed, else a random prefix.
func (c *Chain) GenerateFromSeed(seed string, maxLength int) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	key := c.findPrefix(seed)
	if key == "" {
		return ""
	}
	toks, _ := c.walk(key, maxLength, "")
	return c.join(toks)
}

func (c *Chain) findPrefix(seed string) string {
	if len(c.keys) == 0 {
		return ""
	}
	if seed != "" {
		if ids, ok := c.lookup(strings.Fields(seed)); ok && len(ids) == c.n-1 {
			if k := packKey(ids); c.next[k] != nil {
				return k
			}
		}
		var matching []string
		for _, k := range c.keys {
			if strings.Contains(c.join(unpackKey(k)), seed) {
				matching = append(matching, k)
				if len(matching) >= 200 {
					break
				}
			}
		}
		if len(matching) > 0 {
			return matching[rand.IntN(len(matching))]
		}
	}
	return c.keys[rand.IntN(len(c.keys))]
}

func (c *Chain) lookup(toks []string) ([]uint32, bool) {
	ids := make([]uint32, len(toks))
	for i, t := range toks {
		id, ok := c.ids[t]
		if !ok {
			return nil, false
		}
		ids[i] = id
	}
	return ids, true
}

// GenerateRhyme generates text ending in a token that shares the last three
// letters of word. In one forward pass it records rhyming successors at each
// step, then swaps the latest one at or after maxLength/2 and truncates there.
// Without any rhyming successor the suffix is appended to the last token.
func (c *Chain) GenerateRhyme(word string, maxLength int) string {
	suffix := rhymeSuffix(word)
	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(c.keys) == 0 {
		return ""
	}
	toks, cands := c.walk(c.keys[rand.IntN(len(c.keys))], maxLength, suffix)
	if suffix == "" {
		return c.join(toks)
	}
	for pos := len(toks) - 1; pos >= max(c.n-1, maxLength/2); pos-- {
		if opts := cands[pos]; len(opts) > 0 {
			toks[pos] = pick(opts)
			return c.join(toks[:pos+1])
		}
	}
	out := c.join(toks)
	if out != "" && !strings.HasSuffix(strings.ToLower(out), suffix) {
		out += suffix
	}
	return out
}

func rhymeSuffix(word string) string {
	r := []rune(strings.ToLower(strings.TrimSpace(word)))
	return string(r[len(r)-min(len(r), 3):])
}

func (c *Chain) walk(start string, maxLength int, suffix string) ([]uint32, map[int][]edge) {
	toks := unpackKey(start)
	var cands map[int][]edge
	if suffix != "" {
		cands = map[int][]edge{}
	}
	for range maxLength {
		s := c.next[packKey(toks[len(toks)-c.n+1:])]
		if s == nil || len(s.edges) == 0 {
			break
		}
		toks = append(toks, pick(s.edges))
		if suffix == "" {
			continue
		}
		for _, e := range s.edges {
			if strings.HasSuffix(strings.ToLower(c.tokens[e.tok]), suffix) {
				cands[len(toks)-1] = append(cands[len(toks)-1], e)
			}
		}
	}
	return toks, cands
}

func pick(edges []edge) uint32 {
	var total uint64
	for _, e := range edges {
		total += uint64(e.cnt)
	}
	target := rand.Uint64N(total)
	for _, e := range edges {
		if target < uint64(e.cnt) {
			return e.tok
		}
		target -= uint64(e.cnt)
	}
	return edges[len(edges)-1].tok
}

type Link struct {
	Word  string `json:"w"`
	Count uint32 `json:"c"`
}

// Neighborhood holds the top links of a word; the totals count all distinct
// neighbors before the cut.
type Neighborhood struct {
	Word     string `json:"word"`
	In       []Link `json:"in"`
	Out      []Link `json:"out"`
	InTotal  int    `json:"in_total"`
	OutTotal int    `json:"out_total"`
}

// Neighbors returns the most frequent tokens that follow (out) and precede
// (in) word, summed over every prefix. An empty word picks a random one.
// scans every prefix per call; TODO maybe can index prefixes by last token to speed up
func (c *Chain) Neighbors(word string, limit int) Neighborhood {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(c.keys) == 0 {
		return Neighborhood{}
	}
	id, ok := c.ids[word]
	if word == "" {
		k := c.keys[rand.IntN(len(c.keys))]
		id, ok = binary.LittleEndian.Uint32([]byte(k[len(k)-4:])), true
	}
	if !ok {
		return Neighborhood{}
	}
	in, out := map[uint32]uint32{}, map[uint32]uint32{}
	for k, s := range c.next {
		last := binary.LittleEndian.Uint32([]byte(k[len(k)-4:]))
		for _, e := range s.edges {
			if last == id {
				out[e.tok] += e.cnt
			}
			if e.tok == id {
				in[last] += e.cnt
			}
		}
	}
	return Neighborhood{c.tokens[id], c.top(in, limit), c.top(out, limit), len(in), len(out)}
}

func (c *Chain) top(counts map[uint32]uint32, limit int) []Link {
	links := make([]Link, 0, len(counts))
	for tok, n := range counts {
		links = append(links, Link{c.tokens[tok], n})
	}
	slices.SortFunc(links, func(a, b Link) int { return int(b.Count) - int(a.Count) })
	return links[:min(limit, len(links))]
}

func (c *Chain) join(ids []uint32) string {
	var b strings.Builder
	for i, id := range ids {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(c.tokens[id])
	}
	return b.String()
}

func packKey(ids []uint32) string {
	b := make([]byte, 4*len(ids))
	for i, id := range ids {
		binary.LittleEndian.PutUint32(b[4*i:], id)
	}
	return string(b)
}

func unpackKey(k string) []uint32 {
	ids := make([]uint32, len(k)/4)
	for i := range ids {
		ids[i] = binary.LittleEndian.Uint32([]byte(k[4*i : 4*i+4]))
	}
	return ids
}
