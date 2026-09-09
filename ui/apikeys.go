package ui

import (
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// SecretOnce is the card that shows a freshly issued key: the value, a way to
// copy it, and the sentence that has to be there — this is the only time it
// appears.
//
//	k, secret, err := Keys.Issue(c, f.Nome, f.Escopos, 0)
//	if err != nil {
//		return err
//	}
//	return c.Render(200, ui.SecretOnce(c, secret))
//
// The copy button is the one piece of script here, and it degrades to nothing:
// without it the value is still selectable text in a field somebody can copy
// by hand. What is never done is offering to show it again — the application
// cannot, because only the hash was stored, and a screen that implies
// otherwise teaches the wrong lesson about its own keys.
//
//	see: auth.Keys.Issue
func SecretOnce(c *trilha.Ctx, secret string, children ...h.Node) h.Node {
	pt := langOf(c) == "pt-BR"
	kids := []h.Node{
		h.Class("ui-secret-once"), h.Role("alert"),
		Alert(word(pt, "Copy it now", "Copie agora"),
			AlertDescription(h.Text(word(pt,
				"This is the only time the key is shown. What is stored is its hash, so nobody — including this application — can show it again.",
				"Esta é a única vez que a chave aparece. O que fica guardado é o hash dela, então ninguém — nem esta aplicação — consegue mostrar de novo.")))),
		h.Div(h.Class("ui-secret-once-row"),
			Input(h.Value(secret), h.Attr("readonly", ""), h.Attr("spellcheck", "false"),
				h.Aria("label", word(pt, "The key", "A chave")), h.Class("ui-secret-once-value")),
			Button(Outline(), h.Data("ui-copy", secret), h.Data("ui-copied", word(pt, "Copied", "Copiado")),
				h.Text(word(pt, "Copy", "Copiar")))),
	}
	return h.Div(append(kids, children...)...)
}

// APIKeyRow is one line of the table. It is a plain struct and not an
// auth.Key because ui does not import auth — the kit draws, and what it draws
// is data. The application maps one to the other in three lines, and that
// mapping is where "what a screen shows" is decided.
type APIKeyRow struct {
	ID string
	// Handle identifies the key without opening the hash. The key itself is
	// not here and cannot be: only its hash was stored.
	Handle   string
	Name     string
	Scopes   []string
	Created  time.Time
	LastUsed time.Time
	Revoked  bool
}

// APIKeysOpts is what the table can do besides list.
type APIKeysOpts struct {
	// Revoke is where the revoke button posts, with the id in "id". Empty
	// draws no button.
	Revoke string
	// CSRF is the hidden token field, and a form that revokes a key has to
	// carry one: pass trilha.CSRFInput(c). It is an option and not something
	// taken from the Ctx so the component stays drawable from a test, and so
	// that leaving it out is a visible omission rather than a silent one.
	CSRF h.Node
	// Empty replaces the default empty state.
	Empty h.Node
}

// APIKeysTable lists the keys of an application: what each one is called, what
// it may do, when it was made and when it was last used.
//
//	return ui.APIKeysTable(c, linhas, ui.APIKeysOpts{Revoke: "/chaves/revogar"})
//
// The key itself is not here and cannot be — only the handle, which is the half
// that identifies it without opening the hash. Revoking posts with a
// confirmation, because it is the one action on this screen nobody can undo.
//
//	see: ui.SecretOnce
func APIKeysTable(c *trilha.Ctx, rows []APIKeyRow, opts ...APIKeysOpts) h.Node {
	var o APIKeysOpts
	if len(opts) > 0 {
		o = opts[0]
	}
	pt := langOf(c) == "pt-BR"
	if len(rows) == 0 {
		if o.Empty != nil {
			return o.Empty
		}
		return Empty(EmptyOpts{
			Icon:  "info",
			Title: word(pt, "No keys yet", "Nenhuma chave ainda"),
			Hint: word(pt, "A key is how another system calls this one without a person behind it.",
				"Uma chave é como outro sistema chama este, sem uma pessoa atrás."),
		})
	}
	head := h.Thead(h.Tr(
		h.Th(h.Text(word(pt, "Name", "Nome"))),
		h.Th(h.Text(word(pt, "Key", "Chave"))),
		h.Th(h.Text(word(pt, "May", "Pode"))),
		h.Th(h.Text(word(pt, "Created", "Criada"))),
		h.Th(h.Text(word(pt, "Last used", "Último uso"))),
		h.Th(h.Text("")),
	))
	body := make([]h.Node, 0, len(rows))
	for _, r := range rows {
		body = append(body, h.Tr(
			h.Td(h.Text(r.Name), h.If(r.Revoked, Badge(Outline(), h.Text(word(pt, "revoked", "revogada"))))),
			h.Td(Code(r.Handle)),
			h.Td(escopos(r.Scopes)),
			h.Td(Date(c, r.Created, DateOnly())),
			// Never used is not the same as used long ago, and an empty cell
			// says the first one better than a date from the epoch would.
			h.Td(h.IfElse(r.LastUsed.IsZero(),
				h.Span(h.Class("ui-muted"), h.Text(word(pt, "never", "nunca"))),
				Date(c, r.LastUsed, Relative()))),
			h.Td(revogar(o, r, pt)),
		))
	}
	return Table(head, h.Tbody(body...))
}

func escopos(scopes []string) h.Node {
	kids := make([]h.Node, 0, len(scopes))
	for _, s := range scopes {
		kids = append(kids, Badge(Outline(), Sm(), h.Text(s)), h.Text(" "))
	}
	return h.Span(kids...)
}

func revogar(o APIKeysOpts, r APIKeyRow, pt bool) h.Node {
	if o.Revoke == "" || r.Revoked {
		return h.Text("")
	}
	return h.Form(h.Method("post"), h.Action(o.Revoke), o.CSRF,
		Confirm(word(pt, "Revoke this key?", "Revogar esta chave?"),
			word(pt, "Anything using it stops working immediately.", "O que estiver usando ela para de funcionar na hora.")),
		h.Input(h.Type("hidden"), h.Name("id"), h.Value(r.ID)),
		Button(Destructive(), Sm(), h.Type("submit"), h.Text(word(pt, "Revoke", "Revogar"))))
}
