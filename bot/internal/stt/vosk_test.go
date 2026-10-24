package stt

import "testing"

func TestMissingModelIsRetried(t *testing.T) {
	for range 2 {
		if _, err := NewRecognizer("missing"); err == nil {
			t.Fatal("loaded a missing model")
		}
		if len(models) != 0 {
			t.Fatalf("failed load kept: %v", models)
		}
	}
}
