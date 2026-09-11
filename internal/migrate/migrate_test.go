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
