# Implementation Plan: A régua v2 — medir tokens, rodadas e arquivos antes de cortar

**Branch**: `159-regua-v2` | **Date**: 2026-09-23 | **Spec**: [spec.md](spec.md)
**Input**: Spec 159 do PLANO-TOKENS-70.md (M0 — a régua mede antes de cortar).

## Summary

Estender a régua de `bench/agent` para medir o custo do agente nos dois lados da mesma
tarefa — app trilha e app Go puro equivalente — com prompts congelados em arquivos com hash,
gatilhos de conclusão objetivo (`Gate`), série versionada (`results/results.json`), função
`Savings`, gate de regressão (`make bench-agent-verify`) e a vitrine `/custos`/`/pt/custos`
no site, mais o capítulo "A régua" no Learn e o job semanal de CI. Nada muda no runtime, na
CLI nem na superfície pública: tudo vive em `bench/` (módulo separado), `site/` e
`.github/`.

## Technical Context

**Language/Version**: Go 1.22+ (módulos raiz, `bench`, e um `go.mod` aninhado por app de
partida e por baseline)
**Primary Dependencies**: nenhuma fora da stdlib (módulo raiz e CLI continuam limpos;
`TestNoExternalDeps` intocado). Os apps aninhados só dependem de `github.com/emersonjoe/
trilha` via `replace` local.
**Storage**: arquivos commitados (`results/results.json`, cópia embedada no site)
**Testing**: `go test` stdlib; `make test` na raiz cobre site; `cd bench && go test ./...`
cobre a régua; aceites por tarefa no tasks.md
**Target Platform**: CI Linux + macOS local
**Project Type**: biblioteca + CLI + site de docs (tudo já existente)
**Performance Goals**: n/a (régua mede outros)
**Constraints**: prompts congelados byte a byte; séries históricas nunca reescritas; zero
dependência nova no módulo raiz
**Scale/Scope**: 12 cenários na régua, 8 com baseline; 2 páginas novas no site + 1 capítulo;
1 workflow novo

## Constitution Check

- [x] **I. Convenção sobre configuração** — nenhuma convenção nova em `app/`; os apps de
  partida e baselines são projetos de régua, não exemplos de convenção.
- [x] **II. Só biblioteca padrão em runtime** — nada entra no módulo raiz; os módulos
  aninhados (apps/baselines) seguem o padrão registrado de `otel/` e a exceção do plano
  (baselines da régua com `go.mod` próprio). Baselines: só stdlib.
- [x] **III. Geração explícita** — os apps de partida têm `trilha_gen.go` commitado,
  gerado pela CLI, determinístico.
- [x] **IV. Contrato de handler** — nenhum símbolo público novo na raiz; `api/current.txt`
  não muda.
- [x] **V. Dev < 2s / binário único** — nada toca o pipeline de build/dev.
- [x] **VI. Teste primeiro** — cada bloco de tarefas fecha com teste; a régua ganha testes
  de contrato (prompts congelados, apps verdes, baselines compilando, gate com fixtures).
- [x] **VII. Segurança por padrão** — seção NIST/OWASP na spec; página pública só com dados
  agregados e escape padrão; CI da régua sem gatilho por PR; nenhum segredo commitado.
- [x] **Estilo e idioma** — código e identificadores em inglês; a página e o capítulo
  bilíngues no mesmo commit; spec/plan/tasks em pt-BR.
- [x] **Fluxo** — uma spec por sessão, commits pequenos por tarefa, sem trailer de
  coautoria; release continua sendo do mantenedor.

## Design (o que muda, onde)

```
bench/agent/
  scenario.go          # Scenario estendido (ID, PromptMD, AppDir, BaseDir, Gate,
                       #  MaxRounds, BaselinePromptMD, BaselineGate); 12 cenários;
                       #  prompts carregados do embed
  prompts/             # NOVO: <id>.md + <id>.baseline.md (congelados) + CHECKSUMS.txt
  apps/                # NOVO: s5-login, s6-crud, s7-tela, s8-conserto (módulos aninhados)
  baseline/            # NOVO: 8 apps stdlib (módulos aninhados, só stdlib)
  measure.go           # NOVO: Measurement, Load/Save da série v2, Savings
  verify.go            # NOVO: o gate (média, piso 0,60, regressão 5 pontos, série completa)
  main.go              # -measure e -verify; o resto como está
  fixture.go           # Build entende AppDir; Verify entende Gate e o binário da CLI
  results/results.json # NOVO: série v2 (nasce vazia; executor -measure preenche)
site/
  internal/custos/     # NOVO: embed da cópia do results.json + cálculo + copy das 2 línguas
  app/custos/page.go   # NOVO: /custos
  app/pt/custos/page.go# NOVO: /pt/custos
  custos_test.go       # NOVO: TestCustosPageMatchesResults + sync da cópia
  internal/docs/docs.go# aprenda/aprender ganham o capítulo "A régua"
  internal/docs/content/{en,pt}/... # the-ruler.md / a-regua.md
.github/workflows/
  bench.yml            # NOVO: workflow_dispatch + cron semanal; PR com a série nova
Makefile               # bench-agent-measure, bench-agent-verify
```

Ordem dos blocos: T01–T03 (a régua mede de verdade) → T04–T05 (a série e o gate) →
T06–T07 (a vitrine) → T08 (CI) → T09 (evidências, fechadas junto da spec) → CHANGELOG/ROADMAP.

## Risk Management

- **Prompt v1 movido com erro de transcrição**: os arquivos nascem de um dump do próprio
  `s.Prompt` em execução (não de digitação), e o `TestRender` existente continua passando —
  ele compara o conteúdo que a régua usa.
- **Baseline injusto (feature diferente do lado trilha)**: cada baseline tem o mesmo
  contrato que o prompt do lado trilha fixa (mesmas rotas, campos, credenciais, gatilhos);
  os testes escondidos dos dois lados afirmam sobre o mesmo comportamento externo.
- **Série vazia virar página quebrada**: o estado vazio é um caso de teste de primeira
  classe (`US3` cenário 2), porque é o estado de hoje até o mantenedor medir.
