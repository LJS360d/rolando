package tts

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os/exec"

	"github.com/disgoorg/disgo/voice"
	"layeh.com/gopus"
)

// AudioToDCA converts raw audio data (MP3, WAV, etc.) to DCA format (length-prefixed Opus frames).
// It uses ffmpeg to decode the audio to PCM via stdin/stdout pipes, then gopus to encode to Opus.
func AudioToDCA(audioData []byte) ([]byte, error) {
	cmd := exec.Command("ffmpeg",
		"-i", "pipe:0",
		"-f", "s16le",
		"-ar", "48000",
		"-ac", "1",
		"pipe:1",
	)

	cmd.Stdin = bytes.NewReader(audioData)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	pcmData, err := io.ReadAll(stdout)
	if err != nil {
		cmd.Wait()
		return nil, fmt.Errorf("failed to read PCM from ffmpeg: %w", err)
	}

	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("ffmpeg exited with error: %w", err)
	}

	return PCMToDCA(pcmData)
}

// GenerateProviderFromAudio converts raw audio data to a voice.OpusFrameProvider.
// It decodes the audio to DCA format and wraps it in a standard OpusReader.
func GenerateProviderFromAudio(audioData []byte) (voice.OpusFrameProvider, error) {
	dcaData, err := AudioToDCA(audioData)
	if err != nil {
		return nil, err
	}
	return voice.NewOpusReader(bytes.NewReader(dcaData)), nil
}

// PCMToDCA converts raw PCM audio data (16-bit little-endian, 48kHz, mono) to DCA format.
// It encodes the PCM directly to Opus frames without any intermediate decoding.
func PCMToDCA(pcmData []byte) ([]byte, error) {
	encoder, err := gopus.NewEncoder(48000, 1, gopus.Voip)
	if err != nil {
		return nil, fmt.Errorf("failed to create opus encoder: %w", err)
	}

	var dcaBuf bytes.Buffer
	const (
		frameSamples = 960
		sampleBytes  = 2
	)
	pcmFrameSize := frameSamples * sampleBytes // 1920 bytes for mono

	for i := 0; i < len(pcmData); i += pcmFrameSize {
		end := min(i+pcmFrameSize, len(pcmData))
		chunk := pcmData[i:end]

		// Ensure frame is exactly 1920 bytes by padding with silence if needed
		if len(chunk) < pcmFrameSize {
			padded := make([]byte, pcmFrameSize)
			copy(padded, chunk)
			chunk = padded
		}

		samples := make([]int16, frameSamples)
		for j := range frameSamples {
			samples[j] = int16(chunk[j*2]) | int16(chunk[j*2+1])<<8
		}

		opusData, err := encoder.Encode(samples, frameSamples, 3840)
		if err != nil {
			return nil, fmt.Errorf("opus encode error: %w", err)
		}

		// DCA format: 4-byte little-endian length prefix + Opus payload
		binary.Write(&dcaBuf, binary.LittleEndian, uint32(len(opusData)))
		dcaBuf.Write(opusData)
	}

	return dcaBuf.Bytes(), nil
}

// GenerateProviderFromPCM converts raw PCM audio data to a voice.OpusFrameProvider.
// It encodes the PCM directly to DCA format and wraps it in a standard OpusReader.
func GenerateProviderFromPCM(pcmData []byte) (voice.OpusFrameProvider, error) {
	dcaData, err := PCMToDCA(pcmData)
	if err != nil {
		return nil, err
	}
	return voice.NewOpusReader(bytes.NewReader(dcaData)), nil
}
