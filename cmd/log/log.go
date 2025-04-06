package log

import (
	"os"
	"rolando/config"
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
	var level zapcore.Level
	if config.Env != "production" {
		level = zap.DebugLevel
	} else {
		level = zap.InfoLevel
	}
	consoleCore := zapcore.NewCore(
		getConsoleEncoder(),
		zapcore.AddSync(os.Stdout),
		level,
	)

	cores := []zapcore.Core{consoleCore}

	if webhookURL != "" {
		webhookCore := zapcore.NewCore(
			getWebhookEncoder(),
			zapcore.AddSync(&WebhookSyncer{url: webhookURL}),
			level,
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

	// Caller encoder
	callerEncoder := func(c zapcore.EntryCaller, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(ColorGray + c.TrimmedPath() + ColorReset)
	}

	// Create encoder and core
	return zapcore.NewConsoleEncoder(zapcore.EncoderConfig{
		TimeKey:      "time",
		EncodeTime:   timeEncoder,
		LevelKey:     "level",
		EncodeLevel:  zapcore.CapitalColorLevelEncoder,
		MessageKey:   "msg",
		CallerKey:    "caller",
		EncodeCaller: callerEncoder,
	})
}

func getWebhookEncoder() zapcore.Encoder {
	cfg := zap.NewDevelopmentEncoderConfig()
	cfg.EncodeTime = nil
	cfg.EncodeCaller = nil
	return zapcore.NewConsoleEncoder(cfg)
}
