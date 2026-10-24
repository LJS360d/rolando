package stt

import (
	"encoding/json"
	"fmt"
	"os"
	"rolando/internal/logger"
	"sync"
	"time"

	vosk "github.com/alphacep/vosk-api/go"
)

// SampleRate is the rate of the mono s16le PCM fed to recognizers.
const SampleRate = 16000

// Models load on first use and are freed after modelIdleTTL without
// recognizers; they live in native memory, outside GOMEMLIMIT. Loading takes
// seconds, so it happens outside mu and survives speakers coming and going.
type model struct {
	ready chan struct{} // closed once m or err is set
	m     *vosk.VoskModel
	err   error
	refs  int
	idle  *time.Timer
}

const modelIdleTTL = 10 * time.Minute

var (
	mu     sync.Mutex
	models = map[string]*model{}
)

func init() {
	vosk.SetLogLevel(-1)
}

// Recognizer transcribes one speaker. Not safe for concurrent use.
type Recognizer struct {
	r    *vosk.VoskRecognizer
	lang string
	m    *model
}

func NewRecognizer(lang string) (*Recognizer, error) {
	m := acquireModel(lang)
	if m.err != nil {
		releaseModel(lang, m)
		return nil, m.err
	}
	r, err := vosk.NewRecognizer(m.m, SampleRate)
	if err != nil {
		releaseModel(lang, m)
		return nil, fmt.Errorf("failed to create recognizer for model %s: %w", lang, err)
	}
	return &Recognizer{r: r, lang: lang, m: m}, nil
}

// acquireModel returns the loaded (or failed) model for lang, holding a ref.
func acquireModel(lang string) *model {
	mu.Lock()
	m, ok := models[lang]
	if !ok {
		m = &model{ready: make(chan struct{})}
		models[lang] = m
	}
	m.refs++
	if m.idle != nil {
		m.idle.Stop()
		m.idle = nil
	}
	mu.Unlock()
	if !ok {
		start := time.Now()
		if m.m, m.err = loadModel(lang); m.err != nil {
			m.err = fmt.Errorf("failed to load vosk model %s: %w", lang, m.err)
		} else {
			logger.Infof("Loaded vosk model '%s' in %s", lang, time.Since(start).Round(time.Millisecond))
		}
		close(m.ready)
	}
	<-m.ready
	return m
}

// loadModel checks the folder first: vosk.NewModel never errors, it returns
// a nil model that crashes the first recognizer.
func loadModel(lang string) (*vosk.VoskModel, error) {
	dir := "vosk/models/" + lang
	if _, err := os.Stat(dir); err != nil {
		return nil, err
	}
	return vosk.NewModel(dir)
}

func releaseModel(lang string, m *model) {
	mu.Lock()
	defer mu.Unlock()
	if m.refs--; m.refs > 0 {
		return
	}
	if m.err != nil {
		// drop the failure so the next use retries
		if models[lang] == m {
			delete(models, lang)
		}
		return
	}
	m.idle = time.AfterFunc(modelIdleTTL, func() {
		mu.Lock()
		defer mu.Unlock()
		if m.refs > 0 || models[lang] != m {
			return
		}
		m.m.Free()
		delete(models, lang)
		logger.Infof("Freed vosk model '%s'", lang)
	})
}

// Accept feeds PCM and returns the text of an utterance once Vosk detects
// its end, "" otherwise.
func (r *Recognizer) Accept(pcm []byte) string {
	if r.r.AcceptWaveform(pcm) != 1 {
		return ""
	}
	return text(r.r.Result())
}

// Flush ends the current utterance and returns its text. Discord sends no
// packets during silence, so Vosk never sees the pause that ends one.
func (r *Recognizer) Flush() string {
	return text(r.r.FinalResult())
}

func (r *Recognizer) Free() {
	r.r.Free()
	releaseModel(r.lang, r.m)
}

func text(result string) string {
	var res struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(result), &res); err != nil {
		logger.Errorf("Failed to decode vosk result: %v", err)
		return ""
	}
	return res.Text
}
