package main

import (
	"os"
	"strings"
)

// lang is the CLI language: "en" (default) or "pt".
var lang = detectLang(os.Getenv)

// detectLang picks the CLI language from TRILHA_LANG, then LC_ALL,
// LC_MESSAGES and LANG. A value starting with "pt" (any case) selects
// Portuguese; anything else, including no value, selects English.
func detectLang(getenv func(string) string) string {
	for _, k := range []string{"TRILHA_LANG", "LC_ALL", "LC_MESSAGES", "LANG"} {
		v := strings.TrimSpace(getenv(k))
		if v == "" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(v), "pt") {
			return "pt"
		}
		return "en"
	}
	return "en"
}

// t returns the message for key in the CLI language. Unknown keys return
// the key itself so a typo is visible instead of silent.
func t(key string) string {
	m, ok := msgs[key]
	if !ok {
		return key
	}
	if lang == "pt" && m[1] != "" {
		return m[1]
	}
	return m[0]
}

// msgs holds every user-facing CLI message: {English, Portuguese}.
var msgs = map[string][2]string{
	"usage": {`trilha — Next.js-style web framework for Go with file-based routing

Usage:
  trilha new <dir> [--module path] [--lang en|pt]   create a new project
  trilha gen                                        generate trilha_gen.go from app/
  trilha dev [--addr :3000]                         dev server with live reload
  trilha build [-o bin/<name>]                      generate + compile a single binary
  trilha routes                                     list the discovered routes
  trilha export [-o out] [--base /prefix]           export the static pages as HTML
  trilha audit [--no-vuln]                          check the project's security and configuration
  trilha ui [--force] [--css-only|--js-only]        write/update the ui kit in public/
  trilha version

Language: TRILHA_LANG=en|pt (falls back to LC_ALL, LC_MESSAGES, LANG).
`, `trilha — framework web para Go com roteamento por arquivos

Uso:
  trilha new <dir> [--module caminho] [--lang en|pt]  cria um projeto novo
  trilha gen                                          gera trilha_gen.go a partir de app/
  trilha dev [--addr :3000]                           dev server com recarga automática
  trilha build [-o bin/<nome>]                        gera + compila um binário único
  trilha routes                                       lista as rotas descobertas
  trilha export [-o out] [--base /prefixo]            exporta as páginas estáticas em HTML
  trilha audit [--no-vuln]                            verifica segurança e configuração do projeto
  trilha ui [--force] [--css-only|--js-only]          grava/atualiza o kit ui em public/
  trilha version

Idioma: TRILHA_LANG=en|pt (senão LC_ALL, LC_MESSAGES, LANG).
`},
	"unknown command": {"unknown command: %s\n\n%s", "comando desconhecido: %s\n\n%s"},
	"error:":          {"error:", "erro:"},
	"no app dir":      {"app/ directory not found: run at the project root (or use `trilha new`)", "pasta app/ não encontrada: rode na raiz do projeto (ou use `trilha new`)"},
	"no go.mod":       {"go.mod not found above %s", "go.mod não encontrado acima de %s"},
	"no module line":  {"%s: `module` line not found", "%s: linha `module` não encontrada"},
	"gen done":        {"✓ %s (%d routes)\n", "✓ %s (%d rotas)\n"},
	"routes header":   {"%-22s %-32s %s\n", "%-22s %-32s %s\n"},
	"METHODS":         {"METHODS", "MÉTODOS"},
	"PATTERN":         {"PATTERN", "PADRÃO"},
	"SOURCE":          {"SOURCE", "ORIGEM"},

	// new
	"flag module":     {"Go module path (default: folder name)", "caminho do módulo Go (padrão: nome da pasta)"},
	"flag lang":       {"language of the generated texts: en or pt (default: the CLI language)", "idioma dos textos gerados: en ou pt (padrão: o idioma da CLI)"},
	"flag trilha-dir": {"use a local copy of trilha (adds a replace to go.mod)", "usar uma cópia local do trilha (adiciona replace no go.mod)"},
	"flag no-tidy":    {"do not run go mod tidy", "não rodar go mod tidy"},
	"new usage":       {"usage: trilha new <dir> [--module path] [--lang en|pt]", "uso: trilha new <dir> [--module caminho] [--lang en|pt]"},
	"bad lang":        {"--lang must be en or pt", "--lang deve ser en ou pt"},
	"tidy failed":     {"warning: go mod tidy failed (no network?); run it manually:", "aviso: go mod tidy falhou (sem rede?); rode manualmente:"},
	"project created": {"\n✓ project created in %s\n\n  cd %s\n  trilha dev\n", "\n✓ projeto criado em %s\n\n  cd %s\n  trilha dev\n"},

	// dev / build / export
	"flag addr":     {"public address of the dev server", "endereço público do dev server"},
	"flag build -o": {"output file (default bin/<folder-name>)", "arquivo de saída (padrão bin/<nome-da-pasta>)"},
	"build failed":  {"go build failed: %w", "go build falhou: %w"},
	"flag out":      {"output folder", "pasta de saída"},
	"flag base":     {"URL prefix of the site (e.g. /trilha on GitHub Pages)", "prefixo de URL do site (ex.: /trilha no GitHub Pages)"},
	"export failed": {"export failed: %w", "exportação falhou: %w"},

	// ui
	"flag force":    {"overwrite locally modified ui.css/ui.js", "sobrescrever ui.css/ui.js modificados localmente"},
	"flag css-only": {"only ui.css (and ui.theme.css if missing)", "só ui.css (e ui.theme.css se faltar)"},
	"flag js-only":  {"only ui.js", "só ui.js"},
	"ui modified":   {"ui kit files were modified locally; use --force to overwrite", "arquivos do kit ui foram modificados localmente; use --force para sobrescrever"},
	"ui created":    {"created", "criado"},
	"ui updated":    {"updated", "atualizado"},
	"ui kept":       {"kept", "mantido"},
	"ui kept theme": {"kept (your theme)", "mantido (seu tema)"},
	"ui local":      {"modified locally", "modificado localmente"},

	// audit
	"flag no-vuln":        {"do not run govulncheck (no network)", "não rodar govulncheck (sem rede)"},
	"critical items":      {"%d critical item(s)", "%d item(ns) crítico(s)"},
	"no critical":         {"\nNo critical items. Review the warnings before publishing.", "\nNenhum item crítico. Revise os avisos antes de publicar."},
	"secret unset":        {"TRILHA_SECRET not set in this environment", "TRILHA_SECRET não definido neste ambiente"},
	"secret unset hint":   {"signed cookies (sessions) do not work in production; generate one with: openssl rand -base64 32", "cookies assinados (sessão) não funcionam em produção; gere com: openssl rand -base64 32"},
	"secret short":        {"TRILHA_SECRET too short", "TRILHA_SECRET curto demais"},
	"secret short hint":   {"use at least 32 bytes", "use ao menos 32 bytes"},
	"secret ok":           {"TRILHA_SECRET set", "TRILHA_SECRET definido"},
	"proxies unset":       {"TRILHA_TRUSTED_PROXIES not set", "TRILHA_TRUSTED_PROXIES não definido"},
	"proxies unset hint":  {"behind a proxy (nginx, load balancer) set the CIDRs so HSTS, client IP and rate limit are right", "atrás de um proxy (nginx, load balancer) defina os CIDRs para HSTS, IP do cliente e rate limit corretos"},
	"proxies ok":          {"TRILHA_TRUSTED_PROXIES set", "TRILHA_TRUSTED_PROXIES definido"},
	"app invalid":         {"app/ has invalid conventions", "app/ com convenções inválidas"},
	"gen stale":           {"trilha_gen.go out of date", "trilha_gen.go desatualizado"},
	"gen stale hint":      {"run: trilha gen", "rode: trilha gen"},
	"gen fresh":           {"trilha_gen.go up to date", "trilha_gen.go atualizado"},
	"go unsupported":      {"Go %s unsupported", "Go %s sem suporte"},
	"go unsupported hint": {"Trilha requires Go 1.22+", "o Trilha exige Go 1.22+"},
	"gitignore missing":   {".gitignore without .trilha/ and bin/", ".gitignore sem .trilha/ e bin/"},
	"gitignore hint":      {"temporary binaries may end up in git", "binários temporários podem ir para o git"},
	"gitignore ok":        {".gitignore covers .trilha/ and bin/", ".gitignore cobre .trilha/ e bin/"},
	"vet problems":        {"go vet found problems", "go vet encontrou problemas"},
	"vet clean":           {"go vet clean", "go vet limpo"},
	"vuln found":          {"govulncheck found vulnerabilities", "govulncheck encontrou vulnerabilidades"},
	"vuln failed":         {"govulncheck could not run", "govulncheck não pôde rodar"},
	"vuln failed hint":    {"no network? use --no-vuln; ", "sem rede? use --no-vuln; "},
	"vuln clean":          {"govulncheck found no known vulnerabilities", "govulncheck sem vulnerabilidades conhecidas"},
}
