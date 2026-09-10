package ui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/emersonjoe/trilha"
)

func conexoesDeTeste(t *testing.T) *trilha.Connections {
	t.Helper()
	x := trilha.NewConnections(trilha.ConnectionsOpts{Kinds: []trilha.ConnectionKind{
		{Key: "api", Label: "APIs", Auth: []string{"none", "bearer", "header", "basic"}, Test: trilha.TestHTTP("GET", "/")},
		{Key: "mcp", Label: "MCP servers", Auth: []string{"none", "bearer"}},
	}})
	for _, conn := range []trilha.Connection{
		{Kind: "api", Name: "Receita", URL: "https://api.receita.example", Auth: "bearer", Secret: trilha.Secret("tok-segredo-123")},
		{Kind: "mcp", Name: "Busca", URL: "https://mcp.busca.example", Auth: "none"},
	} {
		if _, err := x.Save(nil, conn); err != nil {
			t.Fatal(err)
		}
	}
	return x
}

// O segredo é a única coisa da tela que não pode estar na tela: nem no valor
// do campo, nem num atributo, nem na lista.
func TestConnectionsPanelNuncaMostraOSegredo(t *testing.T) {
	x := conexoesDeTeste(t)
	list, _ := x.List(nil)
	var editing *trilha.Connection
	for i := range list {
		if list[i].Name == "Receita" {
			editing = &list[i]
		}
	}
	got := render(t, ConnectionsPanel(nil, x, ConnectionsOpts{Path: "/admin/conexoes", Editing: editing}))
	if strings.Contains(got, "tok-segredo-123") {
		t.Fatalf("o segredo está no HTML:\n%s", got)
	}
	for _, want := range []string{"Receita", "Busca", "APIs", "MCP servers", "https://api.receita.example",
		"never tested", `name="_action"`, `value="save"`, `value="test"`, `value="delete"`,
		`action="/admin/conexoes"`, `type="password"`, "Leave blank", "Edit connection", `value="` + editing.ID + `"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("faltou %q em:\n%s", want, got)
		}
	}
	// O tipo sem Test não oferece o botão Testar: só a Receita tem.
	if n := strings.Count(got, `value="test"`); n != 1 {
		t.Fatalf("botões Testar: %d, queria 1:\n%s", n, got)
	}
}

// O badge conta o último teste: verde quando passou, vermelho com a mensagem
// quando falhou.
func TestConnectionsPanelMostraOResultadoDoTeste(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	x := trilha.NewConnections(trilha.ConnectionsOpts{Kinds: []trilha.ConnectionKind{
		{Key: "api", Label: "APIs", Auth: []string{"none"}, Test: trilha.TestHTTP("GET", "/")},
	}})
	// A URL do httptest é loopback; entra pelo store, que não valida, como
	// faria um seed de teste.
	store := trilha.ConnectionMemory()
	if err := store.Save(context.Background(), trilha.Connection{ID: "c1", Kind: "api", Name: "Local", URL: srv.URL, Auth: "none", CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	x = trilha.NewConnections(trilha.ConnectionsOpts{Store: store, Kinds: x.Kinds()})
	if _, err := x.Test(nil, "c1"); err != nil {
		t.Fatal(err)
	}
	got := render(t, ConnectionsPanel(nil, x, ConnectionsOpts{Path: "/c"}))
	if !strings.Contains(got, "ui-badge-danger") || !strings.Contains(got, "HTTP 401") {
		t.Fatalf("badge do teste falhado:\n%s", got)
	}
}

func TestConnectionsPanelVazioExplica(t *testing.T) {
	x := trilha.NewConnections(trilha.ConnectionsOpts{Kinds: []trilha.ConnectionKind{{Key: "api", Label: "APIs"}}})
	got := render(t, ConnectionsPanel(nil, x, ConnectionsOpts{Path: "/c"}))
	if !strings.Contains(got, "No connections yet") || !strings.Contains(got, "New connection") {
		t.Fatalf("vazio:\n%s", got)
	}
}

// O formulário volta com o que foi digitado e o erro no campo, para a pessoa
// corrigir sem redigitar.
func TestConnectionsPanelReexibeErros(t *testing.T) {
	x := trilha.NewConnections(trilha.ConnectionsOpts{Kinds: []trilha.ConnectionKind{{Key: "api", Label: "APIs", Auth: []string{"none", "header"}}}})
	got := render(t, ConnectionsPanel(nil, x, ConnectionsOpts{Path: "/c",
		Editing: &trilha.Connection{Kind: "api", Name: "Sem URL", Auth: "header", Header: "X-Api-Key", Headers: map[string]string{"X-Tenant": "acme"}},
		Errors:  map[string]string{"url": "required"}}))
	for _, want := range []string{`value="Sem URL"`, "required", `aria-invalid`, `value="X-Api-Key"`, "X-Tenant: acme", `data-ui-show-when`} {
		if !strings.Contains(got, want) {
			t.Fatalf("faltou %q em:\n%s", want, got)
		}
	}
}
