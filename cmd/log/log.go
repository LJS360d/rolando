package log

import (
	"fmt"
	"os"
	"rolando/config"
	"strings"
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

func init() {
	webhookURL := config.LogWebhook

	consoleCore := zapcore.NewCore(
		getConsoleEncoder(),
		zapcore.AddSync(os.Stdout),
		zap.DebugLevel,
	)

	cores := []zapcore.Core{consoleCore}

	if webhookURL != "" {
		webhookCore := zapcore.NewCore(
			getWebhookEncoder(),
			zapcore.AddSync(&WebhookSyncer{url: webhookURL}),
			zap.InfoLevel,
		)
		cores = append(cores, webhookCore)
	}

	core := zapcore.NewTee(cores...)

	logger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	defer logger.Sync()
	Log = logger.Sugar()
}

func getConsoleEncoder() zapcore.Encoder {
	// Time encoder
	timeEncoder := func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(ColorGray + t.Format("[02/01/2006 15:04:05]") + ColorReset)
	}

	// Level encoder
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

	// Caller encoder
	callerEncoder := func(c zapcore.EntryCaller, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(ColorGray + fmt.Sprintf("%s:%d", c.Function, c.Line) + ColorReset)
	}

	// Create encoder and core
	return zapcore.NewConsoleEncoder(zapcore.EncoderConfig{
		TimeKey:      "time",
		EncodeTime:   timeEncoder,
		LevelKey:     "level",
		EncodeLevel:  levelEncoder,
		MessageKey:   "msg",
		CallerKey:    "caller",
		EncodeCaller: callerEncoder,
	})
}

func getWebhookEncoder() zapcore.Encoder {
	cfg := zap.NewDevelopmentEncoderConfig()
	cfg.EncodeTime = nil
	return zapcore.NewConsoleEncoder(cfg)
}
