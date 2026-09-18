// Package voz is the voice desk: record, transcribe, answer in the language
// asked for, and say the answer out loud. The three steps that are not chat —
// transcription, speech and the capture island — are the ones spec 158 added,
// and this page is where they meet.
package voz

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Idiomas are the languages of the counter: the ones an attendant actually
// hears in a queue in Brazil.
var Idiomas = []ui.Option{
	{Value: "pt-BR", Label: "Português"},
	{Value: "en", Label: "English"},
	{Value: "es", Label: "Español"},
	{Value: "fr", Label: "Français"},
	{Value: "ht", Label: "Kreyòl ayisyen"},
	{Value: "gn", Label: "Guarani"},
}

// Page draws the counter: the language, the recorder and the place the answer
// lands.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Voz")
	return h.Div(h.Class("layout"),
		ui.Card(h.Class("chat"),
			ui.CardHeader(ui.CardTitle("Atendimento por voz")),
			ui.CardContent(
				ui.Muted(h.Text("Escolha o idioma da resposta, grave a pergunta e ouça a resposta. "+
					"Sem microfone (ou sem permissão) o campo de arquivo continua ali: no celular ele abre o gravador do sistema.")),
				ui.Field("idioma", "Idioma da resposta",
					// O campo vive fora do formulário e é associado por id: o
					// gravador continua sendo o formulário, e o idioma viaja
					// com a gravação como qualquer outro campo.
					ui.Select(h.ID("idioma"), h.Name("idioma"), h.Attr("form", "voz"), ui.SelectOptions(Idiomas, "pt-BR"))),
				ui.Recorder(c, ui.RecorderOpts{
					Action:     "/api/voz",
					Target:     "resposta",
					MaxSeconds: 60,
					Attrs:      []h.Node{h.ID("voz")},
				}),
				h.Div(h.ID("resposta")),
			),
		),
		ui.Card(h.Class("info"),
			ui.CardHeader(ui.CardTitle("Como funciona")),
			ui.CardContent(h.Ul(
				h.Li(ui.Code("ui.Recorder"), h.Text(" grava e manda o arquivo com progresso")),
				h.Li(ui.Code("ai.Client.Transcribe"), h.Text(" transcreve pelo protocolo OpenAI")),
				h.Li(ui.Code("ai.Client.Chat"), h.Text(" responde no idioma pedido")),
				h.Li(ui.Code("ai.Client.Speak"), h.Text(" fala a resposta, servida pelo "), ui.Code("ui.Audio")),
			)),
			ui.CardHeader(ui.CardTitle("Microfone")),
			ui.CardContent(h.P(h.Text("O navegador só entrega o microfone com "),
				ui.Code("Permissions-Policy: microphone=(self)"), h.Text(" — o padrão do framework nega."))),
		),
		// The island needs both: the recorder for the microphone, the upload
		// for the progress and the swap of #resposta.
		ui.RecorderScript(c),
		ui.UploadScript(c),
	), nil
}
