package logger

import (
	"log"
	"os"
	"rolando/internal/config"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	logger *zap.Logger
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
	log.Println("Initializing logger...")
	webhookURL := config.LogWebhook
	var level zapcore.Level
	if config.Env == "development" {
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
			GetWebhookEncoder(),
			zapcore.AddSync(&WebhookSyncer{url: webhookURL}),
			level,
		)
		cores = append(cores, webhookCore)
	}

	core := zapcore.NewTee(cores...)

	logger = zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	defer logger.Sync()
	log.Println("Logger initialized")
}

func getConsoleEncoder() zapcore.Encoder {
	// Time encoder
	timeEncoder := func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(ColorGray + t.Format("[02/01/2006 15:04:05]") + ColorReset)
	}

	// Create encoder and core
	return zapcore.NewConsoleEncoder(zapcore.EncoderConfig{
		TimeKey:     "time",
		EncodeTime:  timeEncoder,
		LevelKey:    "level",
		EncodeLevel: zapcore.CapitalColorLevelEncoder,
		MessageKey:  "msg",
	})
}

// exposed zap logger methods

func Debug(msg string, fields ...zap.Field) {
	logger.Debug(msg, fields...)
}

func Info(msg string, fields ...zap.Field) {
	logger.Info(msg, fields...)
}

func Warn(msg string, fields ...zap.Field) {
	logger.Warn(msg, fields...)
}

func Error(msg string, fields ...zap.Field) {
	logger.Error(msg, fields...)
}

func DPanic(msg string, fields ...zap.Field) {
	logger.DPanic(msg, fields...)
}

func Panic(msg string, fields ...zap.Field) {
	logger.Panic(msg, fields...)
}

func Fatal(msg string, fields ...zap.Field) {
	logger.Fatal(msg, fields...)
}

// sugared logger format methods

func Debugf(template string, args ...any) {
	logger.Sugar().Debugf(template, args...)
}

func Infof(template string, args ...any) {
	logger.Sugar().Infof(template, args...)
}

func Warnf(template string, args ...any) {
	logger.Sugar().Warnf(template, args...)
}

func Errorf(template string, args ...any) {
	logger.Sugar().Errorf(template, args...)
}

func DPanicf(template string, args ...any) {
	logger.Sugar().DPanicf(template, args...)
}

func Panicf(template string, args ...any) {
	logger.Sugar().Panicf(template, args...)
}

func Fatalf(template string, args ...any) {
	logger.Sugar().Fatalf(template, args...)
}

// sugared logger ln methods

func Debugln(args ...any) {
	logger.Sugar().Debugln(args...)
}

func Infoln(args ...any) {
	logger.Sugar().Infoln(args...)
}

func Warnln(args ...any) {
	logger.Sugar().Warnln(args...)
}

func Errorln(args ...any) {
	logger.Sugar().Errorln(args...)
}

func DPanicln(args ...any) {
	logger.Sugar().DPanicln(args...)
}

func Panicln(args ...any) {
	logger.Sugar().Panicln(args...)
}

func Fatalln(args ...any) {
	logger.Sugar().Fatalln(args...)
}
