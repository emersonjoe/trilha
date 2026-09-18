// Package fala speaks a short text: it is what the <audio> of the answer
// fetches, and the only thing it does is hand the provider's bytes over with
// the right media type.
package fala

import (
	"errors"
	"net/http"
	"strings"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/ai"
	"github.com/emersonjoe/trilha/examples/assistente/internal/ferramentas"
)

// MaxTexto is the most that is spoken in one request. The text arrives in the
// address, so the ceiling is also what keeps a hand-written URL from becoming
// a long bill.
const MaxTexto = 600

// vozes picks a voice per language, so the answer at least sounds like it
// belongs to the language it is in.
var vozes = map[string]string{"pt-BR": "nova", "en": "alloy", "es": "nova", "fr": "shimmer", "ht": "nova", "gn": "nova"}

// GET streams the answer as audio.
func GET(c *trilha.Ctx) error {
	texto := strings.TrimSpace(c.Query("texto"))
	if texto == "" {
		return trilha.Errorf(http.StatusUnprocessableEntity, "sem texto para falar")
	}
	if len(texto) > MaxTexto {
		return trilha.Errorf(http.StatusRequestEntityTooLarge, "texto longo demais para falar")
	}
	voz := vozes[c.Query("idioma")]
	if voz == "" {
		voz = "alloy"
	}
	opts := ai.SpeakOpts{Voice: voz}
	body, err := ferramentas.Client.Speak(c.Context(), texto, opts)
	if err != nil {
		var e *ai.Error
		if errors.As(err, &e) {
			return trilha.Errorf(http.StatusBadGateway, "não foi possível falar agora (%d)", e.Status)
		}
		return err
	}
	defer body.Close()
	// What somebody said at the counter is not a file for a cache to keep.
	c.Header("Cache-Control", "private, no-store")
	return c.Inline("resposta.mp3", body, opts.ContentType())
}
