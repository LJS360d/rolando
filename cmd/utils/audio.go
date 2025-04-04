package utils

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"rolando/cmd/data"
	"rolando/cmd/log"
	"sync"
	"time"

	vosk "github.com/alphacep/vosk-api/go"
	"github.com/bwmarrin/discordgo"
	"github.com/jonas747/ogg"
	"github.com/pion/opus"
)

var (
	loadedModels = make(map[string]*vosk.VoskModel)
	modelsMutex  sync.Mutex
)

func init() {
	vosk.SetLogLevel(-1)
	for _, lang := range data.Langs {
		if _, err := loadModel(lang); err != nil {
			log.Log.Errorf("Error loading model %s: %v", lang, err)
		} else {
			log.Log.Infof("Loaded vosk model '%s'", lang)
		}
	}
}

func loadModel(lang string) (*vosk.VoskModel, error) {
	modelsMutex.Lock()
	defer modelsMutex.Unlock()

	if model, ok := loadedModels[lang]; ok {
		return model, nil
	}

	model, err := vosk.NewModel("vosk/models/" + lang)
	if err != nil {
		return nil, fmt.Errorf("failed to load model: %w", err)
	}

	loadedModels[lang] = model
	return model, nil
}

func SpeechToTextNative(audio io.Reader, lang string) (string, error) {
	var bytes []byte
	_, err := audio.Read(bytes)
	if err != nil {
		return "", err
	}
	return SpeechToTextNativeFromBytes(bytes, lang)
}

func SpeechToTextNativeFromBytes(bytes []byte, lang string) (string, error) {
	model, err := loadModel(lang)
	if err != nil {
		return "", err
	}

	sampleRate := 16000.0
	rec, err := vosk.NewRecognizer(model, sampleRate)
	if err != nil {
		return "", err
	}
	defer rec.Free()

	rec.SetWords(1)
	rec.AcceptWaveform(bytes)
	text := rec.PartialResult()
	return text, nil
}

func SpeechToTextNativeFromOpusBytes(opusData []byte, lang string) (string, error) {
	model, err := loadModel(lang)
	if err != nil {
		return "", err
	}

	sampleRate := 16000.0
	rec, err := vosk.NewRecognizer(model, sampleRate)
	if err != nil {
		return "", err
	}
	defer rec.Free()

	rec.SetWords(1)

	decoder := opus.NewDecoder() // 1 channel (mono)
	if err != nil {
		return "", fmt.Errorf("failed to create opus decoder: %w", err)
	}

	pcm := make([]byte, 960) // Allocate a buffer for PCM data
	_, _, err = decoder.Decode(opusData, pcm)
	if err != nil {
		return "", fmt.Errorf("failed to decode opus: %w", err)
	}

	rec.AcceptWaveform(pcm)
	text := rec.PartialResult()
	return text, nil
}

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

func CreateSpeechBuffNative(text, lang string) (io.Reader, error) {
	cmd := exec.Command("espeak-ng",
		"-v", lang,
		"-s", "120",
		"-p", "10",
		"--stdout",
		text,
	)

	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, errors.New("failed to generate speech: " + err.Error())
	}
	if err := cmd.Start(); err != nil {
		return nil, errors.New("failed to generate speech: " + err.Error())
	}

	return pipe, nil
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

// StreamAudioBuffer streams audio from a byte buffer of opus decoded bytes to a voice connection
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
		time.Sleep(time.Millisecond * 20)
	}

	return nil
}
