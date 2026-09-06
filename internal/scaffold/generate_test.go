package scaffold

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha/internal/scan"
)

func raizComApp(t *testing.T) string {
	t.Helper()
	raiz := t.TempDir()
	if err := os.MkdirAll(filepath.Join(raiz, "app"), 0o755); err != nil {
		t.Fatal(err)
	}
	return raiz
}

func TestGenerate(t *testing.T) {
	casos := []struct {
		nome    string
		opts    GenOptions
		arquivo string
		pacote  string
		padrao  string
		contem  []string
	}{
		{
			nome:    "página literal",
			opts:    GenOptions{Kind: "page", Arg: "/eventos"},
			arquivo: "app/eventos/page.go",
			pacote:  "eventos",
			padrao:  "/eventos",
			contem:  []string{"package eventos", "func Page(c *trilha.Ctx) (h.Node, error)", `c.SetTitle("eventos")`},
		},
		{
			nome:    "parâmetro",
			opts:    GenOptions{Kind: "page", Arg: "/blog/{slug}"},
			arquivo: "app/blog/slug_/page.go",
			pacote:  "slug",
			padrao:  "/blog/{slug}",
			contem:  []string{"package slug", `c.Param("slug")`},
		},
		{
			nome:    "catch-all",
			opts:    GenOptions{Kind: "page", Arg: "docs/{path...}"},
			arquivo: "app/docs/path__/page.go",
			pacote:  "path",
			padrao:  "/docs/{path...}",
			contem:  []string{`c.Param("path")`},
		},
		{
			nome:    "grupo não entra na URL",
			opts:    GenOptions{Kind: "page", Arg: "/marketing-/precos"},
			arquivo: "app/marketing-/precos/page.go",
			pacote:  "precos",
			padrao:  "/precos",
		},
		{
			nome:    "raiz",
			opts:    GenOptions{Kind: "page", Arg: "/"},
			arquivo: "app/page.go",
			pacote:  "app",
			padrao:  "/",
		},
		{
			nome:    "rota com ponto na pasta",
			opts:    GenOptions{Kind: "route", Arg: "/api/relatorio.csv"},
			arquivo: "app/api/relatorio.csv/route.go",
			pacote:  "relatoriocsv",
			padrao:  "/api/relatorio.csv",
			contem:  []string{"package relatoriocsv", "func GET(c *trilha.Ctx) error", "c.JSON(200"},
		},
		{
			nome:    "rota com parâmetro",
			opts:    GenOptions{Kind: "route", Arg: "/api/itens/{id}"},
			arquivo: "app/api/itens/id_/route.go",
			pacote:  "id",
			padrao:  "/api/itens/{id}",
			contem:  []string{`"id": c.Param("id")`},
		},
		{
			nome:    "palavra reservada",
			opts:    GenOptions{Kind: "page", Arg: "/type"},
			arquivo: "app/type/page.go",
			pacote:  "type_",
			padrao:  "/type",
		},
		{
			nome:    "pasta que começa com dígito",
			opts:    GenOptions{Kind: "page", Arg: "/2024"},
			arquivo: "app/2024/page.go",
			pacote:  "p2024",
			padrao:  "/2024",
		},
		{
			nome:    "componente",
			opts:    GenOptions{Kind: "component", Arg: "Aviso"},
			arquivo: "internal/components/aviso.go",
			pacote:  "components",
			contem:  []string{"package components", "func Aviso(children ...h.Node) h.Node"},
		},
		{
			nome:    "componente composto vira snake_case",
			opts:    GenOptions{Kind: "component", Arg: "AvisoGrande"},
			arquivo: "internal/components/aviso_grande.go",
			pacote:  "components",
		},
		{
			nome:    "componente em outra pasta",
			opts:    GenOptions{Kind: "component", Arg: "Aviso", Dir: "internal/icones"},
			arquivo: "internal/icones/aviso.go",
			pacote:  "icones",
		},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			raiz := raizComApp(t)
			res, err := Generate(raiz, c.opts)
			if err != nil {
				t.Fatal(err)
			}
			if res.File != c.arquivo {
				t.Errorf("arquivo = %q, queria %q", res.File, c.arquivo)
			}
			if res.Package != c.pacote {
				t.Errorf("pacote = %q, queria %q", res.Package, c.pacote)
			}
			if res.Pattern != c.padrao {
				t.Errorf("padrão = %q, queria %q", res.Pattern, c.padrao)
			}
			corpo, err := os.ReadFile(filepath.Join(raiz, filepath.FromSlash(c.arquivo)))
			if err != nil {
				t.Fatal(err)
			}
			for _, s := range c.contem {
				if !strings.Contains(string(corpo), s) {
					t.Errorf("o arquivo não tem %q:\n%s", s, corpo)
				}
			}
		})
	}
}

func TestGeneratePacoteDoDisco(t *testing.T) {
	raiz := raizComApp(t)
	dir := filepath.Join(raiz, "app", "eventos")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// O Go recusa o diretório se o novo arquivo declarar outro pacote.
	if err := os.WriteFile(filepath.Join(dir, "dados.go"), []byte("package agenda\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := Generate(raiz, GenOptions{Kind: "page", Arg: "/eventos"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Package != "agenda" {
		t.Fatalf("pacote = %q, queria o já declarado na pasta", res.Package)
	}
}

func TestGenerateNaoSobrescreve(t *testing.T) {
	raiz := raizComApp(t)
	if _, err := Generate(raiz, GenOptions{Kind: "page", Arg: "/eventos"}); err != nil {
		t.Fatal(err)
	}
	alvo := filepath.Join(raiz, "app", "eventos", "page.go")
	if err := os.WriteFile(alvo, []byte("package eventos // meu\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Generate(raiz, GenOptions{Kind: "page", Arg: "/eventos"})
	if !errors.Is(err, ErrGenExists) {
		t.Fatalf("err = %v, queria ErrGenExists", err)
	}
	if b, _ := os.ReadFile(alvo); !strings.Contains(string(b), "// meu") {
		t.Fatal("não pode ter sobrescrito")
	}
	if _, err := Generate(raiz, GenOptions{Kind: "page", Arg: "/eventos", Force: true}); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(alvo); strings.Contains(string(b), "// meu") {
		t.Fatal("--force deveria ter sobrescrito")
	}
}

func TestGenerateConflitoPaginaERota(t *testing.T) {
	raiz := raizComApp(t)
	if _, err := Generate(raiz, GenOptions{Kind: "route", Arg: "/api/hello"}); err != nil {
		t.Fatal(err)
	}
	_, err := Generate(raiz, GenOptions{Kind: "page", Arg: "/api/hello"})
	if !errors.Is(err, ErrGenConflict) {
		t.Fatalf("err = %v, queria ErrGenConflict", err)
	}
	if _, err := os.Stat(filepath.Join(raiz, "app", "api", "hello", "page.go")); err == nil {
		t.Fatal("conflito não pode escrever arquivo")
	}
	// --force é para sobrescrita, não para quebrar a convenção.
	if _, err := Generate(raiz, GenOptions{Kind: "page", Arg: "/api/hello", Force: true}); !errors.Is(err, ErrGenConflict) {
		t.Fatalf("err = %v, queria ErrGenConflict mesmo com --force", err)
	}
}

func TestGenerateRecusa(t *testing.T) {
	casos := []struct {
		nome string
		opts GenOptions
	}{
		{"tipo desconhecido", GenOptions{Kind: "layout", Arg: "/x"}},
		{"argumento vazio", GenOptions{Kind: "page", Arg: ""}},
		{"segmento vazio", GenOptions{Kind: "page", Arg: "/blog//x"}},
		{"subida de diretório", GenOptions{Kind: "page", Arg: "/blog/../etc"}},
		{"parâmetro inválido", GenOptions{Kind: "page", Arg: "/blog/{1x}"}},
		{"grupo dinâmico", GenOptions{Kind: "page", Arg: "/blog/{slug}-"}},
		{"catch-all no meio", GenOptions{Kind: "page", Arg: "/docs/{path...}/x"}},
		{"componente minúsculo", GenOptions{Kind: "component", Arg: "aviso"}},
		{"componente inválido", GenOptions{Kind: "component", Arg: "Avi-so"}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			raiz := raizComApp(t)
			if _, err := Generate(raiz, c.opts); err == nil {
				t.Fatal("deveria recusar")
			}
		})
	}
}

// TestGenerateBateComOScanner fecha o círculo: o padrão prometido pelo gerador
// é o que o scanner extrai da pasta que ele criou.
func TestGenerateBateComOScanner(t *testing.T) {
	raiz := raizComApp(t)
	var querido []string
	for _, o := range []GenOptions{
		{Kind: "page", Arg: "/"},
		{Kind: "page", Arg: "/blog/{slug}"},
		{Kind: "page", Arg: "/marketing-/precos"},
		{Kind: "page", Arg: "/docs/{path...}"},
		{Kind: "route", Arg: "/api/relatorio.csv"},
		{Kind: "route", Arg: "/api/itens/{id}"},
	} {
		res, err := Generate(raiz, o)
		if err != nil {
			t.Fatal(o.Arg, err)
		}
		querido = append(querido, res.Pattern)
	}
	r, err := scan.Scan(raiz, "example.com/app")
	if err != nil {
		t.Fatal(err)
	}
	var obtido []string
	for _, rt := range r.Routes {
		obtido = append(obtido, rt.Pattern)
	}
	sort.Strings(querido)
	sort.Strings(obtido)
	if strings.Join(querido, " ") != strings.Join(obtido, " ") {
		t.Fatalf("scanner viu %v, o gerador prometeu %v", obtido, querido)
	}
}
