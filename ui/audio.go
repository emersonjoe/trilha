package ui

import (
	"strings"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// AudioPreload is how much audio the browser may fetch before playback.
type AudioPreload string

const (
	// PreloadNone fetches no audio before the person asks to play it.
	PreloadNone AudioPreload = "none"
	// PreloadMetadata fetches only enough to learn duration and metadata.
	PreloadMetadata AudioPreload = "metadata"
	// PreloadAuto lets the browser decide how much to fetch.
	PreloadAuto AudioPreload = "auto"
)

// AudioOpts describes one recording or sound file.
type AudioOpts struct {
	Title string
	Type  string
	// Duration is shown without waiting for browser metadata.
	Duration time.Duration
	// Download points to a route that serves the file as an attachment.
	Download string
	// Preload defaults to PreloadNone, which is appropriate for lists.
	Preload AudioPreload
	// NoOpen removes the link that opens the audio in a new tab.
	NoOpen bool
}

// Audio renders an accessible native audio player with the same escape routes
// as Preview. Serve src with Ctx.Inline and Download with Ctx.Attachment.
//
//	ui.Audio(c, "/audio/meeting", ui.AudioOpts{Title: "Meeting", Type: "audio/mpeg"})
func Audio(c *trilha.Ctx, src string, opts ...AudioOpts) h.Node {
	var o AudioOpts
	if len(opts) > 0 {
		o = opts[0]
	}
	pt := langOf(c) == "pt-BR"
	title := o.Title
	if title == "" {
		title = word(pt, "Audio", "Áudio")
	}
	preload := o.Preload
	if preload == "" {
		preload = PreloadNone
	}
	if o.Type != "" && !playableAudio(o.Type) {
		var action h.Node
		if o.Download != "" {
			action = ButtonLink(o.Download, h.Text(word(pt, "Download the audio", "Baixar o áudio")))
		}
		return Empty(EmptyOpts{
			Icon:   "info",
			Title:  word(pt, "This audio format cannot be played", "Este formato de áudio não pode ser reproduzido"),
			Hint:   word(pt, "The file is still available for download.", "O arquivo continua disponível para baixar."),
			Action: action,
		})
	}

	bar := []h.Node{h.Class("ui-preview-bar"), h.Span(h.Class("ui-preview-title"), h.Text(title))}
	if o.Duration > 0 {
		bar = append(bar, h.Span(h.Class("ui-audio-duration"), Duration(c, o.Duration)))
	}
	bar = append(bar, Spacer())
	bar = append(bar, previewActions(src, PreviewOpts{Download: o.Download, NoOpen: o.NoOpen}, pt)...)

	player := []h.Node{
		h.Class("ui-audio-player"), h.Controls(), h.Attr("preload", string(preload)), h.Aria("label", title),
	}
	source := []h.Node{h.Src(src)}
	if o.Type != "" {
		source = append(source, h.Type(o.Type))
	}
	player = append(player, h.Source(source...), h.Text(word(pt,
		"Your browser cannot play this audio.", "Seu navegador não consegue reproduzir este áudio.")))
	return h.Div(h.Class("ui-preview ui-audio"), h.Div(bar...), h.Audio(player...))
}

func playableAudio(ctype string) bool {
	kind := strings.ToLower(strings.TrimSpace(strings.SplitN(ctype, ";", 2)[0]))
	return strings.HasPrefix(kind, "audio/") || kind == "application/ogg"
}
