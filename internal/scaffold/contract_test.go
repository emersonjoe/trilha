package scaffold

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite the golden files")

func golden(t *testing.T, name string, got []byte) {
	t.Helper()
	p := filepath.Join("..", "..", "testdata", "golden", "generate", name+".golden")
	if *update {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("golden ausente (rode make golden): %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("%s mudou:\n%s", name, got)
	}
}

const fonteProfile = `package perfil

type Profile struct {
	Nick    string ` + "`form:\"nick\" validate:\"required,min=3\"`" + `
	Age     int    ` + "`form:\"age\" validate:\"min=18\"`" + `
	Public  bool   ` + "`form:\"public\"`" + `
	Created string ` + "`form:\"-\"`" + `
}
`

// TestContratoGolden é a régua das combinações de flags: cada uma tem um
// arquivo, e make golden regrava todos.
func TestContratoGolden(t *testing.T) {
	casos := []struct {
		nome     string
		arquivos map[string]string
		opts     GenOptions
		saida    string // arquivo lido depois; o do resultado quando vazio
	}{
		{
			nome: "route-methods",
			opts: GenOptions{Kind: "route", Arg: "/api/posts/{id}/comments", Methods: []string{"post", "GET"}},
		},
		{
			nome: "route-bind-novo",
			opts: GenOptions{Kind: "route", Arg: "/api/itens", Methods: []string{"GET", "POST"}, Bind: "Item"},
		},
		{
			nome:     "route-bind-importado",
			arquivos: map[string]string{"internal/posts/posts.go": fonteComment},
			opts:     GenOptions{Kind: "route", Arg: "/api/comments", Methods: []string{"POST"}, Bind: "Comment"},
		},
		{
			nome: "page-form",
			opts: GenOptions{Kind: "page", Arg: "/contato", Form: "Contact"},
		},
		{
			nome: "page-form-pt",
			opts: GenOptions{Kind: "page", Arg: "/contato", Form: "Contact", Lang: "pt"},
		},
		{
			nome:     "page-form-tipos",
			arquivos: map[string]string{"internal/perfil/perfil.go": fonteProfile},
			opts:     GenOptions{Kind: "page", Arg: "/perfil", Form: "Profile"},
		},
		{
			nome:  "layout",
			opts:  GenOptions{Kind: "page", Arg: "/painel/relatorios", Layout: "app/painel/layout.go"},
			saida: "app/painel/layout.go",
		},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			raiz := projetoCom(t, c.arquivos)
			o := c.opts
			o.Module = "example.com/loja"
			res, err := Generate(raiz, o)
			if err != nil {
				t.Fatal(err)
			}
			arq := c.saida
			if arq == "" {
				arq = res.File
			}
			corpo, err := os.ReadFile(filepath.Join(raiz, filepath.FromSlash(arq)))
			if err != nil {
				t.Fatal(err)
			}
			golden(t, c.nome, corpo)
		})
	}
}

// TestGenerateTestGolden gera a rota e a página e, em seguida, o teste ao lado
// de cada uma: é o par que a issue pede que nasça verde.
func TestGenerateTestGolden(t *testing.T) {
	raiz := projetoCom(t, nil)
	base := GenOptions{Module: "example.com/loja"}
	rota := base
	rota.Kind, rota.Arg, rota.Methods, rota.Bind = "route", "/api/itens/{id}", []string{"GET", "POST"}, "Item"
	if _, err := Generate(raiz, rota); err != nil {
		t.Fatal(err)
	}
	pagina := base
	pagina.Kind, pagina.Arg, pagina.Form = "page", "/contato", "Contact"
	if _, err := Generate(raiz, pagina); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ nome, url, arquivo string }{
		{"test-route", "/api/itens/{id}", "app/api/itens/id_/route_test.go"},
		{"test-page", "/contato", "app/contato/page_test.go"},
	} {
		t.Run(c.nome, func(t *testing.T) {
			o := base
			o.Kind, o.Arg = "test", c.url
			res, err := Generate(raiz, o)
			if err != nil {
				t.Fatal(err)
			}
			if res.File != c.arquivo {
				t.Errorf("arquivo = %q, queria %q", res.File, c.arquivo)
			}
			corpo, err := os.ReadFile(filepath.Join(raiz, filepath.FromSlash(res.File)))
			if err != nil {
				t.Fatal(err)
			}
			golden(t, c.nome, corpo)
		})
	}
}

func TestContratoRecusa(t *testing.T) {
	casos := []struct {
		nome     string
		arquivos map[string]string
		opts     GenOptions
		diz      []string
	}{
		{
			nome: "método desconhecido",
			opts: GenOptions{Kind: "route", Arg: "/api/x", Methods: []string{"TRACE"}},
			diz:  []string{"TRACE", "GET, POST, PUT, PATCH, DELETE"},
		},
		{
			nome: "método repetido",
			opts: GenOptions{Kind: "route", Arg: "/api/x", Methods: []string{"GET", "get"}},
			diz:  []string{"GET", "twice"},
		},
		{
			nome: "bind numa página",
			opts: GenOptions{Kind: "page", Arg: "/x", Bind: "Item"},
			diz:  []string{"--form"},
		},
		{
			nome: "methods numa página",
			opts: GenOptions{Kind: "page", Arg: "/x", Methods: []string{"POST"}},
			diz:  []string{"route"},
		},
		{
			nome: "form numa rota",
			opts: GenOptions{Kind: "route", Arg: "/api/x", Form: "Item"},
			diz:  []string{"--bind"},
		},
		{
			nome: "layout que não é layout.go",
			opts: GenOptions{Kind: "page", Arg: "/x", Layout: "app/base.go"},
			diz:  []string{"layout.go"},
		},
		{
			nome: "layout fora de app/",
			opts: GenOptions{Kind: "page", Arg: "/x", Layout: "internal/layout.go"},
			diz:  []string{"inside app/"},
		},
		{
			nome: "layout que não é ancestral",
			opts: GenOptions{Kind: "page", Arg: "/x", Layout: "app/y/layout.go"},
			diz:  []string{"app/x", "app/y"},
		},
		{
			nome: "tipo ambíguo",
			arquivos: map[string]string{
				"internal/posts/posts.go": fonteComment,
				"internal/loja/loja.go":   "package loja\n\ntype Comment struct{}\n",
			},
			opts: GenOptions{Kind: "route", Arg: "/api/x", Bind: "Comment"},
			diz:  []string{"posts.Comment", "loja.Comment"},
		},
		{
			nome: "teste de uma URL sem rota",
			opts: GenOptions{Kind: "test", Arg: "/nao-existe"},
			diz:  []string{"no route answers", "/nao-existe"},
		},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			raiz := projetoCom(t, c.arquivos)
			o := c.opts
			o.Module = "example.com/loja"
			_, err := Generate(raiz, o)
			if err == nil {
				t.Fatal("tinha que recusar")
			}
			for _, s := range c.diz {
				if !strings.Contains(err.Error(), s) {
					t.Errorf("a mensagem não diz %q: %v", s, err)
				}
			}
			// Recusa não deixa arquivo pela metade.
			if _, err := os.Stat(filepath.Join(raiz, "app", "x", "page.go")); err == nil {
				t.Error("escreveu o arquivo mesmo recusando")
			}
		})
	}
}

// TestLayoutExistenteNaoEMexido: --force é sobre o arquivo que o comando veio
// escrever, não sobre o layout que já estava lá.
func TestLayoutExistenteNaoEMexido(t *testing.T) {
	raiz := projetoCom(t, map[string]string{"app/painel/layout.go": "package painel\n\n// meu\n"})
	o := GenOptions{Kind: "page", Arg: "/painel/relatorios", Layout: "app/painel/layout.go", Force: true, Module: "example.com/loja"}
	res, err := Generate(raiz, o)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Extra) != 0 {
		t.Errorf("extra = %v, não devia escrever layout nenhum", res.Extra)
	}
	corpo, err := os.ReadFile(filepath.Join(raiz, "app", "painel", "layout.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(corpo), "// meu") {
		t.Errorf("o layout que já existia foi sobrescrito:\n%s", corpo)
	}
}
