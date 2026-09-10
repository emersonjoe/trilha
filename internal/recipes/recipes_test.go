package recipes

import (
	"errors"
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

// #117 — a receita cai onde o template pediu, e os endereços dela vão junto.
// Escrever "/chaves" numa receita que um template põe sob /admin/ é um
// formulário que posta para um 404 — e é o tipo de erro que ninguém vê até
// apertar o botão.
func TestAddSobPastaLevaOsEnderecosJunto(t *testing.T) {
	raiz := projeto(t)
	r, _ := Get("api-keys")
	res, err := Add(raiz, r, Options{Module: "example.com/x", Lang: "en", At: "app/admin/"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(res.Written, " ") != "app/admin/chaves/page.go" {
		t.Fatalf("escreveu %v", res.Written)
	}
	pagina := ler(t, raiz, "app/admin/chaves/page.go")
	for _, quero := range []string{`c.Redirect("/admin/chaves")`, `h.Action("/admin/chaves")`,
		`Revoke: "/admin/chaves"`} {
		if !strings.Contains(pagina, quero) {
			t.Fatalf("faltou %q:\n%s", quero, pagina)
		}
	}
	if strings.Contains(pagina, `"/chaves"`) {
		t.Fatalf("sobrou um endereço da raiz:\n%s", pagina)
	}
	// O import do setup segue a tela, senão o pacote não é o que está lá.
	if setup := ler(t, raiz, "app/setup.go"); !strings.Contains(setup, `"example.com/x/app/admin/chaves"`) {
		t.Fatalf("o import não seguiu a tela:\n%s", setup)
	}
	// E o próximo passo diz o endereço de verdade.
	if !strings.Contains(res.Next, "/admin/chaves") {
		t.Fatalf("o próximo passo aponta para o lugar errado: %q", res.Next)
	}
}

// O que a receita escreve em internal/ não é tela e não se move com uma.
func TestAddNaoMoveOQueNaoEhTela(t *testing.T) {
	raiz := projeto(t)
	r, _ := Get("audit")
	res, err := Add(raiz, r, Options{Module: "example.com/x", Lang: "en", At: "app/admin/"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(res.Written, " ") != "internal/auditoria/store.go app/admin/auditoria/page.go" {
		t.Fatalf("escreveu %v", res.Written)
	}
}

// #116 — a receita login: a tabela de gente, a sessão e as duas telas. É a que
// as outras esperam, e a que não pode escrever uma senha padrão no projeto de
// ninguém.
func TestReceitaLogin(t *testing.T) {
	raiz := projeto(t)
	r, err := Get("login")
	if err != nil {
		t.Fatal(err)
	}
	res, err := Add(raiz, r, Options{Module: "example.com/x", Lang: "en"})
	if err != nil {
		t.Fatal(err)
	}
	quero := "internal/usuarios/usuarios.go internal/usuarios/usuarios_test.go " +
		"internal/sessao/sessao.go app/entrar/page.go app/sair/route.go " +
		"internal/sessao/sessaotest/sessaotest.go login_test.go"
	if got := strings.Join(res.Written, " "); got != quero {
		t.Fatalf("escreveu %q", got)
	}
	setup := ler(t, raiz, "app/setup.go")
	for _, q := range []string{"// trilha:add login", "trilha.Provide(a, usuarios.New(a.Logger()))",
		`"example.com/x/internal/usuarios"`} {
		if !strings.Contains(setup, q) {
			t.Fatalf("faltou %q no setup:\n%s", q, setup)
		}
	}
	// Nenhuma senha vem escrita: o primeiro administrador sai do ambiente, e um
	// projeto que nasce com admin/admin nasce com uma porta que alguém esquece.
	store := ler(t, raiz, "internal/usuarios/usuarios.go")
	for _, q := range []string{"ADMIN_EMAIL", "ADMIN_PASSWORD", "auth.HashPBKDF2", "ErrCredencial"} {
		if !strings.Contains(store, q) {
			t.Fatalf("faltou %q na tabela de usuários", q)
		}
	}
	// A tela e a sessão apontam para onde a receita caiu.
	if pag := ler(t, raiz, "app/entrar/page.go"); !strings.Contains(pag, `h.Action("/entrar")`) {
		t.Fatalf("o formulário não posta para a própria rota:\n%s", pag)
	}

	// E sob outra pasta, tudo acompanha: o endereço da tela e o LoginPath da
	// sessão. Uma receita que escreve /entrar debaixo de /admin manda o
	// visitante para um 404.
	outra := projeto(t)
	if _, err := Add(outra, r, Options{Module: "example.com/x", Lang: "pt", At: "app/admin/"}); err != nil {
		t.Fatal(err)
	}
	if pag := ler(t, outra, "app/admin/entrar/page.go"); !strings.Contains(pag, `h.Action("/admin/entrar")`) {
		t.Fatalf("o formulário aponta para fora da pasta:\n%s", pag)
	}
	if ses := ler(t, outra, "internal/sessao/sessao.go"); !strings.Contains(ses, `LoginPath:  "/admin/entrar"`) {
		t.Fatalf("o LoginPath não acompanhou a pasta:\n%s", ses)
	}
}

// #116 — a receita users depende da login, e é a primeira que depende de
// alguma. Recusar é mais barato do que escrever cinco arquivos que não
// compilam num projeto que a pessoa vai ter de limpar à mão.
func TestUsersPrecisaDoLogin(t *testing.T) {
	raiz := projeto(t)
	r, err := Get("users")
	if err != nil {
		t.Fatal(err)
	}
	res, err := Add(raiz, r, Options{Module: "example.com/x", Lang: "en"})
	if err == nil {
		t.Fatalf("escreveu sem o login: %v", res.Written)
	}
	if !errors.Is(err, ErrMissing) {
		t.Fatalf("err = %v", err)
	}
	for _, quero := range []string{"login", "internal/usuarios/usuarios.go"} {
		if !strings.Contains(err.Error(), quero) {
			t.Fatalf("a recusa não diz %q: %v", quero, err)
		}
	}
	if _, err := os.Stat(filepath.Join(raiz, "app", "usuarios")); err == nil {
		t.Fatal("recusou e escreveu assim mesmo")
	}

	// Com a receita de que ela depende, passa — e escreve no mesmo pacote que a
	// outra abriu, que é o motivo de a dependência existir.
	if _, err := Add(raiz, mustGet(t, "login"), Options{Module: "example.com/x", Lang: "en"}); err != nil {
		t.Fatal(err)
	}
	res, err = Add(raiz, r, Options{Module: "example.com/x", Lang: "en"})
	if err != nil {
		t.Fatal(err)
	}
	quero := "internal/usuarios/convites.go internal/usuarios/convites_test.go " +
		"app/usuarios/page.go app/usuarios/middleware.go app/convite/token_/page.go usuarios_test.go"
	if got := strings.Join(res.Written, " "); got != quero {
		t.Fatalf("escreveu %q", got)
	}
	conv := ler(t, raiz, "internal/usuarios/convites.go")
	for _, q := range []string{"package usuarios", "sha256", "48 * time.Hour"} {
		if !strings.Contains(conv, q) {
			t.Fatalf("faltou %q em convites.go", q)
		}
	}
	if mw := ler(t, raiz, "app/usuarios/middleware.go"); !strings.Contains(mw, `RequireRole("admin")`) {
		t.Fatalf("a pasta não exige papel:\n%s", mw)
	}
}

func mustGet(t *testing.T, nome string) Recipe {
	t.Helper()
	r, err := Get(nome)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
