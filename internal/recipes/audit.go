package recipes

// auditRecipe is the trail of who did what, and the screen that reads it.
//
// Without a sink, c.Audit writes to the log — enough to grep and enough to
// ship a first version with. This is the second version: a place the records
// live and a screen somebody can be sent to when they ask "who deleted this?".
func auditRecipe() Recipe {
	return Recipe{
		Name: "audit",
		Summary: map[string]string{
			"en": "the trail of who did what, and the screen that reads it",
			"pt": "a trilha de quem fez o quê, e a tela que a lê",
		},
		Doc: "/reference/observability",
		Files: []File{
			{Rel: "internal/auditoria/store.go", Go: true, Body: auditStore},
			{Rel: "{{.At}}auditoria/page.go", Go: true, Body: auditPage},
		},
		Setup: []Insert{{
			Marker: "// trilha:add audit",
			Line:   "\ta.Config().Audit = auditoria.Store\n",
		}},
		Imports: []string{"{{.Module}}/internal/auditoria"},
		Next: map[string]string{
			"en": "Run `trilha dev` and open {{.URL}}auditoria. Guard it: the trail says who did what, and that is not for everybody.",
			"pt": "Rode `trilha dev` e abra {{.URL}}auditoria. Guarde a rota: a trilha diz quem fez o quê, e isso não é para todo mundo.",
		},
	}
}

const auditStore = `// Package auditoria keeps the trail of who did what.
//
// It is memory here, which is the honest starting point: the records last as
// long as the process, and the screen works from the first request. A real one
// writes a table — the interface is one method, and the screen does not change.
package auditoria

import (
	"sync"

	"github.com/emersonjoe/trilha"
)

// Store is what Config.Audit receives. Every c.Audit in the application ends
// up here, with the actor, the address and the route already filled in.
var Store = &memoria{}

// Max is how many records are kept. A trail that grows without a limit is a
// process that runs out of memory on a Tuesday; a table has no such limit and
// is where this goes next.
const Max = 5000

type memoria struct {
	mu   sync.RWMutex
	rows []trilha.AuditRecord
}

// Write is the sink. An error coming back from here is logged and the request
// carries on: an audit that takes the operation down with it is worse than one
// that fails loudly.
func (m *memoria) Write(r trilha.AuditRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rows = append(m.rows, r)
	if len(m.rows) > Max {
		m.rows = m.rows[len(m.rows)-Max:]
	}
	return nil
}

// All is the trail, newest first, which is the only order anybody reads it in.
func (m *memoria) All() []trilha.AuditRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]trilha.AuditRecord, 0, len(m.rows))
	for i := len(m.rows) - 1; i >= 0; i-- {
		out = append(out, m.rows[i])
	}
	return out
}
`

const auditPage = `// Package auditoria is the screen that reads the trail: who did what, to
// what, from where.
//
// It is the answer to a question somebody asks months later, and the reason it
// is a screen and not a grep is that the person asking is usually not the
// person who can grep.
package auditoria

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"

	"{{.Module}}/internal/auditoria"
)

// Page draws the trail at GET {{.URL}}auditoria.
//
// Guard this folder: add a middleware.go with the rule your app uses. The
// trail names people and what they did, which is not something to leave open.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("{{.T.audit_title}}")
	return h.Div(
		ui.PageHeader("{{.T.audit_title}}"),
		ui.Muted(h.Text("{{.T.audit_desc}}")),
		ui.AuditTable(c, auditoria.Store.All()),
	), nil
}
`
