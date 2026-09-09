// Package auditoria is the screen that reads the trail c.Audit writes: who did
// what, to what, when and from where — with the same filter, ordering and
// pagination as any other listing, and a button that exports the same slice.
package auditoria

import (
	"sort"
	"strings"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/local-login/internal/sessao"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// consulta is what this screen reads from the URL: the embedded listing plus
// the one filter it has of its own.
type consulta struct {
	trilha.ListParams
	Acao string `form:"action"`
}

func Page(c *trilha.Ctx) (h.Node, error) {
	var q consulta
	if err := c.Bind(&q); err != nil {
		return nil, err
	}
	regs, total := buscar(q)
	tabela := ui.AuditTable(c, regs, ui.AuditOpts{
		Params:  q.ListParams,
		Total:   total,
		Actions: acoes(),
		Action:  q.Acao,
		Export:  "/auditoria/csv",
	})
	if c.Fragment() == "auditoria" {
		return tabela, nil
	}
	c.SetTitle("Auditoria")
	return h.Div(ui.H1(h.Text("Auditoria")), tabela), nil
}

// buscar is the query this app writes, because reading the trail is the app's
// job: the framework's sink has one method and it writes.
func buscar(q consulta) ([]trilha.AuditRecord, int) {
	todos := sessao.Trilha()
	var f []trilha.AuditRecord
	for _, r := range todos {
		if q.Acao != "" && r.Action != q.Acao {
			continue
		}
		if termo := strings.ToLower(q.Q); termo != "" && !casa(r, termo) {
			continue
		}
		f = append(f, r)
	}
	// Newest first: a trail is read from the end, which is where the thing
	// somebody is asking about just happened.
	sort.SliceStable(f, func(i, j int) bool { return f[i].At.After(f[j].At) })
	total := len(f)
	if q.Offset() >= total {
		return nil, total
	}
	fim := min(q.Offset()+q.Limit(), total)
	return f[q.Offset():fim], total
}

func casa(r trilha.AuditRecord, termo string) bool {
	for _, campo := range []string{r.Action, r.Target, r.Actor.Name, r.Actor.Email, r.Actor.Subject} {
		if strings.Contains(strings.ToLower(campo), termo) {
			return true
		}
	}
	return false
}

// acoes are the values the filter offers, taken from what was actually
// recorded: a select with options nobody ever used is a filter that finds
// nothing.
func acoes() []string {
	vistas := map[string]bool{}
	var out []string
	for _, r := range sessao.Trilha() {
		if !vistas[r.Action] {
			vistas[r.Action] = true
			out = append(out, r.Action)
		}
	}
	sort.Strings(out)
	return out
}
