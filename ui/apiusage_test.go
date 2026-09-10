package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// #151 — o painel responde "estão usando? onde? quando pararam?": os totais, a
// forma por dia desenhada no servidor, e a tabela por rota.
func TestAPIUsage(t *testing.T) {
	agora := time.Now()
	dados := APIUsageData{
		Total: 1204, Errors: 61, Last: agora.Add(-2 * time.Hour),
		Days: []APIUsageDay{
			{Day: agora.AddDate(0, 0, -2), Count: 400},
			{Day: agora.AddDate(0, 0, -1), Count: 500},
			{Day: agora, Count: 304},
		},
		Routes: []APIUsageRoute{
			{Method: "GET", Route: "/api/documentos/{id}", Count: 1100, Errors: 1, Last: agora},
			{Method: "POST", Route: "/api/documentos", Count: 104, Errors: 60, Last: agora},
		},
	}
	got := chatPage(t, func(c *trilha.Ctx) h.Node {
		return APIUsage(c, dados, APIUsageOpts{Days: 30})
	})
	for _, quero := range []string{
		"1204", "last 30 days", "/api/documentos/{id}", "POST", "<svg",
		"Calls per day",
	} {
		if !strings.Contains(got, quero) {
			t.Fatalf("falta %q em:\n%s", quero, got)
		}
	}
	// A rota que erra muito é marcada: é o que a pessoa veio ver.
	if n := strings.Count(got, `class="ui-late"`); n != 1 {
		t.Fatalf("%d rotas marcadas:\n%s", n, got)
	}
	// E nenhum script: o desenho é do servidor.
	if strings.Contains(got, "<script") {
		t.Fatalf("o painel trouxe JavaScript:\n%s", got)
	}
}

// Uma chave sem uso não desenha um gráfico vazio: ela diz que nunca foi usada,
// e diz o que isso costuma significar.
func TestAPIUsageSemUso(t *testing.T) {
	got := chatPage(t, func(c *trilha.Ctx) h.Node {
		return APIUsage(c, APIUsageData{}, APIUsageOpts{Days: 30})
	})
	if !strings.Contains(got, "never been used") {
		t.Fatalf("vazio = %s", got)
	}
	if strings.Contains(got, "Calls per day") {
		t.Fatalf("desenhou um gráfico de nada:\n%s", got)
	}
}

// A coluna de chamadas aparece quando é pedida — e zero em todas as chaves é
// exatamente a resposta que alguém abriu a tela para ver.
func TestAPIKeysTableComUso(t *testing.T) {
	linhas := []APIKeyRow{
		{ID: "1", Handle: "abc", Name: "Parceiro X", Created: time.Now(), Calls: 0},
	}
	com := chatPage(t, func(c *trilha.Ctx) h.Node {
		return APIKeysTable(c, linhas, APIKeysOpts{Usage: true, UsageDays: 30})
	})
	if !strings.Contains(com, "Calls (30 d)") {
		t.Fatalf("a coluna não apareceu:\n%s", com)
	}
	sem := chatPage(t, func(c *trilha.Ctx) h.Node {
		return APIKeysTable(c, linhas)
	})
	if strings.Contains(sem, "Calls") {
		t.Fatalf("a coluna apareceu sem ser pedida:\n%s", sem)
	}
}
