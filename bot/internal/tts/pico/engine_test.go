//go:build linux && amd64

package pico

import (
	"os"
	"sync"
	"testing"
)

func useVoices(t *testing.T) {
	t.Helper()
	Dir = "../../../pico/lang"
	if _, err := os.Stat(Dir); err != nil {
		t.Skip("voices not downloaded, run `make pico`")
	}
}

func TestSynthesize(t *testing.T) {
	useVoices(t)
	for lang := range voices {
		pcm, err := Synthesize(lang, "Hello, one two three.")
		if err != nil {
			t.Fatalf("%s: %v", lang, err)
		}
		if secs := float64(len(pcm)) / SampleRate; secs < 0.5 || secs > 10 {
			t.Errorf("%s: %.2fs of audio", lang, secs)
		}
	}
}

func TestSynthesizeConcurrent(t *testing.T) {
	useVoices(t)
	want, err := Synthesize("it", "Sono qui.")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			got, err := Synthesize("it", "Sono qui.")
			if err != nil {
				t.Error(err)
				return
			}
			if len(got) != len(want) {
				t.Errorf("got %d samples, want %d", len(got), len(want))
			}
		})
	}
	wg.Wait()
}

func TestSynthesizeUnknownLanguage(t *testing.T) {
	if _, err := Synthesize("xx", "hi"); err == nil {
		t.Fatal("want error")
	}
}
