package ui

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// AssistantOpts describes the assistant in the corner. Action and Page are the
// two that have to be filled in: the route that answers, and the page that
// answers when the script is not there.
type AssistantOpts struct {
	// ID is the element id and the prefix of every id inside. Empty means
	// "assistant"; two assistants on one page need two ids.
	ID string

	// Action is the route that answers, the one running ai.Serve. It is the
	// Action of the chat inside.
	Action string

	// Page is the conversation as a page: where the launcher points when
	// nothing else runs. Without it the launcher is a button that does
	// nothing without JavaScript, so Assistant asks for it.
	Page string

	// Label is the word on the launcher, and its accessible name. Empty means
	// "Assistant".
	Label string

	// Title is the heading of the panel. Empty means Label.
	Title string

	// Hint is the line under the title: what this assistant knows about right
	// now. It is the part that changes per route — "asking about invoice 12" —
	// and it is text, not markup.
	Hint string

	// Icon is the name of a kit icon to put on the launcher — the names
	// `trilha ui describe` lists. Empty means no icon, and the launcher is the
	// label alone: an icon nobody chose is a guess about what this assistant
	// is for.
	Icon string

	// Chat is the conversation itself. ID and Action are set from the fields
	// above; everything else — the history, the greeting, the context fields —
	// is the app's.
	Chat ChatOpts
}

// Assistant renders the assistant every management app has: a button fixed in
// a corner that opens a panel over the screen, without replacing it.
//
//	ui.Assistant(c, ui.AssistantOpts{
//		Action: "/api/assistant",
//		Page:   "/panel/assistant",
//		Hint:   "Asking about invoice " + id,
//		Chat:   ui.ChatOpts{Context: map[string]string{"invoice": id}},
//	})
//	ui.ChatScript(c)
//
// It is composition and not a second chat: the panel holds a Chat, so the
// streaming, the Markdown and the errors are the ones that already exist.
//
// The launcher is a link before it is a button. Without JavaScript it goes to
// Page, which is the same conversation as a page; with JavaScript the kit's
// script opens the dialog instead, and the browser gives the focus trap and
// the Escape key for free. Mount it once, in the layout of the area that has
// it: it is server HTML, so it does not turn any page into an island.
func Assistant(c *trilha.Ctx, o AssistantOpts) h.Node {
	id := o.ID
	if id == "" {
		id = "assistant"
	}
	label := o.Label
	if label == "" {
		label = "Assistant"
	}
	title := o.Title
	if title == "" {
		title = label
	}
	chat := o.Chat
	chat.ID = id + "-chat"
	chat.Action = o.Action
	if chat.Label == "" {
		chat.Label = title
	}

	dialogID := id + "-dialog"
	head := []h.Node{
		h.H2(h.Class("ui-dialog-title"), h.ID(dialogID+"-title"), h.Text(title)),
	}
	if o.Hint != "" {
		head = append(head, h.P(h.Class("ui-dialog-description"), h.ID(dialogID+"-hint"), h.Text(o.Hint)))
	}

	return h.Div(h.Class("ui-assistant"), h.ID(id),
		// A link, not a button: the fallback is the element's own behaviour,
		// so it cannot be forgotten. ui.js turns the click into showModal()
		// when it is there.
		h.A(h.Class("ui-btn ui-btn-primary ui-assistant-launcher"),
			h.Href(o.Page), h.ID(id+"-launcher"),
			h.Data("ui-dialog-open", dialogID),
			h.Aria("haspopup", "dialog"), h.Aria("controls", dialogID), h.Aria("expanded", "false"),
			h.Group(launcherIcon(o.Icon)...), h.Span(h.Class("ui-assistant-label"), h.Text(label)),
		),
		h.Dialog(h.Class("ui-dialog ui-assistant-dialog"), h.ID(dialogID),
			h.Aria("labelledby", dialogID+"-title"),
			h.Button(h.Class("ui-btn ui-btn-ghost ui-btn-icon ui-btn-sm ui-dialog-close"),
				h.Type("button"), h.Data("ui-dialog-close", ""), h.Aria("label", "Close"), Icon("x")),
			h.Group(head...),
			Chat(c, chat),
		),
	)
}

// launcherIcon is the icon of the launcher, or nothing.
func launcherIcon(name string) []h.Node {
	if name == "" {
		return nil
	}
	return []h.Node{Icon(name)}
}
