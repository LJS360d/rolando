package tts

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"sync"

	"rolando/internal/tts/pico"

	"github.com/disgoorg/disgo/voice"
)

// maxRunes bounds how much of a message is spoken.
const maxRunes = 200

// encodeRate is the Opus input rate; it holds everything below the 5.6 kHz
// the voice has after being played at playbackRate.
const encodeRate = 24000

// render synthesizes text in lang (a data.Langs code) to DCA (length-prefixed Opus frames).
func render(text, lang string) ([]byte, error) {
	text = sanitize(text)
	if strings.TrimSpace(text) == "" {
		return nil, errors.New("nothing to say")
	}
	pcm, err := pico.Synthesize(lang, fmt.Sprintf(prosody, text))
	if err != nil {
		return nil, err
	}
	if len(pcm) == 0 {
		return nil, errors.New("pico produced no audio")
	}
	return PCMToDCA(resample(clarify(pcm), playbackRate, encodeRate), encodeRate)
}

// sanitize strips what Pico would read as markup (<play> and <genfile> touch
// files) and caps the length.
func sanitize(text string) string {
	text = strings.Map(func(r rune) rune {
		switch r {
		case '<', '>':
			return ' '
		case 0:
			return -1
		}
		return r
	}, strings.ToValidUTF8(text, ""))
	if r := []rune(text); len(r) > maxRunes {
		text = string(r[:maxRunes])
	}
	return text
}

// GenerateTTSProvider creates a voice.OpusFrameProvider speaking text.
func GenerateTTSProvider(text, lang string) (voice.OpusFrameProvider, error) {
	dca, err := render(text, lang)
	if err != nil {
		return nil, err
	}
	return voice.NewOpusReader(bytes.NewReader(dca)), nil
}

var phrases struct {
	sync.Mutex
	dca map[string][]byte
}

// PhraseProvider is GenerateTTSProvider for fixed phrases: the encoded audio
// is kept, so they cost no synthesis or encode after the first time.
func PhraseProvider(text, lang string) (voice.OpusFrameProvider, error) {
	key := lang + "\x00" + text
	phrases.Lock()
	dca, ok := phrases.dca[key]
	phrases.Unlock()
	if !ok {
		var err error
		if dca, err = render(text, lang); err != nil {
			return nil, err
		}
		phrases.Lock()
		if phrases.dca == nil {
			phrases.dca = map[string][]byte{}
		}
		phrases.dca[key] = dca
		phrases.Unlock()
	}
	return voice.NewOpusReader(bytes.NewReader(dca)), nil
}
