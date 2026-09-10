package ui

import (
	"sort"
	"strconv"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// VersionRow is one entry of a history, as the screen shows it.
//
// Changed is what this version changed against the one before it — the field
// names, already worked out by the caller, because only the application knows
// what its fields are called in the language of whoever reads them.
type VersionRow struct {
	N         int
	By        string
	At        any
	Published bool
	Note      string
	Changed   []string
}

// VersionOpts configures the list.
type VersionOpts struct {
	// Restore is where the button posts, with the version number in a hidden
	// field. Empty draws no button, which is the read-only history.
	Restore string
	// Publish is where the publish button posts. Empty draws none.
	Publish string
	// CSRF is the token field; a form that changes what everybody reads
	// carries one.
	CSRF h.Node
	// Empty is what a thing with no version says.
	Empty string
}

// VersionList renders the history: who, when, what changed, and the buttons.
//
//	ui.VersionList(c, rows, ui.VersionOpts{Restore: "/admin/modelos/1/restaurar",
//		CSRF: trilha.CSRFInput(c)})
//
// There is no JavaScript and no diff library: what changed is a list of field
// names the caller worked out, which is the honest half of a diff — the half
// somebody reads before opening the version.
func VersionList(c *trilha.Ctx, rows []VersionRow, o VersionOpts) h.Node {
	if len(rows) == 0 {
		empty := o.Empty
		if empty == "" {
			empty = versionWords(c)["empty"]
		}
		return Empty(EmptyOpts{Icon: "info", Title: empty})
	}
	w := versionWords(c)
	// Newest first: the history of a thing is read from what it is now
	// backwards, and the version somebody wants is almost always the last one.
	list := append([]VersionRow{}, rows...)
	sort.Slice(list, func(i, j int) bool { return list[i].N > list[j].N })

	lines := make([]h.Node, 0, len(list))
	for _, r := range list {
		lines = append(lines, h.Tr(
			h.Td(VersionBadge(r.N, r.Published, w)),
			h.Td(h.Text(r.By)),
			h.Td(Date(c, r.At)),
			h.Td(changedCell(r, w)),
			h.Td(versionButtons(r, o, w)),
		))
	}
	return Table(
		h.Thead(h.Tr(
			h.Th(h.Text(w["version"])), h.Th(h.Text(w["by"])), h.Th(h.Text(w["when"])),
			h.Th(h.Text(w["changed"])), h.Th(h.Text("")),
		)),
		h.Tbody(lines...),
	)
}

// VersionBadge is "v3" — with the published mark when it is the one everybody
// reads. It is what goes in the header of a detail screen, so the number on the
// badge and the number in the list are the same thing.
//
//	ui.VersionBadge(v.N, v.Published)
//
// The words are optional: pass none and it reads in English, or hand it the
// same map VersionList uses when the screen is in another language.
func VersionBadge(n int, published bool, words ...map[string]string) h.Node {
	w := map[string]string{"published": "published"}
	if len(words) > 0 && words[0] != nil {
		w = words[0]
	}
	label := "v" + strconv.Itoa(n)
	if !published {
		return Badge(Outline(), h.Text(label))
	}
	return Badge(h.Class("ui-badge-success"), h.Text(label+" · "+w["published"]))
}

func changedCell(r VersionRow, w map[string]string) h.Node {
	if len(r.Changed) == 0 {
		if r.Note != "" {
			return Muted(h.Text(r.Note))
		}
		return Muted(h.Text(w["no change"]))
	}
	items := make([]h.Node, 0, len(r.Changed))
	for _, f := range r.Changed {
		items = append(items, Badge(Outline(), h.Text(f)))
	}
	if r.Note != "" {
		items = append(items, Muted(h.Text(" "+r.Note)))
	}
	return h.Div(items...)
}

func versionButtons(r VersionRow, o VersionOpts, w map[string]string) h.Node {
	var buttons []h.Node
	if o.Publish != "" && !r.Published {
		buttons = append(buttons, h.Form(h.Method("post"), h.Action(o.Publish),
			h.Class("ui-inline-form"), o.CSRF,
			h.Input(h.Type("hidden"), h.Name("n"), h.Value(strconv.Itoa(r.N))),
			Button(Sm(), h.Type("submit"), h.Text(w["publish"]),
				Confirm(w["publish"]+" v"+strconv.Itoa(r.N)+"?", w["publish hint"])),
		))
	}
	if o.Restore != "" && !r.Published {
		buttons = append(buttons, h.Form(h.Method("post"), h.Action(o.Restore),
			h.Class("ui-inline-form"), o.CSRF,
			h.Input(h.Type("hidden"), h.Name("n"), h.Value(strconv.Itoa(r.N))),
			Button(Outline(), Sm(), h.Type("submit"), h.Text(w["restore"]),
				Confirm(w["restore"]+" v"+strconv.Itoa(r.N)+"?", w["restore hint"])),
		))
	}
	if len(buttons) == 0 {
		return h.Fragment()
	}
	return h.Div(h.Class("ui-inline-form"), h.Group(buttons...))
}

// Changed is what two versions of the same struct differ in, by field name.
//
// It is here and not in the runtime because it is a screen's question: the
// answer is a list of names somebody reads, and comparing two maps of strings
// is the whole of it — a field that is not a string is compared by how it was
// written, which is what the reader sees anyway.
func Changed(before, after map[string]string) []string {
	seen := map[string]bool{}
	var out []string
	for k, v := range after {
		seen[k] = true
		if before[k] != v {
			out = append(out, k)
		}
	}
	for k := range before {
		if !seen[k] {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func versionWords(c *trilha.Ctx) map[string]string {
	if langOf(c) == "pt-BR" {
		return map[string]string{
			"version": "Versão", "by": "Quem", "when": "Quando", "changed": "Mudou",
			"empty": "Nenhuma versão ainda.", "no change": "sem mudança",
			"publish": "Publicar", "restore": "Voltar para esta",
			"published":    "publicada",
			"publish hint": "Passa a ser a versão que todo mundo lê.",
			"restore hint": "Cria uma versão nova com este conteúdo. Nada é apagado.",
		}
	}
	return map[string]string{
		"version": "Version", "by": "By", "when": "When", "changed": "Changed",
		"empty": "No version yet.", "no change": "no change",
		"publish": "Publish", "restore": "Go back to this",
		"published":    "published",
		"publish hint": "It becomes the version everybody reads.",
		"restore hint": "Creates a new version with this content. Nothing is deleted.",
	}
}
