package chain

import (
	"math/rand/v2"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
)

type edge struct{ tok, cnt uint32 }

type Stats struct {
	Prefixes int64  `json:"prefixes"`
	Messages int64  `json:"messages"`
	Bytes    uint64 `json:"bytes"`
}

// Chain is one guild Markov chain. Tokens are interned; prefix i is the n-1
// token ids keys[i*(n-1):(i+1)*(n-1)] with successors edges[i]
type Chain struct {
	mu        sync.RWMutex
	id        uint64
	n         int
	tokens    []string
	ids       map[string]uint32
	index     index
	keys      []uint32
	edges     [][]edge
	scratch   []uint32
	messages  int64
	bytes     uint64
	watermark int64
	// gen counts mutations; saved is the gen last written to disk
	gen, saved uint64
}

// estimated heap bytes, see Stats.Bytes
const (
	tokenOverhead  = 48
	prefixOverhead = 48
	edgeSize       = 8
)

// MaxN is the largest supported n-gram size.
const MaxN = maxKey + 1

func New(n int) *Chain {
	n = min(max(n, 2), MaxN)
	c := &Chain{id: lastID.Add(1), n: n, ids: map[string]uint32{}}
	if n-1 <= 2 {
		c.index = smallIndex{}
	} else {
		c.index = bigIndex{}
	}
	return c
}

var lastID atomic.Uint64

// ID is unique per chain in this process, so derived data can be keyed to
// the chain it came from: a reset or rebuild swaps in a new one.
func (c *Chain) ID() uint64 { return c.id }

func (c *Chain) N() int { return c.n }

func (c *Chain) key(i int) []uint32 {
	k := c.n - 1
	return c.keys[i*k : (i+1)*k]
}

func (c *Chain) prefixBytes() uint64 { return uint64(4*(c.n-1) + prefixOverhead) }

func (c *Chain) Stats() Stats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return Stats{Prefixes: int64(len(c.edges)), Messages: c.messages, Bytes: c.bytes}
}

func (c *Chain) Watermark() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.watermark
}

func (c *Chain) Dirty() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.gen != c.saved
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
	c.gen++
}

func (c *Chain) trainOne(text string, maxBranches int) {
	toks := strings.Fields(text)
	if len(toks) < c.n {
		return
	}
	ids := c.scratch[:0]
	for _, t := range toks {
		ids = append(ids, c.intern(t))
	}
	c.scratch = ids
	for i := 0; i+c.n <= len(ids); i++ {
		c.add(ids[i:i+c.n-1], ids[i+c.n-1], maxBranches)
	}
	c.messages++
}

func (c *Chain) intern(t string) uint32 {
	if id, ok := c.ids[t]; ok {
		return id
	}
	// t is a substring of the message; cloning keeps the message collectable
	t = strings.Clone(t)
	id := uint32(len(c.tokens))
	c.tokens = append(c.tokens, t)
	c.ids[t] = id
	c.bytes += uint64(len(t) + tokenOverhead)
	return id
}

func (c *Chain) add(key []uint32, tok uint32, maxBranches int) {
	i, ok := c.index.get(key)
	if !ok {
		i = int32(len(c.edges))
		c.index.put(key, i)
		c.keys = append(c.keys, key...)
		c.edges = append(c.edges, nil)
		c.bytes += c.prefixBytes()
	}
	es := c.edges[i]
	for j := range es {
		if es[j].tok == tok {
			es[j].cnt++
			return
		}
	}
	// a new successor would have the lowest count and be pruned right away
	if maxBranches > 0 && len(es) >= maxBranches {
		return
	}
	c.edges[i] = append(es, edge{tok, 1})
	c.bytes += edgeSize
}

func (c *Chain) prune(i, maxBranches int) int {
	es := c.edges[i]
	if len(es) <= maxBranches {
		return 0
	}
	slices.SortStableFunc(es, func(a, b edge) int { return int(b.cnt) - int(a.cnt) })
	c.edges[i] = slices.Clone(es[:maxBranches])
	return len(es) - maxBranches
}

// Cap applies maxBranches to every prefix and returns the removed transitions.
func (c *Chain) Cap(maxBranches int) int {
	if maxBranches <= 0 {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	removed := 0
	for i := range c.edges {
		removed += c.prune(i, maxBranches)
	}
	if removed > 0 {
		c.bytes -= uint64(edgeSize * removed)
		c.gen++
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
	ids, ok := c.lookup(toks)
	if !ok {
		return
	}
	for i := 0; i+c.n <= len(ids); i++ {
		c.remove(ids[i:i+c.n-1], ids[i+c.n-1])
	}
	c.messages = max(0, c.messages-1)
	c.gen++
}

func (c *Chain) remove(key []uint32, tok uint32) {
	i, ok := c.index.get(key)
	if !ok {
		return
	}
	es := c.edges[i]
	j := slices.IndexFunc(es, func(e edge) bool { return e.tok == tok })
	if j < 0 {
		return
	}
	if es[j].cnt > 1 {
		es[j].cnt--
		return
	}
	c.edges[i] = slices.Delete(es, j, j+1)
	c.bytes -= edgeSize
	if len(c.edges[i]) == 0 {
		c.removePrefix(int(i))
	}
}

// removePrefix moves the last prefix into slot i.
func (c *Chain) removePrefix(i int) {
	last := len(c.edges) - 1
	c.index.del(c.key(i))
	if i != last {
		copy(c.key(i), c.key(last))
		c.edges[i] = c.edges[last]
		c.index.put(c.key(i), int32(i))
	}
	c.edges[last] = nil
	c.edges = c.edges[:last]
	c.keys = c.keys[:last*(c.n-1)]
	c.bytes -= c.prefixBytes()
}

// Generate walks the chain from a random prefix.
func (c *Chain) Generate(maxLength int) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(c.edges) == 0 {
		return ""
	}
	toks, _ := c.walk(rand.IntN(len(c.edges)), maxLength, "")
	return c.join(toks)
}

// GenerateFromSeed starts from a prefix equal to seed, else a random prefix
// containing seed, else a random prefix.
func (c *Chain) GenerateFromSeed(seed string, maxLength int) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	start := c.findPrefix(seed)
	if start < 0 {
		return ""
	}
	toks, _ := c.walk(start, maxLength, "")
	return c.join(toks)
}

// findPrefix returns a prefix index, -1 when the chain is empty.
func (c *Chain) findPrefix(seed string) int {
	if len(c.edges) == 0 {
		return -1
	}
	if seed != "" {
		if ids, ok := c.lookup(strings.Fields(seed)); ok && len(ids) == c.n-1 {
			if i, ok := c.index.get(ids); ok {
				return int(i)
			}
		}
		contains := func(i int) bool { return strings.Contains(c.join(c.key(i)), seed) }
		if !strings.Contains(seed, " ") {
			// a seed without spaces can only match within one token
			hit := map[uint32]bool{}
			for id, t := range c.tokens {
				if strings.Contains(t, seed) {
					hit[uint32(id)] = true
				}
			}
			if len(hit) == 0 {
				return rand.IntN(len(c.edges))
			}
			contains = func(i int) bool { return slices.ContainsFunc(c.key(i), func(id uint32) bool { return hit[id] }) }
		}
		var matching []int
		for i := range c.edges {
			if contains(i) {
				matching = append(matching, i)
				if len(matching) >= 200 {
					break
				}
			}
		}
		if len(matching) > 0 {
			return matching[rand.IntN(len(matching))]
		}
	}
	return rand.IntN(len(c.edges))
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
	if len(c.edges) == 0 {
		return ""
	}
	toks, cands := c.walk(rand.IntN(len(c.edges)), maxLength, suffix)
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
	if out != "" && !hasSuffixFold(out, suffix) {
		out += suffix
	}
	return out
}

func rhymeSuffix(word string) string {
	r := []rune(strings.ToLower(strings.TrimSpace(word)))
	return string(r[len(r)-min(len(r), 3):])
}

// hasSuffixFold is HasSuffix(ToLower(s), suffix) without allocating per token.
func hasSuffixFold(s, suffix string) bool {
	return len(s) >= len(suffix) && strings.EqualFold(s[len(s)-len(suffix):], suffix)
}

func (c *Chain) walk(start, maxLength int, suffix string) ([]uint32, map[int][]edge) {
	toks := slices.Clone(c.key(start))
	var cands map[int][]edge
	if suffix != "" {
		cands = map[int][]edge{}
	}
	for range maxLength {
		i, ok := c.index.get(toks[len(toks)-c.n+1:])
		if !ok || len(c.edges[i]) == 0 {
			break
		}
		es := c.edges[i]
		toks = append(toks, pick(es))
		if suffix == "" {
			continue
		}
		for _, e := range es {
			if hasSuffixFold(c.tokens[e.tok], suffix) {
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
	if len(c.edges) == 0 {
		return Neighborhood{}
	}
	id, ok := c.ids[word]
	if word == "" {
		k := c.key(rand.IntN(len(c.edges)))
		id, ok = k[len(k)-1], true
	}
	if !ok {
		return Neighborhood{}
	}
	in, out := map[uint32]uint32{}, map[uint32]uint32{}
	for i, es := range c.edges {
		last := c.keys[(i+1)*(c.n-1)-1]
		for _, e := range es {
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

const maxKey = 7

// index maps a prefix to its slot. Keys are plain values, so the map holds
// no pointers for the GC to scan and lookups don't allocate.
type index interface {
	get(key []uint32) (int32, bool)
	put(key []uint32, i int32)
	del(key []uint32)
}

// smallIndex packs up to 2 ids (n <= 3) into one word.
type smallIndex map[uint64]int32

func smallKey(key []uint32) uint64 {
	var k uint64
	for i, id := range key {
		k |= uint64(id) << (32 * i)
	}
	return k
}

func (x smallIndex) get(key []uint32) (int32, bool) {
	i, ok := x[smallKey(key)]
	return i, ok
}
func (x smallIndex) put(key []uint32, i int32) { x[smallKey(key)] = i }
func (x smallIndex) del(key []uint32)          { delete(x, smallKey(key)) }

type bigIndex map[[maxKey]uint32]int32

func bigKey(key []uint32) (k [maxKey]uint32) {
	copy(k[:], key)
	return k
}

func (x bigIndex) get(key []uint32) (int32, bool) {
	i, ok := x[bigKey(key)]
	return i, ok
}
func (x bigIndex) put(key []uint32, i int32) { x[bigKey(key)] = i }
func (x bigIndex) del(key []uint32)          { delete(x, bigKey(key)) }
