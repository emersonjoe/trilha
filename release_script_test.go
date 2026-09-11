//go:build !windows

package trilha

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// O ritual de release escreve no remoto em passos, e o primeiro deles — fundir
// a main — não tem volta. Estes testes montam um repositório de mentira com um
// "GitHub" que recusa tag, que é exatamente o que aconteceu na 0.102.0 e na
// 0.103.0, e conferem as duas coisas que a #172 pediu: que a falha diga o que
// ficou para trás, e que rodar de novo termine em vez de recusar.

// repoDeRelease monta o repositório, o remoto e os programas de mentira que o
// script chama. Devolve o diretório de trabalho e o PATH a usar.
func repoDeRelease(t *testing.T) (dir, bin, origin string) {
	t.Helper()
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("sem bash")
	}
	raiz := t.TempDir()
	dir = filepath.Join(raiz, "repo")
	origin = filepath.Join(raiz, "origin.git")
	bin = filepath.Join(raiz, "bin")

	// `gh` e `make` de mentira: registram a chamada e saem bem. O `gh release
	// view` sai mal, que é o que o comando de verdade faz quando a release
	// ainda não existe.
	must(t, os.MkdirAll(bin, 0o755))
	escreve(t, filepath.Join(bin, "gh"), `#!/usr/bin/env bash
echo "gh $*" >> "$RELEASE_LOG"
if [ "$1 $2" = "release view" ]; then cat >/dev/null; exit 1; fi
cat >/dev/null
exit 0
`)
	escreve(t, filepath.Join(bin, "make"), `#!/usr/bin/env bash
echo "make $*" >> "$RELEASE_LOG"
exit 0
`)
	for _, n := range []string{"gh", "make"} {
		must(t, os.Chmod(filepath.Join(bin, n), 0o755))
	}

	must(t, os.MkdirAll(filepath.Join(dir, "cmd", "trilha"), 0o755))
	must(t, os.MkdirAll(filepath.Join(dir, "scripts"), 0o755))
	script, err := filepath.Abs(filepath.Join("scripts", "release.sh"))
	must(t, err)
	b, err := os.ReadFile(script)
	must(t, err)
	escreve(t, filepath.Join(dir, "scripts", "release.sh"), string(b))
	must(t, os.Chmod(filepath.Join(dir, "scripts", "release.sh"), 0o755))
	escreve(t, filepath.Join(dir, "CHANGELOG.md"), "# Changelog\n\n## 9.9.9 — 2026-01-01\n\nO que esta versão faz.\n\n## 9.9.8 — 2025-12-01\n\nA anterior.\n")
	escreve(t, filepath.Join(dir, "ROADMAP.md"), "# Roadmap\n\n## Onde o Trilha está (janeiro de 2026, v9.9.9)\n")
	escreve(t, filepath.Join(dir, "cmd", "trilha", "main.go"), "package main\n\nconst version = \"9.9.9\"\n")

	git(t, dir, "init", "-q", "-b", "main")
	git(t, dir, "config", "user.email", "teste@exemplo.com")
	git(t, dir, "config", "user.name", "Teste")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-q", "-m", "base")
	git(t, raiz, "init", "-q", "--bare", origin)
	git(t, dir, "remote", "add", "origin", origin)
	git(t, dir, "push", "-q", "origin", "main")

	// O branch da spec, porque o script recusa rodar a partir da main.
	git(t, dir, "checkout", "-q", "-b", "spec")
	escreve(t, filepath.Join(dir, "feature.txt"), "a mudança da spec\n")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-q", "-m", "a spec")
	return dir, bin, origin
}

// recusaTag põe no remoto um gancho que recusa qualquer refs/tags/ — a
// credencial que empurra branch e não tag, que é o caso real.
func recusaTag(t *testing.T, origin string, on bool) {
	t.Helper()
	p := filepath.Join(origin, "hooks", "pre-receive")
	if !on {
		must(t, os.RemoveAll(p))
		return
	}
	must(t, os.MkdirAll(filepath.Dir(p), 0o755))
	escreve(t, p, `#!/usr/bin/env bash
while read -r _ _ ref; do
	case "$ref" in refs/tags/*) echo "403 Forbidden" >&2; exit 1 ;; esac
done
exit 0
`)
	must(t, os.Chmod(p, 0o755))
}

// roda executa o script e devolve a saída junta e se ele saiu bem.
func roda(t *testing.T, dir, bin string, args ...string) (string, bool) {
	t.Helper()
	log := filepath.Join(t.TempDir(), "chamadas.log")
	c := exec.Command("./scripts/release.sh", args...)
	c.Dir = dir
	c.Env = append(os.Environ(),
		"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"RELEASE_LOG="+log)
	out, err := c.CombinedOutput()
	chamadas, _ := os.ReadFile(log)
	return string(out) + string(chamadas), err == nil
}

// O ensaio percorre o ritual inteiro sem escrever em lugar nenhum.
func TestReleaseDryRunMostraORitual(t *testing.T) {
	dir, bin, _ := repoDeRelease(t)
	out, ok := roda(t, dir, bin, "9.9.9", "--issues", "172", "--dry-run")
	if !ok {
		t.Fatalf("o ensaio falhou:\n%s", out)
	}
	for _, quero := range []string{
		"git push origin HEAD:main", "git tag -a v9.9.9", "git push origin v9.9.9",
		"gh release create|edit v9.9.9", "gh issue close 172",
	} {
		if !strings.Contains(out, quero) {
			t.Errorf("o ensaio não mostra %q:\n%s", quero, out)
		}
	}
	// Ensaio não escreve: nem tag local, nem nada no remoto.
	if _, err := os.Stat(filepath.Join(dir, ".git", "refs", "tags", "v9.9.9")); err == nil {
		t.Error("o ensaio criou a tag")
	}
}

// #172 — o caso que aconteceu duas vezes: a main funde e a tag é recusada. O
// que a pessoa lê não pode ser só o erro do curl.
func TestReleaseDizOQueFicouParaTras(t *testing.T) {
	dir, bin, origin := repoDeRelease(t)
	recusaTag(t, origin, true)

	out, ok := roda(t, dir, bin, "9.9.9", "--issues", "172")
	if ok {
		t.Fatalf("a tag foi recusada e o script saiu bem:\n%s", out)
	}
	if !strings.Contains(out, "a main já foi fundida em") {
		t.Fatalf("a saída não diz que a main já foi:\n%s", out)
	}
	for _, quero := range []string{
		"scripts/release.sh 9.9.9 --issues \"172\"", // rodar de novo termina
		"git push origin v9.9.9",                    // ou à mão
		"gh issue close 172",
	} {
		if !strings.Contains(out, quero) {
			t.Errorf("a saída não diz o que falta (%q):\n%s", quero, out)
		}
	}
	// E a main foi mesmo: é isso que torna a mensagem necessária.
	if got := revParse(t, origin, "refs/heads/main"); got != revParse(t, dir, "HEAD") {
		t.Fatal("a main não foi fundida, então este teste não está medindo o caso")
	}
	// A tag local ficou, apontando para o HEAD: é por ela que a retomada passa.
	if revParse(t, dir, "refs/tags/v9.9.9^{commit}") != revParse(t, dir, "HEAD") {
		t.Fatal("a tag local não ficou no commit certo")
	}
}

// E rodar de novo termina o que falta em vez de recusar por causa da tag que
// ficou. Antes da #172 esta segunda execução parava em "a tag já existe".
func TestReleaseRetomaDepoisDaFalha(t *testing.T) {
	dir, bin, origin := repoDeRelease(t)
	recusaTag(t, origin, true)
	if _, ok := roda(t, dir, bin, "9.9.9", "--issues", "172"); ok {
		t.Fatal("a primeira execução devia ter falhado")
	}
	recusaTag(t, origin, false)

	out, ok := roda(t, dir, bin, "9.9.9", "--issues", "172")
	if !ok {
		t.Fatalf("a retomada falhou:\n%s", out)
	}
	if !strings.Contains(out, "retomando o que falta") {
		t.Errorf("a retomada não se anunciou:\n%s", out)
	}
	if got := revParse(t, origin, "refs/tags/v9.9.9^{commit}"); got != revParse(t, dir, "HEAD") {
		t.Fatalf("a tag não chegou ao remoto na retomada:\n%s", out)
	}
	for _, quero := range []string{"make test", "gh release create v9.9.9", "gh issue close 172"} {
		if !strings.Contains(out, quero) {
			t.Errorf("a retomada pulou %q:\n%s", quero, out)
		}
	}
}

// A tolerância é para a tag deste commit e só. A mesma versão apontando para
// outra coisa é a versão sendo remarcada em cima de outro código.
func TestReleaseRecusaTagEmOutroCommit(t *testing.T) {
	dir, bin, _ := repoDeRelease(t)
	antes := revParse(t, dir, "HEAD~1")
	git(t, dir, "tag", "-a", "v9.9.9", "-m", "v9.9.9", antes)

	out, ok := roda(t, dir, bin, "9.9.9")
	if ok {
		t.Fatalf("a tag em outro commit passou:\n%s", out)
	}
	if !strings.Contains(out, "aponta para") {
		t.Fatalf("a recusa não diz o motivo:\n%s", out)
	}
	if strings.Contains(out, "main pelo remoto") {
		t.Fatal("a recusa aconteceu depois de mexer no remoto")
	}
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func revParse(t *testing.T, dir, ref string) string {
	t.Helper()
	c := exec.Command("git", "rev-parse", ref)
	c.Dir = dir
	out, err := c.Output()
	if err != nil {
		t.Fatalf("git rev-parse %s em %s: %v", ref, dir, err)
	}
	return strings.TrimSpace(string(out))
}

func escreve(t *testing.T, path, body string) {
	t.Helper()
	must(t, os.WriteFile(path, []byte(body), 0o644))
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
