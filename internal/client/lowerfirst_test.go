package client

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestLowerFirst(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"Hello", "hello"},
		{"ID of the thing", "ID of the thing"},
		{"HTTP client", "HTTP client"},
		{"“exemplo” de texto.", "“exemplo” de texto."},
		{"Última atualização", "última atualização"},
		{"É um teste", "é um teste"},
		{"↑ seta", "↑ seta"},
	}
	for _, c := range cases {
		got := lowerFirst(c.in)
		if got != c.want {
			t.Errorf("lowerFirst(%q) = %q, want %q", c.in, got, c.want)
		}
		if !utf8.ValidString(got) {
			t.Errorf("lowerFirst(%q) is not valid UTF-8: %q", c.in, got)
		}
	}
}

// Spec #200: a description that starts with a multibyte rune used to be cut at
// byte 1, so the generated client had illegal UTF-8 and go/parser refused it.
func TestDescricaoComecaComMultibyte(t *testing.T) {
	doc := `{"openapi":"3.1.0","info":{"title":"repro","version":"1"},
		"paths":{"/x":{"get":{"operationId":"x","description":"“exemplo” de texto.",
		"responses":{"200":{"description":"ok","content":{"application/json":{"schema":{"$ref":"#/components/schemas/X"}}}}}}}},
		"components":{"schemas":{"X":{"type":"object","title":"X","description":"Última atualização do recurso.",
		"properties":{"k":{"type":"string","description":"É o campo k."}}}}}}`
	res, err := Generate([]byte(doc), Options{Package: "api"})
	if err != nil {
		t.Fatalf("leading multibyte description broke the generator: %v", err)
	}
	if !utf8.Valid(res.Source) {
		t.Fatal("the generated file is not valid UTF-8")
	}
	src := string(res.Source)
	for _, needle := range []string{"“exemplo” de texto", "última atualização do recurso"} {
		if !strings.Contains(src, needle) {
			t.Errorf("generated client missing comment fragment %q\n%s", needle, src)
		}
	}
	// Field property docs are emitted differently; only require valid UTF-8 above.
}
