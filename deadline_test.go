package trilha

import (
	"testing"
	"time"
)

// sp is the zone the tests count days in: um prazo é uma data, e a data depende
// de onde é o "hoje" de quem lê.
var sp = time.FixedZone("America/Sao_Paulo", -3*60*60)

func dia(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, sp)
}

// #150 — o dia acaba no fim do dia. Um prazo que vence hoje só está vencido
// amanhã, e é o erro que aparece toda vez que alguém compara time.Time direto.
func TestDeadlinesVenceHojeNaoEstaVencido(t *testing.T) {
	agora := time.Date(2026, 9, 10, 23, 30, 0, 0, sp)
	itens := []Deadline{
		{Title: "hoje", Due: dia(2026, 9, 10)},
		{Title: "ontem", Due: dia(2026, 9, 9)},
	}
	r := Deadlines(itens, DeadlineOpts{Now: agora})

	if len(r.Overdue) != 1 || r.Overdue[0].Title != "ontem" {
		t.Fatalf("vencidos = %+v", r.Overdue)
	}
	if len(r.Within[7]) != 1 || r.Within[7][0].Title != "hoje" {
		t.Fatalf("em 7 dias = %+v", r.Within[7])
	}
	// E um minuto depois da meia-noite ele vira vencido.
	r = Deadlines(itens, DeadlineOpts{Now: agora.Add(time.Hour)})
	if len(r.Overdue) != 2 {
		t.Fatalf("depois da virada, vencidos = %+v", r.Overdue)
	}
}

// O fuso decide qual é "hoje": às 22h de São Paulo já é o dia seguinte em UTC,
// e o painel tem de contar o dia de quem lê.
func TestDeadlinesContaNoFusoDoApp(t *testing.T) {
	agora := time.Date(2026, 9, 10, 22, 0, 0, 0, sp) // 01:00 do dia 11 em UTC
	itens := []Deadline{{Title: "hoje", Due: dia(2026, 9, 10)}}

	if r := Deadlines(itens, DeadlineOpts{Now: agora, In: sp}); len(r.Overdue) != 0 {
		t.Fatalf("vencido no fuso do app: %+v", r.Overdue)
	}
	if r := Deadlines(itens, DeadlineOpts{Now: agora, In: time.UTC}); len(r.Overdue) != 1 {
		t.Fatalf("em UTC o dia 10 já passou: %+v", r.Overdue)
	}
}

// Um item resolvido não é um prazo, é história: não entra em faixa nenhuma e
// não conta em lugar nenhum.
func TestDeadlinesIgnoraResolvido(t *testing.T) {
	agora := dia(2026, 9, 10)
	r := Deadlines([]Deadline{
		{Title: "feito", Due: dia(2026, 1, 1), Done: true},
		{Title: "aberto", Due: dia(2026, 9, 20), Kind: "certidao"},
	}, DeadlineOpts{Now: agora})

	if len(r.Overdue) != 0 || r.Total != 1 {
		t.Fatalf("resolvido entrou na conta: %+v", r)
	}
	if r.ByKind["certidao"].Total != 1 || r.ByKind[""].Total != 0 {
		t.Fatalf("por tipo = %+v", r.ByKind)
	}
}

// As faixas são as pedidas, são cumulativas (o que vence em 7 também vence em
// 30), e o Next é o mais próximo que ainda não venceu.
func TestDeadlinesFaixasEProximo(t *testing.T) {
	agora := dia(2026, 9, 10)
	itens := []Deadline{
		{Title: "atrasado", Due: dia(2026, 8, 1), Kind: "contrato"},
		{Title: "em 3", Due: dia(2026, 9, 13), Kind: "certidao"},
		{Title: "em 20", Due: dia(2026, 9, 30), Kind: "certidao"},
		{Title: "em 200", Due: dia(2027, 3, 29), Kind: "contrato"},
	}
	r := Deadlines(itens, DeadlineOpts{Now: agora, Horizons: []int{7, 30}})

	if len(r.Within[7]) != 1 || len(r.Within[30]) != 2 {
		t.Fatalf("faixas = %d/%d", len(r.Within[7]), len(r.Within[30]))
	}
	if len(r.Later) != 1 || r.Later[0].Title != "em 200" {
		t.Fatalf("depois da última faixa = %+v", r.Later)
	}
	if r.Next == nil || r.Next.Title != "em 3" {
		t.Fatalf("próximo = %+v", r.Next)
	}
	if k := r.ByKind["certidao"]; k.Total != 2 || k.Overdue != 0 {
		t.Fatalf("certidao = %+v", k)
	}
	if k := r.ByKind["contrato"]; k.Total != 2 || k.Overdue != 1 {
		t.Fatalf("contrato = %+v", k)
	}
	// Sem faixa pedida, as três de sempre.
	if got := Deadlines(itens, DeadlineOpts{Now: agora}).Horizons; len(got) != 3 || got[0] != 7 {
		t.Fatalf("faixas padrão = %v", got)
	}
	// E uma lista sem nada aberto não tem próximo.
	if r := Deadlines(nil, DeadlineOpts{Now: agora}); r.Next != nil || r.Total != 0 {
		t.Fatalf("lista vazia = %+v", r)
	}
}

// Quando a regra é dia útil, a faixa pula fim de semana e os feriados que o app
// passar — porque o calendário é do app, e um feriado errado dentro do
// framework seria uma conta errada de que ninguém desconfia.
func TestDeadlinesDiasUteis(t *testing.T) {
	// 2026-09-10 é uma quinta. Cinco dias corridos chegam em 15/09 (terça);
	// cinco dias úteis, pulando o sábado, o domingo e a segunda de feriado,
	// chegam em 18/09 (sexta).
	agora := dia(2026, 9, 10)
	cal := BusinessDays(dia(2026, 9, 14))
	if got := cal.Add(agora, 5); !got.Equal(dia(2026, 9, 18)) {
		t.Fatalf("cinco dias úteis = %s", got.Format("2006-01-02"))
	}
	if cal.IsBusinessDay(dia(2026, 9, 12)) || cal.IsBusinessDay(dia(2026, 9, 14)) {
		t.Fatal("sábado ou feriado contados como dia útil")
	}

	itens := []Deadline{{Title: "em 17/09", Due: dia(2026, 9, 17)}}
	if r := Deadlines(itens, DeadlineOpts{Now: agora, Horizons: []int{5}}); len(r.Within[5]) != 0 {
		t.Fatal("17/09 está a sete dias corridos, fora da faixa de cinco")
	}
	r := Deadlines(itens, DeadlineOpts{Now: agora, Horizons: []int{5}, Business: cal})
	if len(r.Within[5]) != 1 {
		t.Fatalf("cinco dias úteis alcançam 17/09: %+v", r.Within[5])
	}
}
