package dev

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha"
)

// post sends one report to the dev server and gives back what it printed.
func post(t *testing.T, path, ctype, body string) string {
	t.Helper()
	var out bytes.Buffer
	s := &Server{Out: &out}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", ctype)
	s.serveHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d", rec.Code)
	}
	return out.String()
}

// #118 — o erro que só o navegador via passa a chegar no terminal, com o
// conserto junto. As duas formas de relatório são lidas porque cada navegador
// manda a sua, e dizer "depure no Chrome" não é uma resposta.
func TestRelatorioDeCSPImprimeOConserto(t *testing.T) {
	antigo := post(t, trilha.ReportPath, "application/csp-report", `{"csp-report":{
		"document-uri":"http://localhost:8080/documentos/12",
		"violated-directive":"frame-src 'self'",
		"blocked-uri":"http://localhost:8080/nota.pdf"}}`)
	for _, quero := range []string{"CSP refused frame-src", "/nota.pdf", "/documentos/12", "ui.Preview"} {
		if !strings.Contains(antigo, quero) {
			t.Fatalf("falta %q em:\n%s", quero, antigo)
		}
	}

	novo := post(t, trilha.ReportPath, "application/reports+json", `[{"type":"csp-violation",
		"url":"http://localhost:8080/painel",
		"body":{"effectiveDirective":"script-src","blockedURL":"https://cdn.exemplo.com/x.js"}}]`)
	for _, quero := range []string{"CSP refused script-src", "cdn.exemplo.com", "/painel", "c.Asset"} {
		if !strings.Contains(novo, quero) {
			t.Fatalf("falta %q em:\n%s", quero, novo)
		}
	}
}

// O fragmento que voltou página inteira é 200: não há erro nenhum para o
// navegador reclamar, e é por isso que ele precisa contar.
func TestRelatorioDeFragmentoComPaginaInteira(t *testing.T) {
	got := post(t, BrowserPath, "application/json",
		`{"kind":"fragment-html","url":"http://localhost:8080/documentos","id":"fila"}`)
	for _, quero := range []string{"/documentos", "whole page", "c.Fragment()"} {
		if !strings.Contains(got, quero) {
			t.Fatalf("falta %q em:\n%s", quero, got)
		}
	}
}

// Um corpo que não é um relatório não derruba nada e não imprime nada: o
// endereço é público em dev, e qualquer um pode postar nele.
func TestRelatorioInvalidoNaoImprime(t *testing.T) {
	if got := post(t, trilha.ReportPath, "application/json", "{"); got != "" {
		t.Fatalf("imprimiu %q", got)
	}
	if got := post(t, BrowserPath, "application/json", `{"kind":"outra-coisa"}`); got != "" {
		t.Fatalf("imprimiu %q", got)
	}
}
