package tts

import (
	"encoding/binary"
	"os"
	"strings"
	"testing"

	"rolando/internal/tts/pico"
)

func TestSanitize(t *testing.T) {
	for in, want := range map[string]string{
		"ciao":                                 "ciao",
		"<genfile file='/tmp/x'>hi</genfile>": " genfile file='/tmp/x' hi /genfile ",
		"a\x00b":                               "ab",
		"bad \xff utf8":                        "bad  utf8",
		strings.Repeat("è", 300):              strings.Repeat("è", maxRunes),
	} {
		if got := sanitize(in); got != want {
			t.Errorf("sanitize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestResample(t *testing.T) {
	x := make([]float32, playbackRate)
	for i := range x {
		x[i] = 0.5
	}
	out := resample(x, playbackRate, encodeRate)
	if len(out) != encodeRate {
		t.Fatalf("got %d samples, want %d", len(out), encodeRate)
	}
	if v := out[len(out)/2]; v < 16000 || v > 16800 {
		t.Errorf("DC level %d, want ~16384", v)
	}
}

func TestRender(t *testing.T) {
	pico.Dir = "../../pico/lang"
	if _, err := os.Stat(pico.Dir); err != nil {
		t.Skip("voices not downloaded, run `make pico`")
	}
	if _, err := render("   <> ", "en"); err == nil {
		t.Error("want error for empty text")
	}
	for _, lang := range []string{"en", "it", "de", "es"} {
		dca, err := render("I am here", lang)
		if err != nil {
			t.Fatalf("%s: %v", lang, err)
		}
		frames := 0
		for b := dca; len(b) >= 4; frames++ {
			n := binary.LittleEndian.Uint32(b)
			if int(n)+4 > len(b) {
				t.Fatalf("%s: truncated frame", lang)
			}
			b = b[4+n:]
		}
		// 20ms frames
		if frames < 25 || frames > 250 {
			t.Errorf("%s: %d frames", lang, frames)
		}
	}
}
