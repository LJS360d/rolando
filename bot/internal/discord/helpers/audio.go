package helpers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/disgoorg/disgo/voice"
	"github.com/disgoorg/snowflake/v2"
)

// NewAudioSender is the default disgo sender, except that a nil provider gets
// no sender at all: the default one would keep a 20ms loop running for the
// lifetime of the process, since Conn.Close never closes the sender.
func NewAudioSender(logger *slog.Logger, provider voice.OpusFrameProvider, conn voice.Conn) voice.AudioSender {
	if provider == nil {
		return noopSender{}
	}
	return voice.NewAudioSender(logger, provider, conn)
}

type noopSender struct{}

func (noopSender) Open()  {}
func (noopSender) Close() {}

const voiceOpenTimeout = 15 * time.Second

var ErrInVoice = errors.New("already in a voice channel in this guild")

// Session is the bot's presence in one guild's voice. A guild has at most
// one: a second join would reuse the live conn and, on timeout, close it,
// and a late close would disconnect whatever session came next.
type Session struct {
	Conn    voice.Conn
	guildID snowflake.ID
	// mu is held while joining; Leave and Current wait it out
	mu   sync.Mutex
	play sync.Mutex
	left sync.Once
}

var sessions sync.Map // snowflake.ID -> *Session

// Join opens a session in channelID, giving up after voiceOpenTimeout.
// ErrInVoice if the guild already has a session.
func Join(ctx context.Context, vm voice.Manager, guildID, channelID snowflake.ID) (*Session, error) {
	s := &Session{guildID: guildID}
	s.mu.Lock()
	if _, busy := sessions.LoadOrStore(guildID, s); busy {
		return nil, ErrInVoice
	}
	s.Conn = vm.CreateConn(guildID)
	openCtx, cancel := context.WithTimeout(ctx, voiceOpenTimeout)
	err := s.Conn.Open(openCtx, channelID, false, false)
	cancel()
	if err == nil && s.Conn.Gateway().Status() != voice.StatusReady {
		err = fmt.Errorf("voice gateway %v after open", s.Conn.Gateway().Status())
	}
	s.mu.Unlock()
	if err != nil {
		s.Leave()
		return nil, err
	}
	return s, nil
}

// InVoice reports whether the guild has a session, without waiting on a join.
func InVoice(guildID snowflake.ID) bool {
	_, ok := sessions.Load(guildID)
	return ok
}

// Current returns the guild's session once joined, or nil.
func Current(guildID snowflake.ID) *Session {
	v, ok := sessions.Load(guildID)
	if !ok {
		return nil
	}
	s := v.(*Session)
	s.mu.Lock()
	s.mu.Unlock()
	return s
}

// Leave closes the conn and frees the guild for the next Join. Only the
// first call does anything.
func (s *Session) Leave() {
	s.left.Do(func() {
		s.mu.Lock()
		// own deadline, so a done caller context still lets the leave go out
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		s.Conn.Close(ctx)
		cancel()
		s.mu.Unlock()
		sessions.CompareAndDelete(s.guildID, s)
	})
}

// Play streams provider once any playback in progress has finished: disgo
// keeps one unsynchronized audio sender per conn.
func (s *Session) Play(ctx context.Context, provider voice.OpusFrameProvider) error {
	s.play.Lock()
	defer s.play.Unlock()
	conn := s.Conn
	if err := conn.SetSpeaking(ctx, voice.SpeakingFlagMicrophone); err != nil {
		return fmt.Errorf("failed to set speaking: %w", err)
	}

	done := make(chan error, 1)
	var once sync.Once
	wrapped := &trackingProvider{
		OpusFrameProvider: provider,
		onFinished:        func(err error) { once.Do(func() { done <- err }) },
	}

	conn.SetOpusFrameProvider(wrapped)

	var err error
	select {
	case <-ctx.Done():
		err = ctx.Err()
	case err = <-done:
		if errors.Is(err, io.EOF) {
			err = nil
		}
	}
	conn.SetOpusFrameProvider(nil)
	// ctx may be done already, and speaking must still be cleared
	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	conn.SetSpeaking(stopCtx, 0)
	cancel()
	return err
}

type trackingProvider struct {
	voice.OpusFrameProvider
	onFinished func(error)
}

func (p *trackingProvider) ProvideOpusFrame() ([]byte, error) {
	frame, err := p.OpusFrameProvider.ProvideOpusFrame()
	if err != nil {
		p.onFinished(err)
	}
	return frame, err
}
