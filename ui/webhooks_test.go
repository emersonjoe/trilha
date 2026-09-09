package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

func assinaturas() []WebhookRow {
	return []WebhookRow{
		{ID: "whk_1", Label: "Cobrança do parceiro", URL: "https://parceiro.exemplo/hook",
			Events: []string{"documento.processado"}, Created: time.Now()},
		{ID: "whk_2", URL: "https://antigo.exemplo/hook", Created: time.Now(), Revoked: true},
	}
}

func entregas() []DeliveryRow {
	return []DeliveryRow{
		{ID: "dlv_1", Event: "documento.processado", State: "delivered", Attempt: 1,
			Status: 200, When: time.Now()},
		{ID: "dlv_2", Event: "fluxo.concluido", State: "failed", Attempt: 6, Status: 422,
			Response: `{"erro":"campo destinatario obrigatorio"}`, When: time.Now()},
		{ID: "dlv_3", Event: "fluxo.concluido", State: "pending", Attempt: 2, Status: 500,
			When: time.Now(), Next: time.Now().Add(30 * time.Minute)},
	}
}

func opcoes(c *trilha.Ctx) WebhooksOpts {
	return WebhooksOpts{Action: "/webhooks", CSRF: trilha.CSRFInput(c),
		Events: []string{"documento.processado", "fluxo.concluido"}}
}

// O que o parceiro respondeu tem de aparecer. É a diferença entre uma tarde
// adivinhando e um minuto lendo — e é a razão de o corpo ser guardado.
func TestPainelMostraOQueOParceiroDisse(t *testing.T) {
	got, _ := desenha(t, func(c *trilha.Ctx) h.Node {
		return WebhooksPanel(c, assinaturas(), entregas(), opcoes(c))
	})
	for _, quero := range []string{
		"campo destinatario obrigatorio", "422", "documento.processado",
		"parceiro.exemplo/hook", "Cobrança do parceiro",
	} {
		if !strings.Contains(got, quero) {
			t.Fatalf("faltou %q", quero)
		}
	}
}

// O segredo aparece uma vez, com o aviso de que não vai aparecer de novo — e
// não aparece quando não há um.
func TestPainelMostraOSegredoUmaVez(t *testing.T) {
	got, _ := desenha(t, func(c *trilha.Ctx) h.Node {
		o := opcoes(c)
		o.Secret = "whsec_abcdef"
		return WebhooksPanel(c, nil, nil, o)
	})
	if !strings.Contains(got, "whsec_abcdef") || !strings.Contains(got, "ui-secret-once") {
		t.Fatalf("o segredo não apareceu:\n%s", got)
	}
	if !strings.Contains(got, "não consegue mostrar de novo") {
		t.Fatal("não avisou que é a única vez")
	}

	semSegredo, _ := desenha(t, func(c *trilha.Ctx) h.Node {
		return WebhooksPanel(c, assinaturas(), entregas(), opcoes(c))
	})
	if strings.Contains(semSegredo, "ui-secret-once") {
		t.Fatal("desenhou a caixa do segredo sem ter segredo")
	}
}

// Nenhum segredo pode sair numa listagem: a linha não carrega um, e a tela não
// tem de onde imprimir.
func TestOSegredoNaoVazaNaListagem(t *testing.T) {
	got, _ := desenha(t, func(c *trilha.Ctx) h.Node {
		return WebhooksPanel(c, assinaturas(), entregas(), opcoes(c))
	})
	if strings.Contains(got, "whsec_") {
		t.Fatalf("um segredo apareceu na tabela:\n%s", got)
	}
}

// Botão de reenviar só no que já acabou: reenviar uma entrega pendente é uma
// segunda entrega do mesmo evento.
func TestReenviarSoNoQueAcabou(t *testing.T) {
	got, _ := desenha(t, func(c *trilha.Ctx) h.Node {
		return WebhooksPanel(c, nil, entregas(), opcoes(c))
	})
	if n := strings.Count(got, `value="retry"`); n != 2 {
		t.Fatalf("botões de reenviar = %d, queria 2 (a entregue e a que falhou)", n)
	}
	if !strings.Contains(got, `value="dlv_2"`) {
		t.Fatal("o botão não aponta para a entrega que falhou")
	}
	// E a pendente diz quando vai tentar de novo, para ninguém apertar nada.
	if !strings.Contains(got, "próxima tentativa às") {
		t.Fatalf("a pendente não diz quando tenta de novo:\n%s", got)
	}
}

// Revogada não oferece testar nem revogar de novo.
func TestRevogadaNaoTemBotao(t *testing.T) {
	got, _ := desenha(t, func(c *trilha.Ctx) h.Node {
		return WebhooksPanel(c, assinaturas(), nil, opcoes(c))
	})
	if n := strings.Count(got, `value="revoke"`); n != 1 {
		t.Fatalf("botões de revogar = %d, queria 1 (só a ativa)", n)
	}
	if n := strings.Count(got, `value="ping"`); n != 1 {
		t.Fatalf("botões de testar = %d", n)
	}
	if !strings.Contains(got, "Revogado") {
		t.Fatal("a revogada não está marcada como revogada")
	}
}

// Todo formulário que muda alguma coisa leva o CSRF, senão o app recusa os
// próprios botões.
func TestTodoFormularioLevaCSRF(t *testing.T) {
	got, _ := desenha(t, func(c *trilha.Ctx) h.Node {
		return WebhooksPanel(c, assinaturas(), entregas(), opcoes(c))
	})
	forms := strings.Count(got, "<form")
	csrf := strings.Count(got, `name="_csrf"`)
	if forms == 0 || forms != csrf {
		t.Fatalf("%d formulários e %d campos de CSRF", forms, csrf)
	}
}

// Sem Action não se desenha formulário nenhum: é o painel só de leitura, para
// uma tela que mostra sem deixar mexer.
func TestSemAcaoEhSoLeitura(t *testing.T) {
	got, _ := desenha(t, func(c *trilha.Ctx) h.Node {
		return WebhooksPanel(c, assinaturas(), entregas(), WebhooksOpts{})
	})
	if strings.Contains(got, "<form") {
		t.Fatalf("desenhou formulário sem para onde postar:\n%s", got)
	}
	if !strings.Contains(got, "parceiro.exemplo") {
		t.Fatal("o painel só de leitura não lista nada")
	}
}

// Os eventos que a aplicação emite viram caixas, e cada uma precisa do id que
// o rótulo aponta — senão o clique no texto não marca nada.
func TestOsEventosViramCaixasComRotulo(t *testing.T) {
	got, _ := desenha(t, func(c *trilha.Ctx) h.Node {
		return WebhooksPanel(c, nil, nil, opcoes(c))
	})
	for _, e := range []string{"documento.processado", "fluxo.concluido"} {
		if !strings.Contains(got, `value="`+e+`"`) {
			t.Fatalf("faltou a caixa de %s", e)
		}
	}
	if n := strings.Count(got, `name="events"`); n != 2 {
		t.Fatalf("caixas = %d", n)
	}
	if n := strings.Count(got, `for="wh-ev-`); n != 2 {
		t.Fatalf("rótulos ligados = %d", n)
	}
}

// Uma entrega que nunca teve resposta mostra o erro de transporte, e não um
// "0" que não quer dizer nada.
func TestEntregaSemRespostaMostraOErro(t *testing.T) {
	got, _ := desenha(t, func(c *trilha.Ctx) h.Node {
		return WebhooksPanel(c, nil, []DeliveryRow{{ID: "dlv_9", Event: "x", State: "failed",
			Attempt: 6, Status: 0, Err: "dial tcp: i/o timeout", When: time.Now()}}, opcoes(c))
	})
	if !strings.Contains(got, "i/o timeout") {
		t.Fatalf("não mostrou o erro:\n%s", got)
	}
	if strings.Contains(got, ">0<") {
		t.Fatal("mostrou status 0, que não quer dizer nada para quem lê")
	}
}
