package migrate

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite the golden tree and the report")

const (
	nextDir   = "../../testdata/next"
	goldenDir = "../../testdata/next.golden"
)

// TestGoldenTree holds the two things the command promises: the same tree
// produces the same bytes, and the bytes are the ones committed.
func TestGoldenTree(t *testing.T) {
	p, err := Scan(nextDir)
	if err != nil {
		t.Fatal(err)
	}
	files := p.Files()
	files = append(files, File{Path: "MIGRATION.md", Body: Report(p, "en")})
	files = append(files, File{Path: "MIGRATION.pt.md", Body: Report(p, "pt")})
	if *update {
		if err := os.RemoveAll(goldenDir); err != nil {
			t.Fatal(err)
		}
		for _, f := range files {
			dst := filepath.Join(goldenDir, filepath.FromSlash(f.Path))
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(dst, []byte(f.Body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return
	}
	seen := map[string]bool{}
	for _, f := range files {
		seen[filepath.FromSlash(f.Path)] = true
		want, err := os.ReadFile(filepath.Join(goldenDir, filepath.FromSlash(f.Path)))
		if err != nil {
			t.Errorf("%s: not in the golden tree", f.Path)
			continue
		}
		if string(want) != f.Body {
			t.Errorf("%s differs from the golden (run make golden):\n%s", f.Path, f.Body)
		}
	}
	err = filepath.WalkDir(goldenDir, func(fp string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if rel, _ := filepath.Rel(goldenDir, fp); !seen[rel] {
			t.Errorf("%s is in the golden tree and was not generated", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestScanIsDeterministic runs the scan twice: a map iteration leaking into
// the output would show up here before it showed up in a diff.
func TestScanIsDeterministic(t *testing.T) {
	first, err := Scan(nextDir)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Scan(nextDir)
	if err != nil {
		t.Fatal(err)
	}
	if Report(first, "en") != Report(second, "en") {
		t.Error("two scans of the same tree wrote two reports")
	}
	a, b := first.Files(), second.Files()
	if len(a) != len(b) {
		t.Fatalf("%d files then %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("%s is not the same twice", a[i].Path)
		}
	}
}

// TestFolders checks the translation of every kind of segment the App Router
// has, which is the part a person would otherwise do by hand 47 times.
func TestFolders(t *testing.T) {
	p, err := Scan(nextDir)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"page.go":                  "/",
		"marketing-/about/page.go": "/about",
		"documents/page.go":        "/documents",
		"estudo/page.go":           "/estudo",
		"gravar/page.go":           "/gravar",
		"documents/id_/page.go":    "/documents/{id}",
		"users/user_id_/page.go":   "/users/{user_id}",
		"files/path__/page.go":     "/files/{path...}",
		"docs/slug__/page.go":      "/docs/{slug...}",
		"dashboard/page.go":        "/dashboard",
		"dashboard/layout.go":      "/dashboard",
		"api/documents/route.go":   "/api/documents",
		"api/health/route.go":      "/api/health",
		"not_found.go":             "/",
		"error.go":                 "/",
	}
	got := map[string]string{}
	for _, pg := range p.Pages {
		key := pg.File
		if pg.Dir != "" {
			key = pg.Dir + "/" + pg.File
		}
		got[key] = pg.URL
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
	for k := range got {
		if _, ok := want[k]; !ok {
			t.Errorf("%s was generated and is not expected", k)
		}
	}
}

// TestNotes: what has no equivalent is said, not written.
func TestNotes(t *testing.T) {
	p, err := Scan(nextDir)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		NoteParallel: true, NoteIntercept: true, NoteLoading: true, NoteTemplate: true,
		NoteMiddleware: true, NoteRewrite: true, NoteRootLayout: true, NoteRenamed: true,
		NoteOptionalAll: true,
	}
	for _, n := range p.Notes {
		delete(want, n.Code)
	}
	for code := range want {
		t.Errorf("no note of kind %q", code)
	}
	for _, f := range p.Files() {
		if strings.Contains(f.Path, "sidebar") || strings.Contains(f.Path, "modal") {
			t.Errorf("%s: a route with no equivalent must not be written", f.Path)
		}
	}
}

// TestClasses checks the suggestion on the three screens written to earn one.
func TestClasses(t *testing.T) {
	p, err := Scan(nextDir)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"dashboard/page.go":     ClassSPA,    // <svg> and onPointerDown
		"documents/id_/page.go": ClassIsland, // setInterval
		"documents/page.go":     ClassForm,   // a list and a filter
		"gravar/page.go":        ClassSPA,    // MediaRecorder: the browser is doing the work
		// The module it imports also has a FormData in it, in a function this
		// screen does not import: the class is what the screen uses (#167).
		"estudo/page.go": ClassForm,
	}
	for _, pg := range p.Pages {
		key := pg.File
		if pg.Dir != "" {
			key = pg.Dir + "/" + pg.File
		}
		if w, ok := want[key]; ok && pg.Class != w {
			t.Errorf("%s = %s (%s), want %s", key, pg.Class, pg.Why, w)
		}
	}
}

// TestEndpoints: the addresses come out of the text with the method they are
// called with, template literal and all.
func TestEndpoints(t *testing.T) {
	p, err := Scan(nextDir)
	if err != nil {
		t.Fatal(err)
	}
	var doc Page
	for _, pg := range p.Pages {
		if pg.Dir == "documents/id_" {
			doc = pg
		}
	}
	got := strings.Join(endpointStrings(doc.Endpoints), " | ")
	want := "GET /api/documents/:id | POST /api/documents/:id/reprocess | GET /api/documents/:id/status"
	if got != want {
		t.Errorf("endpoints = %q, want %q", got, want)
	}
}

// TestMethods: three exports, three handlers, and an arrow function counts.
func TestMethods(t *testing.T) {
	p, err := Scan(nextDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, pg := range p.Pages {
		switch pg.Dir {
		case "api/documents":
			if strings.Join(pg.Methods, ",") != "GET,POST,DELETE" {
				t.Errorf("methods = %v", pg.Methods)
			}
		case "api/health":
			if strings.Join(pg.Methods, ",") != "GET" {
				t.Errorf("health = %v", pg.Methods)
			}
		}
	}
}

// TestWriteKeepsWhatIsThere: the second run of the command must not cost the
// screens ported between the two.
func TestWriteKeepsWhatIsThere(t *testing.T) {
	p, err := Scan(nextDir)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if _, err := Write(dir, p, false); err != nil {
		t.Fatal(err)
	}
	ported := filepath.Join(dir, "documents", "page.go")
	if err := os.WriteFile(ported, []byte("package documents // ported by hand\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := Write(dir, p, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range res {
		if r.Action != Kept {
			t.Errorf("%s = %s, want kept", r.Path, r.Action)
		}
	}
	if b, _ := os.ReadFile(ported); !strings.Contains(string(b), "by hand") {
		t.Fatal("the ported file was overwritten")
	}
	if res, err = Write(dir, p, true); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(ported); strings.Contains(string(b), "by hand") {
		t.Fatal("--force must overwrite")
	}
}

// Spec 062 (#93): three things the report got wrong on a real Next application,
// each one of them enough to send the agent to the wrong screen first.
func TestReportReadsTheComponentBesideThePage(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// 1. The signal lives in the imported component, not in the thin page.
	write("app/fluxos/id_/page.tsx", `'use client'
import FlowCanvas from "../FlowCanvas"
export default function Page() { return <FlowCanvas /> }
`)
	write("app/fluxos/FlowCanvas.tsx", `'use client'
export default function FlowCanvas() {
  return <svg onPointerDown={() => {}} onPointerMove={() => {}} />
}
`)
	// 2. Dropping a file is an upload, not a pointer.
	write("app/upload/page.tsx", `'use client'
export default function Page() {
  return <div onDrop={e => e.dataTransfer.files} onDragOver={e => e.preventDefault()} />
}
`)
	// 3. A call with a TypeScript generic is still a call.
	write("app/documentos/page.tsx", `'use client'
import { apiGet } from "@/lib/api"
export default function Page() {
  const d = apiGet<DocsResponse>("/api/documents")
  return <div>{d}</div>
}
`)
	write("lib/api.ts", "export function apiGet<T>(u: string) { return fetch(u) }\n")

	p, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]Page{}
	for _, pg := range p.Pages {
		by[pg.Source] = pg
	}

	flow := by["app/fluxos/id_/page.tsx"]
	if flow.Class != ClassSPA {
		t.Errorf("the flow designer came back %s — %s; the signal is in the component it imports", flow.Class, flow.Why)
	}
	if !strings.Contains(flow.Why, "FlowCanvas.tsx") {
		t.Errorf("the reason does not say where the signal is: %q", flow.Why)
	}
	if flow.DepLines == 0 {
		t.Error("the size of the job does not count the component")
	}

	up := by["app/upload/page.tsx"]
	if up.Class != ClassIsland {
		t.Errorf("the upload screen came back %s — %s; a drop area is ui.Dropzone", up.Class, up.Why)
	}

	docs := by["app/documentos/page.tsx"]
	if len(docs.Endpoints) == 0 {
		t.Fatal("apiGet<T>(...) was not read as a call, so the Calls column is empty")
	}
	if docs.Endpoints[0].Path != "/api/documents" {
		t.Errorf("endpoint = %+v", docs.Endpoints[0])
	}
}

// Spec 091 (#140): measured on a real application, twenty screens out of twenty
// came back C, and almost all of them named the same file — the chat the shell
// carries. Reaching a file is not the same as being made of it.
func TestOShellNaoPromoveAsTelas(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// The frame: every page is wrapped by it, and it carries a chat that draws.
	write("app/layout.tsx", `import { Shell } from "@/components"
export default function RootLayout({ children }) {
  return <html><body><Shell>{children}</Shell></body></html>
}
`)
	write("components/index.ts", `export { default as Shell } from "./Shell"
export { default as Chat } from "./Chat"
export { default as DataTable } from "./DataTable"
`)
	write("components/Shell.tsx", `'use client'
import Chat from "./Chat"
export default function Shell({ children }) { return <div>{children}<Chat /></div> }
`)
	// The chat streams, which is what makes it work and not decoration —
	// spec 122 (#156) stopped reading the 16px <svg> beside it as a signal,
	// and a component that only draws an icon is not what this test is about.
	write("components/Chat.tsx", `'use client'
import { useState, useRef, useEffect } from "react"
export default function Chat() {
  const [m, setM] = useState([])
  const r = useRef(null)
  useEffect(() => { const es = new EventSource("/api/chat"); return () => es.close() }, [])
  return <aside ref={r}><svg viewBox="0 0 16 16" /></aside>
}
`)
	write("components/DataTable.tsx", `export default function DataTable({ rows }) {
  return <table><tbody>{rows.map(r => <tr key={r}><td>{r}</td></tr>)}</tbody></table>
}
`)

	// Two listings with nothing of their own: a table and a call. The barrel is
	// how a real project imports one component, and it is what dragged the chat
	// into every screen.
	for _, name := range []string{"usuarios", "rubricas"} {
		write("app/"+name+"/page.tsx", `import { DataTable } from "@/components"
export default async function Page() {
  const rows = await fetch("/api/`+name+`").then(r => r.json())
  return <DataTable rows={rows} />
}
`)
	}

	// And one screen that really is an island, so the fix cannot be "call
	// everything A": the component is this page's, nobody else imports it.
	write("app/mapa/page.tsx", `'use client'
import Mapa from "./Mapa"
export default function Page() { return <Mapa /> }
`)
	write("app/mapa/Mapa.tsx", `'use client'
export default function Mapa() { return <canvas onPointerMove={() => {}} /> }
`)

	p, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]Page{}
	for _, pg := range p.Pages {
		by[pg.Source] = pg
	}

	for _, name := range []string{"usuarios", "rubricas"} {
		pg := by["app/"+name+"/page.tsx"]
		if pg.Class != ClassForm {
			t.Errorf("%s veio %s — %s; a tela não tem sinal próprio, o sinal é do shell", name, pg.Class, pg.Why)
		}
		if len(pg.Deps) != 1 || pg.Deps[0] != "components/DataTable.tsx" {
			t.Errorf("%s trouxe os irmãos do barril: %v", name, pg.Deps)
		}
	}

	mapa := by["app/mapa/page.tsx"]
	if mapa.Class != ClassSPA {
		t.Errorf("o mapa veio %s — %s; o componente é só dele", mapa.Class, mapa.Why)
	}

	// The chat is work, and it is work done once: it belongs in the report as a
	// global dependency, not in twenty rows of the table.
	if len(p.Globals) == 0 {
		t.Fatal("o relatório não lista nenhuma dependência global")
	}
	var chat *Global
	for i, g := range p.Globals {
		if g.Path == "components/Chat.tsx" {
			chat = &p.Globals[i]
		}
	}
	if chat == nil {
		t.Fatalf("o chat não está entre as dependências globais: %+v", p.Globals)
	}
	if len(chat.Signals) == 0 {
		t.Errorf("a dependência global veio sem o sinal que a torna trabalho: %+v", chat)
	}
	md := Report(p, "pt")
	if n := strings.Count(md, "components/Chat.tsx"); n != 1 {
		t.Errorf("o chat aparece %d vezes no relatório; é para aparecer uma", n)
	}
}

// Spec 122 (#156): an inline <svg> is a logo. Reading it as a drawing surface
// sent four of the twenty screens of a real migration to C — the most
// conservative class there is — and pushed whoever was migrating to write an
// island where a server-rendered form was enough. What makes an <svg> a
// drawing is something working on it, not the tag.
func TestSvgDecorativoNaoEhDesenho(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// The logo of a login form: no handler, no ref, nothing draws on it.
	write("app/login/page.tsx", `'use client'
import { useState } from "react"
export default function Login() {
  const [email, setEmail] = useState("")
  return (
    <form method="post">
      <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" aria-hidden>
        <path d="M4 4h16v16H4z" />
      </svg>
      <input value={email} onChange={e => setEmail(e.target.value)} />
    </form>
  )
}
`)
	// The same tag with a ref on it is a surface somebody writes into.
	write("app/grafo/page.tsx", `'use client'
import { useRef } from "react"
export default function Grafo() {
  const r = useRef(null)
  return <svg ref={r} viewBox="0 0 800 600" />
}
`)
	// And the two that never depended on the <svg> to be C.
	write("app/mapa/page.tsx", `'use client'
export default function Mapa() { return <canvas /> }
`)
	write("app/desenho/page.tsx", `'use client'
export default function Desenho() {
  return <svg onPointerMove={() => {}}><circle r="4" /></svg>
}
`)

	p, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"login":   ClassForm,
		"grafo":   ClassSPA,
		"mapa":    ClassSPA,
		"desenho": ClassSPA,
	}
	for _, pg := range p.Pages {
		w, ok := want[pg.Dir]
		if !ok {
			continue
		}
		delete(want, pg.Dir)
		if pg.Class != w {
			t.Errorf("%s = %s — %s, want %s", pg.Dir, pg.Class, pg.Why, w)
		}
		// The reason carries the line, so a false positive can be thrown out
		// without opening the file.
		if pg.Class == ClassSPA && !strings.Contains(pg.Why, "line ") {
			t.Errorf("%s: the reason does not say where: %q", pg.Dir, pg.Why)
		}
	}
	for dir := range want {
		t.Errorf("%s was not scanned", dir)
	}
}

// Spec 131 (#167): a página era classificada pelos sinais de todo módulo que
// ela importa, e não pelos que ela de fato usa. No Verba, uma tela que lê um
// convite e define uma senha herdava `upload` e `storage` de funções que nunca
// chama — e C, ou B, é a classe que manda escrever ilha onde um formulário
// basta. O `import { a, b } from "x"` já diz quais nomes entraram.
func TestSinalDeModuloSegueOsNomesImportados(t *testing.T) {
	p, err := Scan("testdata/unused-import")
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]Page{}
	for _, pg := range p.Pages {
		by[pg.Source] = pg
	}

	convite := by["app/portal/convite/[token]/page.tsx"]
	if convite.Class != ClassForm {
		t.Errorf("o convite veio %s — %s; ele importa dois nomes e não chama o portalUpload", convite.Class, convite.Why)
	}
	if len(convite.Deps) == 0 {
		t.Error("o módulo importado não entrou em Deps — o tamanho do trabalho para de contar")
	}
	for _, d := range convite.Deps {
		if strings.HasSuffix(d, "storage.ts") {
			t.Errorf("o %s foi seguido; só o portalUpload o usa, e o convite não importa o portalUpload", d)
		}
	}

	// Com `import * as`, ninguém disse quais nomes entraram: o módulo conta
	// inteiro, como antes, e o motivo diz isso para a conferência ser barata.
	arquivos := by["app/portal/arquivos/page.tsx"]
	if arquivos.Class != ClassIsland {
		t.Errorf("os arquivos vieram %s — %s; o namespace import lê o módulo inteiro", arquivos.Class, arquivos.Why)
	}
	if !strings.Contains(arquivos.Why, "whole module") {
		t.Errorf("o motivo não diz que o módulo foi lido inteiro: %q", arquivos.Why)
	}

	// E o que o módulo faz por conta própria continua contando: um valor de
	// topo é avaliado no instante em que alguém importa o arquivo, então o
	// canal aberto ao lado das funções é de quem importa qualquer uma delas.
	eventos := by["app/portal/eventos/page.tsx"]
	if eventos.Class != ClassIsland || !strings.Contains(eventos.Why, "polling") {
		t.Errorf("os eventos vieram %s — %s; o EventSource do módulo abre ao importar", eventos.Class, eventos.Why)
	}
	if !strings.Contains(eventos.Why, "eventos.ts:3") {
		t.Errorf("o motivo não aponta a linha do canal: %q", eventos.Why)
	}
}

// Spec 131 (#181): medido no Prosa, 57 Server Actions exportadas em 15 arquivos
// e nenhum `fetch(` nas páginas. O relatório descrevia corretamente o que a
// página desenha e não dizia uma palavra sobre o que ela escreve: o `A` estava
// certo quanto ao formato e enganoso quanto ao tamanho do trabalho.
func TestServerActionsAparecemNoRelatorio(t *testing.T) {
	p, err := Scan("testdata/server-actions")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]Action{}
	for _, a := range p.Actions {
		got[a.Name] = a
	}
	// O arquivo com "use server" no topo: toda função exportada é ação — e só
	// as que alguma tela importa, que é a travessia por nome da #167.
	for _, name := range []string{"registrarResposta", "criarBaralho", "salvarPerfil"} {
		if _, ok := got[name]; !ok {
			t.Errorf("%s não está entre as ações: %+v", name, p.Actions)
		}
	}
	// E o que não é ação não é: a função exportada sem o "use server" ao lado
	// de uma que tem, e a que ninguém importa.
	for _, name := range []string{"formatarNome", "apagarBaralho", "Cards", "Perfil"} {
		if _, ok := got[name]; ok {
			t.Errorf("%s virou ação e não é", name)
		}
	}
	if a := got["registrarResposta"]; a.Path != "lib/estudo-actions.ts" || a.Line != 5 {
		t.Errorf("registrarResposta = %s:%d, quero lib/estudo-actions.ts:5", a.Path, a.Line)
	}
	// "use server" no corpo de uma função é só ela, e ela pode estar na página.
	if a := got["salvarPerfil"]; a.Path != "app/perfil/page.tsx" || a.Line != 3 {
		t.Errorf("salvarPerfil = %s:%d, quero app/perfil/page.tsx:3", a.Path, a.Line)
	}
	if a := got["criarBaralho"]; strings.Join(a.Screens, ", ") != "/cards/{deckId}" {
		t.Errorf("as telas do criarBaralho = %v", a.Screens)
	}

	var cards Page
	for _, pg := range p.Pages {
		if pg.Dir == "cards/deckId_" {
			cards = pg
		}
	}
	// Uma Server Action é o oposto de JavaScript no cliente: não é sinal de
	// ilha, e não muda a classe. O que ela muda é a contagem ao lado dela.
	if cards.Class != ClassForm {
		t.Errorf("a tela dos cards veio %s — %s; ação de servidor não é sinal de ilha", cards.Class, cards.Why)
	}
	if len(cards.Actions) != 2 {
		t.Errorf("a tela dos cards tem %d ações, quero 2: %+v", len(cards.Actions), cards.Actions)
	}

	md := Report(p, "en")
	for _, want := range []string{"## Server Actions", "`registrarResposta`", "`lib/estudo-actions.ts:5`", "· 2 server actions", "· 1 server action"} {
		if !strings.Contains(md, want) {
			t.Errorf("o relatório não traz %q", want)
		}
	}
	if strings.Contains(md, "apagarBaralho") {
		t.Error("o relatório lista uma ação que nenhuma tela importa")
	}
	if pt := Report(p, "pt"); !strings.Contains(pt, "ações de servidor") {
		t.Error("a tradução do relatório não conta as ações")
	}
}

// Spec 131 (#182): gravar voz é o caso mais puro de "o navegador está fazendo o
// trabalho" que existe — permissão de dispositivo, stream, um objeto com ciclo
// de vida próprio e um Blob no fim — e saía A, formulário. A #156 corrigiu um
// falso positivo, que custa uma conferência; este é um falso negativo, que faz
// alguém portar meia tela antes de descobrir o microfone.
func TestCapturaDeMidiaEhIlha(t *testing.T) {
	p, err := Scan("testdata/media")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]struct{ class, why string }{
		"licao":  {ClassSPA, "media capture"},
		"ditado": {ClassSPA, "media capture"},
		// Capturar é C porque depende de permissão e de um objeto vivo;
		// reproduzir é B, porque é um elemento que o servidor desenha.
		"ouvir": {ClassIsland, "media playback"},
	}
	for _, pg := range p.Pages {
		w, ok := want[pg.Dir]
		if !ok {
			continue
		}
		delete(want, pg.Dir)
		if pg.Class != w.class || !strings.Contains(pg.Why, w.why) {
			t.Errorf("%s = %s — %s, quero %s — %s", pg.Dir, pg.Class, pg.Why, w.class, w.why)
		}
		if pg.Dir == "licao" && !strings.Contains(pg.Why, "use-voice-recorder.ts:7") {
			t.Errorf("o motivo da lição não aponta o hook: %q", pg.Why)
		}
	}
	for dir := range want {
		t.Errorf("%s não foi varrido", dir)
	}
}
