package helpers

import (
	"context"
	"errors"
	"fmt"

	"github.com/disgoorg/disgo/voice"
)

// SendTTSToConn sets the provider and waits for it to finish.
func SendTTSToConn(ctx context.Context, conn voice.Conn, provider voice.OpusFrameProvider) error {
	if conn == nil {
		return errors.New("voice connection is nil")
	}

	done := make(chan error, 1)

	// Wrap the provider to detect when it finishes (EOF)
	wrapped := &trackingProvider{
		OpusFrameProvider: provider,
		onFinished: func(err error) {
			done <- err
		},
	}

	if err := conn.SetSpeaking(ctx, voice.SpeakingFlagMicrophone); err != nil {
		return fmt.Errorf("failed to set speaking: %w", err)
	}

	conn.SetOpusFrameProvider(wrapped)

	// Block until the internal audio sender hits EOF or context is cancelled
	select {
	case <-ctx.Done():
		conn.SetOpusFrameProvider(nil)
		return ctx.Err()
	case err := <-done:
		conn.SetSpeaking(ctx, 0)
		conn.SetOpusFrameProvider(nil)
		return err
	}
}

// trackingProvider signals completion when ProvideOpusFrame returns an error/EOF
type trackingProvider struct {
	voice.OpusFrameProvider
	onFinished func(error)
}

func (p *trackingProvider) ProvideOpusFrame() ([]byte, error) {
	frame, err := p.OpusFrameProvider.ProvideOpusFrame()
	if err != nil {
		// Signal EOF or error back to the Wait loop
		p.onFinished(err)
	}
	return frame, err
}
