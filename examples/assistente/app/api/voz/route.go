// Package voz receives the recording of the counter: it transcribes, asks the
// model for an answer in the language the person chose, and renders the piece
// of the page with the transcript, the answer and the player.
package voz

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/ai"
	pagina "github.com/emersonjoe/trilha/examples/assistente/app/voz"
	"github.com/emersonjoe/trilha/examples/assistente/internal/config"
	"github.com/emersonjoe/trilha/examples/assistente/internal/ferramentas"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// MaxAudio is the ceiling for one recording, on top of the body limit: a
// minute at the counter is well under it, and what is over it is not an
// answer anybody is waiting for.
const MaxAudio = 8 << 20

// MaxTexto is how much of the answer is allowed to be spoken. The address of
// the player carries the text, so the ceiling is also the ceiling of a URL
// somebody could hand-write.
const MaxTexto = 600

// POST transcribes the recording and answers in the chosen language.
func POST(c *trilha.Ctx) error {
	c.AllowBody(MaxAudio + (1 << 20)) // the recording plus the form around it
	up, err := c.File("audio", trilha.FileRules{
		// The type is the one detected in the bytes, not the one announced:
		// webm arrives as video/webm even when there is only audio inside.
		Accept: []string{"audio/*", "video/webm", "video/mp4", "application/ogg"},
	})
	if err != nil {
		return err
	}
	defer up.Close()
	// The ceiling is answered as 413 and not as a message under the field: a
	// recording that is too long is a question about the payload, and the
	// queue of the upload shows the status on the line of that recording.
	if up.Size > MaxAudio {
		return trilha.Errorf(http.StatusRequestEntityTooLarge, "gravação longa demais; grave até um minuto")
	}

	idioma := idiomaOf(c.Form("idioma"))
	t, err := ferramentas.Client.Transcribe(c.Context(), up, ai.TranscribeOpts{
		Language: strings.SplitN(idioma.Value, "-", 2)[0],
		Filename: up.Name,
		MaxBytes: MaxAudio,
	})
	if err != nil {
		return provedor(err, "transcrever o áudio")
	}
	texto := strings.TrimSpace(t.Text)
	if texto == "" {
		return trilha.Errorf(http.StatusUnprocessableEntity, "não deu para entender o áudio; grave de novo")
	}

	cfg := config.Cfg.Get()
	res, err := ferramentas.Client.Chat(c.Context(), ai.Request{
		Model:     cfg.Modelo,
		MaxTokens: cfg.MaxTokens,
		Messages: []ai.Message{
			{Role: "system", Content: "Responda em " + idioma.Label + ", de forma curta e falada, sem listas nem markdown."},
			{Role: "user", Content: texto},
		},
	})
	if err != nil {
		return provedor(err, "responder")
	}
	resposta := strings.TrimSpace(res.Text())
	if resposta == "" {
		resposta = "…"
	}
	c.Audit("voz.responder", idioma.Value, trilha.Fields{"chars_pergunta": len(texto), "chars_resposta": len(resposta)})
	return c.Render(http.StatusOK, fragmento(c, texto, resposta, idioma))
}

// fragmento is the piece of the page #resposta is swapped with.
func fragmento(c *trilha.Ctx, pergunta, resposta string, idioma ui.Option) h.Node {
	fala := "/api/voz/fala?" + url.Values{"texto": {corta(resposta)}, "idioma": {idioma.Value}}.Encode()
	return h.Div(h.ID("resposta"),
		ui.Card(
			ui.CardHeader(ui.CardTitle("Você disse")),
			ui.CardContent(h.P(h.Text(pergunta))),
			ui.CardHeader(ui.CardTitle("Resposta em "+idioma.Label)),
			ui.CardContent(
				h.P(h.Text(resposta)),
				ui.Audio(c, fala, ui.AudioOpts{Title: "Resposta", Type: "audio/mpeg", NoOpen: true}),
			),
		),
	)
}

// idiomaOf keeps the language inside the list the page offers: what arrives is
// a form field, and a form field is whatever somebody sent.
func idiomaOf(v string) ui.Option {
	for _, o := range pagina.Idiomas {
		if o.Value == v {
			return o
		}
	}
	return pagina.Idiomas[0]
}

// corta limits what is handed to the speech endpoint, so a long answer does
// not turn into a long URL and a long bill.
func corta(s string) string {
	if len(s) <= MaxTexto {
		return s
	}
	return strings.TrimSpace(s[:MaxTexto])
}

// provedor turns a refusal from the provider into an answer of this app: the
// message of the provider stays in the log, not on the screen.
func provedor(err error, what string) error {
	var e *ai.Error
	if errors.As(err, &e) {
		return trilha.Errorf(http.StatusBadGateway, "não foi possível %s agora (%d)", what, e.Status)
	}
	return err
}
