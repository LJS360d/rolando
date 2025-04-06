package utils

import (
	"bufio"
	"errors"
	"io"
	"os/exec"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/jonas747/ogg"
)

func ConvertMP3ToOpusDecoder(mp3Path string) (*ogg.PacketDecoder, error) {

	cmd := exec.Command("ffmpeg",
		"-i", mp3Path,
		"-acodec", "libopus", // Use libopus codec
		"-b:a", "96K", // Audio bitrate
		"-ar", "48000", // Sample rate
		"-ac", "2", // Channels: stereo
		"-f", "ogg", // Output format: Ogg container
		"pipe:1", // Write output to stdout
	)

	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err = cmd.Start(); err != nil {
		return nil, err
	}

	// decode the ogg buffer
	d := ogg.NewPacketDecoder(ogg.NewDecoder(bufio.NewReaderSize(pipe, 65307)))
	return d, nil
}

// ----- Streaming audio -----

func StreamAudio(vc *discordgo.VoiceConnection, audio io.Reader) error {
	vc.Speaking(true)
	defer vc.Speaking(false)

	buffer := make([]byte, 960)
	packetCount := 0

	for {
		n, err := audio.Read(buffer)
		if err == io.EOF || n == 0 {
			break
		}
		if err != nil {
			return errors.New("error reading audio: " + err.Error())
		}

		vc.OpusSend <- buffer[:n]

		packetCount++
		time.Sleep(time.Millisecond * 20)
	}

	return nil
}

func StreamAudioBuffer(vc *discordgo.VoiceConnection, audioBuffer []byte) error {
	vc.Speaking(true)
	defer vc.Speaking(false)

	chunkSize := 960 // Opus frame size for 20ms of audio at 48kHz, stereo
	totalLength := len(audioBuffer)
	packetCount := 0

	for offset := 0; offset < totalLength; offset += chunkSize {
		end := offset + chunkSize
		if end > totalLength {
			end = totalLength
		}

		vc.OpusSend <- audioBuffer[offset:end]

		packetCount++
		time.Sleep(time.Millisecond * 20) // Maintain the 20ms timing for each frame
	}

	return nil
}

func StreamAudioDecoder(vc *discordgo.VoiceConnection, decoder *ogg.PacketDecoder) error {
	vc.Speaking(true)
	defer vc.Speaking(false)

	// Create a buffer to hold the decoded packets
	packetCount := 0

	for {
		// Decode the next packet from the Ogg stream
		packet, _, err := decoder.Decode()
		if err != nil {
			if err.Error() == "EOF" {
				// End of stream, stop streaming
				break
			}
			return errors.New("error decoding audio packet: " + err.Error())
		}

		// Send the decoded Opus packet to Discord
		vc.OpusSend <- packet

		packetCount++

		// Sleep to maintain the 20ms timing for each Opus frame
		// You may adjust the sleep duration depending on the packet size and rate
		time.Sleep(time.Millisecond * 20)
	}

	return nil
}
