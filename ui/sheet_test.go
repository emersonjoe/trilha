package ui

import (
	"strings"
	"testing"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// #296: the panel is complementary content beside the page — an <aside>
// named by its title — or a <dialog> when it is modal. Its body has the id
// the fragments are asked for, and the close button speaks the app's language.
func TestSheetRendersItsContract(t *testing.T) {
	en := inApp(t, trilha.Config{}, func(c *trilha.Ctx) h.Node {
		return Sheet(c, "leitor", SheetOpts{Title: "Document"}, h.P(h.Text("inside")))
	})
	for _, want := range []string{
		`<aside`, `id="leitor"`, `role="complementary"`, `aria-labelledby="leitor-title"`, `data-ui-sheet=""`, `hidden`,
		`id="leitor-title"`, `tabindex="-1"`, `>Document</h2>`, `data-ui-sheet-close`, `aria-label="Close"`,
		`id="leitor-body"`, `<p>inside</p>`, `ui-sheet-end`,
	} {
		if !strings.Contains(en, want) {
			t.Errorf("Sheet lacks %s:\n%s", want, en)
		}
	}
	pt := inApp(t, trilha.Config{Locale: "pt-BR"}, func(c *trilha.Ctx) h.Node {
		return Sheet(c, "leitor", SheetOpts{Title: "Documento", Modal: true, Open: true, Side: "start", Width: "40rem", Push: true})
	})
	for _, want := range []string{`<dialog`, `aria-label="Fechar"`, `ui-sheet-start`, `--ui-sheet-width: 40rem`, `data-ui-sheet-history="push"`, ` open`} {
		if !strings.Contains(pt, want) {
			t.Errorf("modal, open Sheet lacks %s:\n%s", want, pt)
		}
	}
	if strings.Contains(pt, "role=\"complementary\"") || strings.Contains(pt, " hidden") {
		t.Errorf("a modal, open Sheet is a dialog, shown:\n%s", pt)
	}
	// A width that could break out of the style attribute is not written.
	bad := inApp(t, trilha.Config{}, func(c *trilha.Ctx) h.Node {
		return Sheet(c, "x", SheetOpts{Width: `1px; background: url(//evil)`})
	})
	if strings.Contains(bad, "evil") {
		t.Errorf("Width went into the style unchecked:\n%s", bad)
	}
}

// The trigger is a link: without JavaScript it goes to the page that answers
// the content whole. The script, and the dialog trigger with an address, are
// links too (#296, step 1).
func TestSheetTriggersAreLinks(t *testing.T) {
	if got := render(t, h.A(h.Href("/docs/7"), SheetOpen("leitor"))); !strings.Contains(got, `data-ui-sheet-open="leitor"`) || !strings.Contains(got, `aria-controls="leitor"`) || !strings.Contains(got, `aria-expanded="false"`) {
		t.Fatalf("SheetOpen: %s", got)
	}
	if got := render(t, SheetBody("leitor", h.Text("x"))); got != `<div class="ui-sheet-body" id="leitor-body">x</div>` {
		t.Fatalf("SheetBody: %s", got)
	}
	if got := render(t, SheetClose(h.Text("Close"))); !strings.Contains(got, `data-ui-sheet-close`) || !strings.Contains(got, `<button`) {
		t.Fatalf("SheetClose: %s", got)
	}
	got := render(t, DialogTrigger("ver", Swap("ver-body"), h.Href("/docs/7"), h.Text("See")))
	if !strings.HasPrefix(got, "<a") || !strings.Contains(got, `href="/docs/7"`) || !strings.Contains(got, `data-ui-dialog-open="ver"`) || strings.Contains(got, `type="button"`) {
		t.Fatalf("DialogTrigger with an address is a link: %s", got)
	}
	if got := render(t, DialogTrigger("ver", h.Text("See"))); !strings.HasPrefix(got, "<button") {
		t.Fatalf("DialogTrigger without an address stays a button: %s", got)
	}
}
