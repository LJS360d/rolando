package tts

import (
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/disgoorg/disgo/voice"
)

// FetchTTS downloads TTS audio from Google Translate and returns the raw MP3 bytes.
func FetchTTS(text, lang string) ([]byte, error) {
	if len(text) > 200 {
		text = text[:200]
	}

	ttsURL := fmt.Sprintf(
		"http://translate.google.com/translate_tts?ie=UTF-8&total=1&idx=0&textlen=%d&client=tw-ob&q=%s&tl=%s",
		len(text), url.QueryEscape(text), lang,
	)

	resp, err := http.Get(ttsURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch TTS audio: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TTS request failed with status: %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// GenerateTTSProvider creates a voice.OpusFrameProvider ready for disgo using Google Translate TTS.
// It fetches TTS audio, converts it to DCA format using common utilities, and wraps it in an OpusReader.
func GenerateTTSProvider(text, lang string) (voice.OpusFrameProvider, error) {
	mp3Data, err := FetchTTS(text, lang)
	if err != nil {
		return nil, err
	}
	return GenerateProviderFromAudio(mp3Data)
}
