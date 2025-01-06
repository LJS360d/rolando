package log

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"rolando/config"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	Log *zap.SugaredLogger
)

// ANSI escape codes for coloring console output
const (
	ColorGray   string = "\033[90m"
	ColorWhite  string = "\033[97m"
	ColorGreen  string = "\033[32m"
	ColorYellow string = "\033[33m"
	ColorRed    string = "\033[31m"
	ColorReset  string = "\033[0m"
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

func init() {
	webhookURL := config.LogWebhook
	fmt.Println(webhookURL)
	var writeSyncers []zapcore.WriteSyncer

	writeSyncers = append(writeSyncers, zapcore.AddSync(os.Stdout))

	if webhookURL != "" {
		webhookSyncer := &WebhookSyncer{url: webhookURL}
		writeSyncers = append(writeSyncers, zapcore.AddSync(webhookSyncer))
	}

	timeEncoder := func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(ColorGray + t.Format("[02/01/2006 15:04:05]") + ColorReset)
	}

	levelEncoder := func(l zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
		var color string
		switch l {
		case zapcore.InfoLevel:
			color = ColorGreen
		case zapcore.WarnLevel:
			color = ColorYellow
		case zapcore.ErrorLevel, zapcore.FatalLevel:
			color = ColorRed
		default:
			color = ColorWhite
		}
		enc.AppendString(color + strings.ToUpper(l.String()) + ColorReset)
	}

	encoderCfg := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		EncodeTime:     timeEncoder,
		LevelKey:       "level",
		EncodeLevel:    levelEncoder,
		MessageKey:     "msg",
		CallerKey:      "caller",
		EncodeCaller:   zapcore.ShortCallerEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeName:     zapcore.FullNameEncoder,
	}

	encoder := zapcore.NewConsoleEncoder(encoderCfg)
	core := zapcore.NewCore(
		encoder,
		zapcore.NewMultiWriteSyncer(writeSyncers...),
		zap.InfoLevel,
	)

	logger := zap.New(core)
	defer logger.Sync()
	Log = logger.Sugar()
}
