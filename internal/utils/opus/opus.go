package utils

import (
	"gopkg.in/hraban/opus.v2"
)

var (
	opusDecoder *opus.Decoder
	pcmBuf      []int16
)

func init() {
	var err error

	opusDecoder, err = opus.NewDecoder(48000, 2)
	if err != nil {
		panic(err)
	}
	// Buffer for max frame size (120ms * 48kHz = 5760 samples)
	pcmBuf = make([]int16, 5760)
}

func DecodeOpusPacket(opusPkt []byte) ([]int16, error) {
	n, err := opusDecoder.Decode(opusPkt, pcmBuf)
	if err != nil {
		return nil, err
	}
	return pcmBuf[:n], nil
}
