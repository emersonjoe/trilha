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

// #145 — três coisas que o CRUD gerado mostrava e que ninguém mostraria a um
// usuário: rótulo sem acento, booleano escrito "true", e a data que sumia da
// tela sem aviso.
func TestCrudMostraOQueAPessoaLe(t *testing.T) {
	src := `package docs

import "time"

type Tipo struct {
	ID       string    ` + "`json:\"id\"`" + `
	Nome     string    ` + "`json:\"nome\" form:\"nome\" validate:\"required,max=80\"`" + `
	Retencao int       ` + "`json:\"retencao\" form:\"retencao\" validate:\"min=0,max=100\" label:\"Retenção (anos)\"`" + `
	Ativo    bool      ` + "`json:\"ativo\" form:\"ativo\" label:\"Ativo\"`" + `
	CriadoEm time.Time ` + "`json:\"criado_em\"`" + `
}
`
	raiz := projetoComTipo(t, src)
	if _, err := Crud(raiz, CrudOptions{Type: "docs.Tipo", Module: "example.com/loja", Lang: "pt"}); err != nil {
		t.Fatal(err)
	}
	lista := ler(t, raiz, "app/tipos/page.go")
	// O rótulo vem da tag: acento e unidade não são deriváveis do nome do campo.
	if !strings.Contains(lista, `Label: "Retenção (anos)"`) {
		t.Fatalf("a tag label não chegou na coluna:\n%s", lista)
	}
	// Booleano é Sim/Não, no idioma pedido.
	if !strings.Contains(lista, "sim(v.Ativo)") || !strings.Contains(lista, `h.Text("Sim")`) {
		t.Fatalf("o booleano continua saindo como true/false:\n%s", lista)
	}
	// A data que o store carimba aparece na lista, formatada pelo kit.
	if !strings.Contains(lista, "ui.Date(c, v.CriadoEm)") {
		t.Fatalf("a data sumiu da tela:\n%s", lista)
	}

	// E continua fora do formulário: data que alguém digita é bug esperando.
	form := ler(t, raiz, "app/tipos/new/page.go")
	if strings.Contains(form, "criado_em") {
		t.Fatalf("a data entrou no formulário:\n%s", form)
	}
	// O rótulo da tag vale também no formulário.
	if !strings.Contains(form, `ui.Field("retencao", "Retenção (anos)"`) {
		t.Fatalf("a tag label não chegou no formulário:\n%s", form)
	}
}

// #143 — o CRUD gerado dentro do --template app batia em 401: o teste não
// olhava o que está acima do destino. O gerador passa a olhar.
func TestCrudSobPastaFechada(t *testing.T) {
	raiz := projetoComTipo(t, tipoSrc)
	// Um middleware na raiz do app/, como o do template.
	if err := os.WriteFile(filepath.Join(raiz, "app", "middleware.go"),
		[]byte("package app\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Sem helper conhecido, o teste vem com o Skip que nomeia o arquivo.
	o := CrudOptions{Type: "docs.Tipo", Module: "example.com/loja", Lang: "pt"}
	res, err := Crud(raiz, o)
	if err != nil {
		t.Fatal(err)
	}
	if res.Guard != "app/middleware.go" {
		t.Fatalf("guard = %q", res.Guard)
	}
	teste := ler(t, raiz, "tipo_crud_test.go")
	if !strings.Contains(teste, "t.Skip(") || !strings.Contains(teste, "app/middleware.go") {
		t.Fatalf("o teste não explica a pasta fechada:\n%s", teste)
	}

	// Com o helper da receita login ao lado, ele abre a sessão e testa de
	// verdade.
	outra := projetoComTipo(t, tipoSrc)
	for _, d := range []string{"app", "internal/sessao/sessaotest"} {
		if err := os.MkdirAll(filepath.Join(outra, filepath.FromSlash(d)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(outra, "app", "middleware.go"), []byte("package app\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outra, filepath.FromSlash("internal/sessao/sessaotest/sessaotest.go")),
		[]byte("package sessaotest\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Crud(outra, o); err != nil {
		t.Fatal(err)
	}
	teste = ler(t, outra, "tipo_crud_test.go")
	for _, quero := range []string{
		`"example.com/loja/internal/sessao/sessaotest"`,
		"sessaotest.Entrar(t, c)",
	} {
		if !strings.Contains(teste, quero) {
			t.Fatalf("falta %q no teste gerado:\n%s", quero, teste)
		}
	}
	if strings.Contains(teste, "t.Skip(") {
		t.Fatalf("com helper, o teste não devia pular:\n%s", teste)
	}
}

// #115 — o contexto na interface, antes de qualquer SQL. Sem ele o store não
// honra o prazo da requisição nem cancela quando o navegador desliga, e sem
// erro no lugar do bool um banco fora do ar vira "não existe" — que é a
// mentira mais cara que uma tela pode contar.
func TestCrudInterfaceLevaContextoEErro(t *testing.T) {
	raiz := projetoComTipo(t, tipoSrc)
	if _, err := Crud(raiz, CrudOptions{Type: "docs.Tipo", At: "app/tipos",
		Module: "example.com/loja", Lang: "en"}); err != nil {
		t.Fatal(err)
	}
	store := ler(t, raiz, "internal/docs/tipo_store.go")
	for _, quero := range []string{
		"List(ctx context.Context, q TipoQuery) ([]Tipo, int, error)",
		"Get(ctx context.Context, id string) (Tipo, error)",
		"Create(ctx context.Context, v Tipo) (Tipo, error)",
		"Update(ctx context.Context, id string, v Tipo) (Tipo, error)",
		"Delete(ctx context.Context, id string) error",
	} {
		if !strings.Contains(store, quero) {
			t.Errorf("a interface não tem %q:\n%s", quero, store)
		}
	}
	// "Não achou" é o erro que o framework já sabe transformar em 404, e não
	// um bool paralelo que cada tela interpreta do seu jeito.
	if !strings.Contains(store, "trilha.ErrNotFound") {
		t.Error("o store de memória não diz não-achou com o erro do framework")
	}
}

// Toda chamada ao store, em toda tela, leva o contexto da requisição — que é
// o que faz a consulta parar quando quem pediu foi embora. Uma que não leve é
// a que sobra rodando, e ela não aparece em teste de tela nenhum.
func TestCrudTelasPassamOContextoDaRequisicao(t *testing.T) {
	raiz := projetoComTipo(t, tipoSrc)
	if _, err := Crud(raiz, CrudOptions{Type: "docs.Tipo", At: "app/tipos",
		Module: "example.com/loja", Lang: "en"}); err != nil {
		t.Fatal(err)
	}
	const chamada = "trilha.Use[docs.TipoStore](c)."
	total := 0
	for _, tela := range []string{"app/tipos/page.go", "app/tipos/new/page.go", "app/tipos/id_/page.go"} {
		src := ler(t, raiz, tela)
		for i := 0; ; {
			j := strings.Index(src[i:], chamada)
			if j < 0 {
				break
			}
			i += j + len(chamada)
			resto := src[i:]
			abre := strings.IndexByte(resto, '(')
			if abre < 0 || !strings.HasPrefix(resto[abre+1:], "c.Context()") {
				t.Errorf("%s: %s%s… não leva o contexto da requisição", tela, chamada,
					resto[:min(abre+12, len(resto))])
			}
			total++
		}
	}
	if total < 6 {
		t.Fatalf("as telas chamam o store %d vezes, esperava as seis do CRUD", total)
	}
}

// projetoComReceitaStore é o projeto do teste anterior mais o que a receita
// `trilha add store` deixa: o gerador escreve contra ele, e não contra um
// banco que ele mesmo tenha escolhido.
func projetoComReceitaStore(t *testing.T, src string) string {
	t.Helper()
	raiz := projetoComTipo(t, src)
	for _, d := range []string{"internal/store", "migrations"} {
		if err := os.MkdirAll(filepath.Join(raiz, filepath.FromSlash(d)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	escrever := func(rel, body string) {
		if err := os.WriteFile(filepath.Join(raiz, filepath.FromSlash(rel)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	escrever("internal/store/store.go", "package store\n")
	escrever("migrations/0001_init.sql", "CREATE TABLE exemplo (id TEXT PRIMARY KEY);\n")
	escrever("app/setup.go", `package app

import (
	"github.com/emersonjoe/trilha"

	"example.com/loja/internal/store"
)

func Setup(a *trilha.App) error {
	// trilha:add store
	if err := store.Setup(a); err != nil {
		return err
	}
	return nil
}
`)
	return raiz
}

// #115 — a outra metade: o store SQL sai contra o kit da receita, e a tabela
// sai como a próxima migração.
func TestCrudStoreSQL(t *testing.T) {
	raiz := projetoComReceitaStore(t, tipoSrc)
	res, err := Crud(raiz, CrudOptions{Type: "docs.Tipo", At: "app/tipos",
		Module: "example.com/loja", Lang: "en", Store: "sqlite"})
	if err != nil {
		t.Fatal(err)
	}
	for _, quero := range []string{
		"internal/docs/tipo_store.go",
		"internal/docs/tipo_store_sql.go",
		"internal/docs/tipo_store_sql_test.go",
		"migrations/0002_tipos.sql",
	} {
		if !strings.Contains(strings.Join(res.Files, " "), quero) {
			t.Errorf("%s não foi escrito: %v", quero, res.Files)
		}
	}
	sql := ler(t, raiz, "internal/docs/tipo_store_sql.go")
	// A implementação e a interface não podem andar separadas, e quem diz
	// isso é o compilador e não uma revisão.
	if !strings.Contains(sql, "var _ TipoStore = (*TipoSQL)(nil)") {
		t.Error("nada amarra o store SQL à interface")
	}
	// Tudo o que a 126 escreveu para ser usado é usado: ordenação por tabela
	// declarada, teto de página, LIKE escapado, placeholder por dialeto, e o
	// sql.ErrNoRows virando o 404 do framework.
	for _, quero := range []string{
		"store.Sortable{", "store.OrderBy(", "store.Paginate(", "store.Like(",
		"s.d.Arg(", "store.NotFound(", "context.Context",
	} {
		if !strings.Contains(sql, quero) {
			t.Errorf("o store SQL não usa %s:\n%s", quero, sql)
		}
	}
	// E nada do que vem de fora entra no texto de um comando.
	for _, proibido := range []string{`+ q.Sort`, `+q.Sort`, `+ q.Q`, `+q.Q`} {
		if strings.Contains(sql, proibido) {
			t.Errorf("o store SQL concatena %s na consulta", proibido)
		}
	}
	mig := ler(t, raiz, "migrations/0002_tipos.sql")
	for _, quero := range []string{"CREATE TABLE IF NOT EXISTS tipos", "TEXT PRIMARY KEY",
		"nome      TEXT NOT NULL", "sigla", "retencao  INTEGER", "criado_em TIMESTAMP"} {
		if !strings.Contains(mig, quero) {
			t.Errorf("a migração não tem %q:\n%s", quero, mig)
		}
	}
	// O Provide do store SQL tem de vir depois do store.Setup: store.DB só
	// existe depois dele, e um pool nulo guardado aqui só aparece na primeira
	// requisição.
	setup := ler(t, raiz, "app/setup.go")
	if !strings.Contains(setup, "docs.NewTipoSQL(store.DB, store.D)") {
		t.Fatalf("o setup não liga o store SQL:\n%s", setup)
	}
	if strings.Index(setup, "store.Setup(a)") > strings.Index(setup, "NewTipoSQL") {
		t.Fatalf("o Provide veio antes do store.Setup:\n%s", setup)
	}
}

// O dialeto muda o placeholder e o tipo das colunas, e é só isso que ele
// muda — o resto do store é o mesmo.
func TestCrudStorePostgres(t *testing.T) {
	raiz := projetoComReceitaStore(t, tipoSrc)
	if _, err := Crud(raiz, CrudOptions{Type: "docs.Tipo", At: "app/tipos",
		Module: "example.com/loja", Lang: "en", Store: "postgres"}); err != nil {
		t.Fatal(err)
	}
	mig := ler(t, raiz, "migrations/0002_tipos.sql")
	if !strings.Contains(mig, "retencao  BIGINT") {
		t.Errorf("a migração não está no dialeto pedido:\n%s", mig)
	}
}

// Sem a receita no projeto, o --store é recusa antes da primeira escrita: um
// gerador que escreve contra um pacote que não existe entrega um projeto que
// não compila.
func TestCrudStoreSemAReceitaRecusa(t *testing.T) {
	raiz := projetoComTipo(t, tipoSrc)
	_, err := Crud(raiz, CrudOptions{Type: "docs.Tipo", At: "app/tipos",
		Module: "example.com/loja", Lang: "en", Store: "sqlite"})
	if !errors.Is(err, ErrCrudNoStore) {
		t.Fatalf("erro = %v", err)
	}
	if _, err := os.Stat(filepath.Join(raiz, filepath.FromSlash("app/tipos/page.go"))); err == nil {
		t.Fatal("a recusa deixou tela escrita")
	}
	// E um dialeto que não existe é recusa também, e não um store em branco.
	if _, err := Crud(raiz, CrudOptions{Type: "docs.Tipo", At: "app/tipos",
		Module: "example.com/loja", Lang: "en", Store: "mysql"}); err == nil {
		t.Fatal("--store mysql passou")
	}
}
