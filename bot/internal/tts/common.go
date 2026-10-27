package tts

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/disgoorg/disgo/voice"
	"github.com/hajimehoshi/go-mp3"
	"layeh.com/gopus"
)

// MP3ToDCA decodes MP3 audio to mono PCM and encodes it to DCA format
// (length-prefixed Opus frames).
func MP3ToDCA(mp3Data []byte) ([]byte, error) {
	dec, err := mp3.NewDecoder(bytes.NewReader(mp3Data))
	if err != nil {
		return nil, fmt.Errorf("failed to decode mp3: %w", err)
	}
	raw := bytes.NewBuffer(make([]byte, 0, max(dec.Length(), 0)))
	if _, err := raw.ReadFrom(dec); err != nil {
		return nil, fmt.Errorf("failed to decode mp3: %w", err)
	}
	stereo := raw.Bytes()

	// go-mp3 always yields 16-bit little-endian stereo
	pcm := make([]int16, len(stereo)/4)
	for i := range pcm {
		l := int16(binary.LittleEndian.Uint16(stereo[i*4:]))
		r := int16(binary.LittleEndian.Uint16(stereo[i*4+2:]))
		pcm[i] = int16((int32(l) + int32(r)) / 2)
	}

	rate := dec.SampleRate()
	switch rate {
	case 8000, 12000, 16000, 24000, 48000:
	default:
		pcm, rate = resample(pcm, rate, 48000), 48000
	}
	return PCMToDCA(pcm, rate)
}

// resample converts mono PCM between sample rates by linear interpolation.
func resample(pcm []int16, from, to int) []int16 {
	if len(pcm) == 0 {
		return pcm
	}
	out := make([]int16, len(pcm)*to/from)
	for i := range out {
		pos := float64(i) * float64(from) / float64(to)
		j := int(pos)
		if j+1 >= len(pcm) {
			out[i] = pcm[len(pcm)-1]
			continue
		}
		frac := pos - float64(j)
		out[i] = int16(float64(pcm[j])*(1-frac) + float64(pcm[j+1])*frac)
	}
	return out
}

// maxOpusPacket is the largest packet one Opus frame can produce.
const maxOpusPacket = 1275

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

		opusData, err := encoder.Encode(samples, frameSamples, maxOpusPacket)
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

// GenerateProviderFromMP3 converts MP3 audio to a voice.OpusFrameProvider.
func GenerateProviderFromMP3(mp3Data []byte) (voice.OpusFrameProvider, error) {
	dcaData, err := MP3ToDCA(mp3Data)
	if err != nil {
		return nil, err
	}
	return voice.NewOpusReader(bytes.NewReader(dcaData)), nil
}
