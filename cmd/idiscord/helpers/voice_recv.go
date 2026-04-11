package helpers

import (
	"rolando/internal/logger"
	"sync"

	"github.com/disgoorg/disgo/voice"
	"github.com/disgoorg/snowflake/v2"
	"layeh.com/gopus"
)

// PCMPacket includes the raw PCM data along with Discord packet metadata.
type PCMPacket struct {
	UserID    snowflake.ID
	SSRC      uint32
	Sequence  uint16
	Timestamp uint32
	PCM       []int16
}

// VoskOpusReceiver implements voice.OpusFrameReceiver for Vosk STT processing.
type VoskOpusReceiver struct {
	// decoders holds the Opus decoders, keyed by SSRC.
	decoders  map[uint32]*gopus.Decoder
	mu        sync.Mutex
	pcmChan   chan *PCMPacket
	chainID   string
	lang      string
	closeOnce sync.Once
}

// Technically the below settings can be adjusted however that poses
// a lot of other problems that are not handled well at this time.
// These below values seem to provide the best overall performance
const (
	channels  int = 2                   // 1 for mono, 2 for stereo
	frameRate int = 48000               // audio sampling rate
	frameSize int = 960                 // uint16 size of each audio frame
	maxBytes  int = (frameSize * 2) * 2 // max size of opus data
	// Represents 60ms of stereo audio at 48kHz: 48000 * 0.060 * 2 = 5760 samples
	maxFrameSizeSamples = 5760
)

// NewVoskOpusReceiver creates a new OpusFrameReceiver for Vosk STT.
func NewVoskOpusReceiver(chainID, lang string, pcmChan chan *PCMPacket) *VoskOpusReceiver {
	return &VoskOpusReceiver{
		decoders: make(map[uint32]*gopus.Decoder),
		pcmChan:  pcmChan,
		chainID:  chainID,
		lang:     lang,
	}
}

// ReceiveOpusFrame is called by disgo for each incoming voice packet.
// It decodes the Opus data to PCM and sends it to the pcmChan.
func (r *VoskOpusReceiver) ReceiveOpusFrame(userID snowflake.ID, packet *voice.Packet) error {
	if len(packet.Opus) < 5 {
		// Skip invalid/too-small packets
		return nil
	}

	// Get or create decoder for this SSRC
	r.mu.Lock()
	decoder, ok := r.decoders[packet.SSRC]
	if !ok {
		var err error
		decoder, err = gopus.NewDecoder(frameRate, channels)
		if err != nil {
			r.mu.Unlock()
			logger.Errorf("error creating opus decoder: %v", err)
			return err
		}
		r.decoders[packet.SSRC] = decoder
	}
	r.mu.Unlock()

	// Decode Opus to PCM
	pcm, err := decoder.Decode(packet.Opus, maxFrameSizeSamples, false)
	if err != nil {
		// Drop invalid packets silently - don't spam logs
		return nil
	}

	// Send decoded packet to the channel
	pcmPacket := &PCMPacket{
		UserID:    userID,
		SSRC:      packet.SSRC,
		Sequence:  packet.Sequence,
		Timestamp: packet.Timestamp,
		PCM:       pcm,
	}

	select {
	case r.pcmChan <- pcmPacket:
		// Successfully sent
	default:
		// Channel full/slow consumer, packet is dropped to avoid blocking
	}

	return nil
}

// CleanupUser is called when a user disconnects from voice.
func (r *VoskOpusReceiver) CleanupUser(userID snowflake.ID) {
	// We key decoders by SSRC, not userID, so we can't directly clean up here.
	// The SSRC→userID mapping is managed by disgo internally.
	// In practice, stale decoders don't cause issues since they're just memory.
	// If needed, we could track SSRC→userID and clean up on disconnect.
}

// Close closes the PCM channel and cleans up resources.
func (r *VoskOpusReceiver) Close() {
	r.closeOnce.Do(func() {
		r.mu.Lock()
		defer r.mu.Unlock()

		// Clean up all decoders
		for ssrc := range r.decoders {
			delete(r.decoders, ssrc)
		}

		// Close the PCM channel
		close(r.pcmChan)
	})
}
