package ctx

import (
	"bytes"
	"flag"
	"go/parser"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var update = flag.Bool("update", false, "rewrite golden files")

func build(t *testing.T, app string) *Context {
	t.Helper()
	c, err := Build(filepath.Join("..", "..", "testdata", "apps", app), "example.com/"+app, "0.0.0")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func golden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "golden", name)
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("missing golden (run make golden): %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s mismatch:\n%s", name, got)
	}
}

// TestRoutes is the routing half of the map: order, files, layouts and the
// middleware chains.
func TestRoutes(t *testing.T) {
	c := build(t, "full")
	var got []string
	for _, r := range c.Routes {
		got = append(got, strings.Join(r.Methods, ",")+" "+r.Pattern+" "+r.File)
	}
	want := []string{
		"GET / app/page.go",
		"GET /admin app/admin/page.go",
		"DELETE,GET,POST /api/posts app/api/posts/route.go",
		"GET /api/posts/{id} app/api/posts/id_/route.go",
		"GET /blog app/blog/page.go",
		"GET,POST /blog/novo app/blog/novo/page.go",
		"GET /blog/{slug} app/blog/slug_/page.go",
		"GET /docs/{path...} app/docs/path__/page.go",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("routes:\n%s", strings.Join(got, "\n"))
	}
	blog := c.Routes[6]
	if len(blog.Layouts) != 2 || blog.Layouts[0] != "app/blog/layout.go" || blog.Layouts[1] != "app/layout.go" {
		t.Errorf("layouts of /blog/{slug}: %v", blog.Layouts)
	}
	if len(blog.Params) != 1 || blog.Params[0] != "slug" {
		t.Errorf("params of /blog/{slug}: %v", blog.Params)
	}
	if c.Setup == nil || strings.Join(c.Setup.Funcs, ",") != "Setup,Config" {
		t.Errorf("setup: %+v", c.Setup)
	}
	if c.Generated.Status != "missing" {
		t.Errorf("generated: %+v", c.Generated)
	}
}

// TestAPI is the contract half: what the handler binds and what it answers,
// with the types it names.
func TestAPI(t *testing.T) {
	c := build(t, "openapi")
	var items *Route
	for i, r := range c.Routes {
		if r.Pattern == "/api/items" {
			items = &c.Routes[i]
		}
	}
	if items == nil || len(items.API) != 2 {
		t.Fatalf("/api/items: %+v", items)
	}
	get, post := items.API[0], items.API[1]
	if get.Method != "GET" || len(get.Query) != 1 || get.Query[0] != "q" {
		t.Errorf("GET: %+v", get)
	}
	if post.Method != "POST" || post.Request == "" {
		t.Errorf("POST: %+v", post)
	}
	var has201 bool
	for _, r := range post.Responses {
		if r.Status == 201 {
			has201 = true
		}
	}
	if !has201 {
		t.Errorf("POST responses: %+v", post.Responses)
	}
	var named []string
	for _, ty := range c.Types {
		named = append(named, ty.Name)
	}
	if len(named) == 0 || !strings.Contains(strings.Join(named, ","), "Problem") {
		t.Errorf("types: %v", named)
	}
	var rules string
	for _, ty := range c.Types {
		for _, f := range ty.Fields {
			if f.Required && f.Rules != "" {
				rules = ty.Name + "." + f.Name + ": " + f.Rules
			}
		}
	}
	if rules == "" {
		t.Error("no field carries the rules of its validate tag")
	}
}

// TestDeterministic is the promise the golden rests on.
func TestDeterministic(t *testing.T) {
	a, err := build(t, "openapi").JSON()
	if err != nil {
		t.Fatal(err)
	}
	b, err := build(t, "openapi").JSON()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatal("two builds, two answers")
	}
}

func TestGolden(t *testing.T) {
	j, err := build(t, "openapi").JSON()
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "ctx.json.golden", j)
	golden(t, "ctx.md.golden", []byte(build(t, "full").Markdown(Compact)))
}

// TestViews: what the compact view elides, --all prints, and the narrow views
// print one section each.
func TestViews(t *testing.T) {
	c := build(t, "methodmw")
	compact, all := c.Markdown(Compact), c.Markdown(All)
	if strings.Contains(compact, "· POST:") {
		t.Error("compact view should elide the per-method chain")
	}
	if !strings.Contains(all, "· POST:") {
		t.Errorf("--all should print the per-method chain:\n%s", all)
	}
	o := build(t, "openapi")
	routes, types := o.Markdown(OnlyRoutes), o.Markdown(OnlyTypes)
	if !strings.HasPrefix(routes, "## Routes") || strings.Contains(routes, "## Types") {
		t.Errorf("--routes:\n%s", routes)
	}
	if !strings.HasPrefix(types, "## Types") || strings.Contains(types, "## Routes") {
		t.Errorf("--types:\n%s", types)
	}
}

// TestUnderOneSecond is the budget of a command that runs every turn (SC-004).
func TestUnderOneSecond(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/big\n\ngo 1.24\n"), 0o644)
	for i := 0; i < 40; i++ {
		dir := filepath.Join(root, "app", "s"+string(rune('a'+i%26))+string(rune('a'+i/26)))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		src := "package p\n\nimport \"github.com/emersonjoe/trilha\"\n\nfunc GET(c *trilha.Ctx) error { return nil }\n"
		if err := os.WriteFile(filepath.Join(dir, "route.go"), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	start := time.Now()
	if _, err := Build(root, "example.com/big", "0.0.0"); err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d > time.Second {
		t.Errorf("40 routes took %s; the map is read every turn", d)
	}
}

// #124 — o módulo e o nível que uma rota exige. Sem isto, descobrir "esta tela
// precisa de docs:editar" custa abrir o middleware.go da pasta e seguir uma
// variável, por rota.
func TestPolitica(t *testing.T) {
	c := build(t, "policy")
	rotas := map[string]Route{}
	for _, r := range c.Routes {
		rotas[r.Pattern] = r
	}

	// A pasta declara, e a rota dela exibe.
	docs := rotas["/docs"]
	if docs.Policy == nil || docs.Policy.Module != "docs" || docs.Policy.Level != "ver" {
		t.Fatalf("/docs: %+v", docs.Policy)
	}
	if docs.Policy.From != "app/docs/middleware.go" {
		t.Errorf("de onde veio: %q", docs.Policy.From)
	}
	// O método guardado sozinho aparece por método, e não some no da pasta.
	if p, ok := docs.PolicyByMethod["POST"]; !ok || p.Level != "editar" {
		t.Errorf("POST /docs: %+v", docs.PolicyByMethod)
	}

	// A pasta de baixo ganha: a herança é a do middleware, e a mais funda vale.
	editar := rotas["/docs/editar"]
	if editar.Policy == nil || editar.Policy.Level != "administrar" {
		t.Fatalf("/docs/editar: %+v", editar.Policy)
	}

	// E uma rota que política nenhuma alcança não ganha campo nenhum.
	if raiz := rotas["/"]; raiz.Policy != nil || raiz.PolicyByMethod != nil {
		t.Errorf("a raiz não exige nada: %+v", raiz.Policy)
	}
}

// #124 — os enums registrados, para quem vai escrever a próxima tela não
// inventar um segundo jeito de escrever "status".
func TestEnums(t *testing.T) {
	c := build(t, "policy")
	if len(c.Enums) != 2 {
		t.Fatalf("enums: %+v", c.Enums)
	}
	// Ordenados pelo nome, para a saída não mudar a cada execução.
	if c.Enums[0].Name != "doc.confirma" || c.Enums[1].Name != "doc.situacao" {
		t.Fatalf("ordem: %s, %s", c.Enums[0].Name, c.Enums[1].Name)
	}
	sit := c.Enums[1]
	if len(sit.Values) != 3 {
		t.Fatalf("valores: %+v", sit.Values)
	}
	if sit.Values[1].Value != "enviado" || sit.Values[1].Label != "Enviado" || sit.Values[1].Tone != "info" {
		t.Errorf("segundo valor: %+v", sit.Values[1])
	}
	if sit.From != "app/setup.go" {
		t.Errorf("de onde veio: %q", sit.From)
	}
	// A forma posicional é a mesma lista escrita de outro jeito.
	pos := c.Enums[0]
	if len(pos.Values) != 2 || pos.Values[1].Value != "nao" || pos.Values[1].Tone != "danger" {
		t.Errorf("posicional: %+v", pos.Values)
	}
	// Declarada e nunca registrada não entra: nada a procura por nome.
	for _, e := range c.Enums {
		for _, v := range e.Values {
			if v.Value == "x" {
				t.Fatal("um enum que ninguém registrou apareceu no mapa")
			}
		}
	}
}

// Quem não usa não paga: um projeto sem política e sem enum tem exatamente a
// saída de antes, no Markdown e no JSON.
func TestQuemNaoUsaNaoPaga(t *testing.T) {
	c := build(t, "full")
	for _, r := range c.Routes {
		if r.Policy != nil || r.PolicyByMethod != nil {
			t.Fatalf("%s ganhou política do nada: %+v", r.Pattern, r.Policy)
		}
	}
	if len(c.Enums) != 0 {
		t.Fatalf("enums do nada: %+v", c.Enums)
	}
	if md := c.Markdown(All); strings.Contains(md, "## Enums") || strings.Contains(md, "needs") {
		t.Fatal("o markdown ganhou seção que o projeto não pediu")
	}
	b, err := c.JSON()
	if err != nil {
		t.Fatal(err)
	}
	for _, campo := range []string{`"policy"`, `"policyByMethod"`, `"enums"`} {
		if strings.Contains(string(b), campo) {
			t.Fatalf("o json ganhou %s", campo)
		}
	}
}

// O que a leitura não reconhece não vira palpite.
func TestOQueNaoSeLeNaoAparece(t *testing.T) {
	makers := map[string]bool{"Exige": true}
	for _, src := range []string{
		`acesso.Exige(modulo, nivel)`,  // parâmetros, não literais
		`a.RequirePolicy(P, m, "ver")`, // metade literal
		`acesso.Outra("docs", "ver")`,  // função que não encaminha
		`acesso.Exige("docs")`,         // argumentos de menos
	} {
		e, err := parser.ParseExpr(src)
		if err != nil {
			t.Fatal(err)
		}
		if p, ok := politicaDe(e, makers); ok {
			t.Errorf("%s virou %+v", src, p)
		}
	}
}
