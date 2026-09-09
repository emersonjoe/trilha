package recipes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func projeto(t *testing.T) string {
	t.Helper()
	raiz := t.TempDir()
	if err := os.MkdirAll(filepath.Join(raiz, "app"), 0o755); err != nil {
		t.Fatal(err)
	}
	return raiz
}

func ler(t *testing.T, raiz, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(raiz, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// #116 — a receita escreve no projeto e liga o que precisa ser ligado.
func TestAddEscreveELiga(t *testing.T) {
	raiz := projeto(t)
	r, err := Get("audit")
	if err != nil {
		t.Fatal(err)
	}
	res, err := Add(raiz, r, Options{Module: "example.com/x", Lang: "en"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(res.Written, " ") != "internal/auditoria/store.go app/auditoria/page.go" {
		t.Fatalf("escreveu %v", res.Written)
	}
	if len(res.Setup) != 1 {
		t.Fatalf("setup = %v", res.Setup)
	}
	setup := ler(t, raiz, "app/setup.go")
	for _, quero := range []string{"// trilha:add audit", "a.Config().Audit = auditoria.Store",
		`"example.com/x/internal/auditoria"`} {
		if !strings.Contains(setup, quero) {
			t.Fatalf("faltou %q no setup:\n%s", quero, setup)
		}
	}
	// O módulo do projeto entra nos arquivos: uma receita que deixasse o
	// import do template seria uma receita que não compila em lugar nenhum.
	if pagina := ler(t, raiz, "app/auditoria/page.go"); !strings.Contains(pagina, `"example.com/x/internal/auditoria"`) {
		t.Fatalf("o módulo não chegou na página:\n%s", pagina)
	}
}

// Rodar de novo acrescenta, e não recomeça: os arquivos são pulados e a linha
// marcada não entra duas vezes. É a marca que existe para isso.
func TestAddDeNovoNaoDuplica(t *testing.T) {
	raiz := projeto(t)
	r, _ := Get("audit")
	if _, err := Add(raiz, r, Options{Module: "example.com/x", Lang: "en"}); err != nil {
		t.Fatal(err)
	}
	// Uma edição de quem recebeu a receita: ela é dona do arquivo agora.
	meu := filepath.Join(raiz, filepath.FromSlash("app/auditoria/page.go"))
	if err := os.WriteFile(meu, []byte("package auditoria\n\n// o meu\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := Add(raiz, r, Options{Module: "example.com/x", Lang: "en"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Written) != 0 || len(res.Skipped) != 2 {
		t.Fatalf("escreveu %v, pulou %v", res.Written, res.Skipped)
	}
	if len(res.Setup) != 0 {
		t.Fatalf("mexeu no setup de novo: %v", res.Setup)
	}
	if b := ler(t, raiz, "app/auditoria/page.go"); b != "package auditoria\n\n// o meu\n" {
		t.Fatalf("sobrescreveu o que a pessoa editou:\n%s", b)
	}
	if n := strings.Count(ler(t, raiz, "app/setup.go"), "trilha:add audit"); n != 1 {
		t.Fatalf("a marca apareceu %d vezes", n)
	}
}

// --dry-run mostra e não escreve. É o que se roda antes de deixar um comando
// mexer num projeto que já tem código.
func TestAddDryRunNaoEscreve(t *testing.T) {
	raiz := projeto(t)
	r, _ := Get("settings")
	res, err := Add(raiz, r, Options{Module: "example.com/x", Lang: "en", DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Written) == 0 {
		t.Fatal("não disse o que faria")
	}
	for _, f := range res.Written {
		if _, err := os.Stat(filepath.Join(raiz, filepath.FromSlash(f))); err == nil {
			t.Fatalf("escreveu %s mesmo assim", f)
		}
	}
	if _, err := os.Stat(filepath.Join(raiz, filepath.FromSlash("app/setup.go"))); err == nil {
		t.Fatal("escreveu o setup mesmo assim")
	}
}

// Três receitas no mesmo projeto deixam um bloco de imports, e não três de uma
// linha: o arquivo é de quem recebeu, e ele vai ler isso.
func TestAddTresReceitasUmBlocoDeImports(t *testing.T) {
	raiz := projeto(t)
	for _, nome := range []string{"audit", "settings", "api-keys"} {
		r, err := Get(nome)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Add(raiz, r, Options{Module: "example.com/x", Lang: "en"}); err != nil {
			t.Fatal(nome, err)
		}
	}
	setup := ler(t, raiz, "app/setup.go")
	if n := strings.Count(setup, "import ("); n != 1 {
		t.Fatalf("blocos de import = %d:\n%s", n, setup)
	}
	// Uma linha em branco dentro do bloco seria um grupo a mais.
	bloco := setup[strings.Index(setup, "import ("):strings.Index(setup, "\n)")]
	if strings.Contains(bloco, "\n\n") {
		t.Fatalf("os imports ficaram em grupos separados:\n%s", bloco)
	}
	for _, quero := range []string{"trilha:add audit", "trilha:add settings", "trilha:add api-keys"} {
		if !strings.Contains(setup, quero) {
			t.Fatalf("faltou %q", quero)
		}
	}
}

// O nome que não existe é um erro que diz onde procurar.
func TestGetDesconhecida(t *testing.T) {
	if _, err := Get("nao-existe"); err == nil {
		t.Fatal("achou uma receita que não existe")
	}
}

// Toda receita tem resumo nas duas línguas e uma doc para onde apontar: a
// listagem é o único lugar onde alguém descobre que ela existe.
func TestTodaReceitaSeApresenta(t *testing.T) {
	for _, r := range All() {
		if r.Name == "" || r.Doc == "" {
			t.Fatalf("%+v", r)
		}
		for _, l := range []string{"en", "pt"} {
			if strings.TrimSpace(r.Summary[l]) == "" {
				t.Errorf("%s: sem resumo em %s", r.Name, l)
			}
			if strings.TrimSpace(r.Next[l]) == "" {
				t.Errorf("%s: sem próximo passo em %s", r.Name, l)
			}
		}
		if len(r.Files) == 0 {
			t.Errorf("%s: não escreve nada", r.Name)
		}
	}
}
