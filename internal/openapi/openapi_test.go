package openapi

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha/internal/scan"
)

var update = flag.Bool("update", false, "rewrite golden files")

func gen(t *testing.T, root, module string, o Options) []byte {
	t.Helper()
	res, err := scan.Scan(root, module)
	if err != nil {
		t.Fatal(err)
	}
	out, err := Generate(root, res, o)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// at walks the decoded document: at(doc, "paths", "/api/items", "get") .
func at(t *testing.T, v any, keys ...string) any {
	t.Helper()
	for i, k := range keys {
		m, ok := v.(map[string]any)
		if !ok {
			t.Fatalf("%s is not an object", strings.Join(keys[:i], "."))
		}
		v, ok = m[k]
		if !ok {
			t.Fatalf("missing %s", strings.Join(keys[:i+1], "."))
		}
	}
	return v
}

func decode(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	return doc
}

func TestGolden(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "apps", "openapi")
	got := gen(t, root, "example.com/openapi", Options{Title: "Loja", Version: "1.0.0", Server: "https://api.exemplo.com"})
	again := gen(t, root, "example.com/openapi", Options{Title: "Loja", Version: "1.0.0", Server: "https://api.exemplo.com"})
	if !bytes.Equal(got, again) {
		t.Fatal("generator is not deterministic")
	}
	golden := filepath.Join("..", "..", "testdata", "golden", "openapi.json.golden")
	if *update {
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("missing golden (run with -update): %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("golden mismatch:\n%s", got)
	}
}

// TestSchemaDasTags: o schema sai da mesma tag que o Bind lê, então ele não
// pode divergir da validação.
func TestSchemaDasTags(t *testing.T) {
	doc := decode(t, gen(t, filepath.Join("..", "..", "testdata", "apps", "openapi"), "example.com/openapi", Options{}))
	item := at(t, doc, "components", "schemas", "store.Item").(map[string]any)
	props := item["properties"].(map[string]any)
	if _, ok := props["Note"]; ok {
		t.Error(`json:"-" entrou no schema`)
	}
	if _, ok := props["hidden"]; ok {
		t.Error("campo não exportado entrou no schema")
	}
	if got := at(t, props, "name", "maxLength"); got != float64(40) {
		t.Errorf("maxLength = %v", got)
	}
	if got := at(t, props, "name", "description"); got != "Name is what the buyer reads." {
		t.Errorf("description = %v", got)
	}
	if got := at(t, props, "created", "format"); got != "date-time" {
		t.Errorf("format de time.Time = %v", got)
	}
	if got := at(t, props, "tags", "type"); got != "array" {
		t.Errorf("slice = %v", got)
	}
	if got := at(t, props, "price", "minimum"); got != float64(0) {
		t.Errorf("minimum = %v", got)
	}
	kind := at(t, props, "kind", "enum").([]any)
	if len(kind) != 2 || kind[0] != "book" {
		t.Errorf("enum = %v", kind)
	}
	req := item["required"].([]any)
	if len(req) != 2 || req[0] != "kind" || req[1] != "name" {
		t.Errorf("required = %v", req)
	}
	if got := at(t, props, "owner", "$ref"); got != "#/components/schemas/store.Owner" {
		t.Errorf("$ref = %v", got)
	}
	if got := at(t, doc, "components", "schemas", "store.Owner", "properties", "email", "format"); got != "email" {
		t.Errorf("format do e-mail = %v", got)
	}
}

// TestInferenciaDoHandler: o que o handler faz é o que o documento diz.
func TestInferenciaDoHandler(t *testing.T) {
	doc := decode(t, gen(t, filepath.Join("..", "..", "testdata", "apps", "openapi"), "example.com/openapi", Options{}))
	if _, ok := at(t, doc, "paths").(map[string]any)["/"]; ok {
		t.Error("página entrou no documento")
	}

	get := at(t, doc, "paths", "/api/items", "get").(map[string]any)
	if got := at(t, get, "summary"); got != "GET lists every item." {
		t.Errorf("summary = %v", got)
	}
	if got := at(t, get, "responses", "200", "content", "application/json", "schema", "type"); got != "array" {
		t.Errorf("store.All() devia dar uma lista: %v", got)
	}
	if got := at(t, get, "responses", "200", "content", "application/json", "schema", "items", "$ref"); got != "#/components/schemas/store.Item" {
		t.Errorf("items = %v", got)
	}
	q := at(t, get, "parameters").([]any)[0].(map[string]any)
	if q["name"] != "q" || q["in"] != "query" || q["description"] != "filter by name" {
		t.Errorf("openapi:query = %v", q)
	}
	if got := at(t, get, "tags").([]any)[0]; got != "items" {
		t.Errorf("tag padrão = %v", got)
	}

	post := at(t, doc, "paths", "/api/items", "post").(map[string]any)
	if got := at(t, post, "requestBody", "content", "application/json", "schema", "properties", "name", "maxLength"); got != float64(40) {
		t.Errorf("corpo do BindJSON = %v", got)
	}
	if got := at(t, post, "responses", "201", "content", "application/json", "schema", "$ref"); got != "#/components/schemas/store.Item" {
		t.Errorf("201 = %v", got)
	}
	if got := at(t, post, "responses", "422", "content", ProblemMediaType, "schema", "$ref"); got != "#/components/schemas/Problem" {
		t.Errorf("Bind devia trazer o 422: %v", got)
	}
	if at(t, post, "operationId") != "postApiItems" {
		t.Errorf("operationId = %v", at(t, post, "operationId"))
	}

	one := at(t, doc, "paths", "/api/items/{id}").(map[string]any)
	p := at(t, one, "parameters").([]any)[0].(map[string]any)
	if p["name"] != "id" || p["in"] != "path" || p["required"] != true {
		t.Errorf("parâmetro de caminho = %v", p)
	}
	if got := at(t, one, "get", "responses", "404", "content", ProblemMediaType, "schema", "$ref"); got != "#/components/schemas/Problem" {
		t.Errorf("ErrNotFound devia dar 404: %v", got)
	}
	del := at(t, one, "delete").(map[string]any)
	if _, ok := del["responses"].(map[string]any)["204"].(map[string]any)["content"]; ok {
		t.Error("204 não tem corpo")
	}
	put := at(t, one, "put").(map[string]any)
	if got := at(t, put, "requestBody", "content", "application/json", "schema", "$ref"); got != "#/components/schemas/store.Item" {
		t.Errorf("openapi:body = %v", got)
	}
	if got := at(t, put, "responses", "409", "content", ProblemMediaType, "schema", "$ref"); got != "#/components/schemas/Problem" {
		t.Errorf("openapi:response sem tipo = %v", got)
	}

	csv := at(t, doc, "paths", "/api/report.csv", "get").(map[string]any)
	if _, ok := at(t, csv, "responses", "200", "content").(map[string]any)["text/csv"]; !ok {
		t.Errorf("c.Header devia trocar o media type: %v", at(t, csv, "responses", "200"))
	}
	if got := at(t, csv, "responses", "400", "content", ProblemMediaType, "schema", "$ref"); got != "#/components/schemas/Problem" {
		t.Errorf("trilha.Errorf devia dar 400: %v", got)
	}
	if got := at(t, csv, "tags").([]any)[0]; got != "report" {
		t.Errorf("openapi:tag = %v", got)
	}
	if got := at(t, csv, "responses", "default", "content", ProblemMediaType, "schema", "$ref"); got != "#/components/schemas/Problem" {
		t.Errorf("toda operação tem default: %v", got)
	}
}

// TestTipoDesconhecidoNaDiretiva: nome errado é erro do comando, não schema
// vazio publicado como se estivesse certo.
func TestTipoDesconhecidoNaDiretiva(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "app", "api", "x")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	src := `package x

// GET does nothing.
//
// openapi:response 200 nope.Nope
func GET(c *trilha.Ctx) error { return nil }
`
	if err := os.WriteFile(filepath.Join(dir, "route.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := scan.Scan(root, "example.com/x")
	if err != nil {
		t.Fatal(err)
	}
	_, err = Generate(root, res, Options{})
	if err == nil || !strings.Contains(err.Error(), "nope.Nope") || !strings.Contains(err.Error(), "route.go") {
		t.Fatalf("erro esperado com o tipo e o arquivo, veio: %v", err)
	}
}

// TestExemplos guarda os dois apps de verdade. Golden aqui mudaria a cada
// mexida no exemplo; o que interessa é o contrato continuar de pé.
func TestExemplos(t *testing.T) {
	blog := decode(t, gen(t, filepath.Join("..", "..", "examples", "blog"), "github.com/emersonjoe/trilha/examples/blog", Options{}))
	if got := at(t, blog, "paths", "/api/posts", "post", "responses", "409", "content", ProblemMediaType, "schema", "$ref"); got != "#/components/schemas/Problem" {
		t.Errorf("o 409 do slug repetido sumiu: %v", got)
	}
	if got := at(t, blog, "paths", "/api/posts", "post", "requestBody", "content", "application/json", "schema", "properties", "title", "maxLength"); got != float64(80) {
		t.Errorf("corpo do POST = %v", got)
	}
	if got := at(t, blog, "paths", "/api/posts/{id}", "delete", "responses", "204", "description"); got != "No Content" {
		t.Errorf("204 do DELETE = %v", got)
	}
	if got := at(t, blog, "components", "schemas", "posts.Post", "properties", "created", "format"); got != "date-time" {
		t.Errorf("posts.Post = %v", got)
	}
	// O 429 vem do middleware, que a dedução não lê: sem a diretiva ele some.
	for _, op := range []string{"get", "post"} {
		if got := at(t, blog, "paths", "/api/posts", op, "responses", "429", "content", ProblemMediaType, "schema", "$ref"); got != "#/components/schemas/Problem" {
			t.Errorf("429 do %s = %v", op, got)
		}
	}
	if got := at(t, blog, "paths", "/api/posts/{id}", "delete", "responses", "429", "description"); got != "Too Many Requests" {
		t.Errorf("429 do DELETE = %v", got)
	}

	orc := decode(t, gen(t, filepath.Join("..", "..", "examples", "orcamento"), "github.com/emersonjoe/trilha/examples/orcamento", Options{}))
	csv := at(t, orc, "paths", "/api/relatorio.csv", "get").(map[string]any)
	if _, ok := at(t, csv, "responses", "200", "content").(map[string]any)["text/csv"]; !ok {
		t.Errorf("o CSV devia declarar text/csv: %v", at(t, csv, "responses", "200"))
	}
	if got := at(t, csv, "responses", "400", "content", ProblemMediaType, "schema", "$ref"); got != "#/components/schemas/Problem" {
		t.Errorf("400 do mês inválido = %v", got)
	}
	q := at(t, csv, "parameters").([]any)[0].(map[string]any)
	if q["name"] != "mes" || !strings.Contains(q["description"].(string), "AAAA-MM") {
		t.Errorf("openapi:query = %v", q)
	}
	if got := at(t, csv, "tags").([]any)[0]; got != "relatorio" {
		t.Errorf("openapi:tag = %v", got)
	}
}

// TestInferenciaPorMetodo: um app que alcança seus dados por um valor — uma
// dependência recebida, e não uma função de pacote — continua dizendo o que
// responde. Sem isso o handler injetado publica resposta sem schema.
func TestInferenciaPorMetodo(t *testing.T) {
	root := t.TempDir()
	write := func(rel, src string) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("internal/store/store.go", `package store

// Item is one thing on sale.
type Item struct {
	ID   string `+"`json:\"id\"`"+`
	Name string `+"`json:\"name\"`"+`
}

// Store is the data behind the API.
type Store struct{}

// Open returns the store.
func Open() *Store { return &Store{} }

// Of is the app's own way of reaching a dependency.
func Of[T any]() T { var zero T; return zero }

// All returns every item.
func (s *Store) All() []Item { return nil }

// Get returns one item by id.
func (s *Store) Get(id string) (Item, bool) { return Item{}, false }
`)
	write("app/api/items/route.go", `package items

import (
	"github.com/emersonjoe/trilha"
	"example.com/x/internal/store"
)

// GET lists items through a store held in a variable.
func GET(c *trilha.Ctx) error {
	st := store.Open()
	return c.JSON(200, st.All())
}

// POST answers with one item read straight off a generic call.
func POST(c *trilha.Ctx) error {
	it, _ := store.Of[*store.Store]().Get("x")
	return c.JSON(201, it)
}
`)
	doc := decode(t, gen(t, root, "example.com/x", Options{}))
	get := at(t, doc, "paths", "/api/items", "get").(map[string]any)
	if got := at(t, get, "responses", "200", "content", "application/json", "schema", "type"); got != "array" {
		t.Errorf("st.All() devia dar uma lista: %v", got)
	}
	if got := at(t, get, "responses", "200", "content", "application/json", "schema", "items", "$ref"); got != "#/components/schemas/store.Item" {
		t.Errorf("método em variável local = %v", got)
	}
	post := at(t, doc, "paths", "/api/items", "post").(map[string]any)
	if got := at(t, post, "responses", "201", "content", "application/json", "schema", "$ref"); got != "#/components/schemas/store.Item" {
		t.Errorf("método sobre chamada genérica = %v", got)
	}
}
