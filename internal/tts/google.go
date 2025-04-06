package tts

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"

	"github.com/jonas747/ogg"
)

func CreateSpeechBuff(text string, lang string) (io.Reader, error) {
	data := []rune(text)

	chunkSize := len(data)
	if len(data) > 32 {
		chunkSize = 32
	}

	urls := make([]string, 0)
	for prev, i := 0, 0; i < len(data); i++ {
		if i%chunkSize == 0 && i != 0 {
			chunk := string(data[prev:i])
			url := fmt.Sprintf("http://translate.google.com/translate_tts?ie=UTF-8&total=1&idx=0&textlen=%d&client=tw-ob&q=%s&tl=%s", chunkSize, url.QueryEscape(chunk), lang)
			urls = append(urls, url)
			prev = i
		} else if i == len(data)-1 {
			chunk := string(data[prev:])
			url := fmt.Sprintf("http://translate.google.com/translate_tts?ie=UTF-8&total=1&idx=0&textlen=%d&client=tw-ob&q=%s&tl=%s", chunkSize, url.QueryEscape(chunk), lang)
			urls = append(urls, url)
			prev = i
		}
	}

	buf := new(bytes.Buffer)
	for _, url := range urls {
		r, err := http.Get(url)
		if err != nil {
			return nil, err
		}

		_, err = buf.ReadFrom(r.Body)
		if err != nil {
			return nil, err
		}
		r.Body.Close()
	}

	return buf, nil
}

func GenerateTTSDecoder(text, lang string) (*ogg.PacketDecoder, error) {
	buff, err := CreateSpeechBuff(text, lang)
	if err != nil {
		return nil, err
	}

	// Use ffmpeg to pipe MP3 to OGG (no temporary file)
	cmd := exec.Command("ffmpeg",
		"-i", "pipe:0", // Input: Pipe from stdin (MP3 buffer)
		"-acodec", "libopus", // Use libopus codec
		"-b:a", "96K", // Audio bitrate
		"-ar", "48000", // Sample rate
		"-ac", "2", // Channels: stereo
		"-f", "ogg", // Output format: Ogg container
		"pipe:1", // Output to pipe (stdout)
	)

	// Redirect stderr to os.DevNull (or "NUL" on Windows)
	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	defer devNull.Close()

	cmd.Stderr = devNull
	cmd.Stdin = buff

	// Get the output pipe from ffmpeg
	oggPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	// Start the command asynchronously
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	// Create the OGG decoder from the ffmpeg output stream
	decoder := ogg.NewPacketDecoder(ogg.NewDecoder(bufio.NewReaderSize(oggPipe, 65307)))
	return decoder, nil
}
