package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A heurística que a issue pediu: a consulta que esqueceu a coluna é invisível
// na revisão e muito visível na contagem.
func TestTenantGapsApontaAConsultaQueEsqueceu(t *testing.T) {
	dir := t.TempDir()
	src := "package repo\n\n" +
		"const listar = `SELECT id, nome FROM documents WHERE tenant_id = $1 ORDER BY nome`\n" +
		"const buscar = `SELECT id FROM documents WHERE tenant_id = $1 AND nome ILIKE $2`\n" +
		"const contar = `SELECT count(*) FROM documents`\n" +
		"const outra  = `SELECT id FROM logs ORDER BY at DESC`\n"
	if err := os.WriteFile(filepath.Join(dir, "repo.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	gaps := tenantGaps(dir)
	if len(gaps) != 1 {
		t.Fatalf("achou %d: %v", len(gaps), gaps)
	}
	// Diz a tabela, a contagem e a linha — "possível problema" não serve.
	if !strings.Contains(gaps[0], "documents") || !strings.Contains(gaps[0], "repo.go:5") {
		t.Fatalf("mensagem = %q", gaps[0])
	}
	// A tabela que ninguém filtra não é achado: ela não tem a coluna.
	if strings.Contains(gaps[0], "logs") {
		t.Fatalf("acusou uma tabela sem tenant: %q", gaps[0])
	}
}

// Uma consulta filtrada não é padrão; a segunda é. O limiar é baixo de
// propósito, e ainda assim não acusa o caso de uma só.
func TestTenantGapsNaoAcusaComUmaSoConsultaFiltrada(t *testing.T) {
	dir := t.TempDir()
	src := "package repo\n\n" +
		"const a = `SELECT id FROM notas WHERE tenant_id = $1`\n" +
		"const b = `SELECT count(*) FROM notas`\n"
	os.WriteFile(filepath.Join(dir, "repo.go"), []byte(src), 0o644)
	if gaps := tenantGaps(dir); len(gaps) != 0 {
		t.Fatalf("acusou cedo demais: %v", gaps)
	}
}

// Teste não conta: um _test.go cheio de consultas de fixture faria a ferramenta
// acusar o que não existe em produção.
func TestTenantGapsIgnoraTestes(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "repo.go"), []byte(
		"package repo\n\nconst a = `SELECT id FROM docs WHERE tenant_id = $1`\nconst b = `SELECT id FROM docs WHERE tenant_id = $1 AND x = 1`\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "repo_test.go"), []byte(
		"package repo\n\nconst c = `SELECT id FROM docs`\n"), 0o644)
	if gaps := tenantGaps(dir); len(gaps) != 0 {
		t.Fatalf("acusou um teste: %v", gaps)
	}
}
