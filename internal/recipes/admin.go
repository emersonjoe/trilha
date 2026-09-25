package recipes

// adminRecipe is the backoffice: app/admin/ behind a door that only opens for
// the admin role, and the four screens every backoffice has — people and
// their roles, the trail of who did what, the decisions waiting for somebody,
// and search.
//
// It is made of recipes, not of copies of them (spec 162). users, audit,
// approvals and search already exist, already have tests and already age
// with the framework; Includes applies them under app/admin/, and what this
// recipe adds is the part none of them has: the door, the landing screen,
// and the tests that hold the door shut for everybody but an administrator
// and hold every decision to the trail.
func adminRecipe() Recipe {
	return Recipe{
		Name:        "admin",
		CtxPackCost: 104,
		Summary: map[string]string{
			"en": "the backoffice: app/admin/ deny-by-default for the admin role, with users and roles, the audit trail, the approvals inbox and search",
			"pt": "o backoffice: app/admin/ nega por padrão a quem não é admin, com usuários e papéis, a trilha de auditoria, a caixa de aprovações e a busca",
		},
		Doc:      "/cookbook/backoffice",
		Needs:    []Need{{Recipe: "login", File: "internal/sessao/sessao.go"}},
		At:       "app/admin/",
		Includes: []string{"users", "audit", "approvals", "search"},
		// Its own files have fixed paths: the door is app/admin/ by
		// definition, and a fixed path is also how `trilha ctx` recognises
		// the recipe in a project, since it writes no line into setup.go.
		Files: []File{
			{Rel: "app/admin/middleware.go", Go: true, Body: adminMiddleware},
			{Rel: "app/admin/page.go", Go: true, Body: adminPage},
			{Rel: "admin_test.go", Go: true, Body: adminTest},
		},
		Next: map[string]string{
			"en": "Sign in as ADMIN_EMAIL and open /admin. Everything under app/admin/ answers only to the admin role — " +
				"a screen you write there tomorrow included. If the project already had users, audit, approvals or " +
				"search at the root, their screens now exist under /admin too: delete the old folders to keep one door.",
			"pt": "Entre como ADMIN_EMAIL e abra /admin. Tudo sob app/admin/ só responde ao papel admin — inclusive " +
				"a tela que você escrever ali amanhã. Se o projeto já tinha users, audit, approvals ou search na raiz, " +
				"as telas agora existem também sob /admin: apague as pastas antigas para ficar com uma porta só.",
		},
	}
}

const adminMiddleware = `// Package admin holds the screens that administer the application: people and
// their roles, the audit trail, the decisions waiting, search.
//
// They are here and not next to the others because of what they are. The
// trail names people and what they did; the users screen changes who may do
// what. Leaving them behind the same door as a listing of items would be
// handing out the key with the door.
package admin

import (
	"github.com/emersonjoe/trilha"

	"{{.Module}}/internal/sessao"
)

// exige is the rule, and Middleware is what the scanner reads: middleware.go
// has to export a function with that signature, and a var of the right type
// is not one.
var exige = sessao.Flow.RequireRole("admin")

// Middleware guards this folder and everything below it — the screens written
// tomorrow included. That is what deny by default means here: a new screen is
// closed until somebody opens it on purpose, not open until somebody
// remembers to close it.
//
// Somebody signed in without the role gets 403 and not a redirect to the
// login: they are known, just not permitted, and sending them back to a login
// they already passed is a loop with no exit.
func Middleware(c *trilha.Ctx, next trilha.Next) error { return exige(c, next) }
`

const adminPage = `package admin

import (
	"strconv"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/approval"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Page is the landing screen of /admin: the four doors, and how many
// decisions are waiting for whoever is reading.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("{{.T.admin_title}}")
	pendentes, err := trilha.Use[*approval.Approvals](c).Inbox(c, approval.ListParams{})
	if err != nil {
		return nil, err
	}
	cartao := func(href, titulo, texto string, extra ...h.Node) h.Node {
		return ui.Card(
			ui.CardHeader(ui.CardTitle(titulo), ui.CardDescription(texto)),
			ui.CardContent(append(extra, ui.ButtonLink(href, ui.Outline(), h.Text("{{.T.admin_open}}")))...),
		)
	}
	return ui.Stack(
		ui.PageHeader("{{.T.admin_title}}"),
		ui.Muted(h.Text("{{.T.admin_desc}}")),
		ui.Grid(ui.Cols(1, 2),
			cartao("/admin/usuarios", "{{.T.admin_users}}", "{{.T.admin_users_desc}}"),
			cartao("/admin/auditoria", "{{.T.admin_audit}}", "{{.T.admin_audit_desc}}"),
			cartao("/admin/aprovacoes", "{{.T.admin_approvals}}", "{{.T.admin_approvals_desc}}",
				ui.Stat("{{.T.admin_pending}}", strconv.Itoa(len(pendentes)))),
			cartao("/admin/busca", "{{.T.admin_search}}", "{{.T.admin_search_desc}}"),
		),
	), nil
}
`

// adminTest is the backoffice proving itself in the project: the door refuses
// everybody but the admin role on every screen (a table, so a screen added to
// the list is a row), and the decisions — a role changed, an account
// deactivated, a request approved — are in the trail with who and to whom.
const adminTest = `package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/approval"

	"{{.Module}}/internal/auditoria"
	"{{.Module}}/internal/usuarios"
)

// telasDoAdmin is every screen under /admin. A screen written there tomorrow
// is one more row, and the table says who may open it.
var telasDoAdmin = []string{"/admin", "/admin/usuarios", "/admin/auditoria", "/admin/aprovacoes", "/admin/busca"}

func appDoAdmin(t *testing.T) *trilha.App {
	t.Helper()
	t.Setenv("TRILHA_ENV", "prod")
	t.Setenv("TRILHA_SECRET", "a-test-secret-with-more-than-32-bytes!!")
	t.Setenv("ADMIN_EMAIL", "admin@example.com")
	t.Setenv("ADMIN_PASSWORD", "a-password-nobody-guesses")
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	a := newApp()
	if err := trilha.Use[*usuarios.Store](a).Add("u-leitor", "leitor@example.com", "Leitor", "leitor",
		"a-password-for-the-reader"); err != nil {
		t.Fatal(err)
	}
	return a
}

func entrarNoAdmin(t *testing.T, a *trilha.App, email, senha string) *trilha.TestClient {
	t.Helper()
	c := trilha.NewTestClient(t, a)
	c.PostForm("/entrar", url.Values{"email": {email}, "password": {senha}}).WantStatus(http.StatusSeeOther)
	return c
}

// A porta nega por padrão: anônimo não entra, quem entrou sem o papel recebe
// 403 — e não um login de novo —, e só o admin vê as telas.
func TestAdminDefaultDeny(t *testing.T) {
	a := appDoAdmin(t)
	quem := []struct {
		nome   string
		client *trilha.TestClient
		status int
	}{
		{"anônimo", trilha.NewTestClient(t, a), http.StatusUnauthorized},
		{"leitor", entrarNoAdmin(t, a, "leitor@example.com", "a-password-for-the-reader"), http.StatusForbidden},
		{"admin", entrarNoAdmin(t, a, "admin@example.com", "a-password-nobody-guesses"), http.StatusOK},
	}
	for _, q := range quem {
		for _, tela := range telasDoAdmin {
			if got := q.client.Get(tela).Code; got != q.status {
				t.Errorf("%s em %s: %d, esperava %d", q.nome, tela, got, q.status)
			}
		}
	}
	// O que o admin vê é lido como o navegador recebe: todo formulário que
	// escreve leva o token, todo script inline o nonce, e os cookies são HttpOnly.
	for _, tela := range telasDoAdmin {
		snap := quem[2].client.Get(tela).Snapshot()
		for _, err := range []error{snap.HasCSRFToken(), snap.HasCSPNonce(), snap.HasSafeCookies()} {
			if err != nil {
				t.Errorf("%s: %v", tela, err)
			}
		}
	}
	// E escrever também: um POST do leitor não passa da porta.
	quem[1].client.PostForm("/admin/usuarios", url.Values{"acao": {"papel"}, "id": {"u-leitor"}, "papel": {"admin"}}).
		WantStatus(http.StatusForbidden)
	for _, u := range trilha.Use[*usuarios.Store](a).All() {
		if u.ID == "u-leitor" && u.Papel != "leitor" {
			t.Fatalf("o leitor se deu o papel %q", u.Papel)
		}
	}
}

// Trocar papel e desativar vão para a trilha com quem decidiu e sobre quem —
// e a tela da trilha mostra.
func TestAdminAuditTrail(t *testing.T) {
	a := appDoAdmin(t)
	admin := entrarNoAdmin(t, a, "admin@example.com", "a-password-nobody-guesses")

	admin.PostForm("/admin/usuarios", url.Values{"acao": {"papel"}, "id": {"u-leitor"}, "papel": {"editor"}}).
		WantStatus(http.StatusSeeOther)
	admin.PostForm("/admin/usuarios", url.Values{"acao": {"ativo"}, "id": {"u-leitor"}, "ativo": {"false"}}).
		WantStatus(http.StatusSeeOther)

	achou := map[string]bool{}
	for _, r := range auditoria.Store.All() {
		if r.Target == "u-leitor" && r.Actor.Email == "admin@example.com" {
			achou[r.Action] = true
		}
	}
	for _, quero := range []string{"usuario.papel", "usuario.ativo"} {
		if !achou[quero] {
			t.Errorf("a trilha não tem %s do admin sobre u-leitor: %v", quero, achou)
		}
	}
	tela := admin.Get("/admin/auditoria").WantStatus(http.StatusOK).Body.String()
	if !strings.Contains(tela, "usuario.papel") || !strings.Contains(tela, "u-leitor") {
		t.Fatalf("a tela da trilha não mostra a troca de papel:\n%s", tela)
	}
}

// Um pedido aberto aparece na caixa do admin, a decisão é gravada e vai para
// a trilha com quem decidiu.
func TestAdminApprovalFlow(t *testing.T) {
	a := appDoAdmin(t)
	admin := entrarNoAdmin(t, a, "admin@example.com", "a-password-nobody-guesses")

	var id string
	a.Register(trilha.Route{Pattern: "/_teste/pedir", Kind: trilha.KindAPI,
		Methods: map[string]trilha.HandlerFunc{
			"POST": func(c *trilha.Ctx) error {
				var err error
				id, err = trilha.Use[*approval.Approvals](c).Open(c, approval.Request{
					Kind: "exemplo", Subject: "Estorno de 49,00", Assign: approval.Role("admin"),
				})
				if err != nil {
					return err
				}
				return c.Text(http.StatusOK, id)
			},
		}})
	admin.PostForm("/_teste/pedir", url.Values{}).WantStatus(http.StatusOK)

	admin.Get("/admin").WantStatus(http.StatusOK).WantContains("1")
	admin.Get("/admin/aprovacoes").WantStatus(http.StatusOK).WantContains("Estorno de 49,00")
	admin.PostForm("/admin/aprovacoes", url.Values{"id": {id}, "decision": {approval.Approved},
		"reason": {"conferido"}}).WantStatus(http.StatusSeeOther)

	decidiu := false
	for _, r := range auditoria.Store.All() {
		if r.Action == "approval.decide" && r.Target == id && r.Actor.Email == "admin@example.com" {
			decidiu = true
		}
	}
	if !decidiu {
		t.Fatalf("a decisão não foi para a trilha com o ator: %+v", auditoria.Store.All())
	}
}
`
