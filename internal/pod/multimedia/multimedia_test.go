package multimedia

import (
	"context"
	"strings"
	"testing"
)

func TestMultimediaWhisperPod(t *testing.T) {
	pod := NewMultimediaWhisperPod()

	if pod.ID() != "POD_MULTIMEDIA_WHISPER" {
		t.Fatalf("Expected ID POD_MULTIMEDIA_WHISPER, got %s", pod.ID())
	}

	ctx := context.Background()

	// Test Case 1: WhatsApp Audio Query with DryRun
	res1, err := pod.ProcessQuery(ctx, "tenant_acme", "Quiero transcribir una nota de voz de audio MP3 de WhatsApp", true)
	if err != nil {
		t.Fatalf("Test Case 1 failed: %v", err)
	}
	if !strings.Contains(res1.Answer, "Transcripción de audio") {
		t.Errorf("Expected Transcripción de audio in answer, got: %s", res1.Answer)
	}
	if res1.DryRunResult == nil || !res1.DryRunResult.IsDryRun {
		t.Errorf("Expected valid DryRunResult, got nil or false")
	}
	if res1.DryRunResult.ActionName != "transcribir_audio_whatsapp" {
		t.Errorf("Expected ActionName transcribir_audio_whatsapp, got %s", res1.DryRunResult.ActionName)
	}

	// Test Case 2: MP4 Video Query
	res2, err := pod.ProcessQuery(ctx, "tenant_acme", "Transcribir video mp4 de capacitación", true)
	if err != nil {
		t.Fatalf("Test Case 2 failed: %v", err)
	}
	if res2.DryRunResult.ActionName != "transcribir_video_capacitacion" {
		t.Errorf("Expected ActionName transcribir_video_capacitacion, got %s", res2.DryRunResult.ActionName)
	}
}
