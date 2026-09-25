# Feature Specification: Acabamentos do M3

**Feature Branch**: `165-acabamentos-do-m3` | **Created**: 2026-09-25 | **Status**: Draft
**Input**: as sugestões de issue deixadas pela spec 164 (`specs/164-testes-ui-a-ui/spec.md`,
"Registro da implementação") e pelo relatório de fechamento do Plano Tokens 70, pedidas pelo
mantenedor na mesma sessão. As stores SQL de billing e notify são a spec 166 (mudam o tipo da
store das duas receitas; não cabem aqui).

## O que muda

1. **Foco no 422 de página inteira.** Um formulário sem `ui.Swap` recusado com 422 volta como
   página nova; o kit só punha o foco no primeiro `aria-invalid` num swap. Agora o `ui.js`, ao
   carregar, foca o primeiro campo `aria-invalid="true"` de um formulário, se nada tiver
   `autofocus`. O orçamento do `ui.js` sobe de 29 KB para 30 KB (estava a 3 bytes).
2. **Página de erro nas receitas de login.** O modelo padrão (`blog`) não tem `app/error.go`, e
   um 401/403 de uma pasta protegida caía na página nua do framework (sem layout, em inglês). A
   receita `login` — a que traz os 401/403 — escreve `app/error.go` quando o projeto não tem um
   (o que já existe é pulado, como todo arquivo de receita).
3. **`/sair` com CSRF.** O logout é um `route.go` (nasce API, isento de CSRF): outro site podia
   deslogar quem visitasse. A receita escreve `{{.At}}sair/middleware.go` com `MiddlewarePOST`
   chamando `trilha.RequireCSRF`; o teste gerado prova o 403 sem token.
4. **Custo do pack medido no projeto.** O `trilha add` terminava com o `CtxPackCost` medido num
   projeto mínimo, que num projeto real fica abaixo do que `trilha ctx --pack` cobra. Agora o
   `add` mede o pack no próprio projeto, depois do `gen`; o `--list` continua com o preço de
   referência (`est.`). O e2e volta a exigir igualdade exata com o `ctx --pack --json`.
5. **`trilha mcp` com as ferramentas de contexto.** O servidor do projeto passa a oferecer
   `get_context`, `search_code` e `get_pattern` (`mcp.ContextTools`), só leitura, confinadas à
   raiz do projeto.

## Constitution Check

- II — só biblioteca padrão: nada novo no `go.mod` raiz.
- IV — API pública: nenhuma mudança de símbolo; comportamento aditivo.
- VI — teste primeiro: cada item tem teste (uitest, testes gerados, e2e, `mcp_test`).
- VII — segurança por padrão: itens 2 e 3 fecham um CSRF de logout e a página nua de erro.

## Segurança e privacidade (NIST SSDF 1.1 · OWASP ASVS 5.0 N2)

- **Fronteiras**: POST `/sair` (origem cruzada → agora exige token, ASVS V3.5/V4 CSRF);
  `trilha mcp` (stdio local; as ferramentas novas só leem arquivos sob a raiz, sem shell, sem
  rede — a mesma postura do servidor).
- **Impacto**: logout forçado por outro site deixa de ser possível; a página de erro não expõe
  detalhe (produção mostra só o id da requisição).
- **Exceções**: nenhuma.

## Tarefas

- [ ] **T01** — `trilha mcp` com `ContextTools`. *Aceite:* teste da lista de ferramentas.
- [ ] **T02** — `add` mede o pack no projeto. *Aceite:* e2e exige o mesmo número do `ctx --pack`.
- [ ] **T03** — login: `sair/middleware.go` com `RequireCSRF` + `app/error.go` quando falta.
  *Aceite:* teste gerado do 403 sem token; `TestUIAdminDefaultDeny` vê a página com layout.
- [ ] **T04** — `ui.js` foca o primeiro inválido ao carregar. *Aceite:* `TestUIBillingScreens`
  com `WantFocus("#moeda")` depois do 422 de página inteira.
- [ ] **T05** — Docs bilíngues, CHANGELOG 0.146.0, ROADMAP. *Aceite:* `make test` e
  `UITEST_REQUIRED=1 make test-ui` verdes.
