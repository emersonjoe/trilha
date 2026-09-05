package main

import "testing"

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
