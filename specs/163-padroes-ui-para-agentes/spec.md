# Feature Specification: Padrões de UI para agentes

**Feature Branch**: `163-padroes-ui-para-agentes` | **Created**: 2026-09-24 | **Status**: Draft
**Input**: Plano Tokens 70, spec 163 (`PLANO-TOKENS-70.md` §2) — o agente monta tela sabendo
qual padrão usar e com o snippet mínimo compilável: os padrões que o site já demonstra viram
dados. Marco M3 (com a 164).

## Contexto

O kit `ui` tem ~60 componentes e o catálogo `trilha ui components --json` diz o que cada um é.
O que não existe é a **composição**: qual conjunto de componentes monta uma listagem com
filtro, um formulário assíncrono, uma caixa de aprovações. Hoje o agente descobre lendo demos
e páginas, ou erra e lê o erro. Um padrão endereçável — nome, componentes, snippet que
compila, contrato de dados e notas de acessibilidade — troca essa leitura por uma chamada.

### Decisões que vinculam a implementação

1. **A fonte do snippet é código que compila**: cada padrão é um `page.go` completo em
   `examples/patterns/<nome-sem-hífen>/`, dentro do módulo (o `go vet ./...` compila, e
   `TestPatternsCompile` chama `go build`). `ui` não alcança `examples/` com `go:embed`, então
   o texto chega a `ui.Patterns()` por um arquivo gerado (`ui/patterns_snippets.go`, cabeçalho
   `Code generated`), regravado por `go test ./ui -run TestPatternSnippetsCurrent -update`
   (entra no `make golden`) — o mesmo arranjo do `catalog.json` do `uidoc`.
2. **Superfície pública nova**: só `ui.Pattern` e `ui.Patterns()`, registradas em
   `api/current.txt`. `Pattern` ganha `Summary` além dos campos do plano: a listagem humana e a
   seção do site precisam de uma frase, e ela é dado do padrão, não do site.
3. **`get_pattern` entra no `mcp.ContextTools`** (sem símbolo público novo): é contexto de
   escrita, como `get_context` e `search_code`. O MCP de docs do site ganha a mesma ferramenta.
4. **Padrões → demos do site** (todas já existem nas duas línguas): `list-with-filter` →
   `ui-listagem`, `async-form` → `ui-formulario-assincrono`, `approval-inbox` →
   `ui-aprovacoes`, `dashboard-chart` → `ui-indicadores`, `master-detail` → `ui-tabela`,
   `upload-progress` → `ui-gravador` (o envio com `UploadScript`). A seção "usar este padrão"
   aparece embaixo do cartão da demo.
5. **Bilíngue**: `Summary` e `A11y` são inglês no Go (constituição: código em inglês); o site
   em `/pt` mostra a tradução de uma tabela do site, e `TestPatternsPageBilingual` falha se
   faltar tradução de alguma frase. Snippet e contrato de dados são código e não se traduzem.
6. **Catálogo aponta o padrão**: `uidoc.Component` ganha `patterns` (os padrões de que o
   componente participa), gerado junto do `catalog.json`; `trilha ui describe` mostra.
7. **Golden `uidoc` por padrão**: cada `page.go` é renderizado com dados falsos fixos pelo
   `TestUIDocPatternGoldens`, com nonce e token CSRF normalizados, em
   `internal/uidoc/testdata/patterns/<nome>.html`.

## User Scenarios & Testing

### US1 - O agente pede o padrão, não o componente (P1)

`trilha ui patterns --json` lista os seis com o formato estável; `trilha ui patterns <nome>`
imprime o snippet, o contrato e as notas; `get_pattern(name)` no MCP devolve o mesmo.

**Acceptance Scenarios**:

1. **Given** o agente precisa de uma listagem, **When** chama `get_pattern("list-with-filter")`,
   **Then** recebe um `page.go` de ≤60 linhas que compila, os componentes usados e o contrato.
2. **Given** um nome desconhecido, **Then** a resposta lista os nomes existentes.

### US2 - O snippet é verdade (P1)

Todo snippet compila (`go build ./examples/patterns/...`), cabe em 60 linhas, cita cada
componente que declara (`ui.<Nome>`), cada componente existe no catálogo, e o texto em
`ui.Patterns()` é byte a byte o arquivo.

### US3 - A demo ensina o padrão (P2)

Cada demo mapeada ganha "usar este padrão" (en) / "usar este padrão" (pt): componentes, snippet,
contrato, notas de acessibilidade.

## Edge Cases

- Snippet que passa de 60 linhas: `TestPatternSnippetBudget` falha nomeando o padrão.
- Componente renomeado no kit: `TestPatternsNameRealComponents` falha.
- Snippet editado sem regenerar: `TestPatternSnippetsCurrent` falha e diz o comando.

## Requirements

### Security and privacy impact (NIST SSDF 1.1 · OWASP ASVS 5.0 N2)

- **Fronteiras de confiança**: nenhuma nova. Os padrões são dados estáticos compilados no
  binário; `get_pattern` lê só essa tabela, sem entrada de usuário além do nome (comparado com
  a lista fixa).
- **O que os snippets ensinam** é o que um agente copia, então a revisão é de segurança:
  todo formulário de escrita leva `trilha.CSRFInput(c)`; texto só por `h.Text` (escape por
  padrão, nada de `h.Raw`); upload com `c.Files` e regras de tamanho/tipo; decisão de
  aprovação passa por `approval.Decide` (autorização do pacote); nenhum snippet desliga
  proteção (sem `NoNavigate`, sem `CSRFForAPI`). `TestPatternsKeepProtections` confere isso
  por texto (CSRF em todo `h.Method("post")`, nenhum `h.Raw`).
- **ASVS**: V5 (encoding na saída — escape por padrão), V4 (CSRF), V12 (upload com limites),
  V8 (autorização delegada ao `approval`). **A11y** (WCAG 2.2, não ASVS, mas contrato do kit):
  cada padrão declara foco, `aria-*` e teclado.
- **Segredos**: nenhum. **Exceções**: nenhuma.

### Functional Requirements

- **FR-001**: `ui.Pattern{Name, Summary, Components, Snippet, Data, A11y}` e `ui.Patterns()`
  com os seis padrões, em ordem estável.
- **FR-002**: `trilha ui patterns [--json] [nome]`.
- **FR-003**: `get_pattern(name)` em `mcp.ContextTools` e no MCP do site.
- **FR-004**: seção "usar este padrão" nas seis demos, nas duas línguas.
- **FR-005**: `patterns` no catálogo `uidoc` e no `ui describe`.
- **FR-006**: golden de render por padrão.

## Success Criteria

- **SC-001**: `TestPatternsCompile`, `TestPatternSnippetBudget`, `TestPatternsJSONStable`,
  `TestUIDocPatternGoldens`, `TestPatternsPageBilingual` verdes.
- **SC-002**: `make test` verde com `api/current.txt` atualizado só com `Pattern`/`Patterns`.
