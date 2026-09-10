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
			"en": "the account screen: own name, own password — the id comes from the session",
			"pt": "a tela da própria conta: nome e senha — o id vem da sessão",
		},
		Doc:   "/reference/auth",
		Needs: []Need{{Recipe: "login", File: "internal/usuarios/usuarios.go"}},
		Files: []File{
			{Rel: "internal/usuarios/perfil.go", Go: true, Body: profileStore},
			{Rel: "internal/usuarios/perfil_test.go", Go: true, Body: profileStoreTest},
			{Rel: "{{.At}}perfil/page.go", Go: true, Body: profilePage},
			{Rel: "{{.At}}perfil/middleware.go", Go: true, Body: profileMiddleware},
			{Rel: "perfil_test.go", Go: true, Body: profileTest},
		},
		Next: map[string]string{
			"en": "Run `trilha dev` and open {{.URL}}perfil. Put the link in your shell's menu — a screen " +
				"nobody can reach is a screen that ages. Changing an e-mail is deliberately not here: " +
				"without confirming at the new address, changing an e-mail is changing owner.",
			"pt": "Rode `trilha dev` e abra {{.URL}}perfil. Ponha o link no menu do seu shell — tela que " +
				"ninguém alcança é tela que envelhece. Trocar e-mail não está aqui de propósito: sem " +
				"confirmar no endereço novo, trocar e-mail é trocar de dono.",
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
	"errors"
	"strings"

	"github.com/emersonjoe/trilha/auth"
)

// ErrSenhaAtual is what a wrong current password answers. It is separate from
// ErrCredencial because this one is said to somebody who is already signed in:
// nothing is being revealed that they do not know.
var ErrSenhaAtual = errors.New("the current password does not match")

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
	"net/http"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"

	"{{.Module}}/internal/sessao"
	"{{.Module}}/internal/usuarios"
)

// Page renders GET {{.URL}}perfil.
func Page(c *trilha.Ctx) (h.Node, error) {
	return tela(c, "", "")
}

// POST changes the name or the password, dispatched by the form's action
// field: they are one screen, and a form that posts to itself comes back to
// itself when something is wrong.
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
			n, rerr := tela(c, "", "{{.T.profile_wrong}}")
			if rerr != nil {
				return rerr
			}
			return c.Render(http.StatusUnprocessableEntity, n)
		}
		c.Audit("perfil.senha", u.Subject, nil)
		// The session ends with the password that opened it: signing in again
		// is the proof that the new password is the one they meant. Ending the
		// *other* sessions needs a store that can find them by owner, and the
		// one in memory cannot — that line goes here when it becomes a table.
		c.Flash(ui.FlashSuccess, "{{.T.profile_password_done}}")
		return sessao.Flow.Logout(c)
	default:
		if err := store.TrocarNome(u.Subject, c.Form("nome")); err != nil {
			n, rerr := tela(c, "{{.T.profile_need_name}}", "")
			if rerr != nil {
				return rerr
			}
			return c.Render(http.StatusUnprocessableEntity, n)
		}
		c.Audit("perfil.nome", u.Subject, nil)
		c.Flash(ui.FlashSuccess, "{{.T.profile_saved}}")
		return c.Redirect("{{.URL}}perfil")
	}
}

func tela(c *trilha.Ctx, erroNome, erroSenha string) (h.Node, error) {
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
				h.If(erroNome != "", ui.Alert(erroNome, ui.Destructive(), ui.Icon("triangle-alert"))),
				ui.Field("nome", "{{.T.profile_name}}", ui.Input(h.ID("nome"), h.Name("nome"),
					h.Value(linha.Nome), h.Required())),
				h.Div(ui.Submit(h.Text("{{.T.profile_save}}"))),
			)),
		),
		ui.Card(
			ui.CardHeader(ui.CardTitle("{{.T.profile_password}}"),
				ui.CardDescription("{{.T.profile_password_desc}}")),
			ui.CardContent(h.Form(h.Method("post"), h.Action("{{.URL}}perfil"), h.Class("ui-stack"),
				trilha.CSRFInput(c),
				h.Input(h.Type("hidden"), h.Name("acao"), h.Value("senha")),
				h.If(erroSenha != "", ui.Alert(erroSenha, ui.Destructive(), ui.Icon("triangle-alert"))),
				ui.Field("atual", "{{.T.profile_current}}", ui.Input(h.ID("atual"), h.Name("atual"),
					h.Type("password"), h.Autocomplete("current-password"), h.Required())),
				ui.Field("nova", "{{.T.profile_new}}", ui.Input(h.ID("nova"), h.Name("nova"),
					h.Type("password"), h.Autocomplete("new-password"), h.Required())),
				h.Div(ui.Submit(h.Text("{{.T.profile_change}}"))),
			)),
		),
	), nil
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
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha"
)

// The account screen changes the account of whoever is asking — and there is
// no id anywhere on it to change that.
func TestPerfilMudaAContaDeQuemPede(t *testing.T) {
	t.Setenv("TRILHA_ENV", "prod")
	t.Setenv("TRILHA_SECRET", "a-test-secret-with-more-than-32-bytes!!")
	t.Setenv("ADMIN_EMAIL", "admin@example.com")
	t.Setenv("ADMIN_PASSWORD", "a-password-nobody-guesses")
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	a := newApp()
	c := trilha.NewTestClient(t, a)

	c.Get("{{.URL}}perfil").WantStatus(http.StatusUnauthorized)
	c.PostForm("{{.URL}}entrar", url.Values{
		"email":    {"admin@example.com"},
		"password": {"a-password-nobody-guesses"},
	}).WantStatus(http.StatusSeeOther)

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

	// A certa troca, fecha a sessão, e é a senha nova que entra.
	c.PostForm("{{.URL}}perfil", url.Values{
		"acao":  {"senha"},
		"atual": {"a-password-nobody-guesses"},
		"nova":  {"uma-senha-nova-boa"},
	}).WantStatus(http.StatusSeeOther)
	c.Get("{{.URL}}perfil").WantStatus(http.StatusUnauthorized)

	c.PostForm("{{.URL}}entrar", url.Values{
		"email":    {"admin@example.com"},
		"password": {"a-password-nobody-guesses"},
	}).WantStatus(http.StatusUnprocessableEntity)
	c.PostForm("{{.URL}}entrar", url.Values{
		"email":    {"admin@example.com"},
		"password": {"uma-senha-nova-boa"},
	}).WantStatus(http.StatusSeeOther)
}
`
