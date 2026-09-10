package scaffold

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha/internal/scan"
)

// projetoComTipo é um projeto com um struct para o gerador ler.
func projetoComTipo(t *testing.T, src string) string {
	t.Helper()
	raiz := t.TempDir()
	for _, d := range []string{"app", "internal/docs"} {
		if err := os.MkdirAll(filepath.Join(raiz, filepath.FromSlash(d)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(raiz, "go.mod"),
		[]byte("module example.com/loja\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(raiz, filepath.FromSlash("internal/docs/tipo.go")),
		[]byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return raiz
}

const tipoSrc = `package docs

import "time"

type Tipo struct {
	ID       string    ` + "`json:\"id\"`" + `
	Nome     string    ` + "`json:\"nome\" form:\"nome\" validate:\"required,max=80\"`" + `
	Sigla    string    ` + "`json:\"sigla\" form:\"sigla\" validate:\"required,max=8\"`" + `
	Retencao int       ` + "`json:\"retencao\" form:\"retencao\" validate:\"min=0,max=100\"`" + `
	Segredo  string    ` + "`json:\"-\"`" + `
	CriadoEm time.Time ` + "`json:\"criado_em\"`" + `
}
`

// #115 — do struct à tela: cinco arquivos, três rotas, e o scanner enxergando
// exatamente as rotas que o comando prometeu.
func TestCrud(t *testing.T) {
	raiz := projetoComTipo(t, tipoSrc)
	res, err := Crud(raiz, CrudOptions{Type: "docs.Tipo", At: "app/admin/tipos",
		Module: "example.com/loja", Lang: "en"})
	if err != nil {
		t.Fatal(err)
	}
	querido := []string{
		"internal/docs/tipo_store.go",
		"app/admin/tipos/page.go",
		"app/admin/tipos/new/page.go",
		"app/admin/tipos/id_/page.go",
		"tipo_crud_test.go",
		"app/setup.go",
	}
	if strings.Join(res.Files, " ") != strings.Join(querido, " ") {
		t.Fatalf("arquivos = %v", res.Files)
	}
	// O scanner tem de ver as rotas que o comando anunciou: uma promessa que
	// o gerador faz e o roteador não cumpre é um 404 sem explicação.
	r, err := scan.Scan(raiz, "example.com/loja")
	if err != nil {
		t.Fatal(err)
	}
	var rotas []string
	for _, rt := range r.Routes {
		rotas = append(rotas, rt.Pattern)
	}
	sort := func(v []string) string {
		out := append([]string{}, v...)
		for i := 1; i < len(out); i++ {
			for j := i; j > 0 && out[j] < out[j-1]; j-- {
				out[j], out[j-1] = out[j-1], out[j]
			}
		}
		return strings.Join(out, " ")
	}
	if sort(rotas) != sort(res.Patterns) {
		t.Fatalf("o scanner viu %v, o gerador prometeu %v", rotas, res.Patterns)
	}
}

// O que é do sistema não entra no formulário: uma data preenchida à mão é um
// bug esperando, e um campo fora do wire está fora por escolha de alguém.
func TestCrudDeixaDeForaOQueNaoEhDoFormulario(t *testing.T) {
	raiz := projetoComTipo(t, tipoSrc)
	if _, err := Crud(raiz, CrudOptions{Type: "docs.Tipo", At: "app/tipos",
		Module: "example.com/loja", Lang: "en"}); err != nil {
		t.Fatal(err)
	}
	form := ler(t, raiz, "app/tipos/new/page.go")
	for _, quero := range []string{`ui.Field("nome"`, `ui.Field("sigla"`, `ui.Field("retencao"`} {
		if !strings.Contains(form, quero) {
			t.Fatalf("faltou %s no formulário:\n%s", quero, form)
		}
	}
	for _, nao := range []string{"criado_em", "CriadoEm", "Segredo", `ui.Field("id"`} {
		if strings.Contains(form, nao) {
			t.Fatalf("%s não devia estar no formulário", nao)
		}
	}
	// E o store carimba a data que o formulário não pede.
	if store := ler(t, raiz, "internal/docs/tipo_store.go"); !strings.Contains(store, "v.CriadoEm = time.Now()") {
		t.Fatalf("o store não carimba a data:\n%s", store)
	}
}

// Struct sem chave é recusa com a frase, e não escreve arquivo nenhum: uma
// chave escolhida no chute só aparece no primeiro Update, com cinco telas
// escritas em cima.
func TestCrudRecusaStructSemID(t *testing.T) {
	raiz := projetoComTipo(t, `package docs

type Tipo struct {
	Nome string `+"`form:\"nome\"`"+`
}
`)
	_, err := Crud(raiz, CrudOptions{Type: "docs.Tipo", Module: "example.com/loja", Lang: "en"})
	if !errors.Is(err, ErrCrudNoKey) {
		t.Fatalf("erro = %v", err)
	}
	if _, err := os.Stat(filepath.Join(raiz, "app", "tipos")); err == nil {
		t.Fatal("uma recusa não pode deixar pasta para trás")
	}
}

// Arquivo que já existe é recusa nomeando-o, e o que estava lá continua igual:
// um gerador que sobrescreve é um gerador que ninguém roda duas vezes.
func TestCrudNaoSobrescreve(t *testing.T) {
	raiz := projetoComTipo(t, tipoSrc)
	if err := os.MkdirAll(filepath.Join(raiz, "app", "tipos"), 0o755); err != nil {
		t.Fatal(err)
	}
	meu := filepath.Join(raiz, "app", "tipos", "page.go")
	if err := os.WriteFile(meu, []byte("package tipos\n\n// o meu\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Crud(raiz, CrudOptions{Type: "docs.Tipo", Module: "example.com/loja", Lang: "en"})
	if !errors.Is(err, ErrGenExists) || !strings.Contains(err.Error(), "app/tipos/page.go") {
		t.Fatalf("erro = %v", err)
	}
	if b, _ := os.ReadFile(meu); string(b) != "package tipos\n\n// o meu\n" {
		t.Fatalf("mexeu no arquivo: %s", b)
	}
	// E nada foi escrito ao lado: a recusa é antes da primeira escrita.
	if _, err := os.Stat(filepath.Join(raiz, "internal", "docs", "tipo_store.go")); err == nil {
		t.Fatal("escreveu metade do CRUD antes de recusar")
	}
}

// O Setup é o único arquivo que o gerador edita em vez de recusar, porque sem
// ele o CRUD compila e responde 500 — que é o pior resultado possível, porque
// parece que funcionou.
func TestCrudLigaOStoreNoSetup(t *testing.T) {
	raiz := projetoComTipo(t, tipoSrc)
	setup := "package app\n\nimport \"github.com/emersonjoe/trilha\"\n\nfunc Setup(a *trilha.App) error {\n\treturn nil\n}\n"
	if err := os.WriteFile(filepath.Join(raiz, "app", "setup.go"), []byte(setup), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := Crud(raiz, CrudOptions{Type: "docs.Tipo", Module: "example.com/loja", Lang: "en"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Wired != "app/setup.go" {
		t.Fatalf("não disse que mexeu no setup: %+v", res)
	}
	got := ler(t, raiz, "app/setup.go")
	if !strings.Contains(got, "trilha.Provide[docs.TipoStore](a, docs.NewTipoMemory())") {
		t.Fatalf("não ligou o store:\n%s", got)
	}
	if !strings.Contains(got, `"example.com/loja/internal/docs"`) {
		t.Fatalf("não importou o pacote:\n%s", got)
	}
}

func ler(t *testing.T, raiz, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(raiz, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// #115 — "gerador que sobrescreve é gerador que ninguém roda duas vezes". A
// recusa já existia; o que faltava era o que ela diz. Quem roda de novo é
// porque o struct mudou, e o que essa pessoa quer saber é qual campo entrou e
// onde ele falta.
func TestCrudRodadoDeNovoDizOQueFaltou(t *testing.T) {
	raiz := projetoComTipo(t, tipoSrc)
	o := CrudOptions{Type: "docs.Tipo", Module: "example.com/loja", Lang: "en"}
	if _, err := Crud(raiz, o); err != nil {
		t.Fatal(err)
	}

	// Com o struct igual, as telas têm tudo.
	faltando, err := CrudMissing(raiz, o)
	if err != nil {
		t.Fatal(err)
	}
	if len(faltando) != 0 {
		t.Fatalf("achou o que não falta: %+v", faltando)
	}

	// O struct ganha um campo, que é a razão de alguém rodar o comando de novo.
	novo := strings.Replace(tipoSrc, "\tSegredo ",
		"\tDescricao string    `json:\"descricao\" form:\"descricao\" validate:\"max=200\"`\n\tSegredo ", 1)
	if err := os.WriteFile(filepath.Join(raiz, filepath.FromSlash("internal/docs/tipo.go")),
		[]byte(novo), 0o644); err != nil {
		t.Fatal(err)
	}

	faltando, err = CrudMissing(raiz, o)
	if err != nil {
		t.Fatal(err)
	}
	if len(faltando) != 3 {
		t.Fatalf("faltando = %+v", faltando)
	}
	quero := []string{"app/tipos/page.go", "app/tipos/new/page.go", "app/tipos/id_/page.go"}
	for i, m := range faltando {
		if m.Field != "Descricao" || m.Form != "descricao" {
			t.Errorf("campo = %+v", m)
		}
		if m.File != quero[i] {
			t.Errorf("arquivo = %s, quero %s", m.File, quero[i])
		}
		if m.Line <= 0 {
			t.Errorf("sem linha: %+v", m)
		}
	}

	// E nada disso escreve: a segunda execução continua não tocando em nada.
	if _, err := Crud(raiz, o); !errors.Is(err, ErrGenExists) {
		t.Fatalf("a segunda execução escreveu: %v", err)
	}
}
