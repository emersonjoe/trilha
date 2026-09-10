package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// #148 — o histórico: quem, quando, o que mudou, e os botões. Sem JavaScript e
// sem biblioteca de diff.
func TestVersionList(t *testing.T) {
	agora := time.Now()
	got := chatPage(t, func(c *trilha.Ctx) h.Node {
		return VersionList(c, []VersionRow{
			{N: 1, By: "ana", At: agora.Add(-time.Hour), Published: true, Note: "início"},
			{N: 2, By: "bia", At: agora, Changed: []string{"corpo", "titulo"}},
		}, VersionOpts{Restore: "/x/restaurar", Publish: "/x/publicar", CSRF: trilha.CSRFInput(c)})
	})
	for _, quero := range []string{
		"v2", "v1 · published", "ana", "bia", "corpo", "titulo",
		`action="/x/publicar"`, `action="/x/restaurar"`, `name="n" value="2"`,
	} {
		if !strings.Contains(got, quero) {
			t.Fatalf("falta %q em:\n%s", quero, got)
		}
	}
	// A publicada não traz botão de publicar nem de voltar: já é ela.
	if strings.Contains(got, `name="n" value="1"`) {
		t.Fatalf("a versão publicada trouxe botões:\n%s", got)
	}
	// E o mais novo vem primeiro: um histórico se lê de agora para trás.
	if strings.Index(got, "v2") > strings.Index(got, "v1") {
		t.Fatalf("a ordem está invertida:\n%s", got)
	}
}

func TestVersionListVazia(t *testing.T) {
	got := chatPage(t, func(c *trilha.Ctx) h.Node {
		return VersionList(c, nil, VersionOpts{})
	})
	if !strings.Contains(got, "No version yet") {
		t.Fatalf("vazia = %s", got)
	}
}

// Changed é a metade honesta de um diff: os nomes dos campos que mudaram, que
// é o que alguém lê antes de abrir a versão.
func TestChanged(t *testing.T) {
	before := map[string]string{"nome": "a", "corpo": "x", "saiu": "1"}
	after := map[string]string{"nome": "a", "corpo": "y", "entrou": "2"}
	got := Changed(before, after)
	if strings.Join(got, ",") != "corpo,entrou,saiu" {
		t.Fatalf("changed = %v", got)
	}
	if len(Changed(before, before)) != 0 {
		t.Fatal("achou mudança onde não houve")
	}
}
