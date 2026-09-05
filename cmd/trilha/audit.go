package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/emersonjoe/trilha/internal/gen"
	"github.com/emersonjoe/trilha/internal/scan"
)

type check struct {
	level string // ok | aviso | critico
	title string
	hint  string
}

func cmdAudit(args []string) error {
	fs := flag.NewFlagSet("audit", flag.ContinueOnError)
	noVuln := fs.Bool("no-vuln", false, "não rodar govulncheck (sem rede)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	p, err := findProject()
	if err != nil {
		return err
	}
	checks := runAudit(p, !*noVuln)
	critical := 0
	for _, c := range checks {
		mark := map[string]string{"ok": "✓", "aviso": "!", "critico": "✗"}[c.level]
		fmt.Printf("%s %s\n", mark, c.title)
		if c.hint != "" {
			fmt.Printf("    %s\n", c.hint)
		}
		if c.level == "critico" {
			critical++
		}
	}
	if critical > 0 {
		return fmt.Errorf("%d item(ns) crítico(s)", critical)
	}
	fmt.Println("\nNenhum item crítico. Revise os avisos antes de publicar.")
	return nil
}

func runAudit(p *project, vuln bool) []check {
	var out []check
	add := func(level, title, hint string) { out = append(out, check{level, title, hint}) }

	// Secret.
	if s := os.Getenv("TRILHA_SECRET"); s == "" {
		add("critico", "TRILHA_SECRET não definido neste ambiente", "cookies assinados (sessão) não funcionam em produção; gere com: openssl rand -base64 32")
	} else if len(s) < 32 {
		add("critico", "TRILHA_SECRET curto demais", "use ao menos 32 bytes")
	} else {
		add("ok", "TRILHA_SECRET definido", "")
	}
	if os.Getenv("TRILHA_TRUSTED_PROXIES") == "" {
		add("aviso", "TRILHA_TRUSTED_PROXIES não definido", "atrás de um proxy (nginx, load balancer) defina os CIDRs para HSTS, IP do cliente e rate limit corretos")
	} else {
		add("ok", "TRILHA_TRUSTED_PROXIES definido", "")
	}

	// Generated file up to date.
	res, err := scan.Scan(p.Root, p.Module)
	if err != nil {
		add("critico", "app/ com convenções inválidas", err.Error())
	} else if src, err := gen.Generate(res); err == nil {
		cur, _ := os.ReadFile(filepath.Join(p.Root, gen.FileName))
		if string(cur) != string(src) {
			add("aviso", "trilha_gen.go desatualizado", "rode: trilha gen")
		} else {
			add("ok", "trilha_gen.go atualizado", "")
		}
	}

	// Go version.
	v := strings.TrimPrefix(runtime.Version(), "go")
	if strings.HasPrefix(v, "1.2") && v < "1.22" {
		add("critico", "Go "+v+" sem suporte", "o Trilha exige Go 1.22+")
	} else {
		add("ok", "Go "+v, "")
	}

	// .gitignore.
	if gi, err := os.ReadFile(filepath.Join(p.Root, ".gitignore")); err != nil || !strings.Contains(string(gi), ".trilha") {
		add("aviso", ".gitignore sem .trilha/ e bin/", "binários temporários podem ir para o git")
	} else {
		add("ok", ".gitignore cobre .trilha/ e bin/", "")
	}

	// go vet.
	if outb, err := runCmd(p.Root, "go", "vet", "./..."); err != nil {
		add("aviso", "go vet encontrou problemas", strings.TrimSpace(string(outb)))
	} else {
		add("ok", "go vet limpo", "")
	}

	// govulncheck (optional, needs network).
	if vuln {
		if outb, err := runCmd(p.Root, "go", "run", "golang.org/x/vuln/cmd/govulncheck@latest", "./..."); err != nil {
			txt := strings.TrimSpace(string(outb))
			if strings.Contains(txt, "Vulnerability") || strings.Contains(txt, "vulnerabilit") {
				add("critico", "govulncheck encontrou vulnerabilidades", lastLines(txt, 8))
			} else {
				add("aviso", "govulncheck não pôde rodar", "sem rede? use --no-vuln; "+lastLines(txt, 2))
			}
		} else {
			add("ok", "govulncheck sem vulnerabilidades conhecidas", "")
		}
	}
	return out
}

func runCmd(dir, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	return cmd.CombinedOutput()
}

func lastLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n    ")
}
