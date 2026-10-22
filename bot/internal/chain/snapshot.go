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

// WriteFile writes the snapshot atomically (tmp + fsync + rename) and clears the dirty flag.
func (c *Chain) WriteFile(path string) error {
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	defer os.Remove(tmp)
	defer f.Close()

	c.mu.RLock()
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
	c.dirty = false
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
	u32(uint32(len(c.keys)))
	for _, k := range c.keys {
		w.WriteString(k)
		s := c.next[k]
		u32(uint32(len(s.edges)))
		for _, e := range s.edges {
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

// ReadFile loads a snapshot built with n-gram size n.
func ReadFile(path string, n int) (*Chain, error) {
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
	c.tokens = make([]string, 0, nt)
	for range nt {
		t := string(take(int(le.Uint16(take(2)))))
		if err != nil {
			return nil, err
		}
		c.ids[t] = uint32(len(c.tokens))
		c.tokens = append(c.tokens, t)
		c.bytes += uint64(len(t) + tokenOverhead)
	}
	np := le.Uint32(take(4))
	c.keys = make([]string, 0, np)
	keyLen := 4 * (n - 1)
	for range np {
		k := string(take(keyLen))
		ne := le.Uint32(take(4))
		if err != nil || uint64(ne)*8 > uint64(len(r)) {
			return nil, ErrSnapshot
		}
		s := &state{idx: len(c.keys), edges: make([]edge, ne)}
		for i := range s.edges {
			s.edges[i] = edge{le.Uint32(take(4)), le.Uint32(take(4))}
			if s.edges[i].tok >= nt {
				return nil, ErrSnapshot
			}
		}
		c.next[k] = s
		c.keys = append(c.keys, k)
		c.bytes += uint64(len(k)+prefixOverhead) + uint64(edgeSize*ne)
	}
	if err != nil || len(r) != 0 {
		return nil, ErrSnapshot
	}
	return c, nil
}
