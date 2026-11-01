package main

import (
	"strings"
	"testing"
)

// The dev server serves error pages with source, the route inspector and
// /_trilha/report: by default it stays on this machine (#318). Inside a
// container only every interface reaches the published port, and an -addr
// typed by hand is always the one used.
func TestDevAddr(t *testing.T) {
	cases := []struct {
		name      string
		flag      string
		given     bool
		container bool
		want      string
	}{
		{"default", "127.0.0.1:3000", false, false, "127.0.0.1:3000"},
		{"container", "127.0.0.1:3000", false, true, ":3000"},
		{"given", ":3000", true, false, ":3000"},
		{"given in a container", "127.0.0.1:4000", true, true, "127.0.0.1:4000"},
	}
	for _, c := range cases {
		if got := devAddr(c.flag, c.given, c.container); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestExposedAddr(t *testing.T) {
	for addr, want := range map[string]bool{
		"127.0.0.1:3000": false,
		"localhost:3000": false,
		"[::1]:3000":     false,
		"127.0.0.2:3000": false,
		":3000":          true,
		"0.0.0.0:3000":   true,
		"[::]:3000":      true,
		"192.168.0.7:80": true,
		"meu-pc.local:1": true,
		"3000":           true, // not an address: assume the worst, say so
	} {
		if got := exposedAddr(addr); got != want {
			t.Errorf("%s: got %v, want %v", addr, got, want)
		}
	}
}

// The warning says what is exposed and how to keep it local, in the CLI's
// language; a loopback address says nothing.
func TestDevNotice(t *testing.T) {
	defer func(l string) { lang = l }(lang)
	lang = "en"
	if n := devNotice("127.0.0.1:3000", false); n != "" {
		t.Fatalf("loopback warned: %q", n)
	}
	n := devNotice(":3000", false)
	for _, want := range []string{":3000", "network", "route inspector", "-addr 127.0.0.1:3000"} {
		if !strings.Contains(n, want) {
			t.Errorf("missing %q in %q", want, n)
		}
	}
	if strings.Contains(strings.ToLower(n), "container") {
		t.Errorf("no container here: %q", n)
	}
	if n := devNotice(":3000", true); !strings.Contains(strings.ToLower(n), "container") {
		t.Errorf("container not named: %q", n)
	}
	lang = "pt"
	if n := devNotice(":3000", false); !strings.Contains(n, "rede") || !strings.Contains(n, "-addr 127.0.0.1:3000") {
		t.Errorf("pt: %q", n)
	}
}
