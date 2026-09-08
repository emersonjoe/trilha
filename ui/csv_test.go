package ui

import (
	"strings"
	"testing"

	"github.com/emersonjoe/trilha"
)

func csvRes(n int) trilha.CSVResult {
	res := trilha.CSVResult{Rows: n}
	for i := 0; i < n; i++ {
		res.Errors = append(res.Errors, trilha.CSVError{Line: i + 2, Column: "nome", Message: "obrigatório"})
	}
	return res
}

// A frase inteira da issue: qual linha e qual coluna. Se a tabela perder uma
// das duas, a tela volta a ser "erro no arquivo".
func TestCSVErrorsMostraLinhaEColuna(t *testing.T) {
	got := render(t, CSVErrors(nil, csvRes(2)))
	for _, want := range []string{"Line", "Column", "Problem", ">2<", ">3<", "nome", "obrigatório", "The file has 2 errors"} {
		if !strings.Contains(got, want) {
			t.Fatalf("faltou %q em:\n%s", want, got)
		}
	}
}

// Vinte é o limite: quem tem quatrocentos erros vai voltar para a planilha de
// qualquer jeito, e uma página com quatrocentas linhas de erro ninguém lê.
func TestCSVErrorsCortaEDizQuantosSobraram(t *testing.T) {
	got := render(t, CSVErrors(nil, csvRes(25)))
	if n := strings.Count(got, "<tr>"); n != 21 {
		t.Fatalf("linhas na tabela (com o cabeçalho): %d", n)
	}
	if !strings.Contains(got, "and 5 more") {
		t.Fatalf("não disse quantos sobraram:\n%s", got)
	}
}

// Célula sem coluna é um problema da linha inteira, e a coluna vazia leria como
// valor faltando em vez de "a linha".
func TestCSVErrorsDaLinhaInteira(t *testing.T) {
	res := trilha.CSVResult{Errors: []trilha.CSVError{{Line: 7, Message: "linha não pôde ser lida"}}}
	got := render(t, CSVErrors(nil, res))
	if !strings.Contains(got, ">line<") || !strings.Contains(got, "The file has one error") {
		t.Fatalf("linha inteira:\n%s", got)
	}
}

func TestCSVErrorsMostraOsAvisos(t *testing.T) {
	res := csvRes(1)
	res.Warnings = []string{"unknown column dono, ignored"}
	if got := render(t, CSVErrors(nil, res)); !strings.Contains(got, "dono") {
		t.Fatalf("aviso sumiu:\n%s", got)
	}
}

// O Title do chamador é texto, não formato: um % nele viraria %!s(MISSING) na
// tela que já está reportando um erro.
func TestCSVErrorsNaoFormataOTituloDoChamador(t *testing.T) {
	got := render(t, CSVErrors(nil, csvRes(2), CSVErrorsOpts{Title: "100% das linhas falharam"}))
	if !strings.Contains(got, "100% das linhas falharam") {
		t.Fatalf("título:\n%s", got)
	}
}
