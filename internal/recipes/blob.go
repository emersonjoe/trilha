package recipes

// blobRecipe is keeping a file somebody sent, and handing it back.
//
// It is the feature that looks simplest and has the most traps: the client's
// filename becoming a path, the same file taking space three times, the
// Content-Type the browser executes, and a listing pointing at a key that is
// no longer there. The blob module answers all four; this is the path to it.
func blobRecipe() Recipe {
	return Recipe{
		Name: "blob",
		Summary: map[string]string{
			"en": "keeping files somebody sent: upload, list, serve — with the key as the digest",
			"pt": "guardar arquivo que alguém mandou: enviar, listar, servir — com a chave sendo o digest",
		},
		Doc: "/reference/blob",
		Files: []File{
			{Rel: "internal/arquivos/arquivos.go", Go: true, Body: blobStore},
			{Rel: "internal/arquivos/arquivos_test.go", Go: true, Body: blobStoreTest},
			{Rel: "{{.At}}arquivos/page.go", Go: true, Body: blobPage},
			{Rel: "{{.At}}arquivos/chave__/route.go", Go: true, Body: blobServe},
			{Rel: "arquivos_test.go", Go: true, Body: blobTest},
		},
		Setup: []Insert{{
			Marker: "// trilha:add blob",
			Line:   "\ttrilha.Provide(a, arquivos.Novo())\n",
		}},
		Imports: []string{"{{.Module}}/internal/arquivos"},
		Next: map[string]string{
			"en": "Run `trilha dev` and open {{.URL}}arquivos. The store is memory: swap it for " +
				"blob.FromEnv() and one environment variable decides a folder on a laptop or a bucket in " +
				"production. Guard the folder if what people upload is not public.",
			"pt": "Rode `trilha dev` e abra {{.URL}}arquivos. O store é memória: troque por blob.FromEnv() " +
				"e uma variável de ambiente decide entre uma pasta no notebook e um bucket em produção. " +
				"Guarde a pasta se o que as pessoas mandam não é público.",
		},
	}
}

const blobStore = `// Package arquivos is where this application keeps what people upload, and
// the table that says what each stored blob is.
//
// The two halves are deliberate. The blob store keeps bytes and answers by
// key; knowing that a given key is the attachment of order 12 is the
// application's job, and that is the table below.
package arquivos

import (
	"sync"
	"time"

	"github.com/emersonjoe/trilha/blob"
)

// Arquivo is one row of the table: what the person called it, and the key the
// bytes are under.
type Arquivo struct {
	Nome   string
	Tipo   string
	Tamanho int64
	Chave  string
	Quando time.Time
}

// Store is the blob store plus the table.
type Store struct {
	Files *blob.Files

	mu   sync.RWMutex
	rows []Arquivo
}

// Novo builds it. Memory here, because a recipe has to run from a fresh
// project with nothing configured; a real application writes
// blob.New(blob.FromEnv()), and one environment variable decides between a
// folder on a laptop and a bucket in production.
func Novo() *Store {
	return &Store{Files: blob.New(blob.NewMemory())}
}

// Add records what was stored. It is the application's half: the blob module
// already has the bytes.
func (s *Store) Add(nome, tipo string, tamanho int64, chave string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rows = append(s.rows, Arquivo{Nome: nome, Tipo: tipo, Tamanho: tamanho, Chave: chave, Quando: time.Now()})
}

// All is the listing, newest first.
//
// The order is the order things were added, reversed — and not a comparison of
// timestamps: two files stored in the same millisecond are a normal thing, and
// a clock with millisecond resolution would put them in whichever order the
// sort felt like.
func (s *Store) All() []Arquivo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Arquivo, 0, len(s.rows))
	for i := len(s.rows) - 1; i >= 0; i-- {
		out = append(out, s.rows[i])
	}
	return out
}
`

const blobStoreTest = `package arquivos

import (
	"context"
	"testing"
)

// A chave é o digest do conteúdo: o mesmo arquivo mandado duas vezes ocupa
// espaço uma vez, e o nome que veio do cliente nunca vira caminho.
func TestMesmoConteudoMesmaChave(t *testing.T) {
	s := Novo()
	ctx := context.Background()
	a, err := s.Files.PutBytes(ctx, "nota.txt", []byte("mesmo conteudo"), "text/plain")
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.Files.PutBytes(ctx, "outro-nome.txt", []byte("mesmo conteudo"), "text/plain")
	if err != nil {
		t.Fatal(err)
	}
	if a.Key != b.Key {
		t.Fatalf("chaves diferentes para o mesmo conteúdo: %s e %s", a.Key, b.Key)
	}
	// E a chave não é o nome: um nome vindo de fora que virasse caminho é
	// travessia de diretório esperando alguém digitar "../".
	if a.Key == "nota.txt" {
		t.Fatal("a chave é o nome do cliente")
	}
}

func TestListaVemDoMaisNovo(t *testing.T) {
	s := Novo()
	s.Add("um.txt", "text/plain", 3, "k1")
	s.Add("dois.txt", "text/plain", 3, "k2")
	if lista := s.All(); len(lista) != 2 || lista[0].Nome != "dois.txt" {
		t.Fatalf("lista = %+v", lista)
	}
}
`

const blobPage = `// Package arquivos is the upload screen: send, and see what is there.
package arquivos

import (
	"net/http"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"

	"{{.Module}}/internal/arquivos"
)

// regras are declared, and that is the point: an upload with no limit is a
// full disk waiting for a day. The type is checked against what the content
// actually is, not against what the client said it was.
var regras = trilha.FileRules{
	MaxSize:  4 << 20,
	MaxFiles: 10,
	Accept:   []string{"image/*", "application/pdf", "text/plain"},
}

// Page renders GET {{.URL}}arquivos.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("{{.T.blob_title}}")
	return tela(c, nil), nil
}

// POST receives the files. The kit's queue sends one per request — each with
// its own bar and its own message — and a browser with no JavaScript sends them
// all at once; c.Files answers both with the same code.
func POST(c *trilha.Ctx) error {
	store := trilha.Use[*arquivos.Store](c)
	ups, err := c.Files("arquivos", regras)
	if err != nil {
		errs, ok := err.(trilha.FieldErrors)
		if !ok {
			return err
		}
		return c.Render(http.StatusUnprocessableEntity, tela(c, errs))
	}
	for _, up := range ups {
		// The blob computes the digest, stores by it and hands back the key.
		// The name that came from the client never becomes a path, and the
		// same file sent twice takes space once.
		ref, err := store.Files.Put(c.Context(), up)
		up.Close()
		if err != nil {
			return err
		}
		store.Add(ref.Name, ref.Type, ref.Size, ref.Key)
	}
	return c.Redirect("{{.URL}}arquivos")
}

func tela(c *trilha.Ctx, errs trilha.FieldErrors) h.Node {
	store := trilha.Use[*arquivos.Store](c)
	lista := store.All()
	linhas := make([]h.Node, 0, len(lista))
	for _, a := range lista {
		linhas = append(linhas, h.Tr(
			h.Td(h.A(h.Href("{{.URL}}arquivos/"+a.Chave), h.Text(a.Nome))),
			h.Td(h.Text(a.Tipo)),
			h.Td(ui.Bytes(c, a.Tamanho)),
			h.Td(ui.Date(c, a.Quando)),
		))
	}
	var tabela h.Node = ui.Empty(ui.EmptyOpts{Icon: "upload", Title: "{{.T.blob_empty}}"})
	if len(linhas) > 0 {
		tabela = ui.Table(
			h.Thead(h.Tr(h.Th(h.Text("{{.T.blob_name}}")), h.Th(h.Text("{{.T.blob_type}}")),
				h.Th(h.Text("{{.T.blob_size}}")), h.Th(h.Text("{{.T.blob_when}}")))),
			h.Tbody(linhas...),
		)
	}
	return ui.Stack(
		ui.PageHeader("{{.T.blob_title}}"),
		ui.Muted(h.Text("{{.T.blob_desc}}")),
		h.Form(h.Method("post"), h.Action("{{.URL}}arquivos"), h.Class("ui-stack"),
			h.Attr("enctype", "multipart/form-data"),
			trilha.CSRFInput(c),
			ui.Field("arquivos", "{{.T.blob_field}}",
				ui.Input(h.ID("arquivos"), h.Name("arquivos"), h.Type("file"), h.Attr("multiple", "")),
				ui.Errors(errs, "arquivos")),
			h.Div(ui.Submit(h.Text("{{.T.blob_send}}"))),
		),
		tabela,
	)
}

`

const blobServe = `// Package chave serves what was stored, by key.
//
// The folder is a catch-all — chave__ — because a blob key has slashes in it:
// it is ab/cd/<digest>, so that a bucket listing is not one folder with a
// million entries. A single-segment parameter would answer 404 for every file.
package chave

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/blob"

	"{{.Module}}/internal/arquivos"
)

// GET hands the bytes back.
//
// Serve is what decides the headers, and that is the whole reason not to write
// this by hand: the type is the one the store kept, and what a browser would
// run instead of render is refused. An HTML file somebody uploaded is not
// served as executable HTML on your domain.
func GET(c *trilha.Ctx) error {
	store := trilha.Use[*arquivos.Store](c)
	chave := c.Param("chave")
	nome := chave
	for _, a := range store.All() {
		if a.Chave == chave {
			nome = a.Nome
			break
		}
	}
	return store.Files.Serve(c, chave, blob.ServeOpts{Name: nome})
}
`

const blobTest = `package main

import (
	"bytes"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha"
)

// Enviar, aparecer na lista, e voltar pela chave.
func TestArquivoVaiEVolta(t *testing.T) {
	t.Setenv("TRILHA_ENV", "prod")
	t.Setenv("TRILHA_SECRET", "a-test-secret-with-more-than-32-bytes!!")
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	c := trilha.NewTestClient(t, newApp())

	corpo, ctype := arquivo(t, "nota.txt", "o conteudo do arquivo")
	c.Request("POST", "{{.URL}}arquivos",
		trilha.WithBody(ctype, corpo)).WantStatus(http.StatusSeeOther)

	pagina := c.Get("{{.URL}}arquivos").WantStatus(http.StatusOK).Body.String()
	if !strings.Contains(pagina, "nota.txt") {
		t.Fatalf("o arquivo não apareceu na lista:\n%s", pagina)
	}
	chave := entre(pagina, "{{.URL}}arquivos/", "\"")
	if chave == "" {
		t.Fatalf("a lista não trouxe o link do arquivo:\n%s", pagina)
	}
	c.Get("{{.URL}}arquivos/" + chave).WantStatus(http.StatusOK).WantContains("o conteudo do arquivo")
}

// arquivo monta um multipart com um arquivo só.
func arquivo(t *testing.T, nome, conteudo string) (string, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("arquivos", nome)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write([]byte(conteudo)); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.String(), mw.FormDataContentType()
}

func entre(s, depois, ate string) string {
	i := strings.Index(s, depois)
	if i < 0 {
		return ""
	}
	resto := s[i+len(depois):]
	j := strings.Index(resto, ate)
	if j < 0 {
		return ""
	}
	return resto[:j]
}
`
