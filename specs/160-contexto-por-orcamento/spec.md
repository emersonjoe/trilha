# Feature Specification: Contexto sob orçamento

**Feature Branch**: `160-contexto-por-orcamento` | **Created**: 2026-09-24 | **Status**: Draft
**Input**: Plano Tokens 70, spec 160 — o agente para de abrir arquivos para descobrir o
projeto: `trilha ctx` vira um mapa compacto, orçado em tokens, por pacote de trabalho; o
`AGENTS.md` emagrece; o MCP entrega fatias, não arquivos.

## Contexto

A leitura é ~40% do custo de um agente (hipótese do plano, §1.2). O `trilha ctx` (spec 047)
já responde "o que o projeto tem?" em uma leitura, mas não diz quanto custa, não corta sob
orçamento e não tem fatia por receita. O `AGENTS.md` do scaffold não tem alvo de tamanho
medido. O MCP do site entrega páginas de doc inteiras. Esta spec dá orçamento e fatia a esse
conjunto — sem tocar a superfície pública do módulo raiz (`internal/` não entra em
`api/current.txt`).

### Decisões que vinculam a implementação

1. **`--pack app` é o mapa inteiro; `--pack <receita>` é a fatia.** O `Pack` do plano é a
   forma da saída `--json`; a saída humana é a mesma informação com o custo no rodapé.
2. **Ordem de corte documentada**: contratos (operações de API e tipos) primeiro, receitas
   depois, enums, e rotas por último — rotas são o mínimo útil e nunca são cortadas. O
   estouro nas seções cortadas vai em `Pack.Truncated`; `--strict` torna estouro erro
   (`E_CTX_BUDGET`, exit 1).
3. **`CharsPerToken = 4`** é constante documentada e testada como contrato de estimativa,
   rotulada `est.` em toda saída.
4. **A estimativa do lado do site** (`llms.txt` por receita e o badge das páginas de receita)
   mede o conteúdo da própria fatia de doc com o mesmo `CharsPerToken`; o rótulo é `est.`
   até o marco M1 medir de verdade.
5. **`provided()` mascara literais de string.** O mapa cita expressões de `setup.go`; um
   segredo escrito dentro de um `Provide` não pode vazar para a saída — nomes e tipos ficam,
   conteúdo de literal não.

## User Scenarios & Testing

### US1 - O agente pede o mapa com orçamento (P1)

`trilha ctx --pack app --budget 1500 --json` devolve o mapa do projeto nas seções do `Pack`
do plano; o que não coube está nomeado em `truncated` na ordem documentada; com `--strict`,
estouro é `E_CTX_BUDGET` com a lista do que foi cortado e como re-chamar.

**Independent Test**: testes em `internal/ctx` com fixture (blog) — corte na ordem,
`--strict`, golden do `--json` regenerado.

**Acceptance Scenarios**:

1. **Given** o blog com `--budget` apertado, **When** o pack é montado, **Then** contratos
   somem antes de receitas, receitas antes de enums, rotas nunca.
2. **Given** `--strict` com estouro, **When** o comando roda, **Then** sai 1 com
   `E_CTX_BUDGET` e a lista do que faltou.
3. **Given** qualquer saída humana ou JSON, **When** um segredo está escrito num `Provide`
   do `setup.go`, **Then** a string do segredo não aparece em nenhuma saída.

### US2 - O AGENTS.md nasce com alvo de tamanho (P1)

`trilha new` grava um `AGENTS.md` com seções fixas (mapa → `ctx --json`, receitas instaladas
com link de doc, portões `make test`/`trilha check`, regras de leitura estreita) com alvo
≤ 2.500 tokens estimados, medido por teste no scaffold gerado.

**Independent Test**: `TestAgentsMdBudget` sobre o projeto gerado pelo `trilha new`.

**Acceptance Scenarios**:

1. **Given** o projeto do `trilha new`, **When** o AGENTS.md é medido com `tokbudget`,
   **Then** fica ≤ 2.500 tokens est.
2. **Given** o AGENTS.md, **When** um agente o lê, **Then** encontra as quatro seções fixas
   (mapa, receitas, portões, leitura).

### US3 - Fatias de doc e de código pelo MCP e por arquivo (P2)

O MCP do site ganha `get_context(pack)` (as páginas de doc da receita, com custo `est.`) e
`search_code(query)` (`path:linha±N`, nunca o arquivo inteiro). Cada receita ganha um
`llms.txt` próprio com o custo `est.` por página, e a página da receita no site exibe o badge
mono do custo do pack, rotulado `est.`.

**Independent Test**: testes de ferramenta no docsmcp/ai-mcp e `TestLLMsTxtPerRecipe` +
teste de conteúdo do site nas duas línguas.

### US4 - A regra está documentada (P2)

Capítulo "Context under budget"/"Contexto sob orçamento" no Learn (en+pt, mesmo commit) com
exemplos de chamada e a ordem de corte.

## Edge Cases

- `--budget` menor que o custo das rotas sozinhas: rotas permanecem (mínimo útil), tudo o
  mais vai para `truncated`, e o rodapé diz o custo real de vez em vez o pedido.
- `--pack <receita>` para receita não instalada: erro com a lista de receitas instaladas.
- Projeto sem `setup.go` (nenhuma receita instalada): seção de receitas vazia, sem erro.
- `--pack` com `--routes/--types/--all`: essas views continuam valendo sem pack; `--pack`
  com view específica é rejeitado (uma coisa por vez).

## Requirements

### Security and privacy impact

- **Assets and trust boundaries**: `internal/ctx` e `internal/tokbudget` leem o projeto como
  o scanner já lê; a única fronteira nova é a saída citar menos, nunca mais — literais de
  string em `Provide` são mascarados, e o teste negativo de segredo prende isso.
- **ASVS 5.0 Level 2 controls**: V1.14 (segredos) — o teste `TestCtxNeverLeaksSecrets` monta
  app com segredo selado e varre todas as formas de saída; V14.1 — nenhuma dependência nova,
  nenhuma superfície pública nova no módulo raiz.
- **OWASP Top 10:2025 risks**: A02/A04 não se aplicam (nada de criptografia nova nem
  falha de design nova); o vazamento por citação de código é o risco atacado, com máscara e
  teste.
- **Secrets and personal data**: nenhum segredo novo; o mascaramento impede que um segredo
  escrito em fonte acabe na saída do mapa (que agentes leem).
- **Exceptions**: nenhuma.

### Functional Requirements

- **FR-001**: `internal/tokbudget` MUST expor `CharsPerToken = 4` e `Estimate(s) int`,
  com testes de contrato.
- **FR-002**: `trilha ctx` MUST aceitar `--pack app|<receita>`, `--budget N`, `--strict`,
  mantendo `--json/--routes/--types/--all`; a saída humana ganha receitas instaladas,
  convenções e rodapé com custo `est.`.
- **FR-003**: A ordem de corte sob orçamento MUST ser contratos → receitas → enums, rotas
  por último (nunca cortadas), registrada em `Pack.Truncated` e em doc-comment.
- **FR-004**: `--strict` MUST falhar com `E_CTX_BUDGET`, exit 1, quando algo for cortado.
- **FR-005**: `trilha agents` MUST gravar o AGENTS.md v2 (mapa, receitas, portões, leitura
  estreita) com alvo ≤ 2.500 tokens est., medido por teste.
- **FR-006**: O MCP do site MUST servir `get_context(pack)` e `search_code(query)`
  (`path:linha±N`, nunca arquivo inteiro).
- **FR-007**: Cada receita MUST ter `llms.txt` próprio com custo `est.` por página, e a
  página da receita MUST exibir o badge mono de custo `est.` do pack.
- **FR-008**: Capítulo bilíngue do Learn MUST documentar chamadas e a ordem de corte.

### Key Entities

- **Pack**: a fatia orçada do mapa — rotas, receitas, convenções, contratos, orçamento,
  custo e o que foi cortado.
- **tokbudget**: o estimador único (4 chars/token) que ctx, agents e o site compartilham.

## Success Criteria

### Measurable Outcomes

- **SC-001**: `go test ./internal/tokbudget/... ./internal/ctx/...` verdes, goldens do blog
  regenerados e estáveis.
- **SC-002**: `TestAgentsMdBudget` verde no scaffold do `trilha new`.
- **SC-003**: `make test` verde com as ferramentas MCP, o llms por receita e o capítulo nas
  duas línguas.

## Assumptions

- A estimativa de 4 chars/token é intencionalmente burra e pública: serve para orçar, não
  para faturar; cada saída que a usa carrega `est.`
- O MCP de projeto (apps que expõem suas próprias ferramentas) não é desta spec; o que a spec
  adiciona é o do site, que já existe (spec 155), e os helpers de `ai/mcp` para quem quiser
  servir contexto de projeto.
