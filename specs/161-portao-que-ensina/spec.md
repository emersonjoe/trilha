# Feature Specification: O portão que ensina

**Feature Branch**: `161-portao-que-ensina` | **Created**: 2026-09-24 | **Status**: Draft
**Input**: Plano Tokens 70, spec 161 — um comando agrega todos os portões e cada falha diz
arquivo, linha, causa, conserto e link da doc — para o agente consertar em uma rodada.

## Contexto

O `trilha check` (spec 047) já é o portão único: gen → gofmt → vet → test → audit → openapi
→ i18n, parando na primeira falha, cada problema com arquivo, linha e conserto. O que falta
é o **código estável** por falha — a coisa que se cola num buscador —, o catálogo central de
causa/conserto/exemplo gerando as páginas do site, e o `get_error` no MCP. As rodadas de
erro são ~25% do custo do agente (§1.2).

### Decisões que vinculam a implementação

1. **A fonte da verdade do catálogo é Go** (`internal/checkerr`), e as páginas `/docs/errors`
   passam a ser geradas dele — o catálogo de runtime (4 códigos via `trilha.ErrorGuides`) é
   incorporado ao mesmo índice, sem segunda tabela.
2. **Códigos por família para saída dinâmica**: `E_VET_PRINTF` é literal catalogado; falhas de
   vet sem analisador próprio caem em `E_VET`; `govulncheck` emite `E_VULN_<ID>` (o ID do
   advisory) e o catálogo tem a entrada de família `E_VULN_`, com busca por prefixo.
3. **`check --json` muda de forma** para a do plano (`status` + `failures[]` com
   `tool/code/file/line/message/hint/doc`): saída de CLI não é superfície garantida, e a spec
   161 é quem define o contrato de máquina. Exit codes: 0 verde, 1 portão, 2 uso errado.
4. **Caminho corrigido**: o plano pede páginas `/erros` e `/pt/erros`; o site já tem o
   catálogo de erros em `/docs/errors` (e `/pt/docs/errors`), com página por código. As
   páginas existentes é que passam a ser geradas do catálogo, com busca e âncoras — não há
   catálogo paralelo.
5. **T07 (régua)**: o S8 da régua v2 já conta rodadas-até-verde (as rodadas do agente são
   mediana da série, publicada em `/custos` com coluna própria) e os prompts são congelados —
   o que esta spec adiciona ao S8 é a qualidade do sinal (`check --json` disponível ao agente
   e documentado no AGENTS.md), não uma nova medição.

## User Scenarios & Testing

### US1 - Toda falha tem um código que ensina (P1)

Cada falha do `trilha check` — do scanner, do gofmt, do vet, do test, do audit, do openapi,
do i18n, da trava de superfície e do govulncheck — sai com `code` do catálogo, `hint` (o
conserto) e `doc` (o link da página do código). `TestErrorCatalogComplete` varre o
código-fonte por literais `E_[A-Z_]+` e falha se algum não tiver entrada no catálogo.

**Independent Test**: `go test ./internal/checkerr/` + `go test ./cmd/trilha/ -run Check`.

**Acceptance Scenarios**:

1. **Given** um app com erro de vet de printf, **When** `check --json` roda, **Then** a
   falha carrega `E_VET_PRINTF`, o hint do catálogo e o link da página.
2. **Given** um advisory do govulncheck, **When** o portão security falha, **Then** o código
   é `E_VULN_<ID>` e a página de família explica o upgrade.
3. **Given** qualquer literal `E_*` novo no código, **When** a suíte roda, **Then**
   `TestErrorCatalogComplete` falha até o catálogo ganhar a entrada.

### US2 - O catálogo é a página (P1)

`/docs/errors` (e `/pt/docs/errors`) lista o catálogo inteiro gerado do Go, com busca por
código, âncora por `E_*` e causa/conserto/exemplo por entrada; `/docs/errors/<code>` serve
qualquer código do catálogo — incluindo os de scanner e os de ferramenta.

**Independent Test**: `go test ./site/ -run 'Errors'`.

**Acceptance Scenarios**:

1. **Given** o catálogo, **When** a página renderiza, **Then** cada entrada tem âncora
   `#<code>` e o campo de busca filtra por código/título.
2. **Given** `E_DUPLICATE_ROUTE`, **When** `/docs/errors/E_DUPLICATE_ROUTE` abre, **Then**
   causa, conserto e exemplo aparecem, nas duas línguas.

### US3 - O agente pergunta pelo código (P2)

O MCP do site ganha `get_error(code)` devolvendo a `Doc` do catálogo; a referência MCP é
atualizada nas duas línguas.

## Edge Cases

- Código desconhecido em `get_error` e em `/docs/errors/<code>`: 404 com a lista de famílias.
- Falha de vet sem `file:line` parseável: problema sem posição, código `E_VET`, mensagem com
  as primeiras linhas da saída.
- Portão que não se aplica (openapi sem documento): permanece `skipped` e não gera falha.

## Requirements

### Security and privacy impact

- **Assets and trust boundaries**: o portão agregado é o mesmo baseline do CI (`make test` +
  `make security` no que toca a vuln/superfície), nada enfraquecido; o `govulncheck` entra no
  `check` como portão explícito quando presente no PATH, com saída mapeada — sem rede no
  caminho padrão do check (permanece opt-in, como o audit hoje).
- **ASVS 5.0 Level 2**: V14.2 (dependências) — `E_VULN_<ID>` aponta o upgrade; V6 (log) —
  nenhuma saída nova carrega segredo (o catálogo é conteúdo público).
- **OWASP Top 10:2025**: A06 (componentes vulneráveis) é o risco atacado; o catálogo é
  estático e sem entrada de usuário.
- **Secrets**: nenhum; **Exceptions**: nenhuma.

### Functional Requirements

- **FR-001**: `internal/checkerr` MUST expor `Doc{Code,Title,Cause,Fix,Example,Doc}` e
  `Docs()/ByCode()`, incorporando os códigos de runtime, scanner e ferramentas.
- **FR-002**: `ByCode` MUST aceitar família por prefixo (`E_VULN_<ID>` → `E_VULN_`).
- **FR-003**: `trilha check --json` MUST emitir `{"status","failures":[{tool,code,file,line,
  message,hint,doc}]}` com exit 0/1/2.
- **FR-004**: Mapeadores de gofmt, vet, test, vuln e api MUST ter tabela de fixture (saída
  real capturada → código+campos).
- **FR-005**: `/docs/errors` e `/docs/errors/<code>` (pt também) MUST ser geradas do
  catálogo, com busca e âncoras.
- **FR-006**: O MCP do site MUST servir `get_error(code)` com a `Doc`.

### Key Entities

- **Doc**: a entrada do catálogo — código, título, causa, conserto, exemplo, link.
- **failure**: uma falha do portão com o código que a ensina.

## Success Criteria

- **SC-001**: `go test ./internal/checkerr/ ./cmd/trilha/` verdes (catálogo completo,
  mapeadores com fixtures, schema do `--json`, exit codes).
- **SC-002**: `make test` verde com as páginas geradas nas duas línguas e `get_error` no MCP.

__zcode_status=$?
if [ "$__zcode_status" -eq 0 ]; then pwd -P > '/var/folders/xd/vy0xsb2j23g5mxx8d4gw75tm0000gn/T/zcode-99a6db01-944a-42b9-9216-e86a77c827fa-cwd'; fi
exit "$__zcode_status"