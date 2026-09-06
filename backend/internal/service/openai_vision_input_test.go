package service

import (
	"context"
	"encoding/base64"
	"testing"
)

func TestOpenAIVisionInputFingerprintsDataURL(t *testing.T) {
	const raw = "pixel-canary"
	body := []byte(`{"model":"gpt-5.6-sol","input":[{"type":"message","role":"user","content":[{"type":"input_image","image_url":"data:image/png;base64,` + base64.StdEncoding.EncodeToString([]byte(raw)) + `"}]}]}`)

	fingerprints := openAIVisionInputFingerprints(body)
	if len(fingerprints) != 1 {
		t.Fatalf("expected one image fingerprint, got %d", len(fingerprints))
	}
	if fingerprints[0].representation != "data_url" {
		t.Fatalf("representation = %q", fingerprints[0].representation)
	}
	if fingerprints[0].mimeType != "image/png" {
		t.Fatalf("mime type = %q", fingerprints[0].mimeType)
	}
	if fingerprints[0].byteLength != int64(len(raw)) {
		t.Fatalf("byte length = %d", fingerprints[0].byteLength)
	}
	if fingerprints[0].sha256 == "" {
		t.Fatal("sha256 should be populated")
	}
}

func TestOpenAIRequestBodyMayContainImageInputDetectsResponsesAndMessages(t *testing.T) {
	for _, body := range []string{
		`{"input":[{"content":[{"type":"input_image","image_url":"https://example.test/canary.png"}]}]}`,
		`{"messages":[{"content":[{"type":"image_url","image_url":{"url":"https://example.test/canary.png"}}]}]}`,
	} {
		if !OpenAIRequestBodyMayContainImageInput([]byte(body)) {
			t.Fatalf("image input not detected: %s", body)
		}
	}
	if OpenAIRequestBodyMayContainImageInput([]byte(`{"input":"text only"}`)) {
		t.Fatal("text-only request was classified as image input")
	}
}

func TestLogOpenAIVisionInputDiagnosticsDoesNotFailWithoutLoggerContext(t *testing.T) {
	LogOpenAIVisionInputDiagnostics(context.Background(), "test", nil, "gpt-5.6-sol", []byte(`{"input":[{"type":"input_image","image_url":"data:image/png;base64,QQ=="}]}`))
}
