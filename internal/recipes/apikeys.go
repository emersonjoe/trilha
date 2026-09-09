package recipes

// apiKeysRecipe is keys for the API: issue, revoke, and the middleware that
// requires one.
//
// It is the pattern a project reaches the week after somebody asks "can our
// other system call this?" — and the week after that is when somebody notices
// the key was stored in the clear.
func apiKeysRecipe() Recipe {
	return Recipe{
		Name: "api-keys",
		Summary: map[string]string{
			"en": "keys for the API: issue, revoke, and the middleware that requires one",
			"pt": "chaves para a API: emitir, revogar, e o middleware que exige uma",
		},
		Doc: "/reference/auth",
		Files: []File{
			{Rel: "app/chaves/page.go", Go: true, Body: keysPage},
		},
		Setup: []Insert{{
			Marker: "// trilha:add api-keys",
			Line:   "\ttrilha.Provide(a, chaves.Keys)\n",
		}},
		Imports: []string{"{{.Module}}/app/chaves"},
		Next: map[string]string{
			"en": "Run `trilha dev` and open /chaves. Guard that folder, and put `chaves.Keys.Require(\"read\")` on the API branch the keys are for.",
			"pt": "Rode `trilha dev` e abra /chaves. Guarde essa pasta, e ponha `chaves.Keys.Require(\"read\")` no ramo de API que as chaves servem.",
		},
	}
}

const keysPage = `// Package chaves issues and revokes the API keys of this application.
//
// The key is shown once, when it is created, and never again — what is stored
// is its hash. A screen that could show it a second time would be a screen
// backed by a table where the keys are readable, and then a database backup is
// a set of credentials.
package chaves

import (
	"net/http"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/auth"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Escopos is what a key of this application may carry, declared once. Require
// refuses anything outside this list when the routes are wired, so a typo is a
// panic at boot and not a door left open.
var Escopos = []string{"read", "write"}

// Keys is the issuer. The store is memory here — the keys last as long as the
// process — and a real one is a table behind the same five methods.
var Keys = auth.APIKeys(auth.KeyOptions{
	Scopes:    Escopos,
	RateLimit: trilha.RateLimit{RPS: 10, Burst: 30},
})

// Page lists the keys at GET /chaves, and shows a new one once.
//
// Guard this folder: whoever reaches it can issue a credential for your API.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("{{.T.keys_title}}")
	todas, err := Keys.All()
	if err != nil {
		return nil, err
	}
	linhas := make([]ui.APIKeyRow, 0, len(todas))
	for _, k := range todas {
		linhas = append(linhas, ui.APIKeyRow{
			ID: k.ID, Handle: k.Handle, Name: k.Name, Scopes: k.Scopes,
			Created: k.Created, LastUsed: k.LastUsed, Revoked: !k.RevokedAt.IsZero(),
		})
	}
	corpo := []h.Node{
		ui.PageHeader("{{.T.keys_title}}"),
		ui.Muted(h.Text("{{.T.keys_desc}}")),
	}
	// The plain key crosses the redirect once, in a flash, and is gone from
	// the next render: it exists for one screen.
	for _, f := range c.Flashes() {
		if f.Kind == "key" {
			corpo = append(corpo, ui.SecretOnce(c, f.Text))
		}
	}
	corpo = append(corpo, criar(c), ui.APIKeysTable(c, linhas, ui.APIKeysOpts{
		Revoke: "/chaves", CSRF: trilha.CSRFInput(c),
	}))
	return h.Div(corpo...), nil
}

// POST issues and revokes, dispatched by the form's action field: they are one
// screen, and a form that posts to itself comes back to itself when something
// is wrong.
func POST(c *trilha.Ctx) error {
	switch c.Form("action") {
	case "revoke":
		if err := Keys.Revoke(c, c.Form("id")); err != nil {
			return err
		}
		c.Flash(ui.FlashSuccess, "{{.T.keys_revoked}}")
	default:
		nome := c.Form("nome")
		if nome == "" {
			return trilha.Errorf(http.StatusUnprocessableEntity, "%s", "{{.T.keys_need_name}}")
		}
		_, plain, err := Keys.Issue(c, nome, c.Request().Form["escopos"], 0)
		if err != nil {
			return err
		}
		c.Flash("key", plain)
		c.Flash(ui.FlashSuccess, "{{.T.keys_created}}")
	}
	return c.Redirect("/chaves")
}

func criar(c *trilha.Ctx) h.Node {
	var caixas []h.Node
	for i, e := range Escopos {
		id := "esc-" + e
		_ = i
		caixas = append(caixas, ui.CheckRow(
			ui.Checkbox(h.ID(id), h.Name("escopos"), h.Value(e)), e, id))
	}
	return h.Form(h.Method("post"), h.Action("/chaves"), h.Class("ui-stack"),
		trilha.CSRFInput(c),
		ui.Field("nome", "{{.T.keys_name}}", ui.Input(h.ID("nome"), h.Name("nome"), h.Required())),
		h.Fieldset(h.Legend(h.Text("{{.T.keys_scopes}}")), h.Div(caixas...)),
		ui.Button(h.Type("submit"), h.Text("{{.T.keys_create}}")),
	)
}
`
