package tts

import (
	"errors"
	"io"
	"os/exec"
)

func CreateSpeechBuffNative(text, lang string) (io.Reader, error) {
	cmd := exec.Command("espeak-ng",
		"-v", lang,
		"-s", "120",
		"-p", "10",
		"--stdout",
		text,
	)

	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, errors.New("failed to generate speech: " + err.Error())
	}
	if err := cmd.Start(); err != nil {
		return nil, errors.New("failed to generate speech: " + err.Error())
	}

	return pipe, nil
}
