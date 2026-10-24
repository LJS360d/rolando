//go:build linux && amd64

package pico

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"rolando/internal/logger"
	"sync"
	"time"
	"unsafe"

	"modernc.org/libc"
)

const (
	statusOK = 0
	stepIdle = 200
	stepBusy = 201

	// pico2wave's arena: one language's resources plus working memory
	memSize   = 2500000
	chunkSize = 1024
	// putText takes a pico_Int16 length
	maxPut    = 32767
	maxStalls = 100

	// offsets of the handles pico writes back, in engine.h
	hSys = 0
	hTa  = 8
	hSg  = 16
	hEng = 24
)

// An engine is one language. Engines load on first use and stay: a few MB of
// libc (mmap) memory each, outside GOMEMLIMIT. Pico is single-threaded, so
// every call into it holds the engine's lock.
type engine struct {
	mu      sync.Mutex
	tls     *libc.TLS
	mem     uintptr
	h       uintptr
	name    uintptr
	defined bool
	closed  bool
}

var (
	mu      sync.Mutex
	engines = map[string]*engine{}
)

// Synthesize renders UTF-8 text, which may contain Pico markup, to mono PCM
// at SampleRate. Callers must strip markup from untrusted text: Pico's <play>
// and <genfile> tags read and write files.
func Synthesize(lang, text string) ([]int16, error) {
	for {
		e, err := get(lang)
		if err != nil {
			return nil, err
		}
		e.mu.Lock()
		if e.closed {
			e.mu.Unlock()
			continue
		}
		pcm, err := e.speak(text)
		if err != nil && pico_resetEngine(e.tls, e.handle(hEng), 0) != statusOK {
			mu.Lock()
			if engines[lang] == e {
				delete(engines, lang)
			}
			mu.Unlock()
			e.close()
		}
		e.mu.Unlock()
		return pcm, err
	}
}

func get(lang string) (*engine, error) {
	mu.Lock()
	defer mu.Unlock()
	if e, ok := engines[lang]; ok {
		return e, nil
	}
	start := time.Now()
	e, err := load(lang)
	if err != nil {
		return nil, fmt.Errorf("failed to load pico voice %s: %w", lang, err)
	}
	engines[lang] = e
	logger.Infof("Loaded pico voice '%s' in %s", lang, time.Since(start).Round(time.Millisecond))
	return e, nil
}

func load(lang string) (*engine, error) {
	files, ok := voices[lang]
	if !ok {
		return nil, errors.New("unsupported language")
	}
	ta, sg := filepath.Join(Dir, files[0]), filepath.Join(Dir, files[1])
	for _, p := range []string{ta, sg} {
		if _, err := os.Stat(p); err != nil {
			return nil, err
		}
	}
	e := &engine{tls: libc.NewTLS()}
	if err := e.open(ta, sg); err != nil {
		e.close()
		return nil, err
	}
	return e, nil
}

func (e *engine) open(ta, sg string) error {
	t := e.tls
	e.mem = libc.Xmalloc(t, memSize)
	e.h = libc.Xcalloc(t, 4, 8)
	if e.mem == 0 || e.h == 0 {
		return errors.New("out of memory")
	}
	var err error
	if e.name, err = libc.CString("rolando"); err != nil {
		return err
	}
	if st := pico_initialize(t, e.mem, memSize, e.h+hSys); st != statusOK {
		return fmt.Errorf("pico initialize: status %d", st)
	}
	sys := e.handle(hSys)
	for _, r := range []struct {
		path string
		slot uintptr
	}{{ta, hTa}, {sg, hSg}} {
		p, err := libc.CString(r.path)
		if err != nil {
			return err
		}
		st := pico_loadResource(t, sys, p, e.h+r.slot)
		libc.Xfree(t, p)
		if st != statusOK {
			return e.err("load "+r.path, st)
		}
	}
	if st := pico_createVoiceDefinition(t, sys, e.name); st != statusOK {
		return e.err("createVoiceDefinition", st)
	}
	e.defined = true
	res := libc.Xmalloc(t, PICO_RETSTRINGSIZE)
	if res == 0 {
		return errors.New("out of memory")
	}
	defer libc.Xfree(t, res)
	for _, slot := range []uintptr{hTa, hSg} {
		if st := pico_getResourceName(t, sys, e.handle(slot), res); st != statusOK {
			return e.err("getResourceName", st)
		}
		if st := pico_addResourceToVoiceDefinition(t, sys, e.name, res); st != statusOK {
			return e.err("addResourceToVoiceDefinition", st)
		}
	}
	if st := pico_newEngine(t, sys, e.name, e.h+hEng); st != statusOK {
		return e.err("newEngine", st)
	}
	return nil
}

func (e *engine) speak(text string) ([]int16, error) {
	t := e.tls
	in, err := libc.CString(text)
	if err != nil {
		return nil, err
	}
	defer libc.Xfree(t, in)
	io := libc.Xmalloc(t, 8+chunkSize)
	if io == 0 {
		return nil, errors.New("out of memory")
	}
	defer libc.Xfree(t, io)
	put, got, typ, buf := io, io+2, io+4, io+8
	eng := e.handle(hEng)

	var pcm []int16
	stalls := 0
	// the terminating NUL flushes the last sentence
	for p, left := in, len(text)+1; left > 0; {
		if st := pico_putTextUtf8(t, eng, p, pico_Int16(min(left, maxPut)), put); st != statusOK {
			return nil, e.err("putTextUtf8", st)
		}
		n := int(*(*int16)(unsafe.Pointer(put)))
		if n == 0 {
			if stalls++; stalls > maxStalls {
				return nil, errors.New("pico stopped accepting text")
			}
		} else {
			stalls = 0
		}
		p += uintptr(n)
		left -= n
		for {
			st := pico_getData(t, eng, buf, chunkSize, got, typ)
			n := int(*(*int16)(unsafe.Pointer(got)))
			pcm = append(pcm, unsafe.Slice((*int16)(unsafe.Pointer(buf)), n/2)...)
			if st == stepIdle {
				break
			}
			if st != stepBusy {
				return nil, e.err("getData", st)
			}
		}
	}
	return pcm, nil
}

func (e *engine) handle(slot uintptr) uintptr {
	return *(*uintptr)(unsafe.Pointer(e.h + slot))
}

func (e *engine) err(op string, st pico_Status) error {
	msg := libc.Xmalloc(e.tls, PICO_RETSTRINGSIZE)
	defer libc.Xfree(e.tls, msg)
	if msg != 0 && pico_getSystemStatusMessage(e.tls, e.handle(hSys), st, msg) == statusOK {
		return fmt.Errorf("pico %s: %s (%d)", op, libc.GoString(msg), st)
	}
	return fmt.Errorf("pico %s: status %d", op, st)
}

func (e *engine) close() {
	t := e.tls
	e.closed = true
	if e.h != 0 {
		if sys := e.handle(hSys); sys != 0 {
			if e.handle(hEng) != 0 {
				pico_disposeEngine(t, sys, e.h+hEng)
			}
			if e.defined {
				pico_releaseVoiceDefinition(t, sys, e.name)
			}
			for _, slot := range []uintptr{hSg, hTa} {
				if e.handle(slot) != 0 {
					pico_unloadResource(t, sys, e.h+slot)
				}
			}
			pico_terminate(t, e.h+hSys)
		}
		libc.Xfree(t, e.h)
	}
	libc.Xfree(t, e.name)
	libc.Xfree(t, e.mem)
	t.Close()
}
