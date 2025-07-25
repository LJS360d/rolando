package stt

import (
	"encoding/json"
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
	recognizers  = make(map[string]*vosk.VoskRecognizer)
)

func init() {
	vosk.SetLogLevel(-1)
	for _, lang := range data.Langs {
		if model, err := loadModel(lang); err != nil {
			logger.Errorf("Error loading model %s: %v", lang, err)
		} else {
			logger.Debugf("Loaded vosk model '%s'", lang)
			rec, err := vosk.NewRecognizer(model, 48000.0)
			if err != nil {
				logger.Errorf("Failed to create recognizer for model %s: %v", lang, err)
			}
			rec.SetWords(1)
			recognizers[lang] = rec
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
	rec, ok := recognizers[lang]
	if !ok {
		return "", fmt.Errorf("no recognizer for language %s", lang)
	}
	rec.AcceptWaveform(bytes)
	jsonStr := rec.Result()
	var result struct {
		Text string `json:"text"`
	}
	err := json.Unmarshal([]byte(jsonStr), &result)
	if err != nil {
		return "", err
	}
	return result.Text, nil
}
