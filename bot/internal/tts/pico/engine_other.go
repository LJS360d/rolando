//go:build !(linux && amd64)

package pico

import "errors"

// Synthesize is unavailable: the transpiled engine is generated for linux/amd64 only.
func Synthesize(lang, text string) ([]int16, error) {
	return nil, errors.New("pico TTS is only built for linux/amd64")
}
