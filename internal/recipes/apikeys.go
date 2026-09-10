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
			{Rel: "{{.At}}chaves/page.go", Go: true, Body: keysPage},
		},
		Setup: []Insert{{
			Marker: "// trilha:add api-keys",
			Line:   "\ttrilha.Provide(a, chaves.Keys)\n\tif err := chaves.Keys.Setup(a); err != nil {\n\t\treturn err\n\t}\n",
		}},
		Imports: []string{"{{.Module}}/{{.At}}chaves"},
		Next: map[string]string{
			"en": "Run `trilha dev` and open {{.URL}}chaves. Guard that folder, and put `chaves.Keys.Require(\"read\")` on the API branch the keys are for — that middleware is also what counts the calls the screen shows.",
			"pt": "Rode `trilha dev` e abra {{.URL}}chaves. Guarde essa pasta, e ponha `chaves.Keys.Require(\"read\")` no ramo de API que as chaves servem — é esse middleware que conta as chamadas que a tela mostra.",
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
	"time"

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
	// Usage counts the calls per key, route and day. The counting happens in
	// memory and goes to the store in batches, so what a request pays for it is
	// a map write — and Setup, in app/setup.go, is what makes the count survive
	// a deploy.
	Usage:     auth.UsageMemory(),
	UsageKeep: 400 * 24 * time.Hour,
})

// Janela is how far back the screen looks. Thirty days is the question people
// actually ask — "are they still using it?" — and not the whole history.
const Janela = 30 * 24 * time.Hour

// Page lists the keys at GET {{.URL}}chaves, and shows a new one once.
//
// Guard this folder: whoever reaches it can issue a credential for your API.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("{{.T.keys_title}}")
	todas, err := Keys.All()
	if err != nil {
		return nil, err
	}
	// The counters are read once, for every key: one query instead of one per
	// row is the difference between a screen and a screen that gets slower with
	// each key issued.
	uso, err := Keys.Usage(c.Context(), auth.UsageQuery{Since: time.Now().Add(-Janela)})
	if err != nil {
		return nil, err
	}
	chamadas := map[string]int{}
	for _, k := range uso.ByKey {
		chamadas[k.KeyID] = k.Count
	}
	linhas := make([]ui.APIKeyRow, 0, len(todas))
	for _, k := range todas {
		linhas = append(linhas, ui.APIKeyRow{
			ID: k.ID, Handle: k.Handle, Name: k.Name, Scopes: k.Scopes,
			Created: k.Created, LastUsed: k.LastUsed, Revoked: !k.RevokedAt.IsZero(),
			Calls: chamadas[k.ID],
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
		Revoke: "{{.URL}}chaves", CSRF: trilha.CSRFInput(c),
		Usage: true, UsageDays: 30,
	}), ui.H2(h.Text("{{.T.keys_usage}}")), ui.APIUsage(c, painel(uso), ui.APIUsageOpts{Days: 30}))
	return h.Div(corpo...), nil
}

// painel maps what the auth package counts to what the kit draws. The kit does
// not import auth on purpose — it draws data, not another package's types — so
// this loop is the seam, and it is four lines.
func painel(uso auth.UsageReport) ui.APIUsageData {
	d := ui.APIUsageData{Total: uso.Total, Errors: uso.Errors, Last: uso.Last}
	for _, r := range uso.ByRoute {
		d.Routes = append(d.Routes, ui.APIUsageRoute{
			Method: r.Method, Route: r.Route, Count: r.Count, Errors: r.Errors, Last: r.Last,
		})
	}
	for _, x := range uso.ByDay {
		d.Days = append(d.Days, ui.APIUsageDay{Day: x.Day, Count: x.Count, Errors: x.Errors})
	}
	return d
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
	return c.Redirect("{{.URL}}chaves")
}

func criar(c *trilha.Ctx) h.Node {
	var caixas []h.Node
	for i, e := range Escopos {
		id := "esc-" + e
		_ = i
		caixas = append(caixas, ui.CheckRow(
			ui.Checkbox(h.ID(id), h.Name("escopos"), h.Value(e)), e, id))
	}
	return h.Form(h.Method("post"), h.Action("{{.URL}}chaves"), h.Class("ui-stack"),
		trilha.CSRFInput(c),
		ui.Field("nome", "{{.T.keys_name}}", ui.Input(h.ID("nome"), h.Name("nome"), h.Required())),
		h.Fieldset(h.Legend(h.Text("{{.T.keys_scopes}}")), h.Div(caixas...)),
		ui.Button(h.Type("submit"), h.Text("{{.T.keys_create}}")),
	)
}
`
