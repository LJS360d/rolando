package helpers

import (
	"rolando/internal/logger"
	"rolando/internal/stt"
	"sync"

	"github.com/disgoorg/disgo/voice"
	"github.com/disgoorg/snowflake/v2"
	"layeh.com/gopus"
)

// PCMPacket is one decoded frame of a speaker, mono at stt.SampleRate.
type PCMPacket struct {
	UserID snowflake.ID
	PCM    []int16
}

// SpeechReceiver implements voice.OpusFrameReceiver, decoding straight to
// the mono rate Vosk wants instead of 48kHz stereo.
type SpeechReceiver struct {
	// decoders holds the Opus decoders, keyed by SSRC.
	decoders map[uint32]*gopus.Decoder
	ssrcs    map[snowflake.ID]uint32
	mu       sync.Mutex
	pcmChan  chan *PCMPacket
	closed   bool
}

// 60ms, the longest Opus frame
const maxFrameSamples = stt.SampleRate * 60 / 1000

func NewSpeechReceiver(pcmChan chan *PCMPacket) *SpeechReceiver {
	return &SpeechReceiver{
		decoders: make(map[uint32]*gopus.Decoder),
		ssrcs:    make(map[snowflake.ID]uint32),
		pcmChan:  pcmChan,
	}
}

// ReceiveOpusFrame is called by disgo for each incoming voice packet.
func (r *SpeechReceiver) ReceiveOpusFrame(userID snowflake.ID, packet *voice.Packet) error {
	if len(packet.Opus) < 5 {
		// Skip invalid/too-small packets
		return nil
	}

	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil
	}
	decoder, ok := r.decoders[packet.SSRC]
	if !ok {
		var err error
		decoder, err = gopus.NewDecoder(stt.SampleRate, 1)
		if err != nil {
			r.mu.Unlock()
			logger.Errorf("error creating opus decoder: %v", err)
			return err
		}
		r.decoders[packet.SSRC] = decoder
		if userID != 0 {
			r.ssrcs[userID] = packet.SSRC
		}
	}
	r.mu.Unlock()

	pcm, err := decoder.Decode(packet.Opus, maxFrameSamples, false)
	if err != nil {
		// Drop invalid packets silently - don't spam logs
		return nil
	}

	select {
	case r.pcmChan <- &PCMPacket{UserID: userID, PCM: pcm}:
	default:
		// Channel full/slow consumer, packet is dropped to avoid blocking
	}
	return nil
}

func (r *SpeechReceiver) CleanupUser(userID snowflake.ID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if ssrc, ok := r.ssrcs[userID]; ok {
		delete(r.decoders, ssrc)
		delete(r.ssrcs, userID)
	}
}

// Close drops the decoders and stops forwarding packets. pcmChan is never
// closed: disgo's receive goroutine may still be delivering a frame.
func (r *SpeechReceiver) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	clear(r.decoders)
	clear(r.ssrcs)
}
