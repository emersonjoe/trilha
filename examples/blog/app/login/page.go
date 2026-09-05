package login

import (
	"net/http"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// Page renders the login form.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Entrar")
	return h.Fragment(
		h.H1(h.Text("Entrar")),
		h.P(h.Small(h.Text("Usuário admin, senha trilha."))),
		h.Form(h.Method("post"), h.Action("/login"), trilha.CSRFInput(c),
			h.Input(h.Type("hidden"), h.Name("next"), h.Value(c.Query("next"))),
			h.Label(h.For("u"), h.Text("Usuário")), h.Input(h.ID("u"), h.Name("usuario")),
			h.Label(h.For("p"), h.Text("Senha")), h.Input(h.ID("p"), h.Name("senha"), h.Type("password")),
			h.Button(h.Type("submit"), h.Text("Entrar")),
		),
	), nil
}

// POST authenticates (or signs out with sair=1) and redirects.
func POST(c *trilha.Ctx) error {
	if c.Form("sair") == "1" {
		c.SetCookie(&http.Cookie{Name: "session", Value: "", Path: "/", MaxAge: -1})
		return c.Redirect("/")
	}
	if c.Form("usuario") != "admin" || c.Form("senha") != "trilha" {
		return trilha.Errorf(http.StatusUnauthorized, "usuário ou senha inválidos")
	}
	c.SetCookie(&http.Cookie{Name: "session", Value: "ok", Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode})
	next := c.Form("next")
	if next == "" || next[0] != '/' {
		next = "/admin"
	}
	return c.Redirect(next)
}
