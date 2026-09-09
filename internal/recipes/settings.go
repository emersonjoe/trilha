package recipes

// settingsRecipe is the section somebody edits on a screen instead of in the
// environment.
//
// The line it replaces is an environment variable that only a deploy can
// change — and the thing about "how many days before a document expires" is
// that the person who knows the answer is not the person with the deploy.
func settingsRecipe() Recipe {
	return Recipe{
		Name: "settings",
		Summary: map[string]string{
			"en": "a settings section somebody edits on a screen instead of in the environment",
			"pt": "uma seção de configurações que alguém edita numa tela, e não no ambiente",
		},
		Doc: "/reference/app",
		Files: []File{
			{Rel: "internal/config/config.go", Go: true, Body: settingsDecl},
			{Rel: "{{.At}}config/page.go", Go: true, Body: settingsPage},
		},
		Setup: []Insert{{
			Marker: "// trilha:add settings",
			Line:   "\tif err := config.Secao.Bind(a, nil); err != nil {\n\t\treturn err\n\t}\n",
		}},
		Imports: []string{"{{.Module}}/internal/config"},
		Next: map[string]string{
			"en": "Run `trilha dev` and open {{.URL}}config. Edit internal/config/config.go to say what your app actually has; the screen follows the struct.",
			"pt": "Rode `trilha dev` e abra {{.URL}}config. Edite o internal/config/config.go com o que o seu app tem de verdade; a tela segue o struct.",
		},
	}
}

const settingsDecl = `// Package config is what this application lets somebody change without a
// deploy.
//
// The rule for what belongs here: if the person who knows the right value is
// not the person who can deploy, it is a setting. If changing it needs a code
// review, it is not.
package config

import "github.com/emersonjoe/trilha"

// Geral is one section. The struct is the screen: the labels come from the
// tags, the controls from the types, and the validation from the same rules a
// form uses — one declaration, and nothing to keep in sync.
type Geral struct {
	Nome    string ` + "`form:\"nome\" label:\"{{.T.settings_name}}\" validate:\"required,max=60\"`" + `
	Suporte string ` + "`form:\"suporte\" label:\"{{.T.settings_support}}\" validate:\"required,email\"`" + `
	Dias    int    ` + "`form:\"dias\" label:\"{{.T.settings_days}}\" validate:\"min=1,max=365\"`" + `
}

// Secao is the section itself, with the values it starts with. Those defaults
// are what the app runs on before anybody opens the screen, so they have to be
// a working configuration and not zeroes — the app answers with them from the
// first request, before anybody has saved anything.
var Secao = trilha.NewSettings("geral", Geral{
	Nome:    "{{.T.settings_default_name}}",
	Suporte: "suporte@exemplo.com",
	Dias:    30,
})
`

const settingsPage = `// Package config is the screen of one settings section, drawn from the struct
// that declares it.
package config

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"

	"{{.Module}}/internal/config"
)

// Page draws the form at GET {{.URL}}config.
//
// Guard this folder: a settings screen changes how the application behaves for
// everybody, which is the definition of something not everybody should reach.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("{{.T.settings_title}}")
	return ui.SettingsForm(c, config.Secao, nil), nil
}

// POST saves it. Update reads the form, validates it against the same tags the
// screen was drawn from, and answers 422 with the messages in the fields.
func POST(c *trilha.Ctx) error {
	if err := config.Secao.Update(c); err != nil {
		return err
	}
	c.Flash(ui.FlashSuccess, "{{.T.settings_saved}}")
	return c.Redirect("{{.URL}}config")
}
`
