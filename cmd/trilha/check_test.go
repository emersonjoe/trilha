package main

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha/internal/gen"
)

var update = flag.Bool("update", false, "rewrite the golden files")

func appProject(t *testing.T, name, module string) *project {
	t.Helper()
	return &project{Root: filepath.Join("..", "..", "testdata", "apps", name), Module: module}
}

// Um projeto que não passa no primeiro portão não diz nada sobre os outros:
// eles ficam em "not run", e o relatório inteiro cabe numa leitura.
func TestCheckParaNoPrimeiroPortao(t *testing.T) {
	r := runCheck(appProject(t, "err_no_page_func", "example.com/x"), false)
	if r.OK {
		t.Fatal("um projeto que nem gera trilha_gen.go não pode passar")
	}
	if r.Steps[0].Tool != "gen" || r.Steps[0].Status != statusFailed {
		t.Fatalf("primeiro passo: %+v", r.Steps[0])
	}
	for _, s := range r.Steps[1:] {
		if s.Status != statusNotRun {
			t.Errorf("%s rodou depois de gen falhar: %s", s.Tool, s.Status)
		}
	}
	if len(r.Problems) == 0 {
		t.Fatal("falhou sem dizer o quê")
	}
	p := r.Problems[0]
	if p.File == "" || p.Line == 0 || p.Fix == "" {
		t.Errorf("problema sem onde nem conserto: %+v", p)
	}
}

// O conserto tem que aparecer também para quem lê com os olhos.
func TestCheckTextoTrazOConserto(t *testing.T) {
	out := runCheck(appProject(t, "err_no_page_func", "example.com/x"), false).text()
	if !strings.Contains(out, "✗ gen") {
		t.Errorf("sem o passo que falhou:\n%s", out)
	}
	if !strings.Contains(out, "→ ") {
		t.Errorf("sem o conserto:\n%s", out)
	}
}

// --json é contrato: um agente lê este formato, não a nossa prosa.
func TestCheckGolden(t *testing.T) {
	r := runCheck(appProject(t, "err_no_page_func", "example.com/x"), false)
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	b = append(b, '\n')
	path := filepath.Join("..", "..", "testdata", "golden", "check.json.golden")
	if *update {
		if err := os.WriteFile(path, b, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != string(want) {
		t.Errorf("o relatório mudou de forma; make golden regrava\n--- quero\n%s\n--- tenho\n%s", want, b)
	}
}

// --fix existe para o caso mais comum de todos: a rota nova que ninguém gerou.
func TestCheckFixGeraOArquivo(t *testing.T) {
	dir := t.TempDir()
	copyTree(t, filepath.Join("..", "..", "testdata", "apps", "minimal"), dir)
	p := &project{Root: dir, Module: "example.com/minimal"}

	if status, probs := checkStepGen(p, false); status != statusFailed || len(probs) == 0 {
		t.Fatalf("sem trilha_gen.go o portão tem que fechar: %s %+v", status, probs)
	}
	if status, _ := checkStepGen(p, true); status != statusFixed {
		t.Fatalf("--fix não gerou: %s", status)
	}
	if _, err := os.Stat(filepath.Join(dir, gen.FileName)); err != nil {
		t.Fatal(err)
	}
	if status, _ := checkStepGen(p, false); status != statusOK {
		t.Fatalf("depois de gerado: %s", status)
	}
}

// Toda ferramenta Go se reporta como file:line:col: mensagem; é daí que sai o
// {file, line, message} do relatório.
func TestCheckPosicao(t *testing.T) {
	cases := []struct {
		in   string
		file string
		line int
		msg  string
	}{
		{"app/page.go:12:3: undefined: h.Divv", "app/page.go", 12, "undefined: h.Divv"},
		{"./main.go:4: imported and not used: \"fmt\"", "main.go", 4, "imported and not used: \"fmt\""},
		{"store/item.go:9:2: Errorf format %d has arg s of wrong type string", "store/item.go", 9, "Errorf format %d has arg s of wrong type string"},
		{"# example.com/x", "", 0, ""},
		{"FAIL example.com/x 0.2s", "", 0, ""},
	}
	for _, c := range cases {
		file, line, msg := position(c.in)
		if file != c.file || line != c.line || msg != c.msg {
			t.Errorf("position(%q) = %q %d %q", c.in, file, line, msg)
		}
	}
}

// O openapi.json guarda o título, a versão e o servidor com que foi escrito;
// regerar com outros faria o check acusar uma diferença que não existe.
func TestCheckOpenAPIUsaOQueEstaNoArquivo(t *testing.T) {
	o := optionsOf([]byte(`{"info":{"title":"Blog","version":"2.1.0"},"servers":[{"url":"https://api.example.com"}]}`))
	if o.Title != "Blog" || o.Version != "2.1.0" || o.Server != "https://api.example.com" {
		t.Errorf("%+v", o)
	}
}

func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}
