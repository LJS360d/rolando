package logger

import (
	"bytes"
	"encoding/json"
	"net/http"
	"regexp"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type WebhookSyncer struct {
	zapcore.WriteSyncer
	url   string
	mutex sync.Mutex
}

// Write appends log messages to the buffer.
func (ws *WebhookSyncer) Write(p []byte) (n int, err error) {
	ws.mutex.Lock()
	defer ws.mutex.Unlock()

	content := removeANSICodes(string(p))
	payload := map[string]string{
		"content": content,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}

	_, err = http.Post(ws.url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return 0, err
	}

	return len(p), nil
}

func (ws *WebhookSyncer) Sync() error {
	return nil
}

func removeANSICodes(input string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return re.ReplaceAllString(input, "")
}

func GetWebhookEncoder() zapcore.Encoder {
	cfg := zap.NewDevelopmentEncoderConfig()
	cfg.EncodeTime = nil
	cfg.EncodeCaller = nil
	cfg.StacktraceKey = ""
	return zapcore.NewConsoleEncoder(cfg)
}
