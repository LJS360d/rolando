package tts

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os/exec"
	"time"

	"github.com/disgoorg/disgo/voice"
	"layeh.com/gopus"
)

// AudioToDCA converts raw audio data (MP3, WAV, etc.) to DCA format (length-prefixed Opus frames).
// It uses ffmpeg to decode the audio to PCM via stdin/stdout pipes, then gopus to encode to Opus.
func AudioToDCA(audioData []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ffmpeg",
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

	const (
		frameSamples = 960
		sampleBytes  = 2
	)
	pcmFrameSize := frameSamples * sampleBytes // 1920 bytes for mono

	var dcaBuf bytes.Buffer
	// opus at voip bitrate is far below 1/8 of the PCM size
	dcaBuf.Grow(len(pcmData) / 8)
	samples := make([]int16, frameSamples)
	var prefix [4]byte
	for i := 0; i < len(pcmData); i += pcmFrameSize {
		chunk := pcmData[i:min(i+pcmFrameSize, len(pcmData))]
		if len(chunk) < pcmFrameSize {
			// pad the last frame with silence
			clear(samples)
		}
		for j := range len(chunk) / 2 {
			samples[j] = int16(chunk[j*2]) | int16(chunk[j*2+1])<<8
		}

		opusData, err := encoder.Encode(samples, frameSamples, 3840)
		if err != nil {
			return nil, fmt.Errorf("opus encode error: %w", err)
		}

		// DCA format: 4-byte little-endian length prefix + Opus payload
		binary.LittleEndian.PutUint32(prefix[:], uint32(len(opusData)))
		dcaBuf.Write(prefix[:])
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
