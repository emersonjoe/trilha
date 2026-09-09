package ui

import (
	"strings"
	"testing"

	"github.com/emersonjoe/trilha/h"
)

func TestDeferDesenhaOBuracoEOCaminhoSemScript(t *testing.T) {
	got := render(t, Defer(nil, "insights", "/panel/insights"))
	for _, want := range []string{
		`id="insights"`,
		`data-trilha-defer=""`,
		`data-trilha-src="/panel/insights"`,
		`data-trilha-defer-wait=""`,
		`class="ui-skeleton"`,
		"height: 8rem",
		`<noscript><a href="/panel/insights"`,
		">Load<",
		`data-trilha-defer-fail="" hidden`,
		"Try again",
		"could not be loaded",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("faltou %q em:\n%s", want, got)
		}
	}
}

// A altura não é enfeite: um placeholder mais baixo que o conteúdo faz a página
// pular debaixo do cursor de quem já estava lendo.
func TestDeferRespeitaAAltura(t *testing.T) {
	if got := render(t, Defer(nil, "x", "/x", DeferOpts{Height: "12rem"})); !strings.Contains(got, "height: 12rem") {
		t.Fatalf("altura:\n%s", got)
	}
	got := render(t, Defer(nil, "x", "/x", DeferOpts{Placeholder: h.P(h.Text("carregando o painel"))}))
	if strings.Contains(got, "ui-skeleton") || !strings.Contains(got, "carregando o painel") {
		t.Fatalf("placeholder do chamador:\n%s", got)
	}
}

// Then viaja no mesmo elemento: carrega logo e depois acompanha, sem uma
// segunda peça na página.
func TestDeferComPollDepois(t *testing.T) {
	got := render(t, Defer(nil, "x", "/x", DeferOpts{Then: Poll("30s", "/x")}))
	if !strings.Contains(got, `data-trilha-poll="30s"`) || !strings.Contains(got, `data-trilha-defer=""`) {
		t.Fatalf("Then:\n%s", got)
	}
}

func TestDeferAceitaAsFrasesDoChamador(t *testing.T) {
	got := render(t, Defer(nil, "x", "/x", DeferOpts{Load: "Ver agora", Error: "Falhou", Retry: "De novo"}))
	for _, want := range []string{">Ver agora<", "Falhou", ">De novo<"} {
		if !strings.Contains(got, want) {
			t.Fatalf("faltou %q em:\n%s", want, got)
		}
	}
}
