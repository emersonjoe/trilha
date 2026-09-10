package migrate

import (
	"fmt"
	"strings"
)

// Report writes MIGRATION.md: one row per screen with what the source says
// about itself, and one list of everything that has no equivalent here. It is
// the map whoever ports the app reads — a person or an agent — so that the
// first question of every screen is answered before the file is opened.
//
// lang is "en" or "pt", like the rest of the CLI.
func Report(p Project, lang string) string {
	t := reportText(lang)
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", t["title"])
	fmt.Fprintf(&b, "%s\n\n", fmt.Sprintf(t["intro"], p.AppDir, len(p.Pages)))
	fmt.Fprintf(&b, "## %s\n\n", t["screens"])
	fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s |\n|---|---|---|---|---|---|\n",
		t["col.source"], t["col.file"], t["col.url"], t["col.client"], t["col.calls"], t["col.class"])
	for _, pg := range p.Pages {
		file := pg.File
		if pg.Dir != "" {
			file = pg.Dir + "/" + pg.File
		}
		client := t["no"]
		if pg.Client {
			client = t["yes"]
		}
		hooks := hookSummary(pg.Analysis)
		if hooks != "" {
			client += " (" + hooks + ")"
		}
		calls := strings.Join(endpointStrings(pg.Endpoints), "<br>")
		if calls == "" {
			calls = "—"
		}
		// Only a screen gets a suggestion: a route.ts has no screen and a
		// layout is a frame.
		class := "—"
		if pg.Kind == KindPage && pg.Class != "" {
			class = pg.Class + " — " + pg.Why
		}
		// The size is the page plus what it imports, because that is the size of
		// the job: a fifty-line page in front of a three-hundred-line component
		// is not a fifty-line port.
		size := fmt.Sprintf("%d", pg.Lines)
		if pg.DepLines > 0 {
			size = fmt.Sprintf("%d + %d", pg.Lines, pg.DepLines)
		}
		fmt.Fprintf(&b, "| `%s` (%s) | `%s` | `%s` | %s | %s | %s |\n",
			pg.Source, size, file, pg.URL, client, calls, class)
	}
	fmt.Fprintf(&b, "\n%s\n\n", t["rule"])
	// The frame, once. Without this section the chat the shell carries has two
	// bad places to be: inside every row of the table above, or nowhere.
	if len(p.Globals) > 0 {
		fmt.Fprintf(&b, "## %s\n\n%s\n\n", t["globals"], t["globals.intro"])
		for _, g := range p.Globals {
			fmt.Fprintf(&b, "- `%s` (%d %s): %s — %s %s.\n",
				g.Path, g.Lines, t["lines"], strings.Join(g.Signals, ", "), t["alone"], g.Class)
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "## %s\n\n", t["notes"])
	if len(p.Notes) == 0 {
		fmt.Fprintf(&b, "%s\n", t["notes.none"])
	}
	for _, n := range p.Notes {
		line := noteText(t, n)
		fmt.Fprintf(&b, "- `%s`: %s\n", n.Source, line)
	}
	fmt.Fprintf(&b, "\n## %s\n\n%s\n", t["next"], t["next.body"])
	return b.String()
}

// hookSummary is the hook count as it goes in a table cell.
func hookSummary(a Analysis) string {
	var parts []string
	for _, h := range hookNames {
		if n := a.Hooks[h]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, h))
		}
	}
	return strings.Join(parts, ", ")
}

// noteText renders one note in the report's language.
func noteText(t map[string]string, n Note) string {
	s := t["note."+n.Code]
	if s == "" {
		s = n.Code
	}
	if n.Detail != "" {
		s += " (" + n.Detail + ")"
	}
	return s
}

// reportText holds the two languages of the report. It is separate from the
// CLI's own table because these are the words of a generated document, not of
// a command.
func reportText(lang string) map[string]string {
	en := map[string]string{
		"title":   "Migration report",
		"intro":   "Read from `%s`: %d files with somewhere to go. The tree beside this file is the skeleton — folders, packages and signatures. What each screen does is still yours to port; this table is so you do not have to open them to find out.",
		"screens": "Screens",

		"col.source": "Source",
		"col.file":   "Here",
		"col.url":    "URL",
		"col.client": "Client",
		"col.calls":  "Calls",
		"col.class":  "Suggested",
		"yes":        "yes",
		"no":         "no",

		"rule": "The suggestion is mechanical, and it is here to be argued with: **C** when the file " +
			"shows a pointer handler, a drawing surface (`<svg>`, `<canvas>`) or an editor — the browser is " +
			"doing the work, so it becomes an island; **B** when it polls, opens a modal, has tabs or takes " +
			"a file — the kit does that without a bundle; **A** otherwise, which is a form and a list, and " +
			"the whole screen fits on the server.",

		"globals": "Global dependencies",
		"globals.intro": "Reached from a `layout.tsx`: the frame around every screen, not the work of any one " +
			"of them. It is ported once — into the layout, or into a single island inside it — and it does " +
			"not change the class of the screens it wraps.",
		"lines": "lines",
		"alone": "on its own it would be",

		"screens.none": "No screen was found.",
		"notes":        "No equivalent",
		"notes.none":   "Nothing: everything found has somewhere to go.",

		"note." + NoteParallel:    "parallel route: there is nothing like it here — render the slots as parts of the page",
		"note." + NoteIntercept:   "intercepting route: no equivalent — a modal over a page is ui.Dialog on the page that opens it",
		"note." + NoteLoading:     "loading state: the page arrives filled, so there is no moment to fill",
		"note." + NoteTemplate:    "template: a layout that remounts has no meaning without a client router",
		"note." + NoteDefault:     "default of a parallel route: no equivalent",
		"note." + NoteMiddleware:  "middleware: becomes app/<branch>/middleware.go, or Auth.Require() — there is no automatic translation",
		"note." + NoteRewrite:     "rewrite: becomes a line in Config.Upstreams, with the credential the proxy injects",
		"note." + NoteRootLayout:  "root layout: yours already exists and is a whole <html> document — port the head and the frame into it",
		"note." + NoteNestedFile:  "nested not-found/error: here they answer for the whole app, from app/",
		"note." + NoteRenamed:     "parameter renamed to a Go identifier — the URL and c.Param use the new name",
		"note." + NoteDuplicate:   "two folders answer the same address; the first one won",
		"note." + NoteOptionalAll: "optional catch-all: the address without the segment needs its own page",

		"next": "What to do next",
		"next.body": "1. `trilha gen` and `go build ./...`: the skeleton compiles as it is.\n" +
			"2. Port the **A** screens first — they are forms and lists, and they close fast.\n" +
			"3. `trilha ui describe` says what the kit has, so you do not have to guess at a name.\n" +
			"4. The cookbook page *From Next.js to Trilha* has the React pattern beside the line that replaces it.",
	}
	if lang != "pt" {
		return en
	}
	pt := map[string]string{
		"title":   "Relatório de migração",
		"intro":   "Lido de `%s`: %d arquivos com para onde ir. A árvore ao lado deste arquivo é o esqueleto — pastas, pacotes e assinaturas. O que cada tela faz continua sendo trabalho de quem porta; esta tabela existe para você não precisar abrir uma por uma para descobrir.",
		"screens": "Telas",

		"col.source": "Origem",
		"col.file":   "Aqui",
		"col.url":    "URL",
		"col.client": "Cliente",
		"col.calls":  "Chama",
		"col.class":  "Sugestão",
		"yes":        "sim",
		"no":         "não",

		"rule": "A sugestão é mecânica, e está aqui para ser contestada: **C** quando o arquivo mostra " +
			"tratador de ponteiro, superfície de desenho (`<svg>`, `<canvas>`) ou editor — quem trabalha é o " +
			"browser, então vira ilha; **B** quando faz polling, abre modal, tem abas ou recebe arquivo — o " +
			"kit faz isso sem bundle; **A** no resto, que é formulário e lista, e cabe inteiro no servidor.",

		"globals": "Dependências globais",
		"globals.intro": "Alcançadas a partir de um `layout.tsx`: a moldura de todas as telas, e trabalho de " +
			"nenhuma delas em particular. Porta-se uma vez — no layout, ou numa ilha só — e não muda a classe " +
			"das telas que envolve.",
		"lines": "linhas",
		"alone": "sozinha seria",

		"screens.none": "Nenhuma tela encontrada.",
		"notes":        "Sem equivalente",
		"notes.none":   "Nada: tudo que foi encontrado tem para onde ir.",

		"note." + NoteParallel:    "rota paralela: não há equivalente — renderize os slots como partes da página",
		"note." + NoteIntercept:   "rota interceptadora: sem equivalente — modal sobre uma página é ui.Dialog na página que abre",
		"note." + NoteLoading:     "estado de carregando: a página chega pronta, então não há instante para preencher",
		"note." + NoteTemplate:    "template: layout que remonta não significa nada sem roteador no cliente",
		"note." + NoteDefault:     "default de rota paralela: sem equivalente",
		"note." + NoteMiddleware:  "middleware: vira app/<ramo>/middleware.go, ou Auth.Require() — não há tradução automática",
		"note." + NoteRewrite:     "rewrite: vira uma linha de Config.Upstreams, com a credencial que o proxy injeta",
		"note." + NoteRootLayout:  "layout raiz: o seu já existe e é um documento <html> inteiro — porte o head e a moldura para ele",
		"note." + NoteNestedFile:  "not-found/error aninhado: aqui eles respondem pelo app inteiro, a partir de app/",
		"note." + NoteRenamed:     "parâmetro renomeado para um identificador Go — a URL e o c.Param usam o nome novo",
		"note." + NoteDuplicate:   "duas pastas respondem pelo mesmo endereço; ficou a primeira",
		"note." + NoteOptionalAll: "catch-all opcional: o endereço sem o segmento precisa de uma página própria",

		"next": "O que fazer agora",
		"next.body": "1. `trilha gen` e `go build ./...`: o esqueleto compila como está.\n" +
			"2. Porte primeiro as telas **A** — são formulários e listas, e fecham rápido.\n" +
			"3. `trilha ui describe` diz o que o kit tem, para você não chutar um nome.\n" +
			"4. A receita *Do Next.js para a Trilha* traz o padrão de React ao lado da linha que ocupa o lugar dele.",
	}
	for k, v := range en {
		if _, ok := pt[k]; !ok {
			pt[k] = v
		}
	}
	return pt
}
