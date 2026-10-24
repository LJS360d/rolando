// Package pico is the SVOX Pico TTS engine, transpiled from the AOSP C sources
// to pure Go by ccgo (ccgo_linux_amd64.go, regenerated with `make pico-gen`).
// Only the voice files are downloaded, by `make pico`.
package pico

// SampleRate is the rate of the mono PCM Synthesize returns.
const SampleRate = 16000

// Dir holds the voice files, relative to the working directory like vosk models.
var Dir = "pico/lang"

// voices maps bot languages to Pico's text-analysis and signal-generation files.
var voices = map[string][2]string{
	"en": {"en-GB_ta.bin", "en-GB_kh0_sg.bin"},
	"it": {"it-IT_ta.bin", "it-IT_cm0_sg.bin"},
	"de": {"de-DE_ta.bin", "de-DE_gl0_sg.bin"},
	"es": {"es-ES_ta.bin", "es-ES_zl0_sg.bin"},
}
