package chain

import (
	"bufio"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"io"
	"os"
)

// Snapshot layout (little-endian):
//
//	magic "RLCH" | version u16 | ngram u8 | watermark i64 | messages i64
//	token_count u32 | tokens (len u16 + bytes)...
//	prefix_count u32 | per prefix: key (n-1 x u32) | edge_count u32 | edges (tok u32, cnt u32)...
//	crc32 u32 over everything before it
const (
	magic   = "RLCH"
	version = 1
)

var ErrSnapshot = errors.New("invalid snapshot")

// WriteFile writes the snapshot atomically (tmp + fsync + rename) and marks what it wrote as clean; changes made meanwhile stay dirty.
func (c *Chain) WriteFile(path string) error {
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	defer os.Remove(tmp)
	defer f.Close()

	c.mu.RLock()
	gen := c.gen
	err = c.encode(f)
	c.mu.RUnlock()
	if err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	c.mu.Lock()
	c.saved = max(c.saved, gen)
	c.mu.Unlock()
	return nil
}

func (c *Chain) encode(f io.Writer) error {
	h := crc32.NewIEEE()
	w := bufio.NewWriterSize(io.MultiWriter(f, h), 1<<16)
	le := binary.LittleEndian
	var buf [8]byte
	u16 := func(v uint16) { le.PutUint16(buf[:], v); w.Write(buf[:2]) }
	u32 := func(v uint32) { le.PutUint32(buf[:], v); w.Write(buf[:4]) }
	u64 := func(v uint64) { le.PutUint64(buf[:], v); w.Write(buf[:8]) }

	w.WriteString(magic)
	u16(version)
	w.WriteByte(byte(c.n))
	u64(uint64(c.watermark))
	u64(uint64(c.messages))
	u32(uint32(len(c.tokens)))
	for _, t := range c.tokens {
		u16(uint16(len(t)))
		w.WriteString(t)
	}
	u32(uint32(len(c.edges)))
	for i, es := range c.edges {
		for _, id := range c.key(i) {
			u32(id)
		}
		u32(uint32(len(es)))
		for _, e := range es {
			u32(e.tok)
			u32(e.cnt)
		}
	}
	if err := w.Flush(); err != nil {
		return err
	}
	le.PutUint32(buf[:], h.Sum32())
	_, err := f.Write(buf[:4])
	return err
}

// ReadFile loads a snapshot built with n-gram size n, clamped like New.
func ReadFile(path string, n int) (*Chain, error) {
	n = min(max(n, 2), MaxN)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) < 31 || string(data[:4]) != magic {
		return nil, ErrSnapshot
	}
	body := data[:len(data)-4]
	le := binary.LittleEndian
	if crc32.ChecksumIEEE(body) != le.Uint32(data[len(data)-4:]) {
		return nil, ErrSnapshot
	}
	if le.Uint16(body[4:]) != version || int(body[6]) != n {
		return nil, ErrSnapshot
	}
	r := body[7:]
	take := func(k int) []byte {
		if len(r) < k {
			err = ErrSnapshot
			return make([]byte, k)
		}
		b := r[:k]
		r = r[k:]
		return b
	}
	c := New(n)
	c.watermark = int64(le.Uint64(take(8)))
	c.messages = int64(le.Uint64(take(8)))
	nt := le.Uint32(take(4))
	// one string backs every token: find where the token section ends
	end := 0
	for range nt {
		if end+2 > len(r) {
			return nil, ErrSnapshot
		}
		end += 2 + int(le.Uint16(r[end:]))
	}
	if err != nil || end > len(r) {
		return nil, ErrSnapshot
	}
	section := string(take(end))
	c.tokens = make([]string, 0, nt)
	for off := 0; off < len(section); {
		l := int(le.Uint16([]byte(section[off : off+2])))
		t := section[off+2 : off+2+l]
		off += 2 + l
		c.ids[t] = uint32(len(c.tokens))
		c.tokens = append(c.tokens, t)
		c.bytes += uint64(len(t) + tokenOverhead)
	}
	np := le.Uint32(take(4))
	k := n - 1
	if err != nil || uint64(np)*uint64(4*k+4) > uint64(len(r)) {
		return nil, ErrSnapshot
	}
	c.keys = make([]uint32, 0, int(np)*k)
	c.edges = make([][]edge, 0, np)
	// every edge comes from one allocation; prefixes get capped windows, so
	// a later append moves that prefix out instead of overwriting the next
	rest := uint64(len(r)) - uint64(np)*uint64(4*k+4)
	if rest%8 != 0 {
		return nil, ErrSnapshot
	}
	arena := make([]edge, rest/8)
	for range np {
		key := take(4 * k)
		ne := le.Uint32(take(4))
		if err != nil || uint64(ne)*8 > uint64(len(r)) {
			return nil, ErrSnapshot
		}
		start := len(c.keys)
		for j := range k {
			id := le.Uint32(key[4*j:])
			if id >= nt {
				return nil, ErrSnapshot
			}
			c.keys = append(c.keys, id)
		}
		if _, dup := c.index.get(c.keys[start:]); dup {
			return nil, ErrSnapshot
		}
		c.index.put(c.keys[start:], int32(len(c.edges)))
		if uint64(ne) > uint64(len(arena)) {
			return nil, ErrSnapshot
		}
		es := arena[:ne:ne]
		arena = arena[ne:]
		for i := range es {
			es[i] = edge{le.Uint32(take(4)), le.Uint32(take(4))}
			if es[i].tok >= nt {
				return nil, ErrSnapshot
			}
		}
		c.edges = append(c.edges, es)
		c.bytes += c.prefixBytes() + uint64(edgeSize*ne)
	}
	if err != nil || len(r) != 0 {
		return nil, ErrSnapshot
	}
	return c, nil
}
