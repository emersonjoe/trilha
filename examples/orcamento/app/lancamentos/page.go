// Package lancamentos has the standalone entry page (works without JS) and
// the POST every entry form submits to (dialog included).
package lancamentos

import (
	"net/http"
	"strings"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/orcamento/internal/componentes"
	"github.com/emersonjoe/trilha/examples/orcamento/internal/plano"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Page renders the form on its own page.
func Page(c *trilha.Ctx) (h.Node, error) {
	return pagina(c, plano.Lancamento{Conta: c.Query("conta")}, nil, "/lancamentos"), nil
}

// POST validates and stores the entry; on error the page comes back with the
// messages (422), on success it redirects to where the form was opened.
func POST(c *trilha.Ctx) error {
	var in plano.Lancamento
	if err := c.Bind(&in); err != nil {
		if fe, ok := err.(trilha.FieldErrors); ok {
			return c.Render(http.StatusUnprocessableEntity, pagina(c, in, fe, c.Form("voltar")))
		}
		return err
	}
	if errs := plano.Validar(&in); errs.Any() {
		return c.Render(http.StatusUnprocessableEntity, pagina(c, in, errs, c.Form("voltar")))
	}
	plano.Lancar(in)
	voltar := c.Form("voltar")
	if voltar == "" || !strings.HasPrefix(voltar, "/") {
		voltar = "/contas/" + in.Conta + "?mes=" + plano.Mes(in.Data)
	}
	sep := "?"
	if strings.Contains(voltar, "?") {
		sep = "&"
	}
	return c.Redirect(voltar + sep + "ok=1")
}

func pagina(c *trilha.Ctx, in plano.Lancamento, errs trilha.FieldErrors, voltar string) h.Node {
	c.SetTitle("Novo lançamento")
	return ui.Stack(
		ui.Breadcrumb(ui.Crumb{Label: "Orçamento", Href: "/"}, ui.Crumb{Label: "Novo lançamento"}),
		ui.Card(
			ui.CardHeader(h.H1(h.Class("ui-card-title"), h.Text("Novo lançamento")), ui.CardDescription("O mesmo formulário do diálogo, numa página inteira.")),
			ui.CardContent(componentes.FormLancamento(c, in, errs, voltar)),
		),
	)
}
