package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestDetectLang(t *testing.T) {
	cases := []struct {
		env  map[string]string
		want string
	}{
		{nil, "en"},
		{map[string]string{"LANG": "pt_BR.UTF-8"}, "pt"},
		{map[string]string{"LANG": "en_US.UTF-8"}, "en"},
		{map[string]string{"LANG": "pt_BR.UTF-8", "TRILHA_LANG": "en"}, "en"},
		{map[string]string{"LANG": "en_US.UTF-8", "TRILHA_LANG": "PT"}, "pt"},
		{map[string]string{"LANG": "pt_BR", "LC_ALL": "C"}, "en"},
		{map[string]string{"LANG": "C", "LC_MESSAGES": "pt_PT"}, "pt"},
		{map[string]string{"TRILHA_LANG": "  "}, "en"},
	}
	for _, c := range cases {
		got := detectLang(func(k string) string { return c.env[k] })
		if got != c.want {
			t.Errorf("%v: got %q, want %q", c.env, got, c.want)
		}
	}
}

func TestEveryMessageHasBothLanguages(tt *testing.T) {
	for k, m := range msgs {
		if m[0] == "" || m[1] == "" {
			tt.Errorf("%q: missing a translation", k)
		}
	}
	if t("no such key") != "no such key" {
		tt.Error("unknown key must echo itself")
	}
}

// tCall finds every t("key") in the sources of this command.
var tCall = regexp.MustCompile(`\bt\("([^"]+)"\)`)

// TestEveryKeyIsInMsgs is what would have caught the `generate crud` output
// inside a guarded folder: the key `crud guard helper` was never written into
// msgs, so t() echoed it back — a string with no verb in it — and the Printf
// below it dumped its arguments as %!(EXTRA string=...). An unknown key
// echoing itself is the right behaviour at run time and the wrong one to find
// out about from a user's terminal.
func TestEveryKeyIsInMsgs(tt *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		tt.Fatal(err)
	}
	for _, f := range files {
		// The tests are where an unknown key is passed on purpose, to prove
		// that t() echoes it instead of panicking.
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(f)
		if err != nil {
			tt.Fatal(err)
		}
		for _, m := range tCall.FindAllStringSubmatch(string(raw), -1) {
			if _, ok := msgs[m[1]]; !ok {
				tt.Errorf("%s: t(%q) has no entry in msgs", f, m[1])
			}
		}
	}
}
