// Package anexos shows an upload with progress that still works without
// JavaScript, checked by c.Files: limite por arquivo, teto de arquivos, tipo
// lido no conteúdo e nome sem caminho.
package anexos

import (
	"net/http"
	"net/url"
	"sort"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/blog/internal/anexos"
	arquivos "github.com/emersonjoe/trilha/examples/blog/internal/arquivos"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// regras é o que esta rota aceita. O limite por arquivo é menor que o do corpo
// (middleware.go): um é do arquivo, o outro é da requisição inteira.
var regras = trilha.FileRules{
	MaxSize:  4 << 20,
	MaxFiles: 10,
	Accept:   []string{"image/*", "application/pdf", "text/plain"},
}

// Page renders GET /anexos — a página inteira, ou só o bloco quando o script
// pede o pedaço.
func Page(c *trilha.Ctx) (h.Node, error) {
	if c.Fragment() == "lista" {
		return lista(), nil
	}
	c.SetTitle("Anexos")
	return pagina(c, nil), nil
}

// POST recebe os arquivos. A fila do ui.upload.js manda um por requisição —
// cada um com a sua barra e a sua mensagem — e o navegador sem JavaScript manda
// todos de uma vez; o c.Files atende os dois casos com o mesmo código.
func POST(c *trilha.Ctx) error {
	ups, err := c.Files("arquivos", regras)
	if err != nil {
		errs, ok := err.(trilha.FieldErrors)
		if !ok {
			return err
		}
		// Para a fila, a resposta é a mensagem daquele arquivo, em texto: ela
		// aparece na linha dele. Sem script, volta a página com o campo marcado.
		if c.Fragment() != "" {
			return c.Text(http.StatusUnprocessableEntity, primeira(errs))
		}
		return c.Render(http.StatusUnprocessableEntity, pagina(c, errs))
	}
	for _, up := range ups {
		// O blob calcula o digest, guarda por ele e devolve a chave. O nome
		// que veio do cliente nunca vira caminho, e o mesmo arquivo mandado
		// duas vezes ocupa espaço uma vez.
		ref, err := arquivos.Arquivos.Put(c.Context(), up)
		up.Close()
		if err != nil {
			return err
		}
		anexos.Add(ref.Name, ref.Size, ref.Type, ref.Key)
	}
	if c.Fragment() != "" {
		return c.Render(http.StatusOK, lista())
	}
	return c.Redirect("/anexos")
}

// primeira é a mensagem de menor chave: uma requisição da fila carrega um
// arquivo, então é a mensagem dele.
func primeira(errs trilha.FieldErrors) string {
	chaves := make([]string, 0, len(errs))
	for k := range errs {
		chaves = append(chaves, k)
	}
	sort.Strings(chaves)
	if len(chaves) == 0 {
		return "não enviado"
	}
	return errs[chaves[0]]
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

// bloco é o formulário: ele fica fora do pedaço trocado, porque a fila vive
// dentro dele e não pode ser substituída no meio de um envio.
func bloco(c *trilha.Ctx, errs trilha.FieldErrors) h.Node {
	return h.Div(h.ID("anexos"), h.Class("ui-stack"),
		// O formulário é um formulário: action, método e enctype de sempre.
		// ui.UploadTo diz qual pedaço a resposta troca — a lista, não isto.
		h.Form(h.Method("post"), h.Action("/anexos"), h.Enctype("multipart/form-data"),
			h.Class("ui-stack"), ui.UploadTo("lista"),
			trilha.CSRFInput(c),
			ui.Dropzone(ui.DropzoneOpts{Name: "arquivos", MaxSize: 4 << 20, Accept: "image/*,application/pdf,text/plain"},
				ui.Icon("upload"),
				h.P(h.Text("Solte os arquivos aqui, ou clique para escolher")),
				h.Span(h.Class("ui-muted"), h.Text("Imagem, PDF ou texto, até 4 MB cada, dez por vez.")),
			),
			h.Group(mensagens(errs)...),
			ui.Submit(h.Text("Enviar")),
		),
		lista(),
	)
}

// mensagens mostra o que o servidor recusou. Sem script os arquivos vão todos
// juntos, então pode haver mais de uma mensagem, uma por posição.
func mensagens(errs trilha.FieldErrors) []h.Node {
	chaves := make([]string, 0, len(errs))
	for k := range errs {
		chaves = append(chaves, k)
	}
	sort.Strings(chaves)
	out := make([]h.Node, 0, len(chaves))
	for _, k := range chaves {
		out = append(out, h.P(h.Class("ui-field-error"), h.Role("alert"), h.Text(k+": "+errs[k])))
	}
	return out
}

// lista é a lista de anexos, dentro do bloco trocado.
func lista() h.Node {
	itens := anexos.All()
	if len(itens) == 0 {
		return h.Ul(h.ID("lista"), h.Class("ui-list"), h.Li(h.Class("ui-muted"), h.Text("Nada enviado ainda.")))
	}
	rows := make([]h.Node, 0, len(itens))
	for _, a := range itens {
		href := "/anexos/" + url.PathEscape(a.Nome)
		linhas := []h.Node{
			h.A(h.Href(href), h.Strong(h.Text(a.Nome))),
			h.Text(" — "),
			h.Span(h.Class("ui-muted"), h.Text(a.Tamanho()+" · "+a.Tipo)),
		}
		// O link de ver só aparece para o que o c.Inline aceita: oferecer um
		// "abrir" que o servidor vai recusar é oferecer um erro.
		// O link de ver só aparece para o que o c.Inline aceita, e quem diz o
		// que ele aceita é o próprio pacote: a lista escrita à mão aqui era uma
		// segunda cópia, e a segunda cópia é a que fica para trás.
		if trilha.CanInline(a.Tipo) {
			linhas = append(linhas, h.Text(" "), h.A(h.Href(href+"/ver"), h.Class("ui-muted"), h.Text("ver")))
		}
		rows = append(rows, h.Li(linhas...))
	}
	return h.Ul(append([]h.Node{h.ID("lista"), h.Class("ui-list")}, rows...)...)
}
