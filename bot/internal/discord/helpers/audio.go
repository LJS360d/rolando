package helpers

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/disgoorg/disgo/voice"
)

// SendTTSToConn streams audio from the provider to the voice connection.
func SendTTSToConn(ctx context.Context, conn voice.Conn, provider voice.OpusFrameProvider) error {
	if conn == nil {
		return fmt.Errorf("voice connection is nil")
	}
	if provider == nil {
		return fmt.Errorf("opus frame provider is nil")
	}

	if err := conn.SetSpeaking(ctx, voice.SpeakingFlagMicrophone); err != nil {
		return fmt.Errorf("failed to set speaking: %w", err)
	}

	done := make(chan error, 1)
	wrapped := &trackingProvider{
		OpusFrameProvider: provider,
		onFinished:        func(err error) { done <- err },
	}

	conn.SetOpusFrameProvider(wrapped)

	select {
	case <-ctx.Done():
		conn.SetOpusFrameProvider(nil)
		conn.SetSpeaking(ctx, 0)
		return ctx.Err()
	case err := <-done:
		conn.SetSpeaking(ctx, 0)
		conn.SetOpusFrameProvider(nil)
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
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
