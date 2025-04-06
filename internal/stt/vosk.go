package stt

import (
	"fmt"
	"io"
	"rolando/cmd/data"
	"rolando/internal/logger"
	"sync"

	vosk "github.com/alphacep/vosk-api/go"
)

var (
	loadedModels = make(map[string]*vosk.VoskModel)
	modelsMutex  sync.Mutex
)

func init() {
	vosk.SetLogLevel(-1)
	for _, lang := range data.Langs {
		if _, err := loadModel(lang); err != nil {
			logger.Errorf("Error loading model %s: %v", lang, err)
		} else {
			logger.Debugf("Loaded vosk model '%s'", lang)
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
	text := rec.FinalResult()
	return text, nil
}
