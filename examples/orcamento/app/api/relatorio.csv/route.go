// Package relatoriocsv exports the month as CSV at /api/relatorio.csv?mes=.
// The folder has a dot in its name (spec 008), so the package name differs.
package relatoriocsv

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/orcamento/internal/plano"
)

// GET writes one line per account with budget, realized and variation.
//
// c.Query and the folder name are outside what the generator deduces: the
// month is a parameter it cannot see, and "relatorio.csv" is an ugly tag.
//
// openapi:query mes string  month to export, AAAA-MM (default: the current one)
// openapi:tag relatorio
func GET(c *trilha.Ctx) error {
	m := c.Query("mes")
	if m == "" {
		m = plano.MesPadrao()
	}
	if _, err := time.Parse("2006-01", m); err != nil {
		return trilha.Errorf(http.StatusBadRequest, "mes inválido (use AAAA-MM)")
	}
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="orcamento-`+m+`.csv"`)
	c.Writer().WriteHeader(http.StatusOK)
	w := csv.NewWriter(c.Writer())
	_ = w.Write([]string{"codigo", "conta", "tipo", "nivel", "orcado", "realizado", "variacao_pct"})
	var walk func(cs []*plano.Conta)
	walk = func(cs []*plano.Conta) {
		for _, a := range cs {
			o, r := plano.Orcado(a, m), plano.Realizado(a, m)
			_ = w.Write([]string{a.Codigo, a.Nome, a.Tipo, fmt.Sprint(a.Nivel()), cents(o), cents(r), fmt.Sprintf("%.1f", plano.Variacao(o, r))})
			walk(a.Filhos)
		}
	}
	walk(plano.Raizes())
	w.Flush()
	return w.Error()
}

func cents(v int64) string { return fmt.Sprintf("%d.%02d", v/100, v%100) }
