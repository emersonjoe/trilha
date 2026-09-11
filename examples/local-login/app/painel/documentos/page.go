// Package documentos is the screen that reads the API this app does not own.
//
// The browser used to call that API itself, with a token it kept: the page was
// JavaScript, the credential was in the tab, and anybody who opened the console
// had it. Here the call leaves from the server — the token comes out of the
// session, which the browser can carry and cannot read — and the answer arrives
// as HTML with no loading state and no second endpoint.
//
// The other half of this app is Config.Upstreams, which forwards /api/ with the
// same credential: that is for what the browser has to fetch (a download, an
// island). A listing is not one of those.
package documentos

import (
	"context"
	"net/http"
	"os"
	"strconv"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/local-login/internal/acervo"
	"github.com/emersonjoe/trilha/examples/local-login/internal/sessao"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// tokenKey is how the credential travels from the request to the client. The
// client takes a context and not a *trilha.Ctx, because it also runs in a job
// and in a test, where there is no request to borrow a token from.
type tokenKey struct{}

// chamando is the context to hand the client: the session's token, and nothing
// else about the request.
func chamando(c *trilha.Ctx) context.Context {
	return context.WithValue(c.Context(), tokenKey{}, sessao.Token(c))
}

// autorizacao is the only place the credential appears. It runs for every call
// the client makes, so a call from a context that carries no token goes out
// unauthenticated instead of borrowing somebody else's session — and the
// Authorization the browser sent is never read here, because a header from
// outside is not a credential this app has any reason to trust.
func autorizacao(ctx context.Context) http.Header {
	tok, _ := ctx.Value(tokenKey{}).(string)
	if tok == "" {
		return nil
	}
	return http.Header{"Authorization": {"Bearer " + tok}}
}

// cliente builds the client of the API. It reads the environment on each
// request rather than once at init: a package-level var reading os.Getenv is a
// screen no test can point somewhere else, and an app that has to be restarted
// to change where the API lives. Building it is a struct, not a connection.
func cliente() (*acervo.Client, bool) {
	base := os.Getenv("API_URL")
	if base == "" {
		return nil, false
	}
	return acervo.New(base, acervo.WithHeader(autorizacao)), true
}

// Page answers GET /painel/documentos. The folder above requires a session, so
// there is nobody here without one.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Documentos")
	api, ok := cliente()
	if !ok {
		return h.Div(h.Class("cartao"),
			h.H1(h.Text("Documentos")),
			ui.Alert("A API não está configurada",
				ui.AlertDescription(h.Text("Defina API_URL para a tela ler o acervo."))),
		), nil
	}
	busca := c.Query("q")
	page, err := api.Documents().List(chamando(c), acervo.DocumentsListParams{Q: busca, PageSize: 20})
	if err != nil {
		// The API answering badly is not this app crashing: the person gets
		// the screen, and what the other side said.
		if e, ok := acervo.AsError(err); ok {
			return h.Div(h.Class("cartao"),
				h.H1(h.Text("Documentos")),
				ui.Alert("O acervo respondeu "+strconv.Itoa(e.Status),
					ui.AlertDescription(h.Text(e.Detail))),
			), nil
		}
		return nil, err
	}
	return h.Div(h.Class("cartao"),
		h.H1(h.Text("Documentos")),
		// The filter is a form and not a fetch: the URL is what says which
		// list this is, so it can be shared, reloaded and gone back to.
		h.Form(h.Method("get"), h.Class("filtro"),
			ui.Input(h.Name("q"), h.Value(busca), h.Placeholder("Buscar")),
			h.Button(h.Type("submit"), h.Class("botao"), h.Text("Buscar")),
		),
		tabela(page.Items),
		h.P(h.Class("total"), h.Text(strconv.FormatInt(page.Total, 10)+" documentos no acervo")),
	), nil
}

// tabela renders the rows. The types are the API's, so a field that stops
// existing stops compiling.
func tabela(itens []acervo.Document) h.Node {
	if len(itens) == 0 {
		return ui.Empty(ui.EmptyOpts{Title: "Nenhum documento", Hint: "Nenhum documento do acervo corresponde a esta busca."})
	}
	linhas := make([]h.Node, 0, len(itens)+1)
	linhas = append(linhas, h.Thead(h.Tr(
		h.Th(h.Text("Documento")),
		h.Th(h.Text("Status")),
		h.Th(ui.Num(), h.Text("Páginas")),
	)))
	corpo := make([]h.Node, 0, len(itens))
	for _, d := range itens {
		corpo = append(corpo, h.Tr(
			h.Td(h.Text(d.Filename)),
			h.Td(ui.Badge(h.Text(string(d.Status)))),
			h.Td(ui.Num(), h.Text(strconv.FormatInt(d.Pages, 10))),
		))
	}
	linhas = append(linhas, h.Tbody(corpo...))
	return ui.Table(linhas...)
}
