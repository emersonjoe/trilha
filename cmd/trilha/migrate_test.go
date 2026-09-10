package main

import (
	"os"
	"path/filepath"
	"testing"
)

// #146 — o relatório começa dizendo "a árvore ao lado deste arquivo é o
// esqueleto". Com --out longe, a árvore ia para lá e o arquivo ficava no
// diretório de onde o comando foi rodado.
func TestMigrateRelatorioFicaAoLadoDaArvore(t *testing.T) {
	repo, _ := filepath.Abs(filepath.Join("..", ".."))
	fonte := filepath.Join(repo, "testdata", "next")

	// Com --out longe, o relatório vai junto.
	longe := t.TempDir()
	rodaMigrate(t, longe, fonte, "--out", filepath.Join(longe, "mig", "app"))
	if _, err := os.Stat(filepath.Join(longe, "mig", "MIGRATION.md")); err != nil {
		t.Fatalf("o relatório não foi para o lado da árvore: %v", err)
	}

	// Sem --out, fica como sempre foi: ./MIGRATION.md ao lado de ./app.
	aqui := t.TempDir()
	rodaMigrate(t, aqui, fonte)
	if _, err := os.Stat(filepath.Join(aqui, "MIGRATION.md")); err != nil {
		t.Fatalf("o padrão de sempre mudou: %v", err)
	}

	// E --report continua mandando.
	dado := t.TempDir()
	rodaMigrate(t, dado, fonte, "--report", filepath.Join(dado, "outro.md"))
	if _, err := os.Stat(filepath.Join(dado, "outro.md")); err != nil {
		t.Fatalf("o --report explícito foi ignorado: %v", err)
	}
}

// rodaMigrate roda o comando com o cwd em dir, que é o que decide o padrão.
func rodaMigrate(t *testing.T, dir, fonte string, args ...string) {
	t.Helper()
	antes, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(antes) })
	if err := cmdMigrateNext(append([]string{fonte}, args...)); err != nil {
		t.Fatalf("migrate next: %v", err)
	}
	if err := os.Chdir(antes); err != nil {
		t.Fatal(err)
	}
}
