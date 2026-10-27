package tts

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/disgoorg/disgo/voice"
)

var client = &http.Client{Timeout: 10 * time.Second}

// maxChars is the longest text the endpoint accepts
const maxChars = 200

// maxAudio bounds the response; 200 chars of speech is well under it
const maxAudio = 4 << 20

// FetchTTS downloads TTS audio from Google Translate and returns the raw MP3 bytes.
func FetchTTS(text, lang string) ([]byte, error) {
	if r := []rune(text); len(r) > maxChars {
		text = string(r[:maxChars])
	}

	ttsURL := fmt.Sprintf(
		"http://translate.google.com/translate_tts?ie=UTF-8&total=1&idx=0&textlen=%d&client=tw-ob&q=%s&tl=%s",
		utf8.RuneCountInString(text), url.QueryEscape(text), url.QueryEscape(lang),
	)

	resp, err := client.Get(ttsURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch TTS audio: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TTS request failed with status: %d", resp.StatusCode)
	}

	return io.ReadAll(io.LimitReader(resp.Body, maxAudio))
}

// GenerateTTSProvider creates a voice.OpusFrameProvider ready for disgo using Google Translate TTS.
// It fetches TTS audio, converts it to DCA format and wraps it in an OpusReader.
func GenerateTTSProvider(text, lang string) (voice.OpusFrameProvider, error) {
	mp3Data, err := FetchTTS(text, lang)
	if err != nil {
		return nil, err
	}
	return GenerateProviderFromMP3(mp3Data)
}

var phrases struct {
	sync.Mutex
	dca map[string][]byte
}

// PhraseProvider is GenerateTTSProvider for fixed phrases: the encoded audio
// is kept, so they cost no fetch or encode after the first time.
func PhraseProvider(text, lang string) (voice.OpusFrameProvider, error) {
	key := lang + "\x00" + text
	phrases.Lock()
	dca, ok := phrases.dca[key]
	phrases.Unlock()
	if !ok {
		mp3, err := FetchTTS(text, lang)
		if err != nil {
			return nil, err
		}
		if dca, err = MP3ToDCA(mp3); err != nil {
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
