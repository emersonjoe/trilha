package scaffold

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// projetoCom grava os arquivos dados (caminho → conteúdo) numa raiz nova.
func projetoCom(t *testing.T, arquivos map[string]string) string {
	t.Helper()
	raiz := raizComApp(t)
	for rel, corpo := range arquivos {
		dst := filepath.Join(raiz, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dst, []byte(corpo), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return raiz
}

const fonteComment = `package posts

type Comment struct {
	Author  string ` + "`json:\"author\" form:\"autor\" validate:\"required,min=3\"`" + `
	Body    string ` + "`json:\"body\" validate:\"required\"`" + `
	Stars   int    ` + "`json:\"stars\" validate:\"min=1,max=5\"`" + `
	Hidden  string ` + "`json:\"-\"`" + `
	oculto  string
}
`

func TestFindType(t *testing.T) {
	raiz := projetoCom(t, map[string]string{"internal/posts/posts.go": fonteComment})
	info, err := findType(raiz, "example.com/loja", "Comment")
	if err != nil {
		t.Fatal(err)
	}
	if info.Pkg != "posts" || info.Dir != "internal/posts" {
		t.Errorf("pacote/pasta = %q/%q", info.Pkg, info.Dir)
	}
	if info.Import != "example.com/loja/internal/posts" {
		t.Errorf("import = %q", info.Import)
	}
	if len(info.Fields) != 3 {
		t.Fatalf("campos = %d, queria 3 (json:\"-\" e não exportado ficam de fora): %+v", len(info.Fields), info.Fields)
	}
	autor := info.Fields[0]
	if autor.Name != "Author" || autor.JSON != "author" || autor.Form != "autor" || autor.Type != "string" {
		t.Errorf("primeiro campo = %+v", autor)
	}
	if !autor.Required() || info.Fields[2].Required() {
		t.Errorf("obrigatoriedade errada: %+v", info.Fields)
	}
}

func TestFindTypeAusente(t *testing.T) {
	raiz := raizComApp(t)
	if _, err := findType(raiz, "example.com/loja", "Comment"); !errors.Is(err, errTypeNotFound) {
		t.Fatalf("erro = %v, queria errTypeNotFound", err)
	}
}

func TestFindTypeAmbiguo(t *testing.T) {
	raiz := projetoCom(t, map[string]string{
		"internal/posts/posts.go": fonteComment,
		"internal/loja/loja.go":   "package loja\n\ntype Comment struct{}\n",
	})
	_, err := findType(raiz, "example.com/loja", "Comment")
	if err == nil {
		t.Fatal("dois pacotes com o mesmo nome tinham que ser recusa")
	}
	for _, s := range []string{"internal/posts", "internal/loja", "posts.Comment"} {
		if !strings.Contains(err.Error(), s) {
			t.Errorf("a mensagem não diz %q: %v", s, err)
		}
	}
	// O nome qualificado desempata.
	info, err := findType(raiz, "example.com/loja", "posts.Comment")
	if err != nil {
		t.Fatal(err)
	}
	if info.Dir != "internal/posts" {
		t.Errorf("pasta = %q", info.Dir)
	}
}

func TestFindTypeIgnoraPastasPuladas(t *testing.T) {
	raiz := projetoCom(t, map[string]string{
		"_rascunho/x.go":       "package rascunho\n\ntype Comment struct{}\n",
		"testdata/apps/y.go":   "package apps\n\ntype Comment struct{}\n",
		".idea/z.go":           "package idea\n\ntype Comment struct{}\n",
		"app/.well-known/w.go": "package wellknown\n\ntype Comment struct{}\n",
	})
	info, err := findType(raiz, "example.com/loja", "Comment")
	if err != nil {
		t.Fatalf("só o de .well-known conta, e ele existe: %v", err)
	}
	if info.Dir != "app/.well-known" {
		t.Errorf("pasta = %q", info.Dir)
	}
}

func TestExampleValue(t *testing.T) {
	casos := []struct {
		campo typeField
		quer  any
	}{
		{typeField{Type: "string", Validate: "required,min=10"}, "examplexxx"},
		{typeField{Type: "string", Validate: "required,max=4"}, "exam"},
		{typeField{Type: "string", Validate: "required,email"}, "someone@example.com"},
		{typeField{Type: "string", Validate: "oneof=novo pago"}, "novo"},
		{typeField{Type: "int", Validate: "min=3"}, 3},
		{typeField{Type: "bool"}, true},
	}
	for _, c := range casos {
		got, ok := exampleValue(c.campo)
		if !ok || got != c.quer {
			t.Errorf("%+v deu (%v, %v), queria %v", c.campo, got, ok, c.quer)
		}
	}
	if _, ok := exampleValue(typeField{Type: "time.Time"}); ok {
		t.Error("um tipo que o gerador não sabe montar fica de fora")
	}
}
