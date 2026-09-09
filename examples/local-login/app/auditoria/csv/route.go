// Package auditoriacsv exports the trail as a spreadsheet. It lives inside the
// audited folder on purpose: a download is a different response, not a
// different permission, and the folder's middleware guards it without this
// route saying anything. Outside the folder it would have been the one address
// that hands the whole trail to anybody.
package auditoriacsv

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/local-login/internal/sessao"
)

// linha is the spreadsheet's shape. The csv tags are the headings, and the
// order here is the order of the columns — the AuditRecord itself is not
// exported directly because Fields is a map, and a map has no column.
type linha struct {
	Quando string `csv:"Quando"`
	Quem   string `csv:"Quem"`
	Como   string `csv:"Como"`
	Acao   string `csv:"Ação"`
	Alvo   string `csv:"Alvo"`
	IP     string `csv:"IP"`
	Rota   string `csv:"Rota"`
}

// GET /auditoria/csv answers with what the screen is showing.
//
// openapi:tag auditoria
func GET(c *trilha.Ctx) error {
	regs := sessao.Trilha()
	linhas := make([]linha, 0, len(regs))
	for i := len(regs) - 1; i >= 0; i-- {
		r := regs[i]
		quem := r.Actor.Name
		if quem == "" {
			quem = r.Actor.Subject
		}
		linhas = append(linhas, linha{
			Quando: r.At.Format("2006-01-02 15:04:05"),
			Quem:   quem,
			Como:   r.Actor.Via,
			Acao:   r.Action,
			Alvo:   r.Target,
			IP:     r.IP,
			Rota:   r.Route,
		})
	}
	// Quem baixa a trilha inteira também deixa rastro: é a ação que alguém
	// pergunta depois, e ela não pode ser a única que some.
	c.Audit("auditoria.exportou", "csv", trilha.Fields{"linhas": len(linhas)})
	return c.CSV("auditoria.csv", linhas)
}
