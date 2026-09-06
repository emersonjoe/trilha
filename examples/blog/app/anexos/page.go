// Package anexos shows an upload with progress that still works without
// JavaScript, checked by c.File: limite por arquivo, tipo lido no conteúdo e
// nome sem caminho.
package anexos

import (
	"net/http"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/blog/internal/anexos"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// regras é o que esta rota aceita. O limite por arquivo é menor que o do corpo
// (middleware.go): um é do arquivo, o outro é da requisição inteira.
var regras = trilha.FileRules{
	MaxSize: 4 << 20,
	Accept:  []string{"image/*", "application/pdf", "text/plain"},
}

// Page renders GET /anexos — a página inteira, ou só o bloco quando o script
// pede o pedaço.
func Page(c *trilha.Ctx) (h.Node, error) {
	if c.Fragment() == "anexos" {
		return bloco(c, nil), nil
	}
	c.SetTitle("Anexos")
	return pagina(c, nil), nil
}

// POST recebe o arquivo. O middleware já levantou o limite do corpo; aqui o
// c.File confere o resto e devolve FieldErrors, que o formulário mostra.
func POST(c *trilha.Ctx) error {
	up, err := c.File("arquivo", regras)
	if err != nil {
		errs, ok := err.(trilha.FieldErrors)
		if !ok {
			return err
		}
		return c.Render(http.StatusUnprocessableEntity, resposta(c, errs))
	}
	defer up.Close()
	anexos.Add(up.Name, up.Size, up.MIME)
	if c.Fragment() != "" {
		return c.Render(http.StatusOK, bloco(c, nil))
	}
	return c.Redirect("/anexos")
}

// resposta devolve só o pedaço quando quem perguntou foi o script, e a página
// inteira quando foi o navegador sem JavaScript.
func resposta(c *trilha.Ctx, errs trilha.FieldErrors) h.Node {
	if c.Fragment() != "" {
		return bloco(c, errs)
	}
	return pagina(c, errs)
}

func pagina(c *trilha.Ctx, errs trilha.FieldErrors) h.Node {
	return ui.Card(
		ui.CardHeader(
			h.H1(h.Class("ui-card-title"), h.Text("Anexos")),
			ui.CardDescription("O arquivo sobe com barra de progresso; sem JavaScript, o formulário envia como sempre."),
		),
		ui.CardContent(
			bloco(c, errs),
			ui.UploadScript(c),
		),
	)
}

// bloco é o pedaço trocado: formulário e lista juntos, porque a mensagem de
// erro pertence ao campo, e o campo está no formulário.
func bloco(c *trilha.Ctx, errs trilha.FieldErrors) h.Node {
	return h.Div(h.ID("anexos"), h.Class("ui-stack"),
		// O formulário é um formulário: action, método e enctype de sempre.
		// ui.UploadTo diz qual pedaço a resposta troca, e o script só entra se
		// o navegador o executar.
		h.Form(h.Method("post"), h.Action("/anexos"), h.Enctype("multipart/form-data"),
			h.Class("ui-stack"), ui.UploadTo("anexos"),
			trilha.CSRFInput(c),
			ui.Field("arquivo", "Arquivo",
				ui.Input(h.ID("arquivo"), h.Name("arquivo"), h.Type("file"), h.Required(), ui.InvalidIf(errs, "arquivo")),
				ui.Help("Imagem, PDF ou texto, até 4 MB — o limite deste campo, não o do app."),
				ui.Errors(errs, "arquivo")),
			ui.UploadBar(),
			ui.Submit(h.Text("Enviar")),
		),
		lista(),
	)
}

// lista é a lista de anexos, dentro do bloco trocado.
func lista() h.Node {
	itens := anexos.All()
	if len(itens) == 0 {
		return h.Ul(h.ID("lista"), h.Class("ui-list"), h.Li(h.Class("ui-muted"), h.Text("Nada enviado ainda.")))
	}
	rows := make([]h.Node, 0, len(itens))
	for _, a := range itens {
		rows = append(rows, h.Li(
			h.Strong(h.Text(a.Nome)),
			h.Text(" — "),
			h.Span(h.Class("ui-muted"), h.Text(a.Tamanho()+" · "+a.Tipo)),
		))
	}
	return h.Ul(append([]h.Node{h.ID("lista"), h.Class("ui-list")}, rows...)...)
}
