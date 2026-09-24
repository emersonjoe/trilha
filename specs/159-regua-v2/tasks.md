# Tasks: A régua v2

**Branch**: `159-regua-v2` | **Spec**: [spec.md](spec.md) | **Plan**: [plan.md](plan.md)

Convenções: "golden" = `make golden` + commit do gerado; "superfície" = `make api` +
commit de `api/current.txt` (esta spec não toca superfície); docs bilíngues = inglês em `/`
e pt-BR em `/pt` no mesmo commit. Aceites de régua rodam em `cd bench && go test ./agent/...`
porque `bench` é módulo separado — **correção de caminho** sobre o plano, que escreve
`go test ./bench/agent` (não resolve da raiz).

## Bloco 1 — a régua mede de verdade

- [x] **T01** — Estender `bench/agent/scenario.go` com o `Scenario` do plano (`ID`,
  `PromptMD`, `AppDir`, `BaseDir`, `Gate`, `MaxRounds`) e os campos do lado baseline; mover
  os 8 prompts v1 para `bench/agent/prompts/<nome>.md` byte a byte (dump do próprio
  `s.Prompt`, não digitação) e carregá-los do embed; criar os prompts congelados dos 4
  cenários novos e dos 8 lados baseline (`prompts/<id>.md` e `prompts/<id>.baseline.md`);
  registro `prompts/CHECKSUMS.txt` (SHA-256 por arquivo); `TestScenarioPromptsFrozen`
  (todo cenário tem prompt nos dois arquivos que lhe cabem; hash confere; o `TestRender`
  existente continua provando ordem e conteúdo). *Aceite:*
  `cd bench && go test ./agent/... -run 'TestScenarioPromptsFrozen|TestRender'`.

- [x] **T02** — Apps de partida trilha em `bench/agent/apps/{s5-login,s6-crud,s7-tela,
  s8-conserto}/` (mínimos, commitados, `trilha_gen.go` gerado e `go.mod` com `replace`
  para a raiz; o S8 com os 3 erros plantados: rota duplicada `E_DUPLICATE_ROUTE`, POST de
  página sem token CSRF no formulário, validação por tag quebrada); `Build` passa a entender
  `AppDir`; `TestScenarioApps` roda `trilha check` em cada app de partida e o teste de
  asserts do S8: fixture intocada falha com `E_DUPLICATE_ROUTE` no primeiro portão e, com os
  três consertos canônicos aplicados, `trilha check` fica verde (régua atingível, mesmo
  padrão de `TestPortListingEhAtingivel`). *Aceite:*
  `cd bench && go test ./agent/... -run 'TestScenarioApps|TestS8'`.

## Bloco 2 — a série e o gate

- [x] **T03** — Baselines Go puro em `bench/agent/baseline/{comments,contact-form,
  pagination,generate-crud,s5-login,s6-crud,s7-tela,s8-conserto}/`, um `go.mod` por app,
  só stdlib, mesma feature do lado trilha e o mesmo contrato que o prompt congelado fixa
  (rotas, campos, credenciais, gatilhos); testes escondidos do lado baseline no cenário;
  `TestBaselineBuilds` (`go build ./...` + `go test ./...` em cada um) e fixture do
  `s8-conserto` baseline com 3 erros plantados equivalentes e testes vermelhos. *Aceite:*
  `cd bench && go test ./agent/... -run TestBaselineBuilds`.

- [ ] **T04** — `Measurement` (esquema do plano + `Runs`) e `bench/agent/results/results.json`
  commitado (nasce vazio e honesto — a primeira medição real é do mantenedor, com
  `claude auth login`); executor com modo `-measure` (roda os dois lados dos cenários com
  baseline, grava a série; rodada com erro do agente não entra; `FilesOpened` = arquivos
  gravados na rodada) e alvo `bench-agent-measure` no Makefile — **correção de caminho**:
  o plano diz que a série é preenchida por `make bench-agent-agents`, mas esse alvo é a
  variante v1 do `AGENTS.md`; a medição v2 ganha alvo próprio e os alvos v1 ficam como
  estão. `TestMeasureSeries` com executor de mentira (16 linhas, datas ISO, sem campo de
  economia). *Aceite:* `cd bench && go test ./agent/... -run TestMeasureSeries` e
  `python3 -c "import json;json.load(open('bench/agent/results/results.json'))"`.

- [ ] **T05** — `Savings()` (medianas das rodadas verdes da data mais recente;
  `1 − trilha/baseline`) + `make bench-agent-verify MIN=45` no Makefile: falha se a média <
  MIN, se qualquer cenário < 0,60, se regredir > 5 pontos contra a última medição do
  cenário, ou se a série estiver incompleta; `-verify` no executor. *Aceite:* teste com
  fixtures JSON reproduzindo as três falhas e uma passagem
  (`cd bench && go test ./agent/... -run TestVerifyGate`).

## Bloco 3 — a vitrine

- [ ] **T06** — Página `/custos` e `/pt/custos` (`site/app/custos/page.go`,
  `site/app/pt/custos/page.go`, dados em `site/internal/custos/`): herói com a economia
  média em verde da marca, barra de progresso da meta (45/60/70), tabela cenário · tokens
  baseline · tokens trilha · economia · rodadas com badge por linha e cartões no celular
  (`ui.DataTable` com `ListState.Cards` — **correção de caminho**: o kit não tem `ui.List`),
  `ui.Bars` da economia por cenário, seção Metodologia com as regras do plano, data/modelo/
  agente da medição, rótulos `medido`/`est.`, estado vazio honesto; cópia do
  `results/results.json` embedada com sync testado. *Aceite:*
  `TestCustosPageMatchesResults` (`go test ./site/... -run TestCustos`) +
  `make test` verde.

- [ ] **T07** — Capítulo curto no Learn (en+pt) "The ruler"/"A régua" apontando para
  `/custos`, registrado nas duas `Locales`, no mesmo commit das páginas. *Aceite:*
  `go test ./site/... -run TestLocalesInSync` verde.

## Bloco 4 — CI e governança

- [ ] **T08** — CI: job `bench` em `.github/workflows/bench.yml` (`workflow_dispatch` +
  agenda semanal) rodando a régua e abrindo PR com o `results.json` novo; nada automático
  na `main`; segredo do agente via GitHub Secrets. *Aceite:* YAML válido
  (`python3 -c "import yaml,sys;yaml.safe_load(open('.github/workflows/bench.yml'))"`) e
  mesma forma dos workflows existentes.

- [ ] **T09** — Seção NIST SSDF 1.1 / OWASP ASVS 5.0 nível 2 com evidências na spec
  (a régua não toca fronteiras de confiança do produto; página com dados agregados e escape
  padrão; CI sem gatilho por PR; nenhum segredo commitado). *Aceite:* seção preenchida em
  spec.md (Requirements → Security and privacy impact) e revisada no fim da sessão.

## Fim da spec

- [ ] **T10** — Entrada no topo do `CHANGELOG.md` (inglês, formato `## X.Y.Z — YYYY-MM-DD`)
  e `ROADMAP.md` se a área muda de estado; sem `make release`, sem tocar `const version`,
  sem push, sem issues. *Aceite:* revisão do diff final + `make test` verde.
