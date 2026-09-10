package recipes

// usersRecipe is the screen that names people: who is in, with what role, and
// how somebody new gets a password.
//
// It is the first recipe written on top of another. The table of people is the
// login recipe's, and this one adds a file to that same package — which is why
// Needs exists at all: without login there is no package to add to, and five
// files that do not compile are worse than a refusal.
func usersRecipe() Recipe {
	return Recipe{
		Name: "users",
		Summary: map[string]string{
			"en": "the people screen: invite, role, deactivate, reset — on the login recipe's table",
			"pt": "a tela de gente: convidar, papel, desativar, resetar — sobre a tabela da receita login",
		},
		Doc:   "/reference/auth",
		Needs: []Need{{Recipe: "login", File: "internal/usuarios/usuarios.go"}},
		Files: []File{
			{Rel: "internal/usuarios/convites.go", Go: true, Body: usersInvites},
			{Rel: "internal/usuarios/convites_test.go", Go: true, Body: usersInvitesTest},
			{Rel: "{{.At}}usuarios/page.go", Go: true, Body: usersPage},
			{Rel: "{{.At}}usuarios/middleware.go", Go: true, Body: usersMiddleware},
			// The invitation is answered by somebody who is not signed in yet,
			// so it cannot live under a folder that requires a session — not
			// even the one this recipe was pointed at.
			{Rel: "app/convite/token_/page.go", Go: true, Body: usersAccept},
			{Rel: "usuarios_test.go", Go: true, Body: usersTest},
		},
		Next: map[string]string{
			"en": "Run `trilha dev` and open {{.URL}}usuarios. The invitation link is shown to whoever " +
				"created it; to send it by e-mail instead, one `mail.Send` in the invite branch is the " +
				"whole change. Check that /convite is reachable without a session — that is the point of it.",
			"pt": "Rode `trilha dev` e abra {{.URL}}usuarios. O link do convite aparece para quem o criou; " +
				"para mandá-lo por e-mail, um `mail.Send` no ramo do convite é a mudança inteira. Confira " +
				"que /convite responde sem sessão — é para isso que ele existe.",
		},
	}
}

const usersInvites = `package usuarios

// This file is the users screen's half of the table: who gets in, with what
// role, and how a password is born.
//
// A password is never chosen by an administrator. Inviting creates the row
// without one and hands out a link; the person sets it. An administrator who
// picks somebody's password is an administrator who knows it.

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/emersonjoe/trilha/auth"
)

// Validade is how long an invitation is worth. Long enough for somebody to
// read their e-mail tomorrow, short enough that a link found in a mailbox next
// year is not a way in.
const Validade = 48 * time.Hour

// ErrConvite is a token that is wrong, used or too old. It is one error for
// the three on purpose: which of them it was is not the visitor's business.
var ErrConvite = errors.New("this invitation is no longer valid")

// Convite is what somebody is given: the link's token, and when it stops
// working. The token is returned once, here, and never again — what is kept is
// its hash.
type Convite struct {
	Token   string
	Expira  time.Time
	Usuario Usuario
}

// convite is the stored half.
type convite struct {
	hash   string
	id     string
	expira time.Time
}

var convites = map[string]convite{}

// Convidar creates the person, inactive and with no password, and the
// invitation that lets them set one.
func (s *Store) Convidar(id, email, nome, papel string) (Convite, error) {
	chave := normaliza(email)
	if chave == "" {
		return Convite{}, errors.New("usuarios: an e-mail is required")
	}
	s.mu.Lock()
	if _, ok := s.rows[chave]; ok {
		s.mu.Unlock()
		return Convite{}, errors.New("usuarios: that e-mail is already in the table")
	}
	u := Usuario{ID: id, Email: email, Nome: nome, Papel: papel, Ativo: false, Criado: time.Now()}
	s.rows[chave] = u
	s.mu.Unlock()
	return s.emitir(u)
}

// Resetar issues another invitation for somebody who is already here. It is
// the same path as the first password because there is only one way for a
// password to be born.
func (s *Store) Resetar(id string) (Convite, error) {
	u, ok := s.porID(id)
	if !ok {
		return Convite{}, errors.New("usuarios: nobody with that id")
	}
	return s.emitir(u)
}

// emitir mints the token and keeps its hash. A dump of what is stored — memory
// today, a table tomorrow — is not a set of keys.
func (s *Store) emitir(u Usuario) (Convite, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return Convite{}, err
	}
	token := hex.EncodeToString(b)
	expira := time.Now().Add(Validade)
	s.mu.Lock()
	defer s.mu.Unlock()
	// One live invitation per person: the previous link stops working the
	// moment a new one is handed out.
	for k, c := range convites {
		if c.id == u.ID {
			delete(convites, k)
		}
	}
	convites[hashToken(token)] = convite{hash: hashToken(token), id: u.ID, expira: expira}
	return Convite{Token: token, Expira: expira, Usuario: u}, nil
}

// Convidado is who a token belongs to, while it is still worth something.
func (s *Store) Convidado(token string) (Usuario, error) {
	s.mu.RLock()
	c, ok := convites[hashToken(token)]
	s.mu.RUnlock()
	if !ok || time.Now().After(c.expira) {
		return Usuario{}, ErrConvite
	}
	u, achou := s.porID(c.id)
	if !achou {
		return Usuario{}, ErrConvite
	}
	return u, nil
}

// Definir sets the password the person chose, activates them, and spends the
// token. Used once: a link that still works after it worked is a link somebody
// can find later.
func (s *Store) Definir(token, senha string) error {
	u, err := s.Convidado(token)
	if err != nil {
		return err
	}
	if len(senha) < 12 {
		return errors.New("usuarios: a password of at least 12 characters")
	}
	hash, err := auth.HashPBKDF2(senha)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	chave := normaliza(u.Email)
	linha := s.rows[chave]
	linha.Hash, linha.Ativo = hash, true
	s.rows[chave] = linha
	delete(convites, hashToken(token))
	return nil
}

// Papel changes what somebody may do.
func (s *Store) Papel(id, papel string) error {
	return s.altera(id, func(u *Usuario) { u.Papel = papel })
}

// Ativar turns somebody on or off. Off and not deleted: the audit trail points
// at who did what, and a deleted row leaves the trail talking about an id that
// is not there any more.
func (s *Store) Ativar(id string, ativo bool) error {
	return s.altera(id, func(u *Usuario) { u.Ativo = ativo })
}

func (s *Store) altera(id string, fn func(*Usuario)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, u := range s.rows {
		if u.ID == id {
			fn(&u)
			s.rows[k] = u
			return nil
		}
	}
	return errors.New("usuarios: nobody with that id")
}

func (s *Store) porID(id string) (Usuario, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, u := range s.rows {
		if u.ID == id {
			return u, true
		}
	}
	return Usuario{}, false
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
`

const usersInvitesTest = `package usuarios

import (
	"strings"
	"testing"
	"time"
)

func loja() *Store { return &Store{rows: map[string]Usuario{}} }

// Convidar cria a pessoa sem senha, e é o link que a define. Um administrador
// que escolhe a senha de alguém é um administrador que sabe a senha de alguém.
func TestConviteDefineASenhaEAtiva(t *testing.T) {
	s := loja()
	conv, err := s.Convidar("u-2", "bia@exemplo.com", "Bia", "leitor")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Verify("bia@exemplo.com", ""); err != ErrCredencial {
		t.Fatal("uma pessoa convidada entrou antes de ter senha")
	}
	if err := s.Definir(conv.Token, "uma-senha-longa-o-bastante"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Verify("bia@exemplo.com", "uma-senha-longa-o-bastante"); err != nil {
		t.Fatalf("a senha que a pessoa escolheu não entra: %v", err)
	}
	// Uma vez só: um link que continua valendo depois de usado é um link que
	// alguém acha depois.
	if err := s.Definir(conv.Token, "outra-senha-bem-longa"); err == nil {
		t.Fatal("o convite valeu duas vezes")
	}
	// E o que fica guardado é o hash do token, não ele.
	for k := range convites {
		if strings.Contains(k, conv.Token) {
			t.Fatal("o token ficou guardado como veio")
		}
	}
}

func TestConviteVence(t *testing.T) {
	s := loja()
	conv, err := s.Convidar("u-3", "caio@exemplo.com", "Caio", "leitor")
	if err != nil {
		t.Fatal(err)
	}
	// Envelhece o convite sem esperar dois dias.
	for k, c := range convites {
		c.expira = time.Now().Add(-time.Minute)
		convites[k] = c
	}
	if _, err := s.Convidado(conv.Token); err != ErrConvite {
		t.Fatalf("um convite vencido ainda vale: %v", err)
	}
}

// Desativar não apaga: a trilha de auditoria aponta para quem fez o quê, e uma
// linha apagada deixa a trilha falando de um id que não existe mais.
func TestDesativarImpedeEntrarEMantemALinha(t *testing.T) {
	s := loja()
	if err := s.Add("u-1", "ana@exemplo.com", "Ana", "admin", "uma-senha-boa"); err != nil {
		t.Fatal(err)
	}
	if err := s.Ativar("u-1", false); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Verify("ana@exemplo.com", "uma-senha-boa"); err != ErrCredencial {
		t.Fatal("quem foi desativado continuou entrando")
	}
	if len(s.All()) != 1 {
		t.Fatal("a linha sumiu")
	}
}
`

const usersPage = `// Package usuarios is the screen that names people: who is in, with what
// role, and how somebody new gets a password.
//
// The folder requires a role — see middleware.go — because everything here is
// about somebody else's access.
package usuarios

import (
	"net/http"
	"strconv"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"

	gente "{{.Module}}/internal/usuarios"
)

// Papeis is what this application has. It is a list and not a free field
// because a role nobody wrote down is a role nobody guards.
var Papeis = []string{"admin", "editor", "leitor"}

// Page lists the people at GET {{.URL}}usuarios.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("{{.T.users_title}}")
	store := trilha.Use[*gente.Store](c)
	corpo := []h.Node{
		ui.PageHeader("{{.T.users_title}}"),
		ui.Muted(h.Text("{{.T.users_desc}}")),
	}
	// The link crosses the redirect once, in a flash: it is shown to whoever
	// just created it, and it is gone from the next render.
	for _, f := range c.Flashes() {
		if f.Kind == "convite" {
			corpo = append(corpo, ui.Alert("{{.T.users_invite_link}}",
				ui.AlertDescription(ui.Code(f.Text))))
		}
	}
	return h.Div(append(corpo, convidar(c), tabela(c, store.All()))...), nil
}

// POST does the four things this screen does, dispatched by the form's action
// field: they are one screen, and a form that posts to itself comes back to
// itself when something is wrong.
func POST(c *trilha.Ctx) error {
	store := trilha.Use[*gente.Store](c)
	switch c.Form("acao") {
	case "papel":
		if err := store.Papel(c.Form("id"), papelValido(c.Form("papel"))); err != nil {
			return err
		}
	case "ativo":
		ativo, _ := strconv.ParseBool(c.Form("ativo"))
		if err := store.Ativar(c.Form("id"), ativo); err != nil {
			return err
		}
	case "resetar":
		conv, err := store.Resetar(c.Form("id"))
		if err != nil {
			return err
		}
		c.Flash("convite", "/convite/"+conv.Token)
		c.Flash(ui.FlashSuccess, "{{.T.users_reset_done}}")
	default:
		email := c.Form("email")
		if email == "" {
			return trilha.Errorf(http.StatusUnprocessableEntity, "%s", "{{.T.users_need_email}}")
		}
		conv, err := store.Convidar("u-"+strconv.FormatInt(time.Now().UnixNano(), 36),
			email, c.Form("nome"), papelValido(c.Form("papel")))
		if err != nil {
			return trilha.Errorf(http.StatusUnprocessableEntity, "%s", err.Error())
		}
		c.Flash("convite", "/convite/"+conv.Token)
		c.Flash(ui.FlashSuccess, "{{.T.users_invited}}")
	}
	return c.Redirect("{{.URL}}usuarios")
}

// papelValido keeps what comes from the form inside the list above: a role
// that arrives from outside is a role nobody declared.
func papelValido(p string) string {
	for _, v := range Papeis {
		if v == p {
			return p
		}
	}
	return Papeis[len(Papeis)-1]
}

func convidar(c *trilha.Ctx) h.Node {
	return h.Form(h.Method("post"), h.Action("{{.URL}}usuarios"), h.Class("ui-stack"),
		trilha.CSRFInput(c),
		h.Input(h.Type("hidden"), h.Name("acao"), h.Value("convidar")),
		ui.Field("email", "{{.T.users_email}}", ui.Input(h.ID("email"), h.Name("email"),
			h.Type("email"), h.Required())),
		ui.Field("nome", "{{.T.users_name}}", ui.Input(h.ID("nome"), h.Name("nome"))),
		ui.Field("papel", "{{.T.users_role}}", ui.Select(h.ID("papel"), h.Name("papel"),
			ui.SelectOptions(opcoes(), Papeis[len(Papeis)-1]))),
		ui.Button(h.Type("submit"), h.Text("{{.T.users_invite}}")),
	)
}

func opcoes() []ui.Option {
	out := make([]ui.Option, 0, len(Papeis))
	for _, p := range Papeis {
		out = append(out, ui.Option{Value: p, Label: p})
	}
	return out
}

func tabela(c *trilha.Ctx, pessoas []gente.Usuario) h.Node {
	linhas := make([]h.Node, 0, len(pessoas))
	for _, u := range pessoas {
		linhas = append(linhas, h.Tr(
			h.Td(h.Text(u.Email)),
			h.Td(h.Text(u.Nome)),
			h.Td(papelForm(c, u)),
			h.Td(estado(u)),
			h.Td(ui.Date(c, u.Criado)),
			h.Td(h.Div(h.Class("ui-inline-form"), ativarForm(c, u), resetarForm(c, u))),
		))
	}
	if len(linhas) == 0 {
		return ui.Empty(ui.EmptyOpts{Icon: "user", Title: "{{.T.users_none}}"})
	}
	return ui.Table(
		h.Thead(h.Tr(
			h.Th(h.Text("{{.T.users_email}}")), h.Th(h.Text("{{.T.users_name}}")),
			h.Th(h.Text("{{.T.users_role}}")), h.Th(h.Text("{{.T.users_state}}")),
			h.Th(h.Text("{{.T.users_created}}")), h.Th(h.Text("")),
		)),
		h.Tbody(linhas...),
	)
}

func estado(u gente.Usuario) h.Node {
	if u.Ativo {
		return ui.Badge(h.Text("{{.T.users_active}}"))
	}
	return ui.Badge(ui.Outline(), h.Text("{{.T.users_inactive}}"))
}

func papelForm(c *trilha.Ctx, u gente.Usuario) h.Node {
	return h.Form(h.Method("post"), h.Action("{{.URL}}usuarios"), h.Class("ui-inline-form"),
		trilha.CSRFInput(c),
		h.Input(h.Type("hidden"), h.Name("acao"), h.Value("papel")),
		h.Input(h.Type("hidden"), h.Name("id"), h.Value(u.ID)),
		ui.Select(h.Name("papel"), h.Aria("label", "{{.T.users_role}}"), ui.SelectOptions(opcoes(), u.Papel)),
		ui.Button(ui.Outline(), ui.Sm(), h.Type("submit"), h.Text("{{.T.users_save}}")),
	)
}

func ativarForm(c *trilha.Ctx, u gente.Usuario) h.Node {
	rotulo, valor := "{{.T.users_deactivate}}", "false"
	if !u.Ativo {
		rotulo, valor = "{{.T.users_activate}}", "true"
	}
	return h.Form(h.Method("post"), h.Action("{{.URL}}usuarios"), h.Class("ui-inline-form"),
		trilha.CSRFInput(c),
		h.Input(h.Type("hidden"), h.Name("acao"), h.Value("ativo")),
		h.Input(h.Type("hidden"), h.Name("id"), h.Value(u.ID)),
		h.Input(h.Type("hidden"), h.Name("ativo"), h.Value(valor)),
		ui.Button(ui.Outline(), ui.Sm(), h.Type("submit"), h.Text(rotulo)),
	)
}

func resetarForm(c *trilha.Ctx, u gente.Usuario) h.Node {
	return h.Form(h.Method("post"), h.Action("{{.URL}}usuarios"), h.Class("ui-inline-form"),
		trilha.CSRFInput(c),
		h.Input(h.Type("hidden"), h.Name("acao"), h.Value("resetar")),
		h.Input(h.Type("hidden"), h.Name("id"), h.Value(u.ID)),
		ui.Button(ui.Outline(), ui.Sm(), h.Type("submit"), h.Text("{{.T.users_reset}}"),
			ui.Confirm("{{.T.users_reset_ask}}", "{{.T.users_reset_desc}}")),
	)
}
`

const usersMiddleware = `package usuarios

import (
	"github.com/emersonjoe/trilha"

	"{{.Module}}/internal/sessao"
)

// exige is the rule, and Middleware is what the scanner reads: middleware.go
// has to export a function with that signature, and a var of the right type is
// not one.
var exige = sessao.Flow.RequireRole("admin")

// Middleware guards this folder. Somebody signed in without the role gets 403
// and not a redirect to the login: they are known, just not permitted, and
// sending them back to a login they already passed is a loop with no exit.
func Middleware(c *trilha.Ctx, next trilha.Next) error { return exige(c, next) }
`

const usersAccept = `// Package convite is where an invited person sets their own password.
//
// It is deliberately outside the folder the users screen lives in: whoever
// opens this link has no session yet, and a page that requires one would send
// them to a login they cannot pass.
package convite

import (
	"net/http"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"

	gente "{{.Module}}/internal/usuarios"
)

// Page renders GET /convite/{token}.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("{{.T.invite_title}}")
	u, err := trilha.Use[*gente.Store](c).Convidado(c.Param("token"))
	if err != nil {
		return nil, trilha.Errorf(http.StatusNotFound, "%s", "{{.T.invite_invalid}}")
	}
	return formulario(c, u.Email, ""), nil
}

// POST sets the password and activates the person, then sends them to the
// login: the session is opened by signing in, once, like everybody else's.
func POST(c *trilha.Ctx) error {
	store := trilha.Use[*gente.Store](c)
	u, err := store.Convidado(c.Param("token"))
	if err != nil {
		return trilha.Errorf(http.StatusNotFound, "%s", "{{.T.invite_invalid}}")
	}
	if err := store.Definir(c.Param("token"), c.Form("password")); err != nil {
		return c.Render(http.StatusUnprocessableEntity, formulario(c, u.Email, "{{.T.invite_short}}"))
	}
	return c.Redirect("{{.URL}}entrar")
}

func formulario(c *trilha.Ctx, email, erro string) h.Node {
	return ui.Stack(ui.Card(
		ui.CardHeader(ui.CardTitle("{{.T.invite_title}}"), ui.CardDescription(email)),
		ui.CardContent(h.Form(h.Method("post"), h.Action(c.Request().URL.Path), h.Class("ui-stack"),
			trilha.CSRFInput(c),
			h.If(erro != "", ui.Alert(erro, ui.Destructive(), ui.Icon("triangle-alert"))),
			ui.Field("password", "{{.T.invite_password}}", ui.Input(h.ID("password"),
				h.Name("password"), h.Type("password"), h.Autocomplete("new-password"), h.Required())),
			h.Div(ui.Submit(h.Text("{{.T.invite_submit}}"))),
		)),
	))
}
`

// usersTest walks the whole thing from the outside, in the project that
// received it: the folder refuses whoever is not an administrator, an
// invitation creates somebody with no password, and the link they get is what
// gives them one.
const usersTest = `package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha"
)

func TestConvidarEEntrar(t *testing.T) {
	t.Setenv("TRILHA_ENV", "prod")
	t.Setenv("TRILHA_SECRET", "a-test-secret-with-more-than-32-bytes!!")
	t.Setenv("ADMIN_EMAIL", "admin@example.com")
	t.Setenv("ADMIN_PASSWORD", "a-password-nobody-guesses")
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	a := newApp()
	c := trilha.NewTestClient(t, a)

	// Closed to whoever is not signed in. A browser is sent to the login; an
	// API call would get 401, because redirecting a fetch to an HTML form only
	// produces a confusing parse error.
	c.Get("{{.URL}}usuarios", trilha.WithHeader("Accept", "text/html")).WantStatus(http.StatusFound)
	c.Get("{{.URL}}usuarios").WantStatus(http.StatusUnauthorized)

	c.PostForm("{{.URL}}entrar", url.Values{
		"email":    {"admin@example.com"},
		"password": {"a-password-nobody-guesses"},
	}).WantStatus(http.StatusSeeOther)
	c.Get("{{.URL}}usuarios").WantStatus(http.StatusOK).WantContains("admin@example.com")

	// Inviting hands out a link and creates nobody who can already sign in.
	c.PostForm("{{.URL}}usuarios", url.Values{
		"acao":  {"convidar"},
		"email": {"bia@example.com"},
		"nome":  {"Bia"},
		"papel": {"leitor"},
	}).WantStatus(http.StatusSeeOther)
	// O link atravessa o redirect num flash: aparece uma vez, para quem acabou
	// de criá-lo.
	corpo := c.Get("{{.URL}}usuarios").WantStatus(http.StatusOK).Body.String()
	link := depoisDeConvite(corpo)
	if link == "" {
		t.Fatalf("a tela não mostrou o link do convite:\n%s", corpo)
	}

	// The link answers with no session at all: it is for somebody who has none.
	// The same application, a second visitor — a new app would be a new table,
	// and the invitation lives in the one that issued it.
	sem := trilha.NewTestClient(t, a)
	sem.Get(link).WantStatus(http.StatusOK)
	sem.PostForm(link, url.Values{"password": {"short"}}).WantStatus(http.StatusUnprocessableEntity)

	// The password the person chose is the one that gets them in — nobody else
	// ever knew it.
	sem.PostForm(link, url.Values{"password": {"uma-senha-longa-o-bastante"}}).WantStatus(http.StatusSeeOther)
	nova := trilha.NewTestClient(t, a)
	nova.PostForm("{{.URL}}entrar", url.Values{
		"email":    {"bia@example.com"},
		"password": {"uma-senha-longa-o-bastante"},
	}).WantStatus(http.StatusSeeOther)

	// And the link is spent: one use, so a link found later is not a way in.
	sem.PostForm(link, url.Values{"password": {"outra-senha-bem-longa"}}).WantStatus(http.StatusNotFound)
}

// depoisDeConvite reads the invitation link out of the page: it is the only
// /convite/ in it.
func depoisDeConvite(corpo string) string {
	i := strings.Index(corpo, "/convite/")
	if i < 0 {
		return ""
	}
	resto := corpo[i:]
	for j, r := range resto {
		if r == '<' || r == '"' || r == ' ' {
			return resto[:j]
		}
	}
	return resto
}
`
