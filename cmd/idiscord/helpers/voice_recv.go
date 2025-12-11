package helpers

import (
	"rolando/internal/logger"
	"sync"

	"github.com/bwmarrin/discordgo"
	"layeh.com/gopus"
)

// PCMPacket includes the raw PCM data along with Discord packet metadata.
type PCMPacket struct {
	*discordgo.Packet
	PCM []int16
}

// VoiceReceiver manages the decoding state for incoming voice packets.
type VoiceReceiver struct {
	// decoders holds the Opus decoders, keyed by SSRC.
	decoders map[uint32]*gopus.Decoder
	mu       sync.Mutex
}

// NewVoiceReceiver creates a new state manager for receiving and decoding PCM audio.
func NewVoiceReceiver() *VoiceReceiver {
	return &VoiceReceiver{
		decoders: make(map[uint32]*gopus.Decoder),
	}
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

// ReceivePCM receives Opus packets from a VoiceConnection, decodes them to PCM,
// and sends the results on the provided channel. This function blocks.
func (vr *VoiceReceiver) ReceivePCM(v *discordgo.VoiceConnection, c chan<- *PCMPacket) {
	defer close(c)
	var err error

	for {
		if v.Status != discordgo.VoiceConnectionStatusReady || v.OpusRecv == nil {
			// A small delay to prevent a busy loop if the connection is temporarily down.
			// time.Sleep(100 * time.Millisecond)
			// You may want to log this or handle it gracefully, but returning here
			// will shut down the receiving goroutine.
			logger.Errorf("VoiceReceiver: VoiceConnection is not ready, exiting ReceivePCM")
			return
		}

		// Read Opus Packet
		p, ok := <-v.OpusRecv
		if !ok {
			// Channel closed (VoiceConnection shut down)
			return
		}
		if len(p.Opus) < 5 { // min of 5 bytes is a safe minimum for Discord Opus frames
			continue
		}

		// Get or Create Decoder
		vr.mu.Lock()
		decoder, ok := vr.decoders[p.SSRC]
		if !ok {
			// Create a new decoder for the SSRC
			decoder, err = gopus.NewDecoder(frameRate, channels)
			if err != nil {
				// Use the existing dgvoice OnError logger or a local one
				logger.Errorf("error creating opus decoder: %v", err)
				vr.mu.Unlock()
				continue
			}
			vr.decoders[p.SSRC] = decoder
		}
		vr.mu.Unlock()

		// Decode Opus to PCM
		pcm, err := decoder.Decode(p.Opus, maxFrameSizeSamples, false)
		if err != nil {
			// actually i dont give a fuck if a packet is dropped because invalid or because gopus wants to make a fuss about it
			// just move on, dont even want the logs in prod.
			// logger.Errorf("Error decoding opus data: %v", err)
			continue
		}

		// Send Decoded Packet
		select {
		case c <- &PCMPacket{Packet: p, PCM: pcm}:
			// Successfully sent
		default:
			// Channel full/slow consumer, packet is dropped to avoid blocking
		}
	}
}
