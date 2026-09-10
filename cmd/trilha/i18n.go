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
  trilha new <dir> [--module path] [--lang en|pt] [--agents]   create a new project
  trilha gen [--check]                              generate trilha_gen.go from app/ (--check: fail if stale)
  trilha generate page|route|test <url> | component <Name>   write a skeleton in the right place
    [--methods GET,POST] [--bind Type] [--form Type] [--layout file] [--lang en|pt]
  trilha dev [--addr :3000]                         dev server with live reload
  trilha build [-o bin/<name>]                      generate + compile a single binary
  trilha check [--json] [--fix]                     the single gate: gen, gofmt, vet, test, audit, openapi
  trilha ctx [--json] [--routes|--types|--all]      the map of the project: routes, API, types, setup
  trilha routes                                     list the discovered routes
  trilha export [-o out] [--base /prefix]           export the static pages as HTML
  trilha openapi [-o file] [--check]                write the OpenAPI document of the API routes
  trilha audit [--no-vuln]                          check the project's security and configuration
  trilha secret                                     print a signing key for TRILHA_SECRET
  trilha add [recipe] [--dry-run] [--list --json]   write a framework recipe into the project
  trilha ui [--force] [--css-only|--js-only]        write/update the ui kit in public/
  trilha ui describe [Name] [--json]                the ui catalogue: what exists and how it is called
  trilha agents [--force] [--lang en|pt]            write AGENTS.md and CLAUDE.md for coding agents
  trilha mcp [--write]                              MCP server over stdio for an agent without a shell
  trilha mcp --from-routes [--include /api/v1/*]    the tools mcp.FromRoutes would expose for this API
  trilha migrate next <next-dir> [--out app]        skeleton of app/ and MIGRATION.md from a Next.js project
  trilha client <openapi.json|URL> [--check]       generate the Go client of an API that already exists
  trilha vendor [<pkg@version>] [--check] [--from URL]  pin a JavaScript module in public/vendor
  trilha version

Language: TRILHA_LANG=en|pt (falls back to LC_ALL, LC_MESSAGES, LANG).
`, `trilha — framework web para Go com roteamento por arquivos

Uso:
  trilha new <dir> [--module caminho] [--lang en|pt] [--agents]  cria um projeto novo
  trilha gen [--check]                                gera trilha_gen.go a partir de app/ (--check: falha se desatualizado)
  trilha generate page|route|test <url> | component <Nome>  grava um esqueleto no lugar certo
    [--methods GET,POST] [--bind Tipo] [--form Tipo] [--layout arquivo] [--lang en|pt]
  trilha dev [--addr :3000]                           dev server com recarga automática
  trilha build [-o bin/<nome>]                        gera + compila um binário único
  trilha check [--json] [--fix]                       o portão único: gen, gofmt, vet, test, audit, openapi
  trilha ctx [--json] [--routes|--types|--all]        o mapa do projeto: rotas, API, tipos, setup
  trilha routes                                       lista as rotas descobertas
  trilha export [-o out] [--base /prefixo]            exporta as páginas estáticas em HTML
  trilha openapi [-o arquivo] [--check]               escreve o documento OpenAPI das rotas de API
  trilha audit [--no-vuln]                            verifica segurança e configuração do projeto
  trilha secret                                       imprime uma chave para o TRILHA_SECRET
  trilha add [receita] [--dry-run] [--list --json]    escreve uma receita do framework no projeto
  trilha ui [--force] [--css-only|--js-only]          grava/atualiza o kit ui em public/
  trilha ui describe [Nome] [--json]                  o catálogo do ui: o que existe e como se chama
  trilha agents [--force] [--lang en|pt]              grava AGENTS.md e CLAUDE.md para agentes de código
  trilha mcp [--write]                                servidor MCP por stdio, para agente sem shell
  trilha mcp --from-routes [--include /api/v1/*]      as ferramentas que mcp.FromRoutes exporia para esta API
  trilha migrate next <dir-do-next> [--out app]       esqueleto do app/ e MIGRATION.md a partir de um projeto Next.js
  trilha client <openapi.json|URL> [--check]          gera o cliente Go de uma API que já existe
  trilha vendor [<pkg@versão>] [--check] [--from URL]  fixa um módulo JavaScript em public/vendor
  trilha version

Idioma: TRILHA_LANG=en|pt (senão LC_ALL, LC_MESSAGES, LANG).
`},
	"flag ctx json":     {"print the map as JSON", "imprime o mapa em JSON"},
	"flag ctx routes":   {"only the routes", "só as rotas"},
	"flag ctx types":    {"only the types", "só os tipos"},
	"flag ctx all":      {"everything, with nothing elided", "tudo, sem nada elidido"},
	"ctx one view":      {"choose one of --routes, --types or --all", "escolha um entre --routes, --types e --all"},
	"flag check json":   {"print the report as JSON", "imprime o relatório em JSON"},
	"flag check fix":    {"fix what can be fixed: trilha_gen.go and the formatting", "conserta o que dá: trilha_gen.go e a formatação"},
	"check ok":          {"ok", "ok"},
	"check failed":      {"check failed", "check falhou"},
	"gen missing":       {"trilha_gen.go is missing", "trilha_gen.go não existe"},
	"fix gen":           {"run trilha gen (or trilha check --fix)", "rode trilha gen (ou trilha check --fix)"},
	"gofmt unformatted": {"not gofmt'd", "fora do gofmt"},
	"fix gofmt":         {"run gofmt -w (or trilha check --fix)", "rode gofmt -w (ou trilha check --fix)"},
	"fix vet":           {"fix what go vet reports at this line", "conserte o que o go vet aponta nesta linha"},
	"fix test":          {"run go test ./... and read this test's output", "rode go test ./... e leia a saída deste teste"},
	"fix openapi":       {"run trilha openapi", "rode trilha openapi"},
	"unknown command":   {"unknown command: %s\n\n%s", "comando desconhecido: %s\n\n%s"},
	"error:":            {"error:", "erro:"},
	"no app dir":        {"app/ directory not found: run at the project root (or use `trilha new`)", "pasta app/ não encontrada: rode na raiz do projeto (ou use `trilha new`)"},
	"no go.mod":         {"go.mod not found above %s", "go.mod não encontrado acima de %s"},
	"no module line":    {"%s: `module` line not found", "%s: linha `module` não encontrada"},
	"gen done":          {"✓ %s (%d routes)\n", "✓ %s (%d rotas)\n"},
	"islands done":      {"✓ %s (%d islands)\n", "✓ %s (%d ilhas)\n"},
	"flag vendor check": {"re-hash what is in public/vendor against vendor.lock", "confere o hash do que está em public/vendor com o vendor.lock"},
	"flag vendor from":  {"base URL to download from (default https://esm.sh)", "URL base de onde baixar (padrão https://esm.sh)"},
	"vendor done":       {"✓ %s@%s → %s (%d bytes)\n", "✓ %s@%s → %s (%d bytes)\n"},
	"vendor hint":       {"import it from the island: import x from \"/vendor/<name>.js\"", "importe da ilha: import x from \"/vendor/<nome>.js\""},
	"vendor ok":         {"✓ %d vendored modules match vendor.lock\n", "✓ %d módulos fixados batem com o vendor.lock\n"},
	"vendor scheme":     {"vendor downloads over http or https, not %q", "vendor baixa por http ou https, não %q"},
	"vendor http":       {"HTTP %d from %s", "HTTP %d de %s"},
	"vendor too big":    {"the module is over %d bytes", "o módulo passa de %d bytes"},
	"vendor missing":    {"%s is in vendor.lock (%s) but not on disk", "%s está no vendor.lock (%s) mas não em disco"},
	"vendor changed":    {"%s changed since it was pinned (%s@%s)", "%s mudou desde que foi fixado (%s@%s)"},
	"vendor unpinned":   {"%s is not in vendor.lock", "%s não está no vendor.lock"},
	"vendor lock line":  {"%s:%d: expected: name version sha256 file url", "%s:%d: esperado: nome versão sha256 arquivo url"},
	"MODULE":            {"MODULE", "MÓDULO"},
	"VERSION":           {"VERSION", "VERSÃO"},
	"FILE":              {"FILE", "ARQUIVO"},
	"unknown flag":      {"unknown flag %q; usage: %s", "bandeira desconhecida %q; uso: %s"},
	"bad package name":  {"%q is not a valid package name", "%q não é um nome de pacote válido"},
	"embedded no binary": {
		"this app is package %[1]s, not package main: there is no binary to run here.\nThe host binary runs it: mux.Handle(\"/\", %[1]s.NewApp().Handler())",
		"este app é o pacote %[1]s, não package main: não há binário para rodar aqui.\nQuem roda é o binário hospedeiro: mux.Handle(\"/\", %[1]s.NewApp().Handler())",
	},
	"gen diff":      {"differences (+ generated now, - file on disk):\n", "diferenças (+ gerado agora, - arquivo em disco):\n"},
	"routes header": {"%-22s %-32s %s\n", "%-22s %-32s %s\n"},
	"METHODS":       {"METHODS", "MÉTODOS"},
	"PATTERN":       {"PATTERN", "PADRÃO"},
	"SOURCE":        {"SOURCE", "ORIGEM"},

	// new
	"flag module":     {"Go module path (default: folder name)", "caminho do módulo Go (padrão: nome da pasta)"},
	"flag lang":       {"language of the generated texts: en or pt (default: the CLI language)", "idioma dos textos gerados: en ou pt (padrão: o idioma da CLI)"},
	"flag trilha-dir": {"use a local copy of trilha (adds a replace to go.mod)", "usar uma cópia local do trilha (adiciona replace no go.mod)"},
	"flag no-tidy":    {"do not run go mod tidy", "não rodar go mod tidy"},
	"new usage":       {"usage: trilha new <dir> [--module path] [--lang en|pt]", "uso: trilha new <dir> [--module caminho] [--lang en|pt]"},
	"bad lang":        {"--lang must be en or pt", "--lang deve ser en ou pt"},
	"flag template":   {"shape of the project: blog or app", "formato do projeto: blog ou app"},
	"bad template":    {"--template must be blog or app", "--template deve ser blog ou app"},
	"tidy failed":     {"warning: go mod tidy failed (no network?); run it manually:", "aviso: go mod tidy falhou (sem rede?); rode manualmente:"},
	"project created": {"\n✓ project created in %s\n\n  cd %s\n  trilha dev\n", "\n✓ projeto criado em %s\n\n  cd %s\n  trilha dev\n"},

	// generate
	"flag gen-force":   {"overwrite the file if it already exists", "sobrescrever o arquivo se já existir"},
	"flag gen-dir":     {"folder of the component (default internal/components)", "pasta do componente (padrão internal/components)"},
	"flag gen-methods": {"one handler per method: GET,POST,PUT,PATCH,DELETE,OPTIONS", "um handler por método: GET,POST,PUT,PATCH,DELETE,OPTIONS"},
	"flag gen-bind":    {"type the body binds to; written in the route's package when the project has none", "tipo em que o corpo é lido; nasce no pacote da rota quando o projeto não tem"},
	"flag gen-form":    {"type the form binds to, with the round trip of errors and the redirect", "tipo em que o formulário é lido, com a ida e volta dos erros e o redirect"},
	"flag gen-layout":  {"layout.go to write when the folder above the page has none", "layout.go a gravar quando a pasta acima da página não tem um"},
	"flag with":        {"recipes to apply at creation, comma-separated (`trilha add` lists them); --with \"\" for none", "receitas a aplicar na criação, separadas por vírgula (o `trilha add` lista); --with \"\" para nenhuma"},
	"flag add list":    {"list the recipes and write nothing", "lista as receitas e não escreve nada"},
	"flag add json":    {"with --list, answer JSON", "com --list, responde JSON"},
	"flag add dry":     {"show what would be written and write nothing", "mostra o que seria escrito e não escreve nada"},
	"add see list":     {"run `trilha add` to see what there is", "rode `trilha add` para ver o que existe"},
	"add skipped":      {"already there, kept", "já existe, mantido"},
	"add wired":        {"one line added", "uma linha acrescentada"},
	"add dry":          {"--dry-run: nothing was written", "--dry-run: nada foi escrito"},
	"add doc":          {"Doc:", "Doc:"},
	"flag crud at":     {"where the screens go, under app/ (default app/<plural of the type>)", "onde as telas vão, dentro de app/ (padrão app/<plural do tipo>)"},
	"crud exists":      {"a CRUD is what you generate after editing one, so there is no --force: move or delete the file and run again", "um CRUD é o que se gera depois de já ter editado, então não há --force: mova ou apague o arquivo e rode de novo"},
	"generate usage":   {"usage: trilha generate page|route|test <url> | component <Name> | crud <Type> [--at dir] [--methods GET,POST] [--bind Type] [--form Type] [--layout file] [--force] [--dir path] [--lang en|pt]", "uso: trilha generate page|route|test <url> | component <Nome> | crud <Tipo> [--at dir] [--methods GET,POST] [--bind Tipo] [--form Tipo] [--layout arquivo] [--force] [--dir caminho] [--lang en|pt]"},
	"gen use force":    {"use --force to overwrite it", "use --force para sobrescrever"},
	"gen conflict":     {"a folder answers either a page or a route, never both", "uma pasta responde ou uma página ou uma rota, nunca as duas"},
	"generated route":  {"\n✓ %s answers now; trilha_gen.go is up to date\n", "\n✓ %s já responde; trilha_gen.go está atualizado\n"},

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
	"ui kept own":   {"kept (yours)", "mantido (seu)"},

	// client
	"client needs a doc":  {"give the OpenAPI document: trilha client openapi.json", "informe o documento OpenAPI: trilha client openapi.json"},
	"flag client out":     {"folder of the generated client", "pasta do cliente gerado"},
	"flag client package": {"package clause of the generated file (default: the folder name)", "cláusula package do arquivo gerado (padrão: o nome da pasta)"},
	"flag client check":   {"fail if the file on disk is out of date", "falha se o arquivo no disco estiver desatualizado"},
	"client fresh":        {"%s is up to date", "%s está em dia"},
	"client stale":        {"%s is out of date; run trilha client again", "%s está desatualizado; rode trilha client de novo"},
	"client done":         {"%s written, package %s.", "%s gravado, pacote %s."},
	"client bad scheme":   {"%s:// is not a scheme this reads; give a file or an http(s) URL", "%s:// não é um esquema que dá para ler; informe um arquivo ou uma URL http(s)"},
	"client fetch failed": {"%s answered %d", "%s respondeu %d"},

	// migrate
	"migrate what":         {"say what to migrate: trilha migrate next <next-dir>", "diga o que migrar: trilha migrate next <dir-do-next>"},
	"migrate needs a dir":  {"give the Next.js project directory: trilha migrate next ../web", "informe a pasta do projeto Next.js: trilha migrate next ../web"},
	"flag migrate out":     {"where to write the skeleton", "onde gravar o esqueleto"},
	"flag migrate report":  {"where to write the report", "onde gravar o relatório"},
	"flag migrate dry-run": {"print the report and write nothing", "imprime o relatório e não grava nada"},
	"flag migrate force":   {"overwrite files that are already there", "sobrescrever arquivos que já existem"},
	"migrate dry":          {"%d files would be written to %s/, plus %s.", "%d arquivos seriam gravados em %s/, mais o %s."},
	"migrate where":        {"skeleton: %s (%d screens)\nreport:   %s", "esqueleto: %s (%d telas)\nrelatório: %s"},
	"migrate done":         {"%d written, %d kept, %d lines in the report with no equivalent here.", "%d gravados, %d mantidos, %d linhas do relatório sem equivalente aqui."},

	// ui describe
	"flag describe json": {"print the catalogue (or the component) as JSON", "imprime o catálogo (ou o componente) em JSON"},
	"no such component":  {"no ui component named %q", "não há componente ui chamado %q"},
	"did you mean":       {"did you mean: %s", "você quis dizer: %s"},
	"describe hint":      {"%d components. `trilha ui describe <Name>` describes one of them.", "%d componentes. `trilha ui describe <Nome>` descreve um deles."},
	"fields:":            {"fields:", "campos:"},
	"example:":           {"example:", "exemplo:"},
	"see:":               {"see:", "veja:"},

	// agents
	"flag agents":       {"also write AGENTS.md and CLAUDE.md for coding agents", "também gravar AGENTS.md e CLAUDE.md para agentes de código"},
	"flag force agents": {"overwrite a locally modified AGENTS.md", "sobrescrever um AGENTS.md modificado localmente"},
	"agents modified":   {"AGENTS.md was modified locally; use --force to overwrite", "AGENTS.md foi modificado localmente; use --force para sobrescrever"},

	// openapi
	"flag openapi out":     {`output file ("-" writes to stdout)`, `arquivo de saída ("-" escreve na saída padrão)`},
	"flag openapi title":   {"document title (default: the module name)", "título do documento (padrão: o nome do módulo)"},
	"flag openapi version": {"API version (default: 0.0.0)", "versão da API (padrão: 0.0.0)"},
	"flag openapi server":  {"base URL of the server", "URL base do servidor"},
	"flag openapi check":   {"fail if the file on disk is out of date", "falha se o arquivo no disco estiver desatualizado"},
	"openapi done":         {"✓ %s (%d operations)\n", "✓ %s (%d operações)\n"},
	"openapi fresh":        {"the OpenAPI document is up to date", "o documento OpenAPI está atualizado"},
	"openapi stale":        {"the OpenAPI document is out of date", "o documento OpenAPI está desatualizado"},
	"openapi stale hint":   {"run `trilha openapi`", "rode `trilha openapi`"},

	// audit
	"flag no-vuln":                {"do not run govulncheck (no network)", "não rodar govulncheck (sem rede)"},
	"critical items":              {"%d critical item(s)", "%d item(ns) crítico(s)"},
	"no critical":                 {"\nNo critical items. Review the warnings before publishing.", "\nNenhum item crítico. Revise os avisos antes de publicar."},
	"secret unset":                {"TRILHA_SECRET not set in this environment", "TRILHA_SECRET não definido neste ambiente"},
	"secret unset hint":           {"signed cookies (sessions) do not work in production; generate one with: openssl rand -base64 32", "cookies assinados (sessão) não funcionam em produção; gere com: openssl rand -base64 32"},
	"secret unused":               {"TRILHA_SECRET not set (nothing in this app signs cookies)", "TRILHA_SECRET não definido (nada neste app assina cookie)"},
	"secret unused hint":          {"nothing here calls SetSigned, Signed or trilha/auth; set it when sessions arrive: openssl rand -base64 32", "nada aqui chama SetSigned, Signed ou trilha/auth; defina quando a sessão chegar: openssl rand -base64 32"},
	"secret short":                {"TRILHA_SECRET too short", "TRILHA_SECRET curto demais"},
	"secret short hint":           {"use at least 32 bytes", "use ao menos 32 bytes"},
	"secret ok":                   {"TRILHA_SECRET set", "TRILHA_SECRET definido"},
	"hosts unset":                 {"AllowedHosts not set", "AllowedHosts não definido"},
	"hosts unset hint":            {"list the hosts the app answers for (Config.AllowedHosts or TRILHA_ALLOWED_HOSTS); without it a forged Host header poisons caches and reset links", "liste os hosts que o app atende (Config.AllowedHosts ou TRILHA_ALLOWED_HOSTS); sem isso um cabeçalho Host forjado envenena caches e links de redefinição"},
	"hosts ok":                    {"AllowedHosts set", "AllowedHosts definido"},
	"proxies unset":               {"TRILHA_TRUSTED_PROXIES not set", "TRILHA_TRUSTED_PROXIES não definido"},
	"proxies unset hint":          {"behind a proxy (nginx, load balancer) set the CIDRs so HSTS, client IP and rate limit are right", "atrás de um proxy (nginx, load balancer) defina os CIDRs para HSTS, IP do cliente e rate limit corretos"},
	"proxies ok":                  {"TRILHA_TRUSTED_PROXIES set", "TRILHA_TRUSTED_PROXIES definido"},
	"app invalid":                 {"app/ has invalid conventions", "app/ com convenções inválidas"},
	"csrf open writes":            {"%d write route(s) in route.go without CSRF", "%d rota(s) de escrita em route.go sem CSRF"},
	"csrf open writes hint":       {"a route.go is an API, and an API does not check the token: a form on another site can post to %s. Say the branch is pages with `var Kind = trilha.KindPage` in a kind.go above them, or set Config.CSRFForAPI if the API really is the client", "um route.go é API, e API não confere o token: um formulário de outro site consegue postar em %s. Diga que o ramo é de páginas com `var Kind = trilha.KindPage` num kind.go acima delas, ou ligue Config.CSRFForAPI se a API for mesmo o cliente"},
	"upstream plaintext":          {"upstream target over plain http", "alvo de upstream em http puro"},
	"upstream plaintext hint":     {"the session credential is injected into this request: over http it crosses the network readable by whoever is on the way (%s). Use https, or a target on this machine", "a credencial da sessão é injetada nessa requisição: em http ela atravessa a rede legível para quem estiver no caminho (%s). Use https, ou um alvo nesta máquina"},
	"upstream no credential":      {"upstream without Headers in an app with a login", "upstream sem Headers num app com login"},
	"upstream no credential hint": {"the proxy does not forward the browser's Authorization: without Upstream.Headers reading the session, the API is called anonymously — which is either a forgotten credential or worth a comment saying it is on purpose", "o proxy não repassa o Authorization do navegador: sem Upstream.Headers lendo a sessão, a API é chamada como anônima — o que é uma credencial esquecida ou merece um comentário dizendo que é de propósito"},
	"login no limit":              {"Login without a rate limit", "Login sem limite de taxa"},
	"login no limit hint":         {"a login nobody limits is a password guessing machine with this app's uptime: set Config.RateLimit or TRILHA_RATE_LIMIT", "um login que ninguém limita é uma máquina de adivinhar senha com o uptime deste app: defina Config.RateLimit ou TRILHA_RATE_LIMIT"},
	"tenant gap":                  {"%s is filtered by tenant in %d of %d queries, and not in this one", "%s é filtrada por tenant em %d de %d consultas, e não nesta"},
	"tenant gaps":                 {"queries that may be missing the tenant filter", "consultas que talvez estejam sem o filtro de tenant"},
	"tenant gaps hint":            {"this is a text heuristic and not a verdict: it counts which tables the other queries filter. A query that is right on purpose — a global report, an admin listing — is worth a comment saying so", "isto é uma heurística de texto e não um veredito: ela conta quais tabelas as outras consultas filtram. Consulta que está certa de propósito — relatório global, listagem de admin — merece um comentário dizendo isso"},
	"secret no previous":          {"trilha.Secret in the project and TRILHA_PREVIOUS_SECRET unset", "trilha.Secret no projeto e TRILHA_PREVIOUS_SECRET sem valor"},
	"secret no previous hint":     {"rotating TRILHA_SECRET makes every sealed value unreadable: set the old key as TRILHA_PREVIOUS_SECRET before rotating, keep it until everything has been re-sealed, and only then drop it", "trocar o TRILHA_SECRET torna ilegível tudo o que foi cifrado: ponha a chave antiga em TRILHA_PREVIOUS_SECRET antes de rodar, mantenha até tudo ser re-cifrado, e só então tire"},
	"iframe by hand":              {"<iframe> written by hand", "<iframe> escrito à mão"},
	"iframe by hand hint":         {"what refuses to be framed is the answer inside it, not this page: serve that file with c.Inline (which says it may be framed by this origin) and draw it with ui.Preview, or the frame stays blank with nothing in the console", "quem recusa ser enquadrado é a resposta lá dentro, não esta página: sirva aquele arquivo com c.Inline (que diz que ele pode ser enquadrado por esta origem) e desenhe com ui.Preview, ou o quadro fica em branco sem nada no console"},
	"live no auth":                {"ui.Live in an app with no session", "ui.Live num app sem sessão"},
	"live no auth hint":           {"the stream stays open and anyone can hold it: guard the route that answers it with auth.Require, or say in the route why it is public", "o stream fica aberto e qualquer um o segura: proteja a rota que responde com auth.Require, ou escreva na rota por que ela é pública"},
	"gen stale":                   {"trilha_gen.go out of date", "trilha_gen.go desatualizado"},
	"gen stale hint":              {"run: trilha gen", "rode: trilha gen"},
	"gen fresh":                   {"trilha_gen.go up to date", "trilha_gen.go atualizado"},
	"islands stale":               {"%s out of date", "%s desatualizado"},
	"vendor usage":                {"usage: trilha vendor <pkg@version> [--from URL]", "uso: trilha vendor <pacote@versão> [--from URL]"},
	"vendor failed":               {"vendor.lock does not match public/vendor", "vendor.lock não bate com public/vendor"},
	"vendor empty":                {"nothing vendored yet", "nada fixado ainda"},
	"vendor unpinned hint":        {"run trilha vendor <pkg@version> so the file has a version and a sha256 in vendor.lock, or delete it", "rode trilha vendor <pacote@versão> para o arquivo ter versão e sha256 no vendor.lock, ou apague o arquivo"},
	"islands stale hint":          {"the island props changed; run: trilha gen", "as props das ilhas mudaram; rode: trilha gen"},
	"cli skew":                    {"trilha CLI %s, library %s in go.mod", "CLI do trilha %s, biblioteca %s no go.mod"},
	"cli skew hint":               {"generated code may use what the library does not have yet: install the matching CLI or update go.mod", "o código gerado pode usar o que a biblioteca ainda não tem: instale a CLI da mesma versão ou atualize o go.mod"},
	"cli match":                   {"CLI and library at the same version", "CLI e biblioteca na mesma versão"},
	"go unsupported":              {"Go %s unsupported", "Go %s sem suporte"},
	"go unsupported hint":         {"Trilha requires Go 1.22+", "o Trilha exige Go 1.22+"},
	"gitignore missing":           {".gitignore without .trilha/ and bin/", ".gitignore sem .trilha/ e bin/"},
	"gitignore hint":              {"temporary binaries may end up in git", "binários temporários podem ir para o git"},
	"gitignore ok":                {".gitignore covers .trilha/ and bin/", ".gitignore cobre .trilha/ e bin/"},
	"vet problems":                {"go vet found problems", "go vet encontrou problemas"},
	"vet clean":                   {"go vet clean", "go vet limpo"},
	"vuln found":                  {"govulncheck found vulnerabilities", "govulncheck encontrou vulnerabilidades"},
	"vuln failed":                 {"govulncheck could not run", "govulncheck não pôde rodar"},
	"vuln failed hint":            {"no network? use --no-vuln; ", "sem rede? use --no-vuln; "},
	"vuln clean":                  {"govulncheck found no known vulnerabilities", "govulncheck sem vulnerabilidades conhecidas"},
	// audit: observability and OIDC (specs 014 and 016)
	"obs token short":             {"TRILHA_OBS_TOKEN too short", "TRILHA_OBS_TOKEN curto demais"},
	"obs token short hint":        {"tokens shorter than 32 bytes never authorize; generate one with: openssl rand -hex 32", "tokens com menos de 32 bytes nunca autorizam; gere com: openssl rand -hex 32"},
	"metrics exposed":             {"metrics exposed without a token or a trusted network", "métricas expostas sem token nem rede confiável"},
	"metrics exposed hint":        {"set TRILHA_OBS_TOKEN (32+ bytes) or Observability.Trusted; until then the endpoint answers 401", "defina TRILHA_OBS_TOKEN (32+ bytes) ou Observability.Trusted; sem isso o endereço responde 401"},
	"metrics protected":           {"metrics endpoint protected", "endereço de métricas protegido"},
	"metrics off":                 {"metrics not exposed", "métricas não expostas"},
	"obs open":                    {"observability open to any origin", "observabilidade aberta a qualquer origem"},
	"obs open hint":               {"0.0.0.0/0 in Trusted makes metrics and health details public; restrict it to the collector's CIDR", "0.0.0.0/0 em Trusted deixa métricas e detalhe do health públicos; restrinja ao CIDR do coletor"},
	"island runtime ok":           {"ui.island.js is in public/", "ui.island.js está em public/"},
	"island runtime missing":      {"this project uses c.Island but public/ui.island.js is missing", "este projeto usa c.Island mas falta public/ui.island.js"},
	"island runtime missing hint": {"run `trilha ui` to write the kit files; without it the island shows only its fallback", "rode `trilha ui` para gravar os arquivos do kit; sem ele a ilha mostra só o recuo"},
	"flag mcp write":              {"offer the tool that writes files (generate); off by default", "oferece a ferramenta que grava arquivos (generate); desligada por padrão"},
	"flag mcp from routes":        {"print the tools mcp.FromRoutes would expose for this project's API, and exit", "imprime as ferramentas que mcp.FromRoutes exporia para a API deste projeto, e sai"},
	"flag mcp include":            {"with --from-routes: the route patterns to include, comma-separated (default /api/*)", "com --from-routes: os padrões de rota a incluir, separados por vírgula (padrão /api/*)"},
	"mcp from routes head":        {"tools mcp.FromRoutes would expose (include: %s):", "ferramentas que mcp.FromRoutes exporia (include: %s):"},
	"mcp from routes none":        {"no API route matches", "nenhuma rota de API corresponde"},
	"mcp from routes left out":    {"left out:", "de fora:"},
	"mcp read only":               {"read-only: no tool here writes a file. Start with --write to offer generate.", "somente leitura: nenhuma ferramenta aqui grava arquivo. Use --write para oferecer o generate."},
	"mcp timeout":                 {"the command did not finish within %s", "o comando não terminou em %s"},
	"mcp bad arg":                 {"refused: %s = %q is not a value this tool accepts", "recusado: %s = %q não é um valor que esta ferramenta aceita"},
	"policy ok":                   {"every module of the policy is required by a route", "todo módulo da política é exigido por alguma rota"},
	"policy loose":                {"the policy declares %s and no route requires it", "a política declara %s e nenhuma rota exige"},
	"policy loose hint":           {"add auth.RequirePolicy(Policy, module, level) to the middleware.go of that area, or take the module out of the policy", "ponha auth.RequirePolicy(Policy, módulo, nível) no middleware.go daquela área, ou tire o módulo da política"},
	"time format":                 {"%d date(s) formatted with a layout of their own", "%d data(s) formatada(s) com layout próprio"},
	"time format hint":            {"ui.Date(c, t) writes the date in the app language and zone; a layout in the page ignores Config.TimeZone", "ui.Date(c, t) escreve a data no idioma e no fuso do app; layout na página ignora o Config.TimeZone"},
	"audit anon":                  {"c.Audit is called and no route requires a session", "c.Audit é chamado e nenhuma rota exige sessão"},
	"audit anon hint":             {"the trail would say anonymous did it; guard the route with auth.Require or set the actor with c.SetActor", "a trilha diria que foi anônimo; guarde a rota com auth.Require ou marque o ator com c.SetActor"},
	"upstream no timeout":         {"proxy without Timeout", "proxy sem Timeout"},
	"upstream no timeout hint":    {"Upstream.Timeout is 30 s by default; a slow API then holds 30 s of requests, and the failure arrives as \"our app is down\" — set it to what you are willing to wait", "o Upstream.Timeout é 30 s por padrão; uma API lenta segura 30 s de requisições, e a falha chega como \"nosso app caiu\" — defina o que você está disposto a esperar"},
	"mail unset":                  {"the app sends e-mail and no server is configured", "o app manda e-mail e não há servidor configurado"},
	"mail unset hint":             {"without TRILHA_MAIL_URL, Send answers mail.ErrNotConfigured in production (and writes .eml files into ./mail in dev)", "sem TRILHA_MAIL_URL o Send devolve mail.ErrNotConfigured em produção (e em dev escreve .eml em ./mail)"},
	"mail ok":                     {"mail server configured", "servidor de e-mail configurado"},
	"stream open":                 {"%d event stream(s) with nothing above them", "%d fluxo(s) de eventos sem nada acima"},
	"stream open hint":            {"everyone who connects receives everything the route sends; put a middleware.go on the folder (%s)", "todo mundo que conecta recebe tudo o que a rota manda; ponha um middleware.go na pasta (%s)"},
	"audit anonymous":             {"%d route(s) write to the audit trail with nothing above them", "%d rota(s) escrevem na trilha de auditoria sem nada acima"},
	"audit anonymous hint":        {"the record exists and does not say who: guard the folder, or the trail names an anonymous actor (%s)", "o registro existe e não diz quem: guarde a pasta, ou a trilha nomeia um ator anônimo (%s)"},
	"string unbounded":            {"%d string field(s) come from outside with no size limit", "%d campo(s) string vêm de fora sem limite de tamanho"},
	"string unbounded hint":       {"add max= to the validate tag: without it the column is what refuses, in production, with the driver's message (%s)", "acrescente max= na tag validate: sem ele quem recusa é a coluna, em produção, com a mensagem do driver (%s)"},
	"crud skipped":                {"left off the screens, the CRUD does not draw this type yet:", "ficou fora das telas, o CRUD ainda não desenha este tipo:"},
	"crud already":                {"already generated — nothing was written.", "já gerado — nada foi escrito."},
	"crud complete":               {"The screens have every field of the struct.", "As telas têm todos os campos do struct."},
	"crud missing form":           {"is not in the form", "não está no formulário"},
	"crud missing list":           {"is not in the listing", "não está na lista"},
	"crud missing add":            {"add", "acrescente"},
	"no checks":                   {"no readiness check registered", "nenhuma verificação de prontidão registrada"},
	"no checks hint":              {"/_trilha/health/ready always answers 200; register a.Check(\"db\", ...) in app/setup.go", "/_trilha/health/ready sempre responde 200; registre a.Check(\"banco\", ...) em app/setup.go"},
	"checks ok":                   {"readiness checks registered", "verificações de prontidão registradas"},
	"asset immutable":             {"immutable cache on unversioned addresses", "cache imutável em endereços sem versão"},
	"asset immutable hint":        {"StaticCacheControl with immutable freezes the file in the browser; use c.Asset(\"/style.css\") in the layout to put the content hash in the URL", "StaticCacheControl com immutable congela o arquivo no navegador; use c.Asset(\"/style.css\") no layout para pôr o hash do conteúdo na URL"},
	"oidc secret hard":            {"OIDC client secret in the code", "segredo do cliente OIDC no código"},
	"oidc secret hard hint":       {"pass it through an environment variable (os.Getenv); a committed secret must be rotated at the provider", "passe-o por variável de ambiente (os.Getenv); um segredo commitado precisa ser rotacionado no provedor"},
	"oidc secret ok":              {"OIDC client secret outside the code", "segredo do cliente OIDC fora do código"},
	"oidc cleartext":              {"redirect_uri over http:// outside localhost", "redirect_uri em http:// fora de localhost"},
	"oidc cleartext hint":         {"the authorization code travels in that URL; use https:// (Entra ID and Keycloak refuse cleartext in production)", "o código de autorização viaja nessa URL; use https:// (Entra ID e Keycloak recusam cleartext em produção)"},
}
