package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/emersonjoe/trilha/internal/client/api"
)

var update = flag.Bool("update", false, "rewrite the generated client")

// docPath is the synthetic document: a page, a path parameter, an enum in the
// query, a JSON body, an upload, a binary answer, a oneOf, a $ref that closes a
// cycle, and an operation with no tag.
const docPath = "../../testdata/openapi/acervo.json"

// goldenPath is the generated client, committed like trilha_gen.go: it is what
// `--check` compares against, and it is compiled by the tests below.
const goldenPath = "api/client.go"

func generate(t *testing.T) *Result {
	t.Helper()
	data, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatal(err)
	}
	res, err := Generate(data, Options{Package: "api", Source: "testdata/openapi/acervo.json"})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestGolden(t *testing.T) {
	res := generate(t)
	if *update {
		if err := os.WriteFile(goldenPath, res.Source, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(want, res.Source) {
		t.Fatalf("%s is stale; run `go test ./internal/client -update`", goldenPath)
	}
}

func TestDeterministic(t *testing.T) {
	a, b := generate(t), generate(t)
	if !bytes.Equal(a.Source, b.Source) {
		t.Fatal("two runs, two files")
	}
}

func TestNotes(t *testing.T) {
	res := generate(t)
	var got []string
	for _, n := range res.Notes {
		got = append(got, n.Where+": "+n.What)
	}
	joined := strings.Join(got, "\n")
	for _, want := range []string{
		"Document.payload: oneOf/anyOf",
		"GET /api/folders/{folder_id}/tree: no operationId",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("report missing %q:\n%s", want, joined)
		}
	}
}

// TestSchemasBecameTypes is the part of the issue that pays for the rest: the
// shape of the API is a Go type, so the compiler catches the field that moved.
func TestSchemasBecameTypes(t *testing.T) {
	src, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	s := string(src)
	for _, want := range []string{
		// allOf is flattened: Document has the fields of Base.
		"type Document struct {",
		"ID        string                     `json:\"id\" validate:\"required\"`",
		// A closed list of strings is a defined type and its constants.
		"StatusPending    Status = \"pending\"",
		// required/minLength/format become the tags of the validator.
		"validate:\"required,min=1,max=255\"",
		"validate:\"required,email\"",
		// A $ref that closes a cycle is a pointer; a slice already breaks it.
		"Parent   *Folder",
		"Children []Folder",
		// oneOf keeps arriving, as bytes.
		"Payload   json.RawMessage",
		// The generated client depends on the standard library and nothing else.
		"\"net/http\"",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("generated client missing %q", want)
		}
	}
	if strings.Contains(s, "github.com/emersonjoe/trilha") {
		t.Fatal("the generated client must not import the framework")
	}
}

// TestAgainstServer exercises every shape of operation against a server that
// answers like the document says: query, path parameter, JSON body, upload,
// binary, no content, and the error.
func TestAgainstServer(t *testing.T) {
	var lastAuth, lastQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastAuth = r.Header.Get("Authorization")
		lastQuery = r.URL.RawQuery
		switch {
		case r.URL.Path == "/api/documents" && r.Method == "GET":
			json.NewEncoder(w).Encode(api.PagedDocument{
				Total: 3, Page: 2,
				Items: []api.Document{{ID: "d1", Filename: "nota.pdf", Status: api.StatusReady, Pages: 4}},
			})
		case r.URL.Path == "/api/documents" && r.Method == "POST":
			var in api.DocumentIn
			json.NewDecoder(r.Body).Decode(&in)
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(api.Document{ID: "novo", Filename: in.Filename, Status: api.StatusPending})
		case r.URL.Path == "/api/documents/upload":
			f, hdr, err := r.FormFile("upload")
			if err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			b, _ := io.ReadAll(f)
			json.NewEncoder(w).Encode(api.Document{ID: hdr.Filename, Filename: string(b), Status: api.StatusProcessing})
		case strings.HasSuffix(r.URL.Path, "/file"):
			w.Header().Set("Content-Type", "application/pdf")
			w.Header().Set("Content-Disposition", `attachment; filename="nota.pdf"`)
			io.WriteString(w, "%PDF-1.4")
		case r.URL.Path == "/api/documents/n%C3%A3o+existe" || r.URL.Path == "/api/documents/não existe":
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(http.StatusNotFound)
			io.WriteString(w, `{"detail":"no document with that id"}`)
		case r.Method == "DELETE":
			w.WriteHeader(http.StatusNoContent)
		default:
			json.NewEncoder(w).Encode(api.Document{ID: strings.TrimPrefix(r.URL.Path, "/api/documents/")})
		}
	}))
	defer srv.Close()

	c := api.New(srv.URL, api.WithHeader(func(context.Context) http.Header {
		return http.Header{"Authorization": {"Bearer da-sessao"}}
	}))
	ctx := context.Background()

	page, err := c.Documents().List(ctx, api.DocumentsListParams{Page: 2, Q: "nota", Status: api.StatusReady, Tag: []string{"a", "b"}})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 3 || len(page.Items) != 1 || page.Items[0].Status != api.StatusReady {
		t.Fatalf("page = %+v", page)
	}
	if lastQuery != "page=2&q=nota&status=ready&tag=a&tag=b" {
		t.Fatalf("query = %s", lastQuery)
	}
	if lastAuth != "Bearer da-sessao" {
		t.Fatalf("auth = %q", lastAuth)
	}

	doc, err := c.Documents().Create(ctx, api.DocumentIn{Filename: "contrato.pdf"})
	if err != nil || doc.ID != "novo" || doc.Filename != "contrato.pdf" {
		t.Fatalf("create = %+v (%v)", doc, err)
	}

	up, err := c.Documents().Upload(ctx, api.DocumentsUploadForm{
		Upload: api.FilePart{Filename: "escritura.pdf", Content: strings.NewReader("conteudo")},
	})
	if err != nil || up.ID != "escritura.pdf" || up.Filename != "conteudo" {
		t.Fatalf("upload = %+v (%v)", up, err)
	}

	// A binary answer arrives as the response, so the headers and the stream
	// survive: this is what c.Pipe is handed.
	resp, err := c.Documents().Download(ctx, "d1")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "application/pdf" {
		t.Fatalf("content-type = %s", ct)
	}
	if b, _ := io.ReadAll(resp.Body); string(b) != "%PDF-1.4" {
		t.Fatalf("body = %s", b)
	}

	if err := c.Documents().Delete(ctx, "d1"); err != nil {
		t.Fatal(err)
	}

	// The path parameter is escaped, so it cannot reach for another endpoint.
	if _, err := c.Documents().Get(ctx, "../users"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(lastQuery, "users") || lastAuth == "" {
		t.Fatalf("query = %q", lastQuery)
	}

	_, err = c.Documents().Get(ctx, "não existe")
	var apiErr *api.Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v", err)
	}
	if apiErr.Status != http.StatusNotFound || apiErr.Detail != "no document with that id" {
		t.Fatalf("err = %+v", apiErr)
	}
	if !strings.Contains(apiErr.Error(), "no document with that id") {
		t.Fatalf("message = %s", apiErr.Error())
	}
}

// TestUploadStreams proves the file is not buffered: a reader that never ends
// still reaches the server, one chunk at a time.
func TestUploadStreams(t *testing.T) {
	seen := make(chan int, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mr, err := r.MultipartReader()
		if err != nil {
			t.Error(err)
			return
		}
		part, err := mr.NextPart()
		if err != nil {
			t.Error(err)
			return
		}
		n, _ := io.CopyN(io.Discard, part, 1<<20)
		seen <- int(n)
		json.NewEncoder(w).Encode(api.Document{ID: "ok"})
	}))
	defer srv.Close()
	c := api.New(srv.URL)
	_ = c.Documents().Thumbnail(context.Background(), "doc-1", endless{}, "grande.bin")
	if n := <-seen; n != 1<<20 {
		t.Fatalf("read %d bytes", n)
	}
}

// endless is a file the server can read forever: nothing this big fits in the
// memory of the process that is only passing it along.
type endless struct{}

func (endless) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'x'
	}
	return len(p), nil
}

// TestReadRefuses covers the documents this generator will not pretend to read.
func TestReadRefuses(t *testing.T) {
	for _, tc := range []struct{ name, doc, want string }{
		{"swagger", `{"swagger":"2.0","openapi":"2.0","paths":{}}`, "Swagger 2.0"},
		{"no version", `{"paths":{}}`, "no \"openapi\""},
		{"broken", `{`, "openapi:"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := Read([]byte(tc.doc)); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

// TestMultipartFieldName keeps the generator honest about which part of the
// form carries the file: it is the property the document marks as binary.
func TestMultipartFieldName(t *testing.T) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("upload", "x.pdf")
	fmt.Fprint(fw, "x")
	mw.Close()
	src, _ := os.ReadFile(goldenPath)
	if !strings.Contains(string(src), `formPart{{field: "image", filename: filename, r: file}}`) {
		t.Fatal("the upload field name did not come from the document")
	}
}

// Spec 062 (#95): Pydantic writes every Optional[T] as anyOf [T, null]. In a
// FastAPI document that is not an exotic case, it is the most common shape
// there is — 39% of the fields in the document this generator was tested
// against — and carrying it as json.RawMessage made the caller marshal by hand
// exactly where the client was supposed to help.
func TestOptionalBecomesAPointerAndAUnionDoesNot(t *testing.T) {
	src := string(generate(t).Source)
	for _, want := range []string{
		"TenantID *string",      // anyOf [string, null]
		"PageCount *int64",      // anyOf [integer, null]
		"Owner     *DocumentIn", // anyOf [$ref, null]
	} {
		if !strings.Contains(src, want) {
			t.Errorf("Optional[T] did not become a pointer: no %q", want)
		}
	}
	// Two real members is a union the generator cannot name, and it stays what
	// it was: the rule is "T or null", not "two of anything".
	if !strings.Contains(src, "Either    json.RawMessage") {
		t.Error("a union of three members stopped being json.RawMessage")
	}
	// And the report only mentions what actually stayed raw.
	notes := 0
	for _, n := range generate(t).Notes {
		if strings.Contains(n.What, "oneOf/anyOf") {
			notes++
			if strings.Contains(n.Where, "tenant_id") || strings.Contains(n.Where, "pages") || strings.Contains(n.Where, "owner") {
				t.Errorf("the report still complains about a field that became a pointer: %s", n.Where)
			}
		}
	}
	if notes == 0 {
		t.Error("the report says nothing about the union it did carry raw")
	}
}

// Spec 092 (#141): a multipart operation is a form, not a file. The generator
// read one binary field and dropped everything else — this document has always
// declared `folder` beside `upload`, and it never reached the caller.
func TestMultipartECadaParteDoFormulario(t *testing.T) {
	type parte struct{ field, filename, value string }
	got := make(chan []parte, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mr, err := r.MultipartReader()
		if err != nil {
			t.Error(err)
			return
		}
		var partes []parte
		for {
			p, err := mr.NextPart()
			if err != nil {
				break
			}
			body, _ := io.ReadAll(p)
			partes = append(partes, parte{p.FormName(), p.FileName(), string(body)})
		}
		got <- partes
		w.WriteHeader(202)
		json.NewEncoder(w).Encode(api.Document{ID: "ok"})
	}))
	defer srv.Close()
	c := api.New(srv.URL)

	// A file and the scalars beside it, each typed.
	if _, err := c.Certificates().Upload(context.Background(), api.CertificatesUploadForm{
		File:  api.FilePart{Filename: "cert.pfx", Content: strings.NewReader("PFX")},
		Senha: "abre-te",
		Dias:  30,
	}); err != nil {
		t.Fatal(err)
	}
	// The parts come out in the order of the field names, which is the order of
	// the struct: the document has none of its own to preserve.
	want := []parte{{"dias", "", "30"}, {"file", "cert.pfx", "PFX"}, {"senha", "", "abre-te"}}
	if partes := <-got; !equalPartes(partes, want) {
		t.Fatalf("as partes do certificado = %+v, quero %+v", partes, want)
	}

	// The same name, repeated, in the order of the slice: a list[UploadFile] on
	// the other side is a list, and the order is the answer's order.
	if _, err := c.Documents().Batch(context.Background(), api.DocumentsBatchForm{
		Files: []api.FilePart{
			{Filename: "1.xml", Content: strings.NewReader("um")},
			{Filename: "2.xml", Content: strings.NewReader("dois")},
			{Filename: "3.xml", Content: strings.NewReader("tres")},
		},
	}); err != nil {
		t.Fatal(err)
	}
	want = []parte{{"files", "1.xml", "um"}, {"files", "2.xml", "dois"}, {"files", "3.xml", "tres"}}
	if partes := <-got; !equalPartes(partes, want) {
		t.Fatalf("as partes do lote = %+v, quero %+v", partes, want)
	}

	// An optional scalar that was not filled in does not travel: the same rule
	// the query already follows, so the server sees the difference between an
	// empty folder and no folder at all.
	if _, err := c.Documents().Upload(context.Background(), api.DocumentsUploadForm{
		Upload: api.FilePart{Filename: "nota.pdf", Content: strings.NewReader("PDF")},
	}); err != nil {
		t.Fatal(err)
	}
	if partes := <-got; !equalPartes(partes, []parte{{"upload", "nota.pdf", "PDF"}}) {
		t.Fatalf("o campo opcional vazio viajou: %+v", partes)
	}

	// And the one shape that keeps the signature it had: one binary field and
	// nothing else stays two arguments.
	if err := c.Documents().Thumbnail(context.Background(), "doc-1", strings.NewReader("PNG"), "capa.png"); err != nil {
		t.Fatal(err)
	}
	if partes := <-got; !equalPartes(partes, []parte{{"image", "capa.png", "PNG"}}) {
		t.Fatalf("a miniatura = %+v", partes)
	}
}

func equalPartes[T comparable](got, want []T) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// The stream is the promise: a file bigger than the memory of this process has
// to cross it, and a form with several files must not change that.
func TestMultipartContinuaEmStreaming(t *testing.T) {
	seen := make(chan int, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mr, err := r.MultipartReader()
		if err != nil {
			t.Error(err)
			return
		}
		p, err := mr.NextPart()
		if err != nil {
			t.Error(err)
			return
		}
		n, _ := io.CopyN(io.Discard, p, 1<<20)
		seen <- int(n)
		w.WriteHeader(202)
		json.NewEncoder(w).Encode(api.Document{ID: "ok"})
	}))
	defer srv.Close()
	c := api.New(srv.URL)
	_, _ = c.Documents().Batch(context.Background(), api.DocumentsBatchForm{
		Files: []api.FilePart{{Filename: "grande.bin", Content: endless{}}},
	})
	if n := <-seen; n != 1<<20 {
		t.Fatalf("read %d bytes", n)
	}
}

// Two binary fields in one operation: the shape the issue asks to be
// representable, and the one a signature of positional arguments could not hold
// without the caller counting readers.
func TestDoisCamposBinariosNaMesmaOperacao(t *testing.T) {
	doc := `{"openapi":"3.1.0","info":{"title":"x","version":"1"},"paths":{"/envio":{"post":{
		"operationId":"enviar","requestBody":{"required":true,"content":{"multipart/form-data":{"schema":{
		"type":"object","properties":{"nota":{"type":"string","format":"binary"},"anexo":{"type":"string","format":"binary"}},
		"required":["nota","anexo"]}}}},"responses":{"204":{"description":"ok"}}}}}}`
	res, err := Generate([]byte(doc), Options{Package: "api"})
	if err != nil {
		t.Fatal(err)
	}
	src := string(res.Source)
	for _, want := range []string{
		"type DefaultEnviarForm struct {",
		"Anexo FilePart",
		"Nota  FilePart", // gofmt aligns the pair, which is how we know both are there
		`formPart{field: "anexo", filename: form.Anexo.Filename, r: form.Anexo.Content}`,
		`formPart{field: "nota", filename: form.Nota.Filename, r: form.Nota.Content}`,
	} {
		if !strings.Contains(src, want) {
			t.Errorf("o cliente gerado não tem %q:\n%s", want, src)
		}
	}
}

// Spec 122 (#160): the comment of a schema was cut at byte 107, and a document
// written in Portuguese puts an accent there sooner or later. Cutting a string
// in the middle of a UTF-8 sequence gives a file the parser refuses, so the
// command failed and wrote nothing — with `illegal UTF-8 encoding` as the only
// explanation.
func TestDescricaoComAcentoNoCorte(t *testing.T) {
	// 106 times "a" and then "ção": the ç takes bytes 107 and 108, so the old
	// cut fell inside it.
	long := strings.Repeat("a", 106) + "ção — descrição comprida o bastante para passar de 110 bytes"
	doc := `{"openapi":"3.1.0","info":{"title":"repro","version":"1"},
		"paths":{"/x":{"get":{"operationId":"x","description":` + quote(long) + `,
		"responses":{"200":{"description":"ok","content":{"application/json":{"schema":{"$ref":"#/components/schemas/X"}}}}}}}},
		"components":{"schemas":{"X":{"type":"object","title":"X","description":` + quote(long) + `,
		"properties":{"k":{"type":"string"}}}}}}`
	res, err := Generate([]byte(doc), Options{Package: "api"})
	if err != nil {
		t.Fatalf("a description with an accent at the cut broke the generator: %v", err)
	}
	if !utf8.Valid(res.Source) {
		t.Fatal("the generated file is not valid UTF-8")
	}
	// Both cuts: the schema's comment and the method's.
	src := string(res.Source)
	if n := strings.Count(src, "aaa...") + strings.Count(src, "ç..."); n < 2 {
		t.Fatalf("the two comments were not shortened:\n%s", src)
	}
	if strings.Contains(src, "�") {
		t.Fatal("the cut left a replacement character behind")
	}
}

// Spec 122 (#157): a FastAPI without response_model declares no schema for the
// answer, and every operation comes back as json.RawMessage. The file compiles
// and types nothing, and until now the command said so about schemas but never
// about how much of the API that was.
func TestContaAsOperacoesSemSchema(t *testing.T) {
	doc := `{"openapi":"3.1.0","info":{"title":"x","version":"1"},"paths":{
		"/api/folhas":{"get":{"operationId":"listar","tags":["folhas"],"responses":{"200":{"description":"ok",
			"content":{"application/json":{"schema":{}}}}}}},
		"/api/folhas/{id}":{"get":{"operationId":"ler","tags":["folhas"],
			"parameters":[{"name":"id","in":"path","required":true,"schema":{"type":"string"}}],
			"responses":{"200":{"description":"ok","content":{"application/json":{"schema":{}}}}}}},
		"/api/saude":{"get":{"operationId":"saude","tags":["sistema"],"responses":{"200":{"description":"ok",
			"content":{"application/json":{"schema":{"type":"object","properties":{"ok":{"type":"boolean"}}}}}}}}}}}`
	res, err := Generate([]byte(doc), Options{Package: "api"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Ops != 3 {
		t.Errorf("Ops = %d, want 3", res.Ops)
	}
	want := []string{"GET /api/folhas", "GET /api/folhas/{id}"}
	if !reflect.DeepEqual(res.Untyped, want) {
		t.Errorf("Untyped = %q, want %q", res.Untyped, want)
	}
	// And the same fact is not also a line of the per-schema report: with
	// ninety-seven of them the report was the noise, which is why the count
	// exists at all.
	for _, n := range res.Notes {
		if strings.HasSuffix(n.Where, " response") {
			t.Errorf("the untyped answer is on the count and on the report: %s: %s", n.Where, n.What)
		}
	}
}

// Spec 122 (#158): the 422 of a FastAPI is a list of messages, one per field.
// It used to become one sentence — and the sentence was the whole object,
// serialized — so a form that wanted to mark the field had to parse Body by
// hand on every page.
func TestErro422ChegaPorCampo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		io.WriteString(w, `{"detail":[
			{"loc":["body","email"],"msg":"value is not a valid email address","type":"value_error"},
			{"loc":["body","itens",0,"valor"],"msg":"must be positive","type":"value_error"},
			{"loc":["query","page"],"msg":"input should be greater than 0","type":"greater_than"},
			{"loc":[],"msg":"orphan"}]}`)
	}))
	defer srv.Close()

	_, err := api.New(srv.URL).Documents().Create(context.Background(), api.DocumentIn{Filename: "x"})
	e, ok := api.AsError(err)
	if !ok {
		t.Fatalf("err = %v", err)
	}
	if e.Status != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d", e.Status)
	}
	want := map[string]string{
		"email":         "value is not a valid email address",
		"itens.0.valor": "must be positive",
		"page":          "input should be greater than 0",
	}
	if !reflect.DeepEqual(e.Fields, want) {
		t.Errorf("Fields = %#v, want %#v", e.Fields, want)
	}
	// Detail stays the first message, and it is a sentence now, not the whole
	// object serialized.
	if e.Detail != "value is not a valid email address" {
		t.Errorf("Detail = %q", e.Detail)
	}

	// An error that is not a list of fields keeps everything it had, and Fields
	// stays nil rather than empty-and-meaningless.
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, `{"detail":"no document with that id"}`)
	}))
	defer plain.Close()
	_, err = api.New(plain.URL).Documents().Create(context.Background(), api.DocumentIn{Filename: "x"})
	e, ok = api.AsError(err)
	if !ok {
		t.Fatalf("err = %v", err)
	}
	if e.Fields != nil || e.Detail != "no document with that id" {
		t.Errorf("plain error = %+v", e)
	}
	if _, ok := api.AsError(nil); ok {
		t.Error("AsError(nil) said yes")
	}
}

// Spec 128 (#169): the group's name is cut off the front of the method's name
// so that Documents.ListDocuments reads Documents.List. The cut was literal, so
// a tag that happens to spell the start of the first word took that word apart:
// `config` + `configurar_regra` gave `urarRegra`, a method nobody outside the
// package can call. The file still compiles — an unexported method is valid Go
// — and that is why the bug reached a real migration: it only shows up in the
// `go build` of the caller, or never, if nobody calls that route.
func TestPrefixoDaTagSoCaiEmFronteiraDePalavra(t *testing.T) {
	ok := `"responses":{"200":{"description":"ok","content":{"application/json":{"schema":{"type":"object","properties":{"ok":{"type":"boolean"}}}}}}}`
	key := `"parameters":[{"name":"regra_key","in":"path","required":true,"schema":{"type":"string"}}]`
	doc := `{"openapi":"3.1.0","info":{"title":"x","version":"1"},"paths":{
		"/api/regras/{regra_key}":{
			"put":{"operationId":"configurar_regra","tags":["config"],` + key + `,` + ok + `},
			"delete":{"operationId":"restaurar_regra","tags":["config"],` + key + `,` + ok + `}},
		"/api/config/reset":{"post":{"operationId":"config_reset","tags":["config"],` + ok + `}},
		"/api/regras":{"get":{"operationId":"listar_regras","tags":["regras"],` + ok + `}}}}`
	res, err := Generate([]byte(doc), Options{Package: "api"})
	if err != nil {
		t.Fatal(err)
	}
	f, err := parser.ParseFile(token.NewFileSet(), "client.go", res.Source, 0)
	if err != nil {
		t.Fatalf("the generated client does not parse: %v", err)
	}
	// The method set of each group, read from the AST: an operation that is
	// there under another name is not the same as one that is there.
	groups := map[string][]string{}
	for _, d := range f.Decls {
		fn, isFunc := d.(*ast.FuncDecl)
		if !isFunc || fn.Recv == nil {
			continue
		}
		recv := recvName(fn)
		if recv == "Client" || recv == "Error" {
			continue // the runtime, whose do/call are unexported on purpose
		}
		groups[recv] = append(groups[recv], fn.Name.Name)
		// The whole point: a method the caller's package cannot see is the
		// same as an operation the generator did not write.
		if !fn.Name.IsExported() {
			t.Errorf("%s.%s is not exported — nobody outside the package can call it", recv, fn.Name.Name)
		}
	}
	for _, want := range []struct{ group, method string }{
		{"Config", "ConfigurarRegra"}, // Config is not a word of ConfigurarRegra: no cut
		{"Config", "RestaurarRegra"},  // never matched the tag, and is the neighbour that hid the bug
		{"Config", "Reset"},           // ConfigReset does start with the word Config: still cut
		{"Regras", "Listar"},          // and the suffix side, untouched: ListarRegras in tag regras
	} {
		if !slices.Contains(groups[want.group], want.method) {
			t.Errorf("no %s.%s in the generated client; %s has %v", want.group, want.method, want.group, groups[want.group])
		}
	}
}

// Spec 129 (#166): a tag and a schema of the document can spell the same Go
// name — `auditoria` and `Auditoria` — and the generator gave both to the same
// identifier, so the file had `type Auditoria` twice and did not compile. The
// command still ended with `client.go written`, so the error only surfaced in
// the `go build` of whoever called it. The group is the name the generator
// invents, so the group is the one that yields.
func TestTagQueColideComSchema(t *testing.T) {
	doc := `{"openapi":"3.1.0","info":{"title":"x","version":"1"},"paths":{
		"/api/auditoria":{"get":{"operationId":"listar_auditoria","tags":["auditoria"],
			"parameters":[{"name":"pagina","in":"query","schema":{"type":"integer"}}],
			"responses":{"200":{"description":"ok","content":{"application/json":{
				"schema":{"$ref":"#/components/schemas/Auditoria"}}}}}}}},
		"components":{"schemas":{"Auditoria":{"type":"object","title":"the audit log",
			"properties":{"total":{"type":"integer"},"acoes":{"type":"array","items":{"type":"string"}}},
			"required":["total"]}}}}`
	res, err := Generate([]byte(doc), Options{Package: "api"})
	if err != nil {
		t.Fatal(err)
	}
	compile(t, res.Source)

	f, err := parser.ParseFile(token.NewFileSet(), "client.go", res.Source, 0)
	if err != nil {
		t.Fatalf("the generated client does not parse: %v", err)
	}
	seen := map[string]bool{}
	for _, name := range declaredTypes(f) {
		if seen[name] {
			t.Errorf("type %s is declared twice", name)
		}
		seen[name] = true
	}
	for _, want := range []string{"Auditoria", "AuditoriaAPI", "AuditoriaListarParams"} {
		if !seen[want] {
			t.Errorf("no type %s in the generated client; it has %v", want, declaredTypes(f))
		}
	}
	// The schema keeps the name the document gave it, fields and all.
	src := string(res.Source)
	for _, want := range []string{
		"type Auditoria struct {",
		"Total int64    `json:\"total\" validate:\"required\"`",
		// The call does not carry the suffix: it exists for the compiler.
		"func (c *Client) Auditoria() *AuditoriaAPI { return &AuditoriaAPI{c: c} }",
		"func (g *AuditoriaAPI) Listar(ctx context.Context, p AuditoriaListarParams) (Auditoria, error) {",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("generated client missing %q:\n%s", want, src)
		}
	}
	// And the report says which name it had to invent, instead of ending with
	// `client.go written` and leaving the news for the caller's build.
	var notes []string
	for _, n := range res.Notes {
		notes = append(notes, n.Where+": "+n.What)
	}
	if joined := strings.Join(notes, "\n"); !strings.Contains(joined, "AuditoriaAPI") {
		t.Errorf("the report does not name the group it renamed:\n%s", joined)
	}
}

// Spec 129 (#177): a tag with an accent is the rule and not the exception in a
// FastAPI written in Portuguese — the tag is the label of the /docs page. The
// generator kept the accent in the identifier, which compiles, and that is why
// it went unnoticed until someone had to type `c.VerificaçãoDeAssinaturas()`.
// A name the generator invents is ASCII, like every exported identifier in this
// repository; the label stays in the comment, where it is read and not typed.
func TestIdentificadorDoDocumentoEASCII(t *testing.T) {
	ok := `"responses":{"200":{"description":"OK","content":{"application/json":{"schema":{}}}}}`
	code := `"parameters":[{"name":"code","in":"path","required":true,"schema":{"type":"string"}}]`
	doc := `{"openapi":"3.1.0","info":{"title":"repro","version":"1"},"paths":{
		"/api/mcp/servers":{"get":{"tags":["dados públicos (MCP)"],
			"operationId":"listar_servers_api_mcp_servers_get",` + ok + `}},
		"/api/verificacao/{code}":{"get":{"tags":["verificação de assinaturas"],
			"operationId":"verificar_api_verificacao__code__get",` + code + `,` + ok + `}},
		"/api/formulario/{token}":{"get":{"tags":["formulário externo"],
			"operationId":"abrir_formulario_api_formulario__token__get",
			"parameters":[{"name":"token","in":"path","required":true,"schema":{"type":"string"}}],` + ok + `}}},
		"components":{"schemas":{"Endereço":{"type":"object",
			"properties":{"número":{"type":"string"},"código_postal":{"type":"string"},"área":{"type":"string"}},
			"required":["número"]}}}}`
	res, err := Generate([]byte(doc), Options{Package: "api"})
	if err != nil {
		t.Fatal(err)
	}
	compile(t, res.Source)

	f, err := parser.ParseFile(token.NewFileSet(), "client.go", res.Source, 0)
	if err != nil {
		t.Fatalf("the generated client does not parse: %v", err)
	}
	// Every identifier, not only the ones named below: the rule is the alphabet.
	ast.Inspect(f, func(n ast.Node) bool {
		id, isIdent := n.(*ast.Ident)
		if !isIdent {
			return true
		}
		for _, r := range id.Name {
			if r >= utf8.RuneSelf {
				t.Errorf("identifier %q is not ASCII", id.Name)
				break
			}
		}
		return true
	})
	types := declaredTypes(f)
	for _, want := range []string{"DadosPublicosMCP", "VerificacaoDeAssinaturas", "FormularioExterno", "Endereco"} {
		if !slices.Contains(types, want) {
			t.Errorf("no type %s in the generated client; it has %v", want, types)
		}
	}
	src := string(res.Source)
	for _, want := range []string{
		"Numero       string `json:\"número\" validate:\"required\"`",
		"CodigoPostal string `json:\"código_postal,omitempty\"`",
		// A field whose first letter carried the accent was not even exported:
		// the first byte was part of a UTF-8 sequence, so there was nothing for
		// ToUpper to raise, and `área` stayed a field encoding/json cannot see.
		"Area         string `json:\"área,omitempty\"`",
		// The label the document wrote stays where it is read.
		`// The document's tag is "dados públicos (MCP)".`,
	} {
		if !strings.Contains(src, want) {
			t.Errorf("generated client missing %q:\n%s", want, src)
		}
	}
}

// compile builds the generated client in a module of its own. The generator
// imports the standard library and nothing else, so this needs no network — and
// it is the only check that means what the issues mean: `it does not compile`.
func compile(t *testing.T, src []byte) {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not in PATH")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module gen\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "client.go"), src, 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("the generated client does not compile: %v\n%s\n%s", err, out, src)
	}
}

// declaredTypes are the names of the top-level types, in the order they appear.
func declaredTypes(f *ast.File) []string {
	var out []string
	for _, d := range f.Decls {
		gen, isGen := d.(*ast.GenDecl)
		if !isGen || gen.Tok != token.TYPE {
			continue
		}
		for _, s := range gen.Specs {
			if ts, ok := s.(*ast.TypeSpec); ok {
				out = append(out, ts.Name.Name)
			}
		}
	}
	return out
}

// recvName is the type a method hangs on, pointer or not.
func recvName(fn *ast.FuncDecl) string {
	t := fn.Recv.List[0].Type
	if star, ok := t.(*ast.StarExpr); ok {
		t = star.X
	}
	if id, ok := t.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}
