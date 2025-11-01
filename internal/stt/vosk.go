package stt

import (
	"encoding/json"
	"fmt"
	"io"
	"rolando/cmd/data"
	"rolando/internal/logger"
	"rolando/internal/utils"
	"sync"
	"time"

	vosk "github.com/alphacep/vosk-api/go"
)

var (
	loadedModels = make(map[string]*vosk.VoskModel)
	modelsMutex  sync.Mutex
	// guildId -> Recognizers
	recognizers = make(map[string]*vosk.VoskRecognizer)
)

func init() {
	vosk.SetLogLevel(-1)
	startTime := time.Now()
	err := utils.ParallelTaskRunner(data.Langs, func(lang string) error {
		_, err := loadModel(lang)
		if err != nil {
			return fmt.Errorf("error loading model %s: %v", lang, err)
		}
		logger.Debugf("Loaded vosk model '%s'", lang)
		return nil
	})
	if err != nil {
		logger.Fatalf("Error loading models: %v", err)
	}
	logger.Infof("Loaded %d models in %s", len(data.Langs), time.Since(startTime))
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

func loadRecognizer(lang string, guildId string) (*vosk.VoskRecognizer, error) {
	rec, ok := recognizers[guildId]
	if !ok {
		model, err := loadModel(lang)
		if err != nil {
			return nil, fmt.Errorf("failed to load vosk model %s: %v", lang, err)
		}
		rec, err = vosk.NewRecognizer(model, 48000.0)
		if err != nil {
			return nil, fmt.Errorf("failed to create recognizer for model %s in guild %s: %v", lang, guildId, err)
		}
		// grammar := fmt.Sprintf(`["[unk]", "rolando:%f"]`, 20.0)
		// rec.SetGrm(grammar)
		rec.SetWords(1)
		recognizers[guildId] = rec
	}
	return rec, nil
}

func FreeRecognizer(guildId string) {
	rec, ok := recognizers[guildId]
	if !ok {
		// NTD
		logger.Warnf("Tried to free a non-existent recognizer for guildId %s, ignoring", guildId)
		return
	}
	rec.Free()
	delete(recognizers, guildId)
}

func SpeechToTextNative(audio io.Reader, lang string, guildId string) (string, error) {
	var bytes []byte
	_, err := audio.Read(bytes)
	if err != nil {
		return "", err
	}
	return SpeechToTextNativeFromBytes(bytes, lang, guildId)
}

func SpeechToTextNativeFromBytes(bytes []byte, lang string, guildId string) (string, error) {
	rec, err := loadRecognizer(lang, guildId)
	if err != nil {
		return "", err
	}
	rec.AcceptWaveform(bytes)
	jsonStr := rec.PartialResult()
	var result struct {
		Text string `json:"partial"`
	}
	err = json.Unmarshal([]byte(jsonStr), &result)
	if err != nil {
		return "", err
	}
	return result.Text, nil
}
