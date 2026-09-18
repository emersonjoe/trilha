package ui

import (
	"io"
	"log/slog"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// #269 — the recorder is a form that posts a file: the island is what the
// script adds on top, and it is never what the markup depends on.
func TestRecorderIsAFormThatPostsAFile(t *testing.T) {
	got := inApp(t, trilha.Config{}, func(c *trilha.Ctx) h.Node {
		return Recorder(c, RecorderOpts{Action: "/api/voice", Target: "answer", MaxSeconds: 60})
	})
	for _, want := range []string{
		`class="ui-recorder"`, `method="post"`, `action="/api/voice"`,
		`enctype="multipart/form-data"`, `data-trilha-upload="answer"`,
		`data-ui-recorder=""`, `data-ui-recorder-mime="audio/webm"`, `data-ui-recorder-max="60"`,
		// The fallback that always works, and the CSRF field of any form.
		`type="file" name="audio" accept="audio/*" capture=""`,
		`type="hidden" name="_csrf"`,
		// The island's parts: the button the script shows, the clock, the note.
		`data-ui-recorder-toggle="" hidden`, `data-ui-recorder-time=""`, `data-ui-recorder-note=""`,
		// The progress bar ui.upload.js fills in, and the button that sends.
		`data-trilha-progress=""`, `<button type="submit"`, `>Record<`, `>Send<`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %s", want, got)
		}
	}
	if strings.Contains(got, "<script") || regexp.MustCompile(` on[a-z]+="`).MatchString(got) {
		t.Fatalf("the recorder must not write script or inline handlers: %s", got)
	}
}

// The words are the kit's, in the language of the request: the page does not
// pass a label to get one that is not English.
func TestRecorderSpeaksBothLanguages(t *testing.T) {
	pt := inApp(t, trilha.Config{Locale: "pt-BR"}, func(c *trilha.Ctx) h.Node {
		return Recorder(c, RecorderOpts{Action: "/api/voz"})
	})
	for _, want := range []string{`>Gravar<`, `>Enviar<`, `data-ui-recorder-stop="Parar"`, "O microfone não foi liberado"} {
		if !strings.Contains(pt, want) {
			t.Fatalf("missing %q in %s", want, pt)
		}
	}
	en := inApp(t, trilha.Config{}, func(c *trilha.Ctx) h.Node {
		return Recorder(c, RecorderOpts{Action: "/api/voice", Label: "Speak", Name: "gravacao", Mime: "audio/ogg"})
	})
	for _, want := range []string{`>Speak<`, `name="gravacao"`, `data-ui-recorder-mime="audio/ogg"`, `data-ui-recorder-stop="Stop"`} {
		if !strings.Contains(en, want) {
			t.Fatalf("missing %q in %s", want, en)
		}
	}
	// Without a Target there is nothing to swap, so the form is a plain form.
	if strings.Contains(en, "data-trilha-upload") {
		t.Fatalf("a recorder with no Target must not claim a swap: %s", en)
	}
}

// The behaviour rides in a file of its own, like navigation and upload: a page
// without a recorder downloads nothing.
func TestRecorderScript(t *testing.T) {
	for _, name := range Files {
		if name == "ui.recorder.js" {
			goto found
		}
	}
	t.Fatal("ui.recorder.js is not in Files: trilha ui would not write it")
found:
	if n := len(Asset("ui.recorder.js")); n == 0 || n > 6<<10 {
		t.Fatalf("ui.recorder.js is %d bytes", n)
	}
	if strings.Contains(string(Asset("ui.js")), "data-ui-recorder") {
		t.Fatal("the recorder leaked into ui.js")
	}
	a := trilha.New(trilha.Config{BasePath: "/app", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	var out string
	a.Register(trilha.Route{Pattern: "/", Page: func(c *trilha.Ctx) (h.Node, error) {
		out = render(t, RecorderScript(c))
		return h.Div(), nil
	}})
	a.Handler().ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	if want := `<script src="/app/ui.recorder.js" defer></script>`; out != want {
		t.Fatalf("RecorderScript = %s, want %s", out, want)
	}
}
