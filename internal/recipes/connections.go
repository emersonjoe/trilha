package recipes

// connectionsRecipe is the list of external services the application talks
// to: an API, an MCP server, a provider — with the secret sealed and a Test
// button.
//
// It is the screen a management app writes three times under three names
// before somebody notices it is one screen.
func connectionsRecipe() Recipe {
	return Recipe{
		Name: "connections",
		Summary: map[string]string{
			"en": "external services this app talks to: name, URL, sealed secret, and a Test button",
			"pt": "serviços externos com que este app fala: nome, URL, segredo selado e o botão Testar",
		},
		Doc: "/reference/connections",
		Files: []File{
			{Rel: "internal/conexoes/conexoes.go", Go: true, Body: connectionsDecl},
			{Rel: "internal/conexoes/conexoes_test.go", Go: true, Body: connectionsDeclTest},
			{Rel: "{{.At}}conexoes/page.go", Go: true, Body: connectionsPage},
			{Rel: "conexoes_test.go", Go: true, Body: connectionsTest},
		},
		Setup: []Insert{{
			Marker: "// trilha:add connections",
			Line:   "\tconexoes.Setup(a)\n",
		}},
		Imports: []string{"{{.Module}}/internal/conexoes"},
		Next: map[string]string{
			"en": "Run `trilha dev` and open {{.URL}}conexoes. Guard that folder. Where the app calls a service, " +
				"ask for its client: `cli, err := conexoes.Conexoes.Client(c, id)` — it carries the credential " +
				"and only speaks to that host. Add a `ConnectionKind` in internal/conexoes for each kind of thing you connect to.",
			"pt": "Rode `trilha dev` e abra {{.URL}}conexoes. Guarde essa pasta. Onde o app chama um serviço, " +
				"peça o cliente dele: `cli, err := conexoes.Conexoes.Client(c, id)` — ele leva a credencial " +
				"e só fala com aquele host. Acrescente um `ConnectionKind` no internal/conexoes para cada tipo de coisa a que você se conecta.",
		},
	}
}

const connectionsDecl = `// Package conexoes is the list of external services this application talks
// to, and the one place their credentials live.
//
// A connection is a name, a URL, how to authenticate and a sealed secret.
// What this file owns is the list of kinds — the only part only this
// application knows — and the test each kind runs when somebody presses the
// button.
package conexoes

import (
	"context"
	"net/http"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/ai/mcp"
)

// Tipos is what this application connects to. Each kind says which
// authentications it accepts and how it is tested. Add one per kind of
// service; a kind without Test has no Test button.
var Tipos = []trilha.ConnectionKind{
	{Key: "api", Label: "{{.T.conn_api}}", Auth: []string{"none", "bearer", "header", "basic"},
		// Any answer below 400 passes: the point is "does the credential
		// open the door", not what is behind it.
		Test: trilha.TestHTTP("GET", "/")},
	{Key: "mcp", Label: "{{.T.conn_mcp}}", Auth: []string{"none", "bearer", "header"},
		Test: testarMCP},
}

// Conexoes is the list. The store is memory here — the connections last as
// long as the process — and a real one is a table behind the same four
// methods, with the secret sealed by trilha.Secret's Value on the way in.
var Conexoes = trilha.NewConnections(trilha.ConnectionsOpts{Kinds: Tipos})

// Setup hands the list to the application.
func Setup(a *trilha.App) {
	trilha.Provide(a, Conexoes)
}

// Chamar is how the rest of the application uses a connection: ask for its
// client and make the request. The credential is put on by the transport, and
// the request only leaves for that host — a redirect elsewhere is refused.
func Chamar(c *trilha.Ctx, id, path string) (*http.Response, error) {
	conn, err := Conexoes.Get(c, id)
	if err != nil {
		return nil, err
	}
	cli, err := Conexoes.Client(c, id)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(c.Context(), http.MethodGet, conn.URL+path, nil)
	if err != nil {
		return nil, err
	}
	return cli.Do(req)
}

// testarMCP is the test of an MCP server: the handshake and the list of
// tools, through the connection's own client, so the credential goes along
// and never has to be copied into a headers map.
func testarMCP(ctx context.Context, conn trilha.Connection) error {
	cli, err := mcp.Dial(ctx, mcp.HTTPWith(conn.URL, conn.Client(0), nil))
	if err != nil {
		return err
	}
	defer cli.Close()
	_, err = cli.ListTools(ctx)
	return err
}
`

const connectionsDeclTest = `package conexoes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/emersonjoe/trilha"
)

// O teste do tipo api usa a credencial da conexão e conta a resposta: 200
// passa, 401 é a mensagem.
func TestTipoAPI(t *testing.T) {
	var visto string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		visto = r.Header.Get("Authorization")
		if visto != "Bearer tok-1" {
			w.WriteHeader(http.StatusUnauthorized)
		}
	}))
	defer srv.Close()
	conn := trilha.Connection{Kind: "api", Name: "x", URL: srv.URL, Auth: "bearer", Secret: trilha.Secret("tok-1")}
	if err := Tipos[0].Test(context.Background(), conn); err != nil {
		t.Fatal(err)
	}
	conn.Secret = trilha.Secret("errado")
	if err := Tipos[0].Test(context.Background(), conn); err == nil {
		t.Fatal("401 passou")
	}
}
`

const connectionsPage = `// Package conexoes is the screen of the external services: the list, the
// form, and the three buttons.
//
// One GET and one POST. The POST is save, test and delete, dispatched by a
// hidden _action field, because they are one screen — and a form that posts
// to itself comes back to itself when something is wrong.
//
// Guard this folder. Whoever reaches it can point this application at a
// server of their own.
package conexoes

import (
	"errors"
	"net/http"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Page lists the connections at GET {{.URL}}conexoes; ?edit=id opens one in
// the form.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("{{.T.conn_title}}")
	conns := trilha.Use[*trilha.Connections](c)
	var editing *trilha.Connection
	if id := c.Query("edit"); id != "" {
		conn, err := conns.Get(c, id)
		if err != nil {
			return nil, err
		}
		editing = &conn
	}
	return tela(c, conns, editing, nil), nil
}

// POST is save, test and delete.
func POST(c *trilha.Ctx) error {
	conns := trilha.Use[*trilha.Connections](c)
	switch c.Form("_action") {
	case "test":
		res, err := conns.Test(c, c.Form("id"))
		if err != nil {
			return err
		}
		if res.OK {
			c.Flash(ui.FlashSuccess, "{{.T.conn_test_ok}}")
		} else {
			c.Flash(ui.FlashError, "{{.T.conn_test_failed}} " + res.Message)
		}
	case "delete":
		if err := conns.Delete(c, c.Form("id")); err != nil {
			return err
		}
		c.Flash(ui.FlashSuccess, "{{.T.conn_deleted}}")
	default:
		conn := ui.ParseConnectionForm(c)
		if _, err := conns.Save(c, conn); err != nil {
			var fe trilha.FieldErrors
			if !errors.As(err, &fe) {
				return err
			}
			// The typed values go back with the message on the field, and
			// the secret stays out: the field renders empty either way.
			return c.Render(http.StatusUnprocessableEntity, tela(c, conns, &conn, fe))
		}
		c.Flash(ui.FlashSuccess, "{{.T.conn_saved}}")
	}
	return c.Redirect("{{.URL}}conexoes")
}

func tela(c *trilha.Ctx, conns *trilha.Connections, editing *trilha.Connection, errs trilha.FieldErrors) h.Node {
	return h.Div(
		ui.PageHeader("{{.T.conn_title}}"),
		ui.Muted(h.Text("{{.T.conn_desc}}")),
		ui.ConnectionsPanel(c, conns, ui.ConnectionsOpts{
			Path: "{{.URL}}conexoes", CSRF: trilha.CSRFInput(c),
			Editing: editing, Errors: errs,
		}),
	)
}
`

const connectionsTest = `package main

import (
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha"
)

// A tela cria uma conexão, mostra o nome e nunca o segredo; um endereço de
// rede privada é recusado em produção com a mensagem no campo.
func TestTelaDeConexoes(t *testing.T) {
	t.Setenv("TRILHA_ENV", "prod")
	t.Setenv("TRILHA_SECRET", "a-test-secret-with-more-than-32-bytes!!")
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	c := trilha.NewTestClient(t, newApp())

	c.Get("{{.URL}}conexoes").WantStatus(http.StatusOK).WantContains("{{.T.conn_title}}")

	c.PostForm("{{.URL}}conexoes", map[string][]string{
		"_action": {"save"}, "kind": {"api"}, "name": {"Receita"},
		"url": {"https://api.receita.example"}, "auth": {"bearer"}, "secret": {"tok-segredo-123"},
	}).WantStatus(http.StatusSeeOther)
	depois := c.Get("{{.URL}}conexoes").WantStatus(http.StatusOK).Body.String()
	if !strings.Contains(depois, "Receita") {
		t.Fatalf("a conexão não apareceu:\n%s", depois)
	}
	if strings.Contains(depois, "tok-segredo-123") {
		t.Fatalf("o segredo está na tela:\n%s", depois)
	}

	// Um endereço privado em produção volta como 422 com o formulário: quem
	// errou o endereço está com ele aberto.
	c.PostForm("{{.URL}}conexoes", map[string][]string{
		"_action": {"save"}, "kind": {"api"}, "name": {"Interna"},
		"url": {"http://127.0.0.1:9"}, "auth": {"none"},
	}).WantStatus(http.StatusUnprocessableEntity).WantContains("production")
}
`
