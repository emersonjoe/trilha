package approval

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// linkApp é o app sem login do #285: a página do link, como o app a escreveria
// em app/aprovar/{token}/page.go.
func linkApp(t *testing.T, fila *Approvals, app *trilha.App) *trilha.TestClient {
	t.Helper()
	app.Register(trilha.Route{Pattern: "/aprovar/{token}", Kind: trilha.KindPage,
		Page: func(c *trilha.Ctx) (h.Node, error) {
			r, err := fila.ByLink(c, c.Param("token"))
			if err != nil {
				return nil, err
			}
			return h.Text("decidir: " + r.Subject), nil
		},
		Methods: map[string]trilha.HandlerFunc{"POST": func(c *trilha.Ctx) error {
			if err := fila.DecideByLink(c, c.Param("token"), c.Form("decisao"), c.Form("motivo")); err != nil {
				return err
			}
			return c.Text(http.StatusOK, "feito")
		}}})
	return trilha.NewTestClient(t, app)
}

func TestDecidirPorLink(t *testing.T) {
	fila, app := fila(t)
	var decidido Record
	fila.On("teste", func(c *trilha.Ctx, r Record) error { decidido = r; return nil })
	id, err := fila.Open(nil, Request{Kind: "teste", Subject: "Teste grátis de ana@org.br", Assign: User("dono")})
	if err != nil {
		t.Fatal(err)
	}
	token, err := fila.Link(context.Background(), id, LinkOptions{To: "dono@org.br"})
	if err != nil {
		t.Fatal(err)
	}
	if len(token) < 40 || strings.Contains(token, id) {
		t.Fatalf("token fraco ou revelador: %q", token)
	}
	cli := linkApp(t, fila, app)

	// GET só mostra — a pré-visualização do cliente de e-mail não decide nada —
	// e a página é privada, porque o endereço é a credencial.
	cli.Get("/aprovar/"+token).WantStatus(200).WantContains("Teste grátis").
		WantHeader("Cache-Control", "no-store").WantHeader("Referrer-Policy", "no-referrer").
		WantHeader("X-Robots-Tag", "noindex, nofollow")
	cli.Get("/aprovar/" + token).WantStatus(200)
	if r, _ := fila.Get(context.Background(), id); r.State != Pending {
		t.Fatalf("GET decidiu: %+v", r)
	}

	// POST sem CSRF é recusado e não gasta o link.
	form := url.Values{"decisao": {Approved}, "motivo": {"parece gente"}}
	if res := cli.PostForm("/aprovar/"+token, form, trilha.WithoutCSRF()); res.Code == 200 {
		t.Fatal("POST sem CSRF decidiu")
	}
	cli.PostForm("/aprovar/"+token, url.Values{"decisao": {"talvez"}}).WantStatus(400)
	cli.PostForm("/aprovar/"+token, form).WantStatus(200)
	if decidido.State != Approved || decidido.By != "link:dono@org.br" || decidido.Reason != "parece gente" {
		t.Fatalf("decisão: %+v", decidido)
	}

	// Uso único: depois da decisão, o link é 404, no GET e no POST.
	cli.Get("/aprovar/" + token).WantStatus(404)
	cli.PostForm("/aprovar/"+token, form).WantStatus(404)
	cli.Get("/aprovar/nao-existe").WantStatus(404)
}

func TestLinkExpiraEEsquecido(t *testing.T) {
	fila, app := fila(t)
	agora := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	fila.now = func() time.Time { return agora }
	id, _ := fila.Open(nil, Request{Kind: "teste", Subject: "x", Assign: User("dono")})
	token, err := fila.Link(nil, id, LinkOptions{To: "dono@org.br", TTL: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	cli := linkApp(t, fila, app)
	cli.Get("/aprovar/" + token).WantStatus(200)
	agora = agora.Add(time.Hour)
	cli.Get("/aprovar/" + token).WantStatus(404)
	if n, _ := fila.links.PurgeLinks(context.Background(), agora); n != 1 {
		t.Fatalf("esqueceu %d links", n)
	}

	// Um pedido que deixou de estar pendente leva os links junto.
	id2, _ := fila.Open(nil, Request{Kind: "teste", Subject: "y", Assign: User("dono")})
	t2, _ := fila.Link(nil, id2, LinkOptions{To: "dono@org.br"})
	pedido(t, app, "dono", nil, func(c *trilha.Ctx) {
		if err := fila.Decide(c, id2, Rejected, "não"); err != nil {
			t.Fatal(err)
		}
	})
	cli.Get("/aprovar/" + t2).WantStatus(404)
	if _, err := fila.Link(nil, id2, LinkOptions{To: "x@y"}); err == nil {
		t.Fatal("link para pedido já decidido")
	}
	if _, err := fila.Link(nil, id, LinkOptions{}); err == nil {
		t.Fatal("link sem destinatário")
	}
}
