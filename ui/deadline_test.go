package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

func prazo(t *testing.T, dias int, resto ...func(*trilha.Deadline)) trilha.Deadline {
	t.Helper()
	d := trilha.Deadline{Title: "Certidão FGTS", Kind: "certidao",
		Due: time.Now().AddDate(0, 0, dias), URL: "/fornecedores/1"}
	for _, f := range resto {
		f(&d)
	}
	return d
}

// #150 — os cartões são a conta em cima: o que venceu, e quanto vem em cada
// faixa, na ordem das faixas.
func TestDeadlineCards(t *testing.T) {
	agora := time.Now()
	itens := []trilha.Deadline{
		prazo(t, -5, func(d *trilha.Deadline) { d.Title = "Alvará vencido" }),
		prazo(t, 3),
		prazo(t, 40, func(d *trilha.Deadline) { d.Title = "Contrato 12"; d.Kind = "contrato" }),
	}
	got := chatPage(t, func(c *trilha.Ctx) h.Node {
		return DeadlineCards(c, trilha.Deadlines(itens, trilha.DeadlineOpts{Now: agora}))
	})
	for _, quero := range []string{
		"Overdue", "In 7 days", "In 30 days", "In 90 days", "Next",
		"ui-deadline-alarm", "Alvará vencido",
		`href="/fornecedores/1"`,
	} {
		if !strings.Contains(got, quero) {
			t.Fatalf("falta %q em:\n%s", quero, got)
		}
	}
	// Sem nada vencido o cartão não grita: um zero vermelho ensina a ignorar a
	// cor, e aí o 3 também é ignorado.
	limpo := chatPage(t, func(c *trilha.Ctx) h.Node {
		return DeadlineCards(c, trilha.Deadlines(itens[1:], trilha.DeadlineOpts{Now: agora}))
	})
	if strings.Contains(limpo, "ui-deadline-alarm") {
		t.Fatalf("cartão de vencidos gritando com zero:\n%s", limpo)
	}
}

// A lista escreve a data em relativo, marca a linha atrasada, corta no limite e
// diz quantas ficaram de fora.
func TestDeadlineList(t *testing.T) {
	agora := time.Now()
	itens := []trilha.Deadline{
		prazo(t, -2, func(d *trilha.Deadline) { d.Title = "Atrasado"; d.Owner = "ana" }),
		prazo(t, 3, func(d *trilha.Deadline) { d.Title = "Em três" }),
		prazo(t, 9, func(d *trilha.Deadline) { d.Title = "Em nove" }),
	}
	got := chatPage(t, func(c *trilha.Ctx) h.Node {
		return DeadlineList(c, itens, DeadlineListOpts{Limit: 2, More: "/prazos",
			Now: agora, Owner: true})
	})
	for _, quero := range []string{
		"Atrasado", "Em três", `class="ui-late"`, "Owner", "ana",
		"and 1 more", `href="/prazos"`,
	} {
		if !strings.Contains(got, quero) {
			t.Fatalf("falta %q em:\n%s", quero, got)
		}
	}
	if strings.Contains(got, "Em nove") {
		t.Fatalf("o limite não cortou:\n%s", got)
	}
	// Só a linha atrasada é marcada.
	if n := strings.Count(got, `class="ui-late"`); n != 1 {
		t.Fatalf("%d linhas marcadas como atrasadas", n)
	}
}

// Os três desenham com lista vazia, e o que não tem nada a dizer não diz nada.
func TestDeadlineVazio(t *testing.T) {
	got := chatPage(t, func(c *trilha.Ctx) h.Node {
		return h.Div(
			DeadlineCards(c, trilha.Deadlines(nil, trilha.DeadlineOpts{})),
			DeadlineList(c, nil, DeadlineListOpts{}),
			DeadlineBadge(c, nil),
		)
	})
	if !strings.Contains(got, "No open deadline.") {
		t.Fatalf("estado vazio:\n%s", got)
	}
	if strings.Contains(got, "ui-deadline-badge") || strings.Contains(got, "ui-deadline-alarm") {
		t.Fatalf("badge ou alarme com lista vazia:\n%s", got)
	}
	// Um item resolvido não é um prazo: a lista some do mesmo jeito.
	feito := chatPage(t, func(c *trilha.Ctx) h.Node {
		return DeadlineList(c, []trilha.Deadline{prazo(t, -1, func(d *trilha.Deadline) { d.Done = true })},
			DeadlineListOpts{})
	})
	if !strings.Contains(feito, "No open deadline.") {
		t.Fatalf("o resolvido apareceu:\n%s", feito)
	}
}

// A badge é o número no menu, e ela diz em texto o que a cor diz.
func TestDeadlineBadge(t *testing.T) {
	got := chatPage(t, func(c *trilha.Ctx) h.Node {
		return DeadlineBadge(c, []trilha.Deadline{prazo(t, -1), prazo(t, -2)})
	})
	if !strings.Contains(got, `aria-label="2 overdue"`) || !strings.Contains(got, ">2<") {
		t.Fatalf("badge = %s", got)
	}
}
