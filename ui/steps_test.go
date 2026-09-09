package ui

import (
	"strings"
	"testing"
)

var passos = []Step{
	{Label: "Dados", Href: "/1"},
	{Label: "Endereço", Href: "/2"},
	{Label: "Revisão"},
}

// "Você está aqui" numa lista de cinco é o aria-current, não a cor.
func TestStepsDizOndeAPessoaEsta(t *testing.T) {
	got := render(t, Steps(passos, 2))
	if !strings.Contains(got, `data-state="current" aria-current="step"`) {
		t.Fatalf("passo atual:\n%s", got)
	}
	for _, want := range []string{`data-state="done"`, `data-state="todo"`, "Dados", "Endereço", "Revisão"} {
		if !strings.Contains(got, want) {
			t.Fatalf("faltou %q em:\n%s", want, got)
		}
	}
}

// Só o passo já feito é link. Um assistente em que o passo 3 está a um clique
// é um assistente cujos passos não precisavam ser em ordem.
func TestStepsSoVoltaParaTras(t *testing.T) {
	got := render(t, Steps(passos, 2))
	if !strings.Contains(got, `<a href="/1">Dados</a>`) {
		t.Fatalf("o passo feito devia ser link:\n%s", got)
	}
	if strings.Contains(got, `href="/2"`) {
		t.Fatalf("o passo atual não é link:\n%s", got)
	}
}

func TestStepsNoPrimeiroNinguemVolta(t *testing.T) {
	if got := render(t, Steps(passos, 1)); strings.Contains(got, "<a href") {
		t.Fatalf("no passo 1 não há para onde voltar:\n%s", got)
	}
}
