package ui

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// SecretField is the input for a value that must never come back whole: an API
// key, a webhook secret, an SMTP password.
//
//	ui.SecretField(c, "token", "Provider token", cfg.Token)
//
// It draws an empty password field and, under it, what is stored — masked —
// plus the sentence that makes the whole thing work: leave it blank to keep
// what is there. That is not decoration. A form has no way to send
// "unchanged", so a screen that pre-fills the field either sends the secret
// back to the browser (where the DevTools show it) or wipes it on the first
// save that somebody did not retype. trilha.Secret and Bind already agree on
// this: an empty value leaves the stored one alone.
//
//	see: trilha.Secret, trilha.Seal
func SecretField(c *trilha.Ctx, name, label string, current trilha.Secret, opts ...FieldOpt) h.Node {
	pt := langOf(c) == "pt-BR"
	hint := word(pt, "Leave blank to keep the current one.", "Deixe em branco para manter o atual.")
	if !current.Empty() {
		// The mask says which key is there — the prefix is what somebody is
		// actually checking — without being a key anybody can use.
		hint = current.String() + " · " + hint
	} else {
		hint = word(pt, "Nothing stored yet.", "Nada guardado ainda.") + " " + hint
	}
	input := Input(h.ID(name), h.Name(name), h.Type("password"),
		h.Attr("autocomplete", "new-password"), h.Value(""),
		h.Placeholder(word(pt, "unchanged", "sem alteração")))
	return Field(name, label, input, append([]FieldOpt{Help(hint)}, opts...)...)
}
