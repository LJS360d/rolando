package tts

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"

	"github.com/disgoorg/disgo/voice"
)

const (
	// DefaultPiperBinary is the default path to the piper binary.
	DefaultPiperBinary = "piper"
)

var (
	PiperBinary = DefaultPiperBinary

	piperModels = map[string]string{
		"en": "/usr/share/piper-voices/en/en_US/lessac/medium/en_US-lessac-medium.onnx",
		"it": "/usr/share/piper-voices/it/it_IT/riccardo/x_low/it_IT-riccardo-x_low.onnx",
		"de": "/usr/share/piper-voices/de/de_DE/thorsten/medium/de_DE-thorsten-medium.onnx",
		"es": "/usr/share/piper-voices/es/es_ES/ald/medium/es_ES-ald-medium.onnx",
	}
)

// getVoiceModel returns the hardcoded path to the Piper voice model for the given language.
func getVoiceModel(lang string) (string, error) {
	modelPath, ok := piperModels[lang]
	if !ok {
		return "", fmt.Errorf("unsupported language for Piper TTS: %s", lang)
	}
	return modelPath, nil
}

// GeneratePiperTTS generates audio using Piper TTS and returns raw PCM bytes.
func GeneratePiperTTS(text, lang string) ([]byte, error) {
	if len(text) == 0 {
		return nil, fmt.Errorf("empty text provided for TTS")
	}

	modelPath, err := getVoiceModel(lang)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(PiperBinary,
		"--model", modelPath,
		"--output-raw",
	)
	cmd.Stdin = bytes.NewReader([]byte(text))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("piper TTS failed: %w\nstderr: %s", err, stderr.String())
	}

	if stdout.Len() == 0 {
		return nil, fmt.Errorf("piper TTS produced empty output")
	}

	return stdout.Bytes(), nil
}

// GeneratePiperTTSProvider creates a voice.OpusFrameProvider using Piper TTS.
// It generates raw PCM audio with Piper, converts it to DCA format,
// and wraps it in a standard OpusReader for use with disgo.
func GeneratePiperTTSProvider(text, lang string) (voice.OpusFrameProvider, error) {
	pcmData, err := GeneratePiperTTS(text, lang)
	if err != nil {
		return nil, err
	}
	return GenerateProviderFromPCM(pcmData)
}

// GeneratePiperTTSProviderWithConfig creates a voice.OpusFrameProvider using Piper TTS
// with explicit model path configuration, bypassing the default language mapping.
func GeneratePiperTTSProviderWithConfig(text, modelPath string) (voice.OpusFrameProvider, error) {
	if _, err := os.Stat(modelPath); err != nil {
		return nil, fmt.Errorf("invalid piper model path: %w", err)
	}

	cmd := exec.Command(PiperBinary,
		"--model", modelPath,
		"--output-raw",
	)
	cmd.Stdin = bytes.NewReader([]byte(text))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("piper TTS failed: %w\nstderr: %s", err, stderr.String())
	}

	if stdout.Len() == 0 {
		return nil, fmt.Errorf("piper TTS produced empty output")
	}

	return GenerateProviderFromPCM(stdout.Bytes())
}
