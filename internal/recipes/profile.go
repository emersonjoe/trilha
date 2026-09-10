package recipes

// profileRecipe is the screen where somebody changes their own name and their
// own password.
//
// It is the smallest of the four screens the admin issue lists, and the one
// where the same bug is written most often: the form carries the id and the
// server trusts it. Here there is no id on the screen at all — it comes from
// the session, which is the only place that knows whose account this is.
func profileRecipe() Recipe {
	return Recipe{
		Name: "profile",
		Summary: map[string]string{
			"en": "the account screen: own name, password, e-mail confirmed at the new address, sessions — the id comes from the session",
			"pt": "a tela da própria conta: nome, senha, e-mail confirmado no endereço novo, sessões — o id vem da sessão",
		},
		Doc:   "/reference/auth",
		Needs: []Need{{Recipe: "login", File: "internal/usuarios/usuarios.go"}},
		Files: []File{
			{Rel: "internal/usuarios/perfil.go", Go: true, Body: profileStore},
			{Rel: "internal/usuarios/perfil_test.go", Go: true, Body: profileStoreTest},
			{Rel: "{{.At}}perfil/page.go", Go: true, Body: profilePage},
			{Rel: "{{.At}}perfil/middleware.go", Go: true, Body: profileMiddleware},
			{Rel: "{{.At}}perfil/email/token_/route.go", Go: true, Body: profileEmailRoute},
			{Rel: "perfil_test.go", Go: true, Body: profileTest},
		},
		// With the mail recipe there, the confirmation of a new e-mail goes
		// out through it. The same line the mail recipe carries, each
		// conditioned on the other's file.
		Setup: []Insert{{
			Marker:  "// trilha:link profile-mail",
			Line:    "\tusuarios.EnviarConfirmacao = correio.Confirmacao\n",
			If:      "internal/correio/correio.go",
			Imports: []string{"{{.Module}}/internal/correio", "{{.Module}}/internal/usuarios"},
		}},
		Next: map[string]string{
			"en": "Run `trilha dev` and open {{.URL}}perfil. Put the link in your shell's menu — a screen " +
				"nobody can reach is a screen that ages. Changing the e-mail needs somebody to send the " +
				"confirmation: `trilha add mail` wires it; until then the screen says so and refuses.",
			"pt": "Rode `trilha dev` e abra {{.URL}}perfil. Ponha o link no menu do seu shell — tela que " +
				"ninguém alcança é tela que envelhece. Trocar o e-mail precisa de alguém que mande a " +
				"confirmação: `trilha add mail` liga isso; até lá a tela avisa e recusa.",
		},
	}
}

const profileStore = `package usuarios

// This file is the account's own half of the table: the two things somebody
// changes about themselves.
//
// Every function here takes an id and never an e-mail from a form. The screen
// gets that id from the session, which is the only place that knows whose
// account is being changed — a form that carries the id is a form somebody
// edits, and then the profile screen is everybody's profile screen.

import (
	"context"
	"errors"
	"strings"

	"github.com/emersonjoe/trilha/auth"
)

// ErrSenhaAtual is what a wrong current password answers. It is separate from
// ErrCredencial because this one is said to somebody who is already signed in:
// nothing is being revealed that they do not know.
var ErrSenhaAtual = errors.New("the current password does not match")

// ErrEmailEmUso is a new address somebody else already has.
var ErrEmailEmUso = errors.New("that e-mail belongs to another account")

// ErrEmailInvalido is a new address that is empty, has no @, or is the one
// the account already has.
var ErrEmailInvalido = errors.New("that is not an e-mail, or it is the current one")

// EnviarConfirmacao sends the link that confirms a new e-mail at the new
// address. Nil means nobody sends e-mail here, and the account screen refuses
// the change and says so — an e-mail changed without a confirmation at the
// new address is an account handed to whoever typed it. With the mail recipe,
// app/setup.go sets it to correio.Confirmacao.
var EnviarConfirmacao func(ctx context.Context, para, link string) error

// TrocarEmail moves the account to a confirmed address. It is called by the
// route the confirmation link points at, and by nothing that a form reaches
// directly: the confirmation is the whole point.
func (s *Store) TrocarEmail(id, novo string) error {
	chave := normaliza(novo)
	if chave == "" || !strings.Contains(chave, "@") {
		return ErrEmailInvalido
	}
	u, ok := s.Perfil(id)
	if !ok {
		return errors.New("usuarios: nobody with that id")
	}
	antiga := normaliza(u.Email)
	if antiga == chave {
		return ErrEmailInvalido
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, taken := s.rows[chave]; taken {
		return ErrEmailEmUso
	}
	linha := s.rows[antiga]
	linha.Email = strings.TrimSpace(novo)
	delete(s.rows, antiga)
	s.rows[chave] = linha
	return nil
}

// Perfil is the row of whoever is asking.
func (s *Store) Perfil(id string) (Usuario, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, u := range s.rows {
		if u.ID == id {
			return u, true
		}
	}
	return Usuario{}, false
}

// TrocarNome changes the name shown beside somebody's actions.
func (s *Store) TrocarNome(id, nome string) error {
	nome = strings.TrimSpace(nome)
	if nome == "" {
		return errors.New("usuarios: a name is required")
	}
	u, ok := s.Perfil(id)
	if !ok {
		return errors.New("usuarios: nobody with that id")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	chave := normaliza(u.Email)
	linha := s.rows[chave]
	linha.Nome = nome
	s.rows[chave] = linha
	return nil
}

// TrocarSenha changes a password, and asks for the current one first.
//
// The session is not enough: a machine left unlocked for two minutes should
// not become an account somebody lost, and the current password is the thing
// the owner knows and whoever walked past does not.
func (s *Store) TrocarSenha(id, atual, nova string) error {
	u, ok := s.Perfil(id)
	if !ok {
		return errors.New("usuarios: nobody with that id")
	}
	if !auth.CheckPBKDF2(u.Hash, atual) {
		return ErrSenhaAtual
	}
	if len(nova) < 12 {
		return errors.New("usuarios: a password of at least 12 characters")
	}
	hash, err := auth.HashPBKDF2(nova)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	chave := normaliza(u.Email)
	linha := s.rows[chave]
	linha.Hash = hash
	s.rows[chave] = linha
	return nil
}
`

const profileStoreTest = `package usuarios

import "testing"

func conta(t *testing.T) *Store {
	t.Helper()
	s := &Store{rows: map[string]Usuario{}}
	if err := s.Add("u-1", "ana@exemplo.com", "Ana", "admin", "a-senha-de-agora"); err != nil {
		t.Fatal(err)
	}
	return s
}

// A senha atual é o que o dono sabe e quem passou ali não: sem ela, uma
// máquina destravada por dois minutos vira uma conta perdida.
func TestTrocarSenhaPedeAAtual(t *testing.T) {
	s := conta(t)
	if err := s.TrocarSenha("u-1", "a-errada", "uma-senha-nova-boa"); err != ErrSenhaAtual {
		t.Fatalf("trocou com a senha errada: %v", err)
	}
	if _, err := s.Verify("ana@exemplo.com", "a-senha-de-agora"); err != nil {
		t.Fatal("a senha antiga parou de valer depois de uma tentativa recusada")
	}
	if err := s.TrocarSenha("u-1", "a-senha-de-agora", "curta"); err == nil {
		t.Fatal("aceitou uma senha curta")
	}
	if err := s.TrocarSenha("u-1", "a-senha-de-agora", "uma-senha-nova-boa"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Verify("ana@exemplo.com", "uma-senha-nova-boa"); err != nil {
		t.Fatalf("a senha nova não entra: %v", err)
	}
	if _, err := s.Verify("ana@exemplo.com", "a-senha-de-agora"); err != ErrCredencial {
		t.Fatal("a senha antiga continuou entrando")
	}
}

// O e-mail novo não pode ser vazio, o atual, nem o de outra conta — e a linha
// muda de chave junto, porque a tabela é por e-mail.
func TestTrocarEmail(t *testing.T) {
	s := conta(t)
	if err := s.Add("u-2", "outra@exemplo.com", "Outra", "leitor", "a-senha-de-agora"); err != nil {
		t.Fatal(err)
	}
	if err := s.TrocarEmail("u-1", "sem-arroba"); err != ErrEmailInvalido {
		t.Fatalf("sem arroba: %v", err)
	}
	if err := s.TrocarEmail("u-1", "ana@exemplo.com"); err != ErrEmailInvalido {
		t.Fatalf("o mesmo: %v", err)
	}
	if err := s.TrocarEmail("u-1", "Outra@Exemplo.com"); err != ErrEmailEmUso {
		t.Fatalf("de outra conta: %v", err)
	}
	if err := s.TrocarEmail("u-1", "nova@exemplo.com"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Verify("nova@exemplo.com", "a-senha-de-agora"); err != nil {
		t.Fatal("o e-mail novo não entra")
	}
	if _, err := s.Verify("ana@exemplo.com", "a-senha-de-agora"); err == nil {
		t.Fatal("o e-mail antigo ainda entra")
	}
}

func TestTrocarNome(t *testing.T) {
	s := conta(t)
	if err := s.TrocarNome("u-1", "  Ana Maria  "); err != nil {
		t.Fatal(err)
	}
	u, _ := s.Perfil("u-1")
	if u.Nome != "Ana Maria" {
		t.Fatalf("nome = %q", u.Nome)
	}
	if err := s.TrocarNome("u-1", "   "); err == nil {
		t.Fatal("aceitou um nome vazio")
	}
}
`

const profilePage = `// Package perfil is where somebody changes their own account.
//
// There is no id field on this screen, in any form, on purpose. The id comes
// from the session — the only place that knows whose account this is — and a
// form that carried it would be a form somebody edits, turning the profile
// screen into everybody else's profile screen.
package perfil

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/auth"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"

	"{{.Module}}/internal/sessao"
	"{{.Module}}/internal/usuarios"
)

// Page renders GET {{.URL}}perfil.
func Page(c *trilha.Ctx) (h.Node, error) {
	return tela(c, erros{})
}

// erros is what each form shows back when it is refused.
type erros struct{ nome, senha, email string }

// POST is every form of the screen, dispatched by the form's action field:
// they are one screen, and a form that posts to itself comes back to itself
// when something is wrong.
func POST(c *trilha.Ctx) error {
	u := sessao.Atual(c)
	if u == nil {
		return trilha.Errorf(http.StatusUnauthorized, "%s", "{{.T.profile_gone}}")
	}
	store := trilha.Use[*usuarios.Store](c)
	switch c.Form("acao") {
	case "senha":
		err := store.TrocarSenha(u.Subject, c.Form("atual"), c.Form("nova"))
		if err != nil {
			return recusa(c, erros{senha: "{{.T.profile_wrong}}"})
		}
		c.Audit("perfil.senha", u.Subject, nil)
		// Every other session ends with the password that opened it: a
		// password is changed because somebody may have the old one, and
		// that somebody may be signed in right now. This one ends too —
		// signing in again is the proof that the new password is the one
		// they meant.
		if err := sessao.Flow.LogoutOthers(c); err != nil && !errors.Is(err, auth.ErrNoSessionList) {
			return err
		}
		c.Flash(ui.FlashSuccess, "{{.T.profile_password_done}}")
		return sessao.Flow.Logout(c)
	case "sessoes":
		if err := sessao.Flow.LogoutOthers(c); err != nil {
			return err
		}
		c.Flash(ui.FlashSuccess, "{{.T.profile_ended_others}}")
		return c.Redirect("{{.URL}}perfil")
	case "email":
		// Two steps: the link goes to the new address, and only the route it
		// points at changes anything. Nobody sending e-mail is a refusal
		// with a reason, not a change without a confirmation.
		if usuarios.EnviarConfirmacao == nil {
			return recusa(c, erros{email: "{{.T.profile_email_nomail}}"})
		}
		novo := strings.TrimSpace(c.Form("email"))
		linha, _ := store.Perfil(u.Subject)
		if !strings.Contains(novo, "@") || strings.EqualFold(novo, linha.Email) {
			return recusa(c, erros{email: "{{.T.profile_email_invalid}}"})
		}
		link, err := c.Link("email", trilha.LinkOpts{
			Data: map[string]string{"id": u.Subject, "email": novo},
			TTL:  time.Hour,
			Uses: 1,
			Path: "{{.URL}}perfil/email",
		})
		if err != nil {
			return err
		}
		if err := usuarios.EnviarConfirmacao(c.Context(), novo, origem(c)+link); err != nil {
			return err
		}
		// The trail says a change was asked for, and to where; the change
		// itself is recorded by the route that confirms it.
		c.Audit("perfil.email_pedido", u.Subject, trilha.Fields{"para": novo})
		c.Flash(ui.FlashSuccess, "{{.T.profile_email_sent}}")
		return c.Redirect("{{.URL}}perfil")
	default:
		if err := store.TrocarNome(u.Subject, c.Form("nome")); err != nil {
			return recusa(c, erros{nome: "{{.T.profile_need_name}}"})
		}
		c.Audit("perfil.nome", u.Subject, nil)
		c.Flash(ui.FlashSuccess, "{{.T.profile_saved}}")
		return c.Redirect("{{.URL}}perfil")
	}
}

func recusa(c *trilha.Ctx, e erros) error {
	n, err := tela(c, e)
	if err != nil {
		return err
	}
	return c.Render(http.StatusUnprocessableEntity, n)
}

// origem is the scheme and host the confirmation link is built on: what the
// request came in with. Behind a proxy that does not say X-Forwarded-Proto,
// the link comes out as http — set the header on the proxy, or replace this
// with the public address of the application.
func origem(c *trilha.Ctx) string {
	r := c.Request()
	esquema := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		esquema = "https"
	}
	return esquema + "://" + r.Host
}

func tela(c *trilha.Ctx, e erros) (h.Node, error) {
	u := sessao.Atual(c)
	if u == nil {
		return nil, trilha.Errorf(http.StatusUnauthorized, "%s", "{{.T.profile_gone}}")
	}
	linha, _ := trilha.Use[*usuarios.Store](c).Perfil(u.Subject)
	c.SetTitle("{{.T.profile_title}}")
	return ui.Stack(
		ui.PageHeader("{{.T.profile_title}}"),
		ui.Card(
			ui.CardHeader(ui.CardTitle("{{.T.profile_name}}"), ui.CardDescription(linha.Email)),
			ui.CardContent(h.Form(h.Method("post"), h.Action("{{.URL}}perfil"), h.Class("ui-stack"),
				trilha.CSRFInput(c),
				h.Input(h.Type("hidden"), h.Name("acao"), h.Value("nome")),
				h.If(e.nome != "", ui.Alert(e.nome, ui.Destructive(), ui.Icon("triangle-alert"))),
				ui.Field("nome", "{{.T.profile_name}}", ui.Input(h.ID("nome"), h.Name("nome"),
					h.Value(linha.Nome), h.Required())),
				h.Div(ui.Submit(h.Text("{{.T.profile_save}}"))),
			)),
		),
		ui.Card(
			ui.CardHeader(ui.CardTitle("{{.T.profile_email}}"),
				ui.CardDescription("{{.T.profile_email_desc}}")),
			ui.CardContent(h.Form(h.Method("post"), h.Action("{{.URL}}perfil"), h.Class("ui-stack"),
				trilha.CSRFInput(c),
				h.Input(h.Type("hidden"), h.Name("acao"), h.Value("email")),
				h.If(e.email != "", ui.Alert(e.email, ui.Destructive(), ui.Icon("triangle-alert"))),
				ui.Field("email", "{{.T.profile_new_email}}", ui.Input(h.ID("email"), h.Name("email"),
					h.Type("email"), h.Autocomplete("email"), h.Required())),
				h.Div(ui.Submit(h.Text("{{.T.profile_email_change}}"))),
			)),
		),
		ui.Card(
			ui.CardHeader(ui.CardTitle("{{.T.profile_password}}"),
				ui.CardDescription("{{.T.profile_password_desc}}")),
			ui.CardContent(h.Form(h.Method("post"), h.Action("{{.URL}}perfil"), h.Class("ui-stack"),
				trilha.CSRFInput(c),
				h.Input(h.Type("hidden"), h.Name("acao"), h.Value("senha")),
				h.If(e.senha != "", ui.Alert(e.senha, ui.Destructive(), ui.Icon("triangle-alert"))),
				ui.Field("atual", "{{.T.profile_current}}", ui.Input(h.ID("atual"), h.Name("atual"),
					h.Type("password"), h.Autocomplete("current-password"), h.Required())),
				ui.Field("nova", "{{.T.profile_new}}", ui.Input(h.ID("nova"), h.Name("nova"),
					h.Type("password"), h.Autocomplete("new-password"), h.Required())),
				h.Div(ui.Submit(h.Text("{{.T.profile_change}}"))),
			)),
		),
		sessoes(c),
	), nil
}

// sessoes is where else this account is signed in, and the button that ends
// everything but here. The list needs a session store that can find sessions
// by owner; the one in memory can, and a table of yours implements
// auth.SessionLister to keep the card.
func sessoes(c *trilha.Ctx) h.Node {
	lista, err := sessao.Flow.Sessions(c)
	if err != nil {
		return ui.Card(
			ui.CardHeader(ui.CardTitle("{{.T.profile_sessions}}")),
			ui.CardContent(ui.Muted(h.Text("{{.T.profile_no_list}}"))))
	}
	// Sessions puts this one first; the badge says so.
	linhas := make([]h.Node, 0, len(lista))
	for i, s := range lista {
		onde := h.Text("")
		if i == 0 {
			onde = ui.Badge(h.Text("{{.T.profile_this}}"))
		}
		linhas = append(linhas, h.Tr(
			h.Td(ui.Date(c, s.IssuedAt, ui.Relative())),
			h.Td(ui.Date(c, s.Seen, ui.Relative())),
			h.Td(onde),
		))
	}
	return ui.Card(
		ui.CardHeader(ui.CardTitle("{{.T.profile_sessions}}"), ui.CardDescription("{{.T.profile_sessions_desc}}")),
		ui.CardContent(ui.Stack(
			ui.Table(
				h.Thead(h.Tr(h.Th(h.Text("{{.T.profile_since}}")), h.Th(h.Text("{{.T.profile_seen}}")), h.Th(h.Text("")))),
				h.Tbody(linhas...)),
			h.If(len(lista) > 1, h.Form(h.Method("post"), h.Action("{{.URL}}perfil"), h.Class("ui-inline-form"),
				trilha.CSRFInput(c),
				h.Input(h.Type("hidden"), h.Name("acao"), h.Value("sessoes")),
				ui.Button(ui.Outline(), h.Type("submit"), h.Text("{{.T.profile_end_others}}")))),
		)),
	)
}
`

const profileEmailRoute = `// Package token_ is the route the confirmation link points at: the one place
// where an e-mail actually changes.
//
// It sits under perfil/, so it asks for a session like the rest of the
// screen: the link proves the new address, the session proves the owner, and
// the two have to agree. A link opened from another account changes nothing.
package token_

import (
	"errors"
	"net/http"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/ui"

	"{{.Module}}/internal/sessao"
	"{{.Module}}/internal/usuarios"
)

// GET confirms the new address and comes back to the account screen.
func GET(c *trilha.Ctx) error {
	u := sessao.Atual(c)
	if u == nil {
		return trilha.Errorf(http.StatusUnauthorized, "%s", "{{.T.profile_gone}}")
	}
	link, err := c.Claim("email")
	if err != nil {
		return err // 404: invalid, expired or spent — never which
	}
	if link.Data["id"] != u.Subject {
		return trilha.Errorf(http.StatusForbidden, "%s", "{{.T.profile_email_not_yours}}")
	}
	if err := link.Consume(); err != nil {
		return err
	}
	err = trilha.Use[*usuarios.Store](c).TrocarEmail(u.Subject, link.Data["email"])
	switch {
	case errors.Is(err, usuarios.ErrEmailEmUso), errors.Is(err, usuarios.ErrEmailInvalido):
		c.Flash(ui.FlashError, "{{.T.profile_email_invalid}}")
		return c.Redirect("{{.URL}}perfil")
	case err != nil:
		return err
	}
	c.Audit("perfil.email", u.Subject, trilha.Fields{"para": link.Data["email"]})
	c.Flash(ui.FlashSuccess, "{{.T.profile_email_done}}")
	return c.Redirect("{{.URL}}perfil")
}
`

const profileMiddleware = `package perfil

import (
	"github.com/emersonjoe/trilha"

	"{{.Module}}/internal/sessao"
)

// exige is the rule, and Middleware is what the scanner reads: middleware.go
// has to export a function with that signature, and a var of the right type is
// not one.
var exige = sessao.Flow.Require()

// Middleware asks for a session and nothing else. This screen is about the
// account of whoever is asking, so having one is the whole permission.
func Middleware(c *trilha.Ctx, next trilha.Next) error { return exige(c, next) }
`

const profileTest = `package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha"

	"{{.Module}}/internal/sessao"
	"{{.Module}}/internal/usuarios"
)

func entrar(t *testing.T, a *trilha.App, senha string) *trilha.TestClient {
	t.Helper()
	c := trilha.NewTestClient(t, a)
	c.PostForm(sessao.Flow.LoginPath(), url.Values{
		"email":    {"admin@example.com"},
		"password": {senha},
	}).WantStatus(http.StatusSeeOther)
	return c
}

func ambiente(t *testing.T) {
	t.Setenv("TRILHA_ENV", "prod")
	t.Setenv("TRILHA_SECRET", "a-test-secret-with-more-than-32-bytes!!")
	t.Setenv("ADMIN_EMAIL", "admin@example.com")
	t.Setenv("ADMIN_PASSWORD", "a-password-nobody-guesses")
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
}

// The account screen changes the account of whoever is asking — and there is
// no id anywhere on it to change that.
func TestPerfilMudaAContaDeQuemPede(t *testing.T) {
	ambiente(t)
	a := newApp()
	trilha.NewTestClient(t, a).Get("{{.URL}}perfil").WantStatus(http.StatusUnauthorized)
	c := entrar(t, a, "a-password-nobody-guesses")

	corpo := c.Get("{{.URL}}perfil").WantStatus(http.StatusOK).Body.String()
	if strings.Contains(corpo, "name=\"id\"") {
		t.Fatal("a tela tem um campo de id: o dono da conta viria do formulário")
	}

	c.PostForm("{{.URL}}perfil", url.Values{
		"acao": {"nome"},
		"nome": {"Pessoa Nova"},
	}).WantStatus(http.StatusSeeOther)
	c.Get("{{.URL}}perfil").WantStatus(http.StatusOK).WantContains("Pessoa Nova")

	// A senha atual errada é recusada, e a sessão continua de pé.
	c.PostForm("{{.URL}}perfil", url.Values{
		"acao":  {"senha"},
		"atual": {"not the password"},
		"nova":  {"uma-senha-nova-boa"},
	}).WantStatus(http.StatusUnprocessableEntity)
	c.Get("{{.URL}}perfil").WantStatus(http.StatusOK)

	// Um segundo navegador com a mesma conta: a tela lista as duas sessões.
	outro := entrar(t, a, "a-password-nobody-guesses")
	if got := strings.Count(c.Get("{{.URL}}perfil").WantStatus(http.StatusOK).Body.String(), "<tr>"); got < 3 {
		t.Fatalf("a tela lista %d linhas de sessão (cabeçalho + 2 esperadas)", got)
	}

	// A senha certa troca, fecha esta sessão e as outras, e é a senha nova
	// que entra.
	c.PostForm("{{.URL}}perfil", url.Values{
		"acao":  {"senha"},
		"atual": {"a-password-nobody-guesses"},
		"nova":  {"uma-senha-nova-boa"},
	}).WantStatus(http.StatusSeeOther)
	c.Get("{{.URL}}perfil").WantStatus(http.StatusUnauthorized)
	outro.Get("{{.URL}}perfil").WantStatus(http.StatusUnauthorized)

	c.PostForm(sessao.Flow.LoginPath(), url.Values{
		"email":    {"admin@example.com"},
		"password": {"a-password-nobody-guesses"},
	}).WantStatus(http.StatusUnprocessableEntity)
	entrar(t, a, "uma-senha-nova-boa")
}

// "End the others" ends the others and keeps this one.
func TestEncerrarAsOutrasSessoes(t *testing.T) {
	ambiente(t)
	a := newApp()
	c := entrar(t, a, "a-password-nobody-guesses")
	outro := entrar(t, a, "a-password-nobody-guesses")
	outro.Get("{{.URL}}perfil").WantStatus(http.StatusOK)

	c.PostForm("{{.URL}}perfil", url.Values{"acao": {"sessoes"}}).WantStatus(http.StatusSeeOther)
	c.Get("{{.URL}}perfil").WantStatus(http.StatusOK)
	outro.Get("{{.URL}}perfil").WantStatus(http.StatusUnauthorized)
}

// Changing the e-mail is two steps: a link to the new address, and the route
// it points at. Without somebody to send it, the screen refuses and says so.
func TestTrocarEmailConfirmaNoEnderecoNovo(t *testing.T) {
	ambiente(t)
	a := newApp()
	c := entrar(t, a, "a-password-nobody-guesses")

	antes := usuarios.EnviarConfirmacao
	t.Cleanup(func() { usuarios.EnviarConfirmacao = antes })
	usuarios.EnviarConfirmacao = nil
	c.PostForm("{{.URL}}perfil", url.Values{"acao": {"email"}, "email": {"nova@example.com"}}).
		WantStatus(http.StatusUnprocessableEntity)

	var link string
	usuarios.EnviarConfirmacao = func(_ context.Context, para, l string) error {
		if para != "nova@example.com" {
			t.Errorf("a confirmação foi para %q", para)
		}
		link = l
		return nil
	}
	// The current address and something that is not an address are refused
	// before anything is sent.
	c.PostForm("{{.URL}}perfil", url.Values{"acao": {"email"}, "email": {"admin@example.com"}}).
		WantStatus(http.StatusUnprocessableEntity)
	c.PostForm("{{.URL}}perfil", url.Values{"acao": {"email"}, "email": {"nova@example.com"}}).
		WantStatus(http.StatusSeeOther)
	if link == "" {
		t.Fatal("nenhum link foi mandado")
	}
	// Nothing changed yet: the old address still signs in.
	entrar(t, a, "a-password-nobody-guesses")

	// The link is absolute (it goes in an e-mail) and its path is the route.
	u, err := url.Parse(link)
	if err != nil || u.Host == "" || !strings.HasPrefix(u.Path, "{{.URL}}perfil/email/") {
		t.Fatalf("link = %q", link)
	}
	// Another account opening it changes nothing — it is not theirs.
	trilha.NewTestClient(t, a).Get(u.Path).WantStatus(http.StatusUnauthorized)

	c.Get(u.Path).WantStatus(http.StatusSeeOther)
	c.Get("{{.URL}}perfil").WantStatus(http.StatusOK).WantContains("nova@example.com")
	// Once: the link is spent.
	c.Get(u.Path).WantStatus(http.StatusNotFound)

	trilha.NewTestClient(t, a).PostForm(sessao.Flow.LoginPath(), url.Values{
		"email":    {"nova@example.com"},
		"password": {"a-password-nobody-guesses"},
	}).WantStatus(http.StatusSeeOther)
}
`
