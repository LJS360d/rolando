package tts

import (
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"

	"github.com/disgoorg/disgo/voice"
	"layeh.com/gopus"
)

// cmdCleanupReader wraps an io.ReadCloser to clean up resources when closed
type cmdCleanupReader struct {
	io.ReadCloser
	cmd       *exec.Cmd
	tempFiles []string
	listFile  string
}

func (r *cmdCleanupReader) Close() error {
	// Close the underlying reader
	err := r.ReadCloser.Close()

	// Wait for the command to finish
	cmdErr := r.cmd.Wait()

	// Clean up temp files
	for _, tempFile := range r.tempFiles {
		os.Remove(tempFile)
	}
	os.Remove(r.listFile)

	// Return the first error encountered, if any
	if err != nil {
		return err
	}
	if cmdErr != nil {
		return fmt.Errorf("command failed: %w", cmdErr)
	}
	return nil
}

// pcmOpusReader reads PCM audio, encodes it to Opus frames, and outputs them
// with the 4-byte little-endian length prefix format expected by disgo's OpusReader.
type pcmOpusReader struct {
	pcmReader io.ReadCloser
	encoder   *gopus.Encoder
	cleanup   io.Closer
}

func newPCMOpusReader(pcmReader io.ReadCloser, cleanup io.Closer) (*pcmOpusReader, error) {
	encoder, err := gopus.NewEncoder(48000, 2, gopus.Voip)
	if err != nil {
		return nil, fmt.Errorf("failed to create opus encoder: %w", err)
	}

	return &pcmOpusReader{
		pcmReader: pcmReader,
		encoder:   encoder,
		cleanup:   cleanup,
	}, nil
}

// ProvideOpusFrame reads PCM data, encodes it to Opus, and returns the frame
// with a 4-byte length prefix as required by disgo.
func (r *pcmOpusReader) ProvideOpusFrame() ([]byte, error) {
	// Read PCM data: 960 samples per channel × 2 channels × 2 bytes = 3840 bytes per 20ms frame
	pcmBytes := make([]byte, 3840)
	n, err := io.ReadFull(r.pcmReader, pcmBytes)
	if err != nil && err != io.ErrUnexpectedEOF {
		if err == io.EOF && n == 0 {
			return nil, io.EOF
		}
		return nil, err
	}
	if n == 0 {
		return nil, io.EOF
	}

	// Convert bytes to int16 samples (little-endian)
	bytesRead := n
	samples := bytesRead / 2
	pcmSamples := make([]int16, 1920)
	for i := 0; i < samples; i++ {
		pcmSamples[i] = int16(pcmBytes[i*2]) | int16(pcmBytes[i*2+1])<<8
	}

	// Pad with zeros if we got less than a full frame
	for i := samples; i < 1920; i++ {
		pcmSamples[i] = 0
	}

	// Encode to Opus (frameSize is samples per channel: 960 for 20ms at 48kHz)
	opusData, err := r.encoder.Encode(pcmSamples, 960, 3840)
	if err != nil {
		return nil, fmt.Errorf("opus encode error: %w", err)
	}

	// Create output buffer with 4-byte little-endian length prefix
	output := make([]byte, 4+len(opusData))
	binary.LittleEndian.PutUint32(output[:4], uint32(len(opusData)))
	copy(output[4:], opusData)

	return output, nil
}

// Close releases all resources associated with the reader.
func (r *pcmOpusReader) Close() {
	r.pcmReader.Close()
	if r.cleanup != nil {
		r.cleanup.Close()
	}
}

// CreateSpeechPCM generates PCM audio from TTS using Google Translate.
// Returns an io.ReadCloser that provides raw PCM data (s16le, 48kHz, stereo).
func CreateSpeechPCM(text string, lang string) (io.ReadCloser, error) {
	data := []rune(text)

	maxChunkSize := 32
	chunkSize := min(len(data), maxChunkSize)

	tempFiles := make([]string, 0)

	// Process text in chunks and download each to temporary files
	for i := 0; i < len(data); i += chunkSize {
		end := min(i+chunkSize, len(data))

		chunk := string(data[i:end])
		url := fmt.Sprintf("http://translate.google.com/translate_tts?ie=UTF-8&total=1&idx=0&textlen=%d&client=tw-ob&q=%s&tl=%s", len(chunk), url.QueryEscape(chunk), lang)
		resp, err := http.Get(url)
		if err != nil {
			return nil, err
		}

		// Check if the response is successful
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("HTTP request failed with status: %d", resp.StatusCode)
		}

		// Create a temporary file for this chunk
		tempFile, err := os.CreateTemp("", "tts_chunk_*.mp3")
		if err != nil {
			resp.Body.Close()
			return nil, err
		}

		_, err = io.Copy(tempFile, resp.Body)
		resp.Body.Close()
		if err != nil {
			tempFile.Close()
			os.Remove(tempFile.Name())
			return nil, err
		}
		tempFile.Close()

		tempFiles = append(tempFiles, tempFile.Name())
	}

	// Create a file listing all the temp files for FFmpeg concat
	listFile, err := os.CreateTemp("", "tts_list_*.txt")
	if err != nil {
		for _, tempFile := range tempFiles {
			os.Remove(tempFile)
		}
		return nil, err
	}

	for _, tempFile := range tempFiles {
		fmt.Fprintf(listFile, "file '%s'\n", tempFile)
	}
	listFile.Close()

	// Decode to raw PCM: s16le, 48000Hz, stereo
	pcmCmd := exec.Command("ffmpeg",
		"-f", "concat",
		"-safe", "0",
		"-i", listFile.Name(),
		"-f", "s16le",
		"-ar", "48000",
		"-ac", "2",
		"-f", "ogg",
		"-loglevel", "error",
		"pipe:1",
	)

	stdout, err := pcmCmd.StdoutPipe()
	if err != nil {
		for _, tempFile := range tempFiles {
			os.Remove(tempFile)
		}
		os.Remove(listFile.Name())
		return nil, err
	}

	if err := pcmCmd.Start(); err != nil {
		for _, tempFile := range tempFiles {
			os.Remove(tempFile)
		}
		os.Remove(listFile.Name())
		return nil, err
	}

	// Create a cleanup reader that waits for the command to finish
	cleanupReader := &cmdCleanupReader{
		ReadCloser: stdout,
		cmd:        pcmCmd,
		tempFiles:  tempFiles,
		listFile:   listFile.Name(),
	}

	return cleanupReader, nil
}

// GenerateTTSProvider creates an OpusFrameProvider that reads TTS audio,
// encodes it to Opus frames with proper length prefixes for disgo.
func GenerateTTSProvider(text, lang string) (voice.OpusFrameProvider, error) {
	pcmReader, err := CreateSpeechPCM(text, lang)
	if err != nil {
		return nil, err
	}

	opusReader, err := newPCMOpusReader(pcmReader, pcmReader)
	if err != nil {
		pcmReader.Close()
		return nil, err
	}

	return opusReader, nil
}
