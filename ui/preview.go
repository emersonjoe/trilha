package ui

import (
	"strings"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// PreviewOpts is the file being shown and the two ways out of it.
type PreviewOpts struct {
	// Title names the document. It is the accessible name of the frame — a
	// frame without one is a region a screen reader announces as "frame" —
	// and the label on the bar.
	Title string
	// Type is the media type, so the preview knows whether it is an image, a
	// document a browser renders, or something nobody can show. Empty leaves
	// it to the browser, which is right when the file comes from a route that
	// sets the type itself.
	Type string
	// Height is how tall the frame is. Default "70vh": tall enough to read a
	// page of a PDF, short enough to leave the metadata beside it visible.
	Height string
	// Download is the address that sends the file as a download — a route
	// that answers with Ctx.Attachment. Empty draws no button.
	Download string
	// NoOpen removes the "open in a new tab" link. It is on by default
	// because it is the way out of every preview that did not work.
	NoOpen bool
}

// Preview shows a file beside its metadata: the bar, the frame, and a way out.
//
//	ui.Preview(c, "/documents/"+id+"/file", ui.PreviewOpts{
//		Title:    doc.Filename,
//		Type:     doc.MIME,
//		Download: "/documents/" + id + "/download",
//	})
//
// What it does that a hand-written <iframe> does not:
//
// An image is an <img>, not a frame, and clicking it opens the full size — a
// frame around a photograph is a scrollbar around a photograph.
//
// A type no browser shows in place — HTML, SVG, anything Ctx.Inline refuses —
// is a card saying so, with the download button, instead of a frame that
// renders blank and explains nothing.
//
// The frame is sandboxed, lazy and named, and src must be served with
// Ctx.Inline: that is the response that says it may be framed by a page of
// this origin. A file served any other way carries the default
// X-Frame-Options: DENY, and the frame stays blank however the page around it
// is configured — the browser blames the framed answer, not the page.
//
//	see: trilha.Ctx.Inline, trilha.CanInline, ui.Empty
func Preview(c *trilha.Ctx, src string, opts ...PreviewOpts) h.Node {
	var o PreviewOpts
	if len(opts) > 0 {
		o = opts[0]
	}
	pt := langOf(c) == "pt-BR"
	title := o.Title
	if title == "" {
		title = word(pt, "Preview", "Pré-visualização")
	}
	height := o.Height
	if height == "" {
		height = "70vh"
	}

	bar := append([]h.Node{
		h.Class("ui-preview-bar"),
		h.Span(h.Class("ui-preview-title"), h.Text(title)),
		Spacer(),
	}, previewActions(src, o, pt)...)
	return h.Div(h.Class("ui-preview"),
		h.Div(bar...),
		previewBody(src, title, height, o.Type, pt, o.Download))
}

func previewActions(src string, o PreviewOpts, pt bool) []h.Node {
	var out []h.Node
	if o.Download != "" {
		out = append(out, h.A(h.Href(o.Download), h.Class("ui-btn ui-btn-outline ui-btn-sm"),
			h.Text(word(pt, "Download", "Baixar"))))
	}
	if !o.NoOpen {
		out = append(out, h.A(h.Href(src), h.Target("_blank"), h.Rel("noopener noreferrer"),
			h.Class("ui-btn ui-btn-ghost ui-btn-sm"),
			h.Text(word(pt, "Open in a new tab", "Abrir em nova aba"))))
	}
	return out
}

// previewBody is the one decision this component makes: what a browser does
// with this type.
func previewBody(src, title, height, ctype string, pt bool, download string) h.Node {
	switch {
	case strings.HasPrefix(strings.ToLower(ctype), "image/") && trilha.CanInline(ctype):
		// The link is the zoom: a click opens the file at its own size, and
		// it costs no script and works with none.
		return h.A(h.Href(src), h.Target("_blank"), h.Rel("noopener noreferrer"), h.Class("ui-preview-image"),
			h.Img(h.Src(src), h.Alt(title), h.Attr("loading", "lazy")))
	case ctype != "" && !trilha.CanInline(ctype):
		var action h.Node
		if download != "" {
			action = ButtonLink(download, h.Text(word(pt, "Download the file", "Baixar o arquivo")))
		}
		return Empty(EmptyOpts{
			Icon:  "info",
			Title: word(pt, "This file cannot be previewed", "Este arquivo não dá para pré-visualizar"),
			Hint: word(pt,
				"A browser would run it instead of showing it, so it is offered as a download.",
				"O navegador rodaria o arquivo em vez de mostrar, então ele é oferecido para baixar."),
			Action: action,
		})
	}
	frame := []h.Node{
		h.Class("ui-preview-frame"), h.Src(src), h.Attr("title", title),
		h.Attr("loading", "lazy"), h.StyleAttr("height: " + height),
	}
	// A PDF gets no sandbox, and that is measured rather than assumed: the
	// browser's own viewer refuses to run inside a sandboxed frame (the
	// request comes back blocked, and the frame is blank with nothing in the
	// console). The two flags that would bring it back — allow-scripts with
	// allow-same-origin — are exactly the pair that lets a same-origin
	// document take its own sandbox off, so the attribute would be a label and
	// not a fence. What holds here instead is the answer: Inline refuses HTML,
	// SVG and XML, sets the type itself and sends nosniff, so what is framed
	// is a document the browser renders and not one it runs.
	//
	// Every other type keeps the sandbox, because there it costs nothing:
	// text and images render with allow-same-origin alone.
	if !strings.HasPrefix(strings.ToLower(ctype), "application/pdf") {
		frame = append(frame, h.Attr("sandbox", "allow-same-origin"))
	}
	return h.Iframe(frame...)
}
