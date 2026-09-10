package recipes

// loginRecipe is a session of the application's own: a table of people, the
// screen that checks a password, and the end of it.
//
// It is the recipe the others wait for. A screen that invites somebody, gives
// them a role or resets their password is a screen about a table of people,
// and until this exists there is no table to be about.
//
// What it deliberately does not write is a password. `usuarios.New()` reads
// ADMIN_EMAIL and ADMIN_PASSWORD from the environment; with neither, the table
// comes up empty and says so. A recipe that seeded admin/admin into somebody's
// project would be a door left open by a tool they trusted.
func loginRecipe() Recipe {
	return Recipe{
		Name: "login",
		Summary: map[string]string{
			"en": "a session of your own: a users table, the login screen and the way out",
			"pt": "uma sessão própria: a tabela de gente, a tela de entrar e a saída",
		},
		Doc: "/reference/auth",
		Files: []File{
			{Rel: "internal/usuarios/usuarios.go", Go: true, Body: loginStore},
			{Rel: "internal/usuarios/usuarios_test.go", Go: true, Body: loginStoreTest},
			{Rel: "internal/sessao/sessao.go", Go: true, Body: loginSession},
			{Rel: "{{.At}}entrar/page.go", Go: true, Body: loginPage},
			{Rel: "{{.At}}sair/route.go", Go: true, Body: loginLogout},
			{Rel: "internal/sessao/sessaotest/sessaotest.go", Go: true, Body: loginTestHelper},
			{Rel: "login_test.go", Go: true, Body: loginTest},
		},
		Setup: []Insert{{
			Marker: "// trilha:add login",
			Line:   "\ttrilha.Provide(a, usuarios.New(a.Logger()))\n",
		}},
		Imports: []string{"{{.Module}}/internal/usuarios"},
		Next: map[string]string{
			"en": "Set ADMIN_EMAIL and ADMIN_PASSWORD, run `trilha dev` and open {{.URL}}entrar. " +
				"To close a folder, put `func Middleware(c *trilha.Ctx, next trilha.Next) error " +
				"{ return sessao.Flow.Require()(c, next) }` in its middleware.go — the recipe does " +
				"not guess which folder is yours to guard.",
			"pt": "Defina ADMIN_EMAIL e ADMIN_PASSWORD, rode `trilha dev` e abra {{.URL}}entrar. " +
				"Para fechar uma pasta, ponha `func Middleware(c *trilha.Ctx, next trilha.Next) error " +
				"{ return sessao.Flow.Require()(c, next) }` no middleware.go dela — a receita não " +
				"adivinha qual pasta é sua para guardar.",
		},
	}
}

const loginStore = `// Package usuarios is the table of people this application owns: e-mail,
// name, role and the hash of a password.
//
// It is memory here, which is the honest starting point: the rows last as long
// as the process. A real one is a table behind the same methods, and no screen
// changes when it arrives.
package usuarios

import (
	"errors"
	"log/slog"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/emersonjoe/trilha/auth"
)

// Usuario is one row. The password is never here: what is stored is
// pbkdf2_sha256$iterations$salt$hash, and there is no way back from it.
type Usuario struct {
	ID      string
	Email   string
	Nome    string
	Papel   string
	Ativo   bool
	Criado  time.Time
	Hash    string
}

// ErrCredencial is the single answer to a wrong e-mail and to a wrong
// password. Saying which of the two was wrong tells whoever is guessing that
// the account exists, which is half of what they came for.
var ErrCredencial = errors.New("wrong e-mail or password")

// Store is the table.
type Store struct {
	mu   sync.RWMutex
	rows map[string]Usuario
}

// New builds the store and seeds the first administrator from the environment:
// ADMIN_EMAIL and ADMIN_PASSWORD. With neither, it comes up empty and says so
// once — an application whose first user is a password in a source file is an
// application with a door somebody forgets.
func New(log *slog.Logger) *Store {
	s := &Store{rows: map[string]Usuario{}}
	email, senha := os.Getenv("ADMIN_EMAIL"), os.Getenv("ADMIN_PASSWORD")
	if email == "" || senha == "" {
		if log != nil {
			log.Warn("usuarios: nobody can sign in yet", "fix", "set ADMIN_EMAIL and ADMIN_PASSWORD and restart")
		}
		return s
	}
	if err := s.Add("u-1", email, email, "admin", senha); err != nil && log != nil {
		log.Error("usuarios: the first administrator was not created", "err", err)
	}
	return s
}

// Add hashes the password and writes the row. An e-mail already in the table
// is an error and not an overwrite: creating a user twice by accident should
// not silently replace the first one.
func (s *Store) Add(id, email, nome, papel, senha string) error {
	chave := normaliza(email)
	if chave == "" || senha == "" {
		return errors.New("usuarios: e-mail and password are required")
	}
	hash, err := auth.HashPBKDF2(senha)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.rows[chave]; ok {
		return errors.New("usuarios: that e-mail is already in the table")
	}
	s.rows[chave] = Usuario{ID: id, Email: email, Nome: nome, Papel: papel, Ativo: true, Criado: time.Now(), Hash: hash}
	return nil
}

// Verify answers the row when the password checks out.
//
// The comparison runs even for an e-mail nobody has, against a hash of
// something nobody knows: without it, a wrong e-mail answers faster than a
// wrong password, and the difference is a list of who has an account.
func (s *Store) Verify(email, senha string) (Usuario, error) {
	s.mu.RLock()
	u, ok := s.rows[normaliza(email)]
	s.mu.RUnlock()
	if !ok {
		auth.CheckPBKDF2(semUsuario, senha)
		return Usuario{}, ErrCredencial
	}
	if !auth.CheckPBKDF2(u.Hash, senha) || !u.Ativo {
		return Usuario{}, ErrCredencial
	}
	return u, nil
}

// All is every row, by e-mail, for a screen that lists them.
func (s *Store) All() []Usuario {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Usuario, 0, len(s.rows))
	for _, u := range s.rows {
		out = append(out, u)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Email < out[j].Email })
	return out
}

// Vazio says whether nobody can sign in yet. The login screen asks, so that
// the first run explains itself instead of refusing every password.
func (s *Store) Vazio() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.rows) == 0
}

func normaliza(email string) string { return strings.ToLower(strings.TrimSpace(email)) }

// semUsuario is a valid hash of something nobody knows, kept only for the time
// a failed lookup has to take.
var semUsuario = func() string {
	h, err := auth.HashPBKDF2("no password matches this hash")
	if err != nil {
		panic(err)
	}
	return h
}()
`

const loginStoreTest = `package usuarios

import "testing"

func TestSenhaCertaESenhaErrada(t *testing.T) {
	s := &Store{rows: map[string]Usuario{}}
	if err := s.Add("u-1", "Ana@Exemplo.com", "Ana", "admin", "uma-senha-boa"); err != nil {
		t.Fatal(err)
	}
	// The e-mail is a name, not a string: the case somebody typed is not part
	// of who they are.
	if _, err := s.Verify("ana@exemplo.com", "uma-senha-boa"); err != nil {
		t.Fatalf("a senha certa foi recusada: %v", err)
	}
	// A wrong password and an e-mail nobody has answer the same thing. Two
	// different answers are a way to find out who has an account here.
	if _, err := s.Verify("ana@exemplo.com", "outra"); err != ErrCredencial {
		t.Fatalf("senha errada = %v", err)
	}
	if _, err := s.Verify("ninguem@exemplo.com", "outra"); err != ErrCredencial {
		t.Fatalf("e-mail inexistente = %v", err)
	}
	// And the password is not in the row.
	if u := s.All()[0]; u.Hash == "uma-senha-boa" || len(u.Hash) < 20 {
		t.Fatalf("a senha ficou guardada: %q", u.Hash)
	}
}

func TestOMesmoEmailDuasVezes(t *testing.T) {
	s := &Store{rows: map[string]Usuario{}}
	if err := s.Add("u-1", "ana@exemplo.com", "Ana", "admin", "uma-senha-boa"); err != nil {
		t.Fatal(err)
	}
	if err := s.Add("u-2", "ana@exemplo.com", "Outra Ana", "leitor", "outra-senha"); err == nil {
		t.Fatal("o segundo cadastro substituiu o primeiro em silêncio")
	}
	if _, err := s.Verify("ana@exemplo.com", "uma-senha-boa"); err != nil {
		t.Fatalf("a primeira conta se perdeu: %v", err)
	}
}
`

const loginSession = `// Package sessao is the session of this application: there is no provider,
// because nobody outside says who the person is.
//
// The cookie is signed with TRILHA_SECRET and carries no data of its own — the
// session lives in the store, and what the browser holds is a name for it.
package sessao

import (
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/auth"

	"{{.Module}}/internal/usuarios"
)

// Flow is the session. Idle is what makes an open tab on a shared machine stop
// being a session at some point.
var Flow = auth.Sessions(auth.Options{
	Store:      auth.NewMemoryStore(),
	Idle:       30 * time.Minute,
	LoginPath:  "{{.URL}}entrar",
	AfterLogin: "{{.URL}}",
})

// Entrar opens the session for a row of the users table. What goes in it is
// what a page needs to draw itself — never a password, never a token that is
// not the app's to keep.
func Entrar(c *trilha.Ctx, u usuarios.Usuario) error {
	return Flow.Login(c, &auth.User{
		Subject: u.ID,
		Email:   u.Email,
		Name:    u.Nome,
		Roles:   []string{u.Papel},
	})
}

// Atual is who is asking, or nil. A page uses it to greet somebody and to hide
// what they cannot do; hiding is cosmetic, and the rule that holds is the
// middleware.
func Atual(c *trilha.Ctx) *auth.User {
	u, err := Flow.Session(c)
	if err != nil {
		return nil
	}
	return u
}
`

const loginPage = `// Package entrar is the login screen: a form, a password checked against the
// users table, and the session.
package entrar

import (
	"net/http"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"

	"{{.Module}}/internal/sessao"
	"{{.Module}}/internal/usuarios"
)

// Page renders GET {{.URL}}entrar.
func Page(c *trilha.Ctx) (h.Node, error) {
	return formulario(c, "", ""), nil
}

// POST checks the password and opens the session. The message is the same for
// a wrong e-mail and for a wrong password.
func POST(c *trilha.Ctx) error {
	email, senha := c.Form("email"), c.Form("password")
	u, err := trilha.Use[*usuarios.Store](c).Verify(email, senha)
	if err != nil {
		return c.Render(http.StatusUnprocessableEntity, formulario(c, email, "{{.T.login_wrong}}"))
	}
	return sessao.Entrar(c, u)
}

func formulario(c *trilha.Ctx, email, erro string) h.Node {
	c.SetTitle("{{.T.login_title}}")
	// The first run explains itself: with nobody in the table every password is
	// wrong, and a screen that only says "wrong password" to that is a screen
	// that lies. It is dev only — in production this line would tell a stranger
	// how the application is configured.
	var vazio h.Node = h.Fragment()
	if c.Env() == trilha.Dev && trilha.Use[*usuarios.Store](c).Vazio() {
		vazio = ui.Alert("{{.T.login_empty}}", ui.Icon("info"))
	}
	return ui.Stack(
		ui.Card(
			ui.CardHeader(ui.CardTitle("{{.T.login_title}}")),
			ui.CardContent(
				vazio,
				h.Form(h.Method("post"), h.Action("{{.URL}}entrar"), h.Class("ui-stack"), trilha.CSRFInput(c),
					h.If(erro != "", ui.Alert(erro, ui.Destructive(), ui.Icon("triangle-alert"))),
					ui.Field("email", "{{.T.login_email}}", ui.Input(h.ID("email"), h.Name("email"),
						h.Type("email"), h.Value(email), h.Autocomplete("username"), h.Required())),
					ui.Field("password", "{{.T.login_password}}", ui.Input(h.ID("password"), h.Name("password"),
						h.Type("password"), h.Autocomplete("current-password"), h.Required())),
					h.Div(ui.Submit(h.Text("{{.T.login_title}}"))),
				),
			),
		),
	)
}
`

const loginLogout = `// Package sair ends the session.
package sair

import (
	"github.com/emersonjoe/trilha"

	"{{.Module}}/internal/sessao"
)

// POST clears the session. It is a POST and not a link because a GET that
// changes something is a GET a page can do to somebody without asking.
func POST(c *trilha.Ctx) error { return sessao.Flow.Logout(c) }
`

// loginTest lives at the root of the project because that is where newApp() is:
// what is worth testing here is not the store — it has its own test — but that
// the screen, the session and the wiring in setup.go agree with each other.
const loginTest = `package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"testing"

	"github.com/emersonjoe/trilha"
)

// TestEntrarESair walks the recipe: the wrong password is refused, the right
// one opens a session, and signing out closes it.
func TestEntrarESair(t *testing.T) {
	t.Setenv("TRILHA_ENV", "prod")
	t.Setenv("TRILHA_SECRET", "a-test-secret-with-more-than-32-bytes!!")
	t.Setenv("ADMIN_EMAIL", "admin@example.com")
	t.Setenv("ADMIN_PASSWORD", "a-password-nobody-guesses")
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	c := trilha.NewTestClient(t, newApp())

	c.Get("{{.URL}}entrar").WantStatus(200)

	// Wrong password: the screen comes back with the message, and no session.
	rec := c.PostForm("{{.URL}}entrar", url.Values{
		"email":    {"admin@example.com"},
		"password": {"not the password"},
	}).WantStatus(http.StatusUnprocessableEntity)
	if rec.Cookie("trilha_session") != nil {
		t.Fatal("a wrong password opened a session")
	}

	// The right one: a redirect and a session.
	c.PostForm("{{.URL}}entrar", url.Values{
		"email":    {"admin@example.com"},
		"password": {"a-password-nobody-guesses"},
	}).WantStatus(http.StatusSeeOther)

	// And out again.
	c.PostForm("{{.URL}}sair", url.Values{}).WantStatus(http.StatusSeeOther)
}
`

// loginTestHelper is how a test of a screen behind a middleware opens a
// session. It is a package of its own because it imports testing: a helper
// that drags the testing package into the binary costs something in
// production.
//
// The generator looks for this package by name — a CRUD written under a
// guarded folder calls it instead of walking into a 401.
const loginTestHelper = `// Package sessaotest opens a session in a test.
package sessaotest

import (
	"net/http"
	"net/url"
	"os"
	"testing"

	"github.com/emersonjoe/trilha"
)

// Entrar signs in as the administrator seeded from the environment, which is
// what a test of a screen behind a middleware needs before its first request.
//
//	c := trilha.NewTestClient(t, newApp())
//	sessaotest.Entrar(t, c)
//
// It posts to the sign-in screen instead of forging a cookie: what it proves
// along the way is that signing in still works, and a helper that built the
// session by hand would keep passing after the login stopped.
func Entrar(t *testing.T, c *trilha.TestClient) {
	t.Helper()
	email, senha := os.Getenv("ADMIN_EMAIL"), os.Getenv("ADMIN_PASSWORD")
	if email == "" || senha == "" {
		t.Fatal("sessaotest: set ADMIN_EMAIL and ADMIN_PASSWORD with t.Setenv before newApp()")
	}
	rec := c.PostForm("{{.URL}}entrar", url.Values{"email": {email}, "password": {senha}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("sessaotest: signing in answered %d, not a redirect", rec.Code)
	}
}
`
