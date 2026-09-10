package approval

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/emersonjoe/trilha"
)

// pedido roda uma função dentro de uma requisição com um ator, que é onde o
// pacote lê quem está decidindo.
func pedido(t *testing.T, a *trilha.App, subject string, roles []string, fn func(c *trilha.Ctx)) {
	t.Helper()
	a.Register(trilha.Route{Pattern: "/x", Kind: trilha.KindAPI, Methods: map[string]trilha.HandlerFunc{
		"GET": func(c *trilha.Ctx) error {
			c.SetActor(trilha.Actor{Subject: subject})
			c.Set("roles", roles)
			fn(c)
			return c.Text(http.StatusOK, "ok")
		},
	}})
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/x", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
}

func fila(t *testing.T) (*Approvals, *trilha.App) {
	t.Helper()
	return New(Options{Roles: func(c *trilha.Ctx) []string {
		r, _ := c.Get("roles").([]string)
		return r
	}}), trilha.New(trilha.Config{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
}

// #147 — abrir, decidir, e a decisão com quem e por quê. É o mínimo que
// separa uma fila de aprovação de uma tabela de linhas paradas.
func TestAbrirEDecidir(t *testing.T) {
	fila, app := fila(t)
	var id string
	var rodou Record
	fila.On("eliminacao", func(c *trilha.Ctx, r Record) error { rodou = r; return nil })

	pedido(t, app, "u-1", []string{"cpad"}, func(c *trilha.Ctx) {
		var err error
		id, err = fila.Open(c, Request{Kind: "eliminacao", Subject: "Listagem 2024/07",
			Target: "/admin/retencao/123", Assign: Role("cpad")})
		if err != nil {
			t.Fatal(err)
		}
		if err := fila.Decide(c, id, Approved, "ok pelo quórum"); err != nil {
			t.Fatal(err)
		}
	})

	rec, err := fila.Get(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if rec.State != Approved || rec.By != "u-1" || rec.Reason != "ok pelo quórum" {
		t.Fatalf("record = %+v", rec)
	}
	if rec.Decided.IsZero() {
		t.Fatal("a decisão não tem hora")
	}
	// O gancho do tipo roda depois da decisão, com o registro já decidido: é
	// ali que o app faz o que a decisão significa.
	if rodou.ID != id || rodou.State != Approved {
		t.Fatalf("o gancho recebeu %+v", rodou)
	}
}

// Quem não é dono do pedido não decide, e a checagem é do pacote e não da
// tela: um botão escondido continua sendo um endereço.
func TestQuemNaoEDonoNaoDecide(t *testing.T) {
	fila, app := fila(t)
	var id string
	pedido(t, app, "u-1", []string{"cpad"}, func(c *trilha.Ctx) {
		var err error
		if id, err = fila.Open(c, Request{Kind: "x", Subject: "s", Assign: Role("cpad")}); err != nil {
			t.Fatal(err)
		}
	})

	outro := trilha.New(trilha.Config{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	pedido(t, outro, "u-2", []string{"leitor"}, func(c *trilha.Ctx) {
		if err := fila.Decide(c, id, Approved, ""); err != ErrNotYours {
			t.Fatalf("err = %v", err)
		}
	})

	// E decidir duas vezes é a mesma recusa: o segundo clique não muda o que
	// já foi decidido.
	pedido(t, trilha.New(trilha.Config{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}),
		"u-1", []string{"cpad"}, func(c *trilha.Ctx) {
			if err := fila.Decide(c, id, Approved, ""); err != nil {
				t.Fatal(err)
			}
			if err := fila.Decide(c, id, Rejected, "mudei de ideia"); err != ErrNotYours {
				t.Fatalf("decidiu duas vezes: %v", err)
			}
		})
}

// O prazo vence sozinho: um pedido vencido não é um pendente para sempre.
func TestPrazoVence(t *testing.T) {
	fila, app := fila(t)
	pedido(t, app, "u-1", []string{"cpad"}, func(c *trilha.Ctx) {
		if _, err := fila.Open(c, Request{Kind: "x", Subject: "vencido",
			Assign: Role("cpad"), Due: time.Now().Add(-time.Minute)}); err != nil {
			t.Fatal(err)
		}
		if _, err := fila.Open(c, Request{Kind: "x", Subject: "no prazo",
			Assign: Role("cpad"), Due: time.Now().Add(time.Hour)}); err != nil {
			t.Fatal(err)
		}
	})
	if n := fila.Expire(context.Background()); n != 1 {
		t.Fatalf("venceu %d", n)
	}
	lista, err := fila.List(context.Background(), ListParams{State: Pending})
	if err != nil {
		t.Fatal(err)
	}
	if len(lista) != 1 || lista[0].Subject != "no prazo" {
		t.Fatalf("pendentes = %+v", lista)
	}
}

// A caixa é o que a pessoa da vez pode decidir, e não a fila inteira.
func TestCaixaEDeQuemOlha(t *testing.T) {
	fila, app := fila(t)
	pedido(t, app, "u-1", []string{"cpad"}, func(c *trilha.Ctx) {
		if _, err := fila.Open(c, Request{Kind: "x", Subject: "do papel", Assign: Role("cpad")}); err != nil {
			t.Fatal(err)
		}
		if _, err := fila.Open(c, Request{Kind: "x", Subject: "da bia", Assign: User("u-2")}); err != nil {
			t.Fatal(err)
		}
	})

	ver := func(subject string, roles []string) []Record {
		var out []Record
		pedido(t, trilha.New(trilha.Config{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}),
			subject, roles, func(c *trilha.Ctx) {
				var err error
				if out, err = fila.Inbox(c, ListParams{}); err != nil {
					t.Fatal(err)
				}
			})
		return out
	}
	if got := ver("u-1", []string{"cpad"}); len(got) != 1 || got[0].Subject != "do papel" {
		t.Fatalf("a caixa do papel = %+v", got)
	}
	if got := ver("u-2", nil); len(got) != 1 || got[0].Subject != "da bia" {
		t.Fatalf("a caixa da pessoa = %+v", got)
	}
	if got := ver("u-3", []string{"leitor"}); len(got) != 0 {
		t.Fatalf("quem não decide nada viu %+v", got)
	}
}

// Um pedido sem dono é um pedido que ninguém decide: é erro na hora de abrir,
// e não uma linha parada que alguém descobre depois.
func TestPedidoPrecisaDeDono(t *testing.T) {
	fila, app := fila(t)
	pedido(t, app, "u-1", nil, func(c *trilha.Ctx) {
		if _, err := fila.Open(c, Request{Kind: "x", Subject: "s"}); err == nil {
			t.Fatal("abriu sem dono")
		}
		if _, err := fila.Open(c, Request{Kind: "x", Assign: Role("cpad")}); err == nil {
			t.Fatal("abriu sem assunto")
		}
	})
}
