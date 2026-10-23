package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"

	"rolando/internal/config"
)

// app logs this codebase, debug included in development. Libraries log via
// slog.Default, capped at info: disgo's debug output includes the bot token.
var app *slog.Logger

func init() {
	h := &handler{w: os.Stdout, mu: &sync.Mutex{}, level: slog.LevelInfo, color: config.Env == "development"}
	slog.SetDefault(slog.New(h))
	if config.Env == "development" {
		dev := *h
		dev.level = slog.LevelDebug
		h = &dev
	}
	app = slog.New(h)
}

func Debugf(format string, args ...any) { app.Debug(fmt.Sprintf(format, args...)) }
func Infof(format string, args ...any)  { app.Info(fmt.Sprintf(format, args...)) }
func Warnf(format string, args ...any)  { app.Warn(fmt.Sprintf(format, args...)) }
func Errorf(format string, args ...any) { app.Error(fmt.Sprintf(format, args...)) }

func Fatalf(format string, args ...any) {
	app.Error(fmt.Sprintf(format, args...))
	os.Exit(1)
}

const (
	gray    = "\033[90m"
	magenta = "\033[35m"
	blue    = "\033[34m"
	yellow  = "\033[33m"
	red     = "\033[31m"
	reset   = "\033[0m"
)

// handler prints "[02/01/2006 15:04:05]\tLEVEL\tmsg key=value...", colored in
// development.
type handler struct {
	w     io.Writer
	mu    *sync.Mutex
	level slog.Level
	color bool
	attrs string
}

func (h *handler) Enabled(_ context.Context, l slog.Level) bool { return l >= h.level }

func (h *handler) Handle(_ context.Context, r slog.Record) error {
	var b strings.Builder
	b.WriteString(h.paint(gray, r.Time.Format("[02/01/2006 15:04:05]")))
	b.WriteByte('\t')
	b.WriteString(h.paint(levelColor(r.Level), r.Level.String()))
	b.WriteByte('\t')
	b.WriteString(r.Message)
	b.WriteString(h.attrs)
	r.Attrs(func(a slog.Attr) bool {
		writeAttr(&b, a)
		return true
	})
	b.WriteByte('\n')
	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := io.WriteString(h.w, b.String())
	return err
}

func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	var b strings.Builder
	for _, a := range attrs {
		writeAttr(&b, a)
	}
	c := *h
	c.attrs += b.String()
	return &c
}

func (h *handler) WithGroup(string) slog.Handler { return h }

func writeAttr(b *strings.Builder, a slog.Attr) {
	if a.Equal(slog.Attr{}) {
		return
	}
	fmt.Fprintf(b, " %s=%v", a.Key, a.Value)
}

func (h *handler) paint(color, s string) string {
	if !h.color {
		return s
	}
	return color + s + reset
}

func levelColor(l slog.Level) string {
	switch {
	case l >= slog.LevelError:
		return red
	case l >= slog.LevelWarn:
		return yellow
	case l >= slog.LevelInfo:
		return blue
	}
	return magenta
}
