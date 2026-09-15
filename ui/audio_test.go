package ui

import (
	"strings"
	"testing"
	"time"
)

func TestAudioDefaultsToNoPreload(t *testing.T) {
	got := render(t, Audio(nil, "/voice/1", AudioOpts{
		Title: "Recording", Type: "audio/webm", Duration: 12 * time.Second, Download: "/voice/1/download",
	}))
	for _, want := range []string{
		`<audio class="ui-audio-player" controls preload="none" aria-label="Recording">`,
		`<source src="/voice/1" type="audio/webm">`,
		`12.0 s`, `href="/voice/1/download"`, `target="_blank"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %s", want, got)
		}
	}
}

func TestAudioUnsupportedTypeFallsBackToDownload(t *testing.T) {
	got := render(t, Audio(nil, "/voice/1", AudioOpts{Type: "application/octet-stream", Download: "/download"}))
	if strings.Contains(got, "<audio") || !strings.Contains(got, "cannot be played") || !strings.Contains(got, `href="/download"`) {
		t.Fatalf("unsupported audio did not fall back: %s", got)
	}
}

func TestAudioOptions(t *testing.T) {
	got := render(t, Audio(nil, "/voice/1", AudioOpts{Type: "audio/mpeg", Preload: PreloadMetadata, NoOpen: true}))
	if !strings.Contains(got, `preload="metadata"`) || strings.Contains(got, `target="_blank"`) {
		t.Fatalf("audio options not applied: %s", got)
	}
}
