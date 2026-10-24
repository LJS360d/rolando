package tts

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"layeh.com/gopus"
)

// PCMToDCA encodes mono PCM at an Opus-supported sample rate to DCA format.
func PCMToDCA(pcm []int16, sampleRate int) ([]byte, error) {
	encoder, err := gopus.NewEncoder(sampleRate, 1, gopus.Voip)
	if err != nil {
		return nil, fmt.Errorf("failed to create opus encoder: %w", err)
	}

	// 20ms frames
	frameSamples := sampleRate / 50

	var dcaBuf bytes.Buffer
	// opus at voip bitrate is far below 1/4 of the PCM size
	dcaBuf.Grow(len(pcm) / 2)
	samples := make([]int16, frameSamples)
	var prefix [4]byte
	for i := 0; i < len(pcm); i += frameSamples {
		chunk := pcm[i:min(i+frameSamples, len(pcm))]
		n := copy(samples, chunk)
		// pad the last frame with silence
		clear(samples[n:])

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
