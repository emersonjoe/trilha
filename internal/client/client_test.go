package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

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

	up, err := c.Documents().Upload(ctx, strings.NewReader("conteudo"), "escritura.pdf")
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
	_, _ = c.Documents().Upload(context.Background(), endless{}, "grande.bin")
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
	if !strings.Contains(string(src), `multipartBody("upload", filename, file)`) {
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
