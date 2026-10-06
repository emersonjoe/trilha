package ui

import (
	"regexp"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// SheetOpts is how a Sheet sits on the page. The zero value is a panel at the
// end side, 32rem wide, closed, beside a page that stays usable.
type SheetOpts struct {
	// Side is "end" (the default), "start" or "bottom".
	Side string
	// Width is a CSS length — "40rem", "480px", "50vw". Anything else is
	// ignored: it is written into a style attribute.
	Width string
	// Title is the visible title, and the panel's accessible name.
	Title string
	// Modal makes it a <dialog> opened with showModal: the rest of the page
	// goes inert. The default is a complementary <aside> beside the page.
	Modal bool
	// Open renders it open, with children as its body: what a direct link or
	// a reload show (the route reads ?painel=… and renders the same body its
	// fragment answers).
	Open bool
	// Push makes opening add a history entry, so Back closes the panel.
	// Without it the address does not change: a panel is a moment.
	Push bool
}

var cssLength = regexp.MustCompile(`^\d+(\.\d+)?(rem|em|px|vw|vh|%|ch)$`)

// Sheet is a side panel whose body is loaded on demand and that stays open
// while the region beside it navigates — a document viewer open next to the
// list, the next item one click away (#296).
//
//	// layout, outside the ui.Navigate region:
//	ui.Sheet(c, "leitor", ui.SheetOpts{Title: "Document"}), ui.SheetScript(c),
//	// the list, inside it:
//	h.A(h.Href("/docs/"+id), ui.SheetOpen("leitor"), h.Text(doc.Name))
//	// the route of /docs/{id}:
//	if c.Fragment() == "leitor-body" { return ui.SheetBody("leitor", leitor(c, doc)), nil }
//
// A SheetOpen link opens the panel, marks its body pending and asks the link's
// address for the fragment "<id>-body" — the same route, answering the piece
// with Ctx.Fragment. Another link swaps only the body. The panel lives in the
// layout, outside the region, so ui.Navigate never touches it and a
// ui.Preview inside it does not reload while the page navigates; closing it
// empties the body, which is what frees a heavy frame.
//
// Opening moves the focus to the title; Escape and the close button close it
// and give the focus back to the link, whose aria-expanded follows. A panel
// that is not modal does not trap the focus — it is complementary content.
// Below 768px it covers the screen from the bottom, since there is no page
// left beside it, and it closes the phone drawer of ui.Shell.
//
// Without JavaScript the link goes to its address — the document's page, or
// the list with ?painel=… rendered with Open — so nothing is out of reach.
//
//	see: ui.SheetOpen, ui.SheetClose, ui.SheetScript, ui.Dialog
func Sheet(c *trilha.Ctx, id string, o SheetOpts, children ...h.Node) h.Node {
	pt := langOf(c) == "pt-BR"
	side := "end"
	if o.Side == "start" || o.Side == "bottom" {
		side = o.Side
	}
	attrs := []h.Node{h.ID(id), h.Class("ui-sheet ui-sheet-" + side), h.Data("ui-sheet", ""), h.Aria("labelledby", id+"-title")}
	if cssLength.MatchString(o.Width) {
		attrs = append(attrs, h.StyleAttr("--ui-sheet-width: "+o.Width))
	}
	if o.Push {
		attrs = append(attrs, h.Data("ui-sheet-history", "push"))
	}
	inner := []h.Node{
		h.Div(h.Class("ui-sheet-head"),
			h.H2(h.Class("ui-sheet-title"), h.ID(id+"-title"), h.Attr("tabindex", "-1"), h.Text(o.Title)),
			h.Button(h.Class("ui-btn ui-btn-ghost ui-btn-icon ui-btn-sm"), h.Type("button"), h.Data("ui-sheet-close", ""),
				h.Aria("label", word(pt, "Close", "Fechar")), Icon("x"))),
		SheetBody(id, children...),
	}
	if o.Modal {
		if o.Open {
			attrs = append(attrs, h.Attr("open", ""))
		}
		return h.Dialog(append(attrs, inner...)...)
	}
	attrs = append(attrs, h.Role("complementary"))
	if !o.Open {
		attrs = append(attrs, h.Attr("hidden", ""))
	}
	return h.Aside(append(attrs, inner...)...)
}

// SheetBody is the body of the Sheet with this id: the element the fragment
// "<id>-body" replaces. The route answers it, so the swap keeps its class.
//
//	if c.Fragment() == "leitor-body" {
//		return ui.SheetBody("leitor", leitor(c, doc)), nil
//	}
func SheetBody(id string, children ...h.Node) h.Node {
	return h.Div(append([]h.Node{h.Class("ui-sheet-body"), h.ID(id + "-body")}, children...)...)
}

// SheetOpen goes on the <a href> that opens the Sheet with this id and loads
// its address into the body. It starts as aria-expanded="false".
func SheetOpen(id string) h.Node {
	return h.Attrs(h.Data("ui-sheet-open", id), h.Aria("controls", id), h.Aria("expanded", "false"))
}

// SheetClose is a Button that closes the enclosing Sheet.
func SheetClose(children ...h.Node) h.Node {
	return Button(append(children, h.Data("ui-sheet-close", ""))...)
}

// SheetScript loads ui.sheet.js, the behavior behind Sheet. Put it once, in
// the layout that has the panel; ui.Head does not load it.
func SheetScript(c *trilha.Ctx) h.Node {
	return h.Script(h.Src(c.Asset("/ui.sheet.js")), h.Defer())
}
