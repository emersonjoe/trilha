package ui

import (
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// offlinePage renders one node inside a request of an app that has an offline
// page and a public folder, which is what OfflineScript reads.
func offlinePage(t *testing.T, node func(c *trilha.Ctx) h.Node) string {
	t.Helper()
	a := trilha.New(trilha.Config{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Public: fstest.MapFS{"ui.css": &fstest.MapFile{Data: []byte("body{}")}},
	})
	var out string
	a.Register(trilha.Route{Pattern: "/", Page: func(c *trilha.Ctx) (h.Node, error) {
		out = render(t, node(c))
		return h.Div(), nil
	}})
	a.Register(trilha.Route{Pattern: "/coleta", Offline: true, Page: func(c *trilha.Ctx) (h.Node, error) {
		return h.Div(), nil
	}})
	a.Register(trilha.Route{Pattern: "/admin", Page: func(c *trilha.Ctx) (h.Node, error) {
		return h.Div(), nil
	}})
	a.Handler().ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	return out
}

// #268 — the script tag is where the browser learns what may be kept: the
// pages that declared `var Offline = true`, and nothing else.
func TestOfflineScriptCarriesTheDeclaredRoutes(t *testing.T) {
	got := offlinePage(t, OfflineScript)
	for _, want := range []string{
		`src="/ui.offline.js"`, `defer`,
		`data-ui-offline-routes="/coleta"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("OfflineScript is missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "/admin") {
		t.Fatalf("a route nobody declared must not reach the worker:\n%s", got)
	}
	// The version is the content hash of the stylesheet: a deploy changes it
	// and the worker drops the cache it filled before.
	v := got[strings.Index(got, `data-ui-offline-version="`)+len(`data-ui-offline-version="`):]
	v = v[:strings.Index(v, `"`)]
	if v == "" || strings.Contains(v, "?") {
		t.Fatalf("version = %q in %s", v, got)
	}
}

func TestOutboxDrawsTheCountTheErrorAndTheButton(t *testing.T) {
	got := offlinePage(t, Outbox)
	for _, want := range []string{
		`class="ui-outbox"`, `data-ui-outbox=""`, `hidden`,
		`data-ui-outbox-count=""`, `data-ui-outbox-one="1 pending"`, `data-ui-outbox-many="{n} pending"`,
		`data-ui-outbox-error=""`, `data-ui-outbox-send=""`, `Send now`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("Outbox is missing %q:\n%s", want, got)
		}
	}
}

func TestOutboxSpeaksTheAppLocale(t *testing.T) {
	a := trilha.New(trilha.Config{Locale: "pt-BR", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	var got string
	a.Register(trilha.Route{Pattern: "/", Page: func(c *trilha.Ctx) (h.Node, error) {
		got = render(t, Outbox(c))
		return h.Div(), nil
	}})
	a.Handler().ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	for _, want := range []string{"pendentes", "Enviar agora"} {
		if !strings.Contains(got, want) {
			t.Fatalf("Outbox in pt-BR is missing %q:\n%s", want, got)
		}
	}
}

// The contract of ui.offline.js, which no Go test can run: the attributes it
// reads, the header it sends, the field it stamps and the store it keeps the
// queue in. A rename on either side breaks here instead of in the field.
func TestOfflineAssetContract(t *testing.T) {
	js := string(Asset("ui.offline.js"))
	if js == "" {
		t.Fatal("ui.offline.js is not embedded")
	}
	for _, want := range []string{
		"data-ui-offline-routes", "data-ui-offline-version",
		"form[data-trilha-offline]", "data-ui-outbox", "data-ui-outbox-send",
		"data-ui-outbox-count", "data-ui-outbox-error",
		"Idempotency-Key", "_idempotency_key", "_queued_at",
		"trilha-outbox", "navigator.onLine", "same-origin", "/sw.js",
	} {
		if !strings.Contains(js, want) {
			t.Errorf("ui.offline.js does not name %q", want)
		}
	}
	// It is loaded only by the page that asks for it, so it does not live in
	// ui.js and does not weigh on the page with no outbox.
	if strings.Contains(string(Asset("ui.js")), "trilha-outbox") {
		t.Error("the outbox leaked into ui.js")
	}
	if n := len(js); n > 8<<10 {
		t.Errorf("ui.offline.js is %d bytes", n)
	}
	for _, f := range Files {
		if f == "ui.offline.js" {
			return
		}
	}
	t.Fatal("ui.offline.js is not in Files: trilha ui would not write it")
}
