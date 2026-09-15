package ui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

func TestInstallAppRendersProgressiveInvitation(t *testing.T) {
	got := chatPage(t, func(c *trilha.Ctx) h.Node { return InstallApp(c) })
	for _, want := range []string{
		`data-ui-install-app=""`, `data-ui-install-button="" hidden`,
		`data-ui-install-fallback=""`, `data-ui-install-ios="" hidden`,
		`data-ui-install-manifest="/manifest.webmanifest"`, `src="/pwa.js" defer`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %s", want, got)
		}
	}
}

func TestInstallAppIsAbsentWhenStandalone(t *testing.T) {
	a := trilha.New(trilha.Config{})
	var got string
	a.Register(trilha.Route{Pattern: "/", Page: func(c *trilha.Ctx) (h.Node, error) {
		got = render(t, InstallApp(c))
		return h.Div(), nil
	}})
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "trilha_standalone", Value: "1"})
	a.Handler().ServeHTTP(httptest.NewRecorder(), req)
	if got != "" {
		t.Fatalf("standalone invitation = %q", got)
	}
}
