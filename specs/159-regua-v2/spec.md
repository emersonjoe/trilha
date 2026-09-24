# Feature Specification: A régua v2 — medir tokens, rodadas e arquivos antes de cortar

**Feature Branch**: `159-regua-v2` | **Created**: 2026-09-23 | **Status**: Draft
**Input**: Plano Tokens 70 (PLANO-TOKENS-70.md), spec 159 — primeira dos seis marcos do
programa "um agente gasta 70% menos tokens para criar um app com o trilha". O plano é a
fonte do escopo; este documento fixa o contrato e as decisões que o plano deixou em aberto.

## Contexto

A régua atual (`bench/agent/`, módulo `bench`) mede o custo de um agente para entregar uma
feature em um exemplo do trilha e compara Trilha com Trilha (versões, variantes de
`AGENTS.md`). Ela não tem: lado de partida em Go puro (`net/http` + `html/template`) para a
mesma tarefa, gatilho de conclusão objetivo por cenário (comandos que precisam passar),
prompts congelados em arquivos com hash registrado, série versionada como fonte de uma
página pública, nem gate de regressão. Sem a linha de base Go puro, a meta "70% menos
tokens" não tem denominador — e sem a página `/custos`, não tem vitrine.

### Decisões que o plano deixou em aberto (registradas aqui porque vinculam a implementação)

1. **A série de economia tem 8 cenários × 2 lados.** O plano diz "os 4 cenários fixos atuais
   e os dois extras (`port-listing`, `api-call`) continuam" (§1.1 — 10 cenários na régua) e
   também "primeira série completa (8 cenários × 2 lados)" (T04). Lê-se assim: os 10
   continuam na régua; a série de economia publicada cobre os 8 que têm equivalente stdlib
   honesto — os 4 novos (`S5`–`S8`) mais `comments`, `contact-form`, `pagination` e
   `generate-crud`. `cognito` não tem equivalente stdlib (trocar de provedor OIDC em stdlib
   não é a mesma tarefa), `fix-hint` mede o Hint do próprio framework (o que a régua mede lá
   é o trilha, não a tarefa), e `port-listing`/`api-call` medem a portagem para o trilha e
   exigem serviço de fora — os quatro continuam medidos do lado trilha e entram na série de
   economia quando ganharem baseline.
2. **Dois prompts congelados por cenário**: `prompts/<id>.md` (lado trilha) e
   `prompts/<id>.baseline.md` (lado Go puro) — o mesmo contrato de feature, com os idiomas
   de cada lado. Os 8 prompts v1 saem do código-fonte e entram em arquivos **byte a byte
   iguais** (a série histórica não reabre: nada muda de conteúdo); um registro de SHA-256
   (`prompts/CHECKSUMS.txt`) prende todos.
3. **`results/results.json` nasce vazio e honesto.** O executor v2 (`-measure`) é quem o
   preenche; a primeira medição real exige `claude auth login` e custo do mantenedor
   (dezenas de dólares por rodada completa), então esta spec entrega a régua pronta e a
   página no estado "primeira medição pendente". Nenhum número estimado é inventado aqui.
4. **`FilesOpened` mede arquivos gravados pelo agente** (mtime posterior ao início da
   rodada), porque o JSON de resultado do agente não relata leituras. O nome do campo é do
   plano; o significado medido fica documentado na metodologia da página.
5. **Caminhos corrigidos** (o repositório evoluiu depois do plano): `bench/agent` é um
   pacote do módulo separado `bench` (não da raiz), então os aceites de teste rodam com
   `cd bench && go test ./agent/...`; e a tabela de cartões usa `ui.DataTable` com
   `ListState.Cards` (o kit não tem `ui.List`; `ui.Cards()` é a flag de cartões). Ambos
   registrados no tasks.md.

## User Scenarios & Testing

### US1 - O mantenedor mede a régua e publica a série (P1)

O mantenedor roda `make bench-agent-measure` (após `claude auth login`): para cada cenário
com baseline, o executor roda o agente dos dois lados — a cópia do app trilha e a cópia do
baseline Go puro — com os prompts congelados, verifica os gatilhos (`Gate`) e grava uma
linha `Measurement` por (data, cenário, lado) em `bench/agent/results/results.json`. A
economia nunca é gravada: é calculada de onde for lida.

**Independent Test**: `TestMeasureSeries` em `bench/agent/measure_test.go` — com o executor
de mentira (`stub`), duas rodadas fictícias produzem o JSON com as 16 linhas esperadas,
datas ISO, sem campo de economia.

**Acceptance Scenarios**:

1. **Given** um executor de testes que devolve usage fixo, **When** `-measure` roda os 8
   cenários com baseline, **Then** `results/results.json` tem 16 linhas `Measurement`
   (8 cenários × 2 lados) com `date` ISO e nenhum campo de economia.
2. **Given** a série v1 em `bench/agent/results.json`, **When** uma medição nova é gravada,
   **Then** a série v1 não é reescrita (só recebe execuções novas, como sempre).
3. **Given** um cenário cuja verificação falha dos dois lados, **When** a medição é gravada,
   **Then** a linha registra o que foi gasto mesmo assim (`Passed` não existe em
   `Measurement`; a economia do cenário só é publicada quando os dois lados têm rodada
   verde na mesma data — ver US3).

### US2 - O gate de regressão bloqueia o avanço (P1)

`make bench-agent-verify MIN=45` lê a série e falha quando: (a) a média das economias fica
abaixo de MIN; (b) qualquer cenário fica abaixo de 60 pontos (regra anti-vício do plano:
"nenhum cenário < 0,60"); (c) qualquer cenário regrediu mais de 5 pontos contra a medição
anterior dele; (d) a série está incompleta (cenário com baseline sem medição dos dois
lados). É o gate que entra no portão de release a partir do marco M1.

**Independent Test**: `TestVerifyGate` em `bench/agent/measure_test.go` — quatro fixtures
JSON: as três falhas do plano e uma passagem.

**Acceptance Scenarios**:

1. **Given** média de economia 0,38 com `MIN=45`, **When** o gate roda, **Then** sai 1
   citando a média.
2. **Given** um cenário com economia 0,52 (≥ 60 − 8) e os outros bons, **When** o gate roda,
   **Then** sai 1 citando o cenário abaixo do piso de 0,60.
3. **Given** um cenário que media 0,72 na medição anterior e 0,66 na atual, **When** o gate
   roda, **Then** sai 1 citando a regressão de 6 pontos.
4. **Given** a série completa com todos os pisos atendidos, **When** o gate roda com
   `MIN=45`, **Then** sai 0.

### US3 - Quem chega ao site entende a promessa e a prova em 10 segundos (P1)

`/custos` (e `/pt/custos`) abre com o número-herói — a economia média da série mais
recente, em verde da marca —, a barra de progresso da meta (45/60/70), a tabela por cenário
(cenário · tokens baseline · tokens trilha · economia · rodadas, com badge de meta por
linha) que vira cartões no celular, o gráfico de barras da economia por cenário e a seção
"Metodologia" com as regras anti-vício em texto corrido, a data da medição, o modelo e o
agente usados, e o rótulo `medido` em todo número que veio da série (`est.` é o rótulo
reservado para quando o plano pedir estimativa). Sem medição, a página diz isso e mostra o
comando que mede — nunca um número inventado.

**Independent Test**: `TestCustosPageMatchesResults` em `site/custos_test.go` — a página
renderiza a partir do JSON embutido e todo número exibido confere com o que o JSON traz;
`TestLocalesInSync` do site cobre as duas línguas.

**Acceptance Scenarios**:

1. **Given** a série com 16 linhas, **When** `/custos` renderiza, **Then** o herói mostra a
   média calculada das economias, cada linha da tabela traz os tokens medianos dos dois
   lados e a economia do cenário, e o gráfico tem uma barra por cenário.
2. **Given** a série vazia (hoje), **When** `/custos` renderiza, **Then** a página mostra o
   estado "primeira medição pendente" com o comando `make bench-agent-measure` e nenhum
   número.
3. **Given** qualquer estado da série, **When** a página renderiza em inglês e em português,
   **Then** as duas têm as mesmas seções e o mesmo conjunto de números.

### US4 - O cenário de conserto é atingível e conta rodadas (P2)

O app `s8-conserto` chega com 3 erros plantados (rota duplicada, POST de página sem token
CSRF no formulário, validação por tag quebrada). `trilha check` falha nele — primeiro com
`E_DUPLICATE_ROUTE`, e um erro de cada vez, porque o portão para no primeiro grupo que
falha; é essa sequência que vira a métrica "rodadas-até-verde". O teste de asserts prova os
dois lados: a fixture falha com o código esperado, e a aplicação dos três consertos
canônicos deixa `trilha check` verde (a régua é atingível — mesmo padrão de
`TestPortListingEhAtingivel`).

**Independent Test**: `TestS8FixtureFailsWithPlantedErrors` e `TestS8EhAtingivel` em
`bench/agent/agent_test.go`.

**Acceptance Scenarios**:

1. **Given** a fixture s8 intocada, **When** `trilha check --json` roda, **Then** `ok` é
   false e o primeiro problema é o do portão gen com `E_DUPLICATE_ROUTE` na mensagem.
2. **Given** os três consertos canônicos aplicados, **When** `trilha check` roda, **Then**
   sai 0 (verde) — prova de que os 3 erros são todos os que a fixture tem.
3. **Given** cada app de partida S5–S7, **When** `trilha check` roda nele, **Then** sai 0.

## Edge Cases

- Cenário sem baseline (`cognito`, `fix-hint`, `port-listing`, `api-call`): o `-measure`
  pula o lado baseline e o registra como `skipped`; o gate de série completa não o exige.
- Execução do agente com erro (autenticação, rede): a medição da rodada não entra na série
  (o mesmo que a régua v1 faz — `Usage.Error` descarta a rodada).
- `results/results.json` corrompido ou ausente: `Load` devolve série vazia e o gate falha
  com "série incompleta", não com pânico.
- Página `/custos` com série de uma data só: não há regressão a exibir; a metodologia diz
  que a comparação passa a valer da segunda medição em diante.
- Um cenário novo entra na régua depois do M0: acrescenta linha ao `CHECKSUMS.txt` e à
  lista de cenários; o gate passa a exigir os dois lados dele (o plano manda congelar o
  cenário novo antes da otimização).

## Requirements

### Security and privacy impact

- **Assets and trust boundaries**: a régua vive em `bench/agent/` (módulo `bench`, fora do
  runtime e da CLI) e os apps de partida/baseline em módulos aninhados próprios — nada do
  que esta spec entrega entra no binário do framework nem na superfície pública
  (`api/current.txt` não muda). A página `/custos` é conteúdo estático do site: só números
  agregados.
- **ASVS 5.0 Level 2 controls**: V14.1 (arquitetura) — nenhum componente novo no runtime;
  V14.3 (segurança da cadeia de build) — o CI da régua roda por `workflow_dispatch` e
  agenda semanal, nunca por PR, e os prompts/checksums são commitados para que a medição
  seja auditável; V1.14 (segredos) — o workflow referencia o segredo de autenticação do
  agente via GitHub Secrets, nunca em texto.
- **OWASP Top 10:2025 risks**: nenhum risco novo — a página publica dados agregados que
  passam pelo escape padrão do `h.*` (A03 injeção não se aplica: nenhum dado do agente ou
  de app chega ao HTML, só o JSON commitado); a régua não altera fronteiras de confiança
  do produto (o site continua servindo o que sempre serviu, com CSP/nonce do framework).
- **Secrets and personal data**: nenhum segredo no repositório; os apps de partida usam
  credenciais fictícias de teste (`admin@exemplo.com`), documentadas como tais. O log do
  agente não é commitado; o `results/results.json` guarda só contadores.
- **Exceptions**: nenhuma.

### Functional Requirements

- **FR-001**: `bench/agent` MUST estender `Scenario` com `ID`, `PromptMD`, `AppDir`,
  `BaseDir`, `Gate`, `MaxRounds` (formato do plano) e os campos do lado baseline
  (`BaselinePromptMD`, `BaselineGate`), mantendo os campos v1 intactos.
- **FR-002**: A régua MUST ter os 12 cenários: os 8 v1 (com os prompts movidos byte a byte
  para `prompts/<nome>.md` e carregados de lá) e os 4 novos `s5-login`, `s6-crud`,
  `s7-tela`, `s8-conserto` com `ID` do formato `S<n>-<slug>`.
- **FR-003**: Todo prompt de cenário MUST viver em `bench/agent/prompts/` com SHA-256
  registrado em `prompts/CHECKSUMS.txt`; `TestScenarioPromptsFrozen` MUST falhar se um
  arquivo faltar ou o hash divergir.
- **FR-004**: Os apps de partida `bench/agent/apps/{s5-login,s6-crud,s7-tela,s8-conserto}/`
  MUST ser apps trilha mínimos commitados (com `trilha_gen.go` gerado e `go.mod` próprio com
  `replace` para a raiz), cada um passando em `trilha check`; o s8 MUST falhar com os 3
  erros plantados na ordem documentada.
- **FR-005**: Os baselines `bench/agent/baseline/<id>/` MUST existir para os 8 cenários da
  série, um `go.mod` por app, só biblioteca padrão, mesma feature do lado trilha, e MUST
  passar em `go build ./...` e `go test ./...`.
- **FR-006**: O esquema `Measurement` MUST seguir o plano (`Date`, `Model`, `Scenario`,
  `Side`, `TokensIn`, `TokensOut`, `Rounds`, `FilesOpened`) com `Runs` a mais; a série vive
  em `bench/agent/results/results.json`; `TokensIn` é a entrada do agente somada à leitura
  de cache, `TokensOut` a saída — definido na metodologia.
- **FR-007**: `Savings(trilha, baseline []Measurement) map[string]float64` MUST devolver, por
  cenário, `1 − tokens(trilha)/tokens(baseline)` sobre as medianas das rodadas verdes da
  data mais recente; cenário sem lado verde não tem economia.
- **FR-008**: `make bench-agent-verify MIN=<pontos>` MUST sair 1 quando a média < MIN, algum
  cenário < 0,60, alguma regressão > 5 pontos contra a medição anterior do cenário, ou a
  série incompleta; e 0 quando tudo passa.
- **FR-009**: O executor MUST ter o modo `-measure` (alvo `bench-agent-measure` no Makefile)
  que roda os dois lados dos cenários com baseline e grava a série; os alvos v1
  (`bench-agent`, `bench-agent-agents`, `bench-agent-dry`) MUST continuar funcionando como
  estão.
- **FR-010**: O site MUST ter `/custos` e `/pt/custos` renderizadas de um embed do
  `results/results.json` (cópia commitada em `site/internal/custos/`, com teste que falha
  quando a cópia diverge da fonte), com herói, barra de progresso da meta, tabela→cartões
  (`ui.DataTable` com `Cards`), `ui.Bars`, metodologia e estados vazios honestos, nas duas
  línguas.
- **FR-011**: O Learn MUST ter o capítulo "The ruler"/"A régua" (en e pt no mesmo commit)
  apontando para `/custos`, registrado nas duas `Locales`.
- **FR-012**: O CI MUST ter o job `bench` (`.github/workflows/bench.yml`) com
  `workflow_dispatch` e agenda semanal que roda a régua e abre PR com a série nova; nada
  automático na `main`.

### Key Entities

- **Scenario (v2)**: o cenário da régua com contrato completo — prompt congelado, app de
  partida, baseline, gatilho de conclusão por lado e teto de rodadas.
- **Measurement**: uma linha da série — o que o agente gastou em (data, cenário, lado),
  medianas das rodadas verdes; a economia é derivada, nunca armazenada.
- **App de partida / baseline**: os dois pontos de partida da mesma tarefa — trilha e
  stdlib — commitados e construíveis sozinhos.

## Success Criteria

### Measurable Outcomes

- **SC-001**: `cd bench && go test ./agent/...` verde: prompts congelados com hash, apps de
  partida verdes no `trilha check` (s8 vermelho como especificado), baselines compilando e
  testando, fixtures de gate reproduzindo as 4 saídas.
- **SC-002**: `make test` (raiz) verde com as páginas `/custos` e `/pt/custos`, o capítulo
  do Learn nas duas línguas e os testes do site conferindo página contra JSON.
- **SC-003**: `make bench-agent-verify MIN=45` existe, é testado com fixtures e documenta o
  que faz falhar; a primeira medição real fica registrada como pendência do mantenedor na
  página (estado vazio) e no relatório da spec.

## Assumptions

- A primeira medição real (custo do agente, `claude auth login`) é do mantenedor, como o
  risco 1 do plano prevê ("revisão humana da série antes de publicar"); esta spec entrega a
  régua pronta e a página em estado vazio honesto.
- O modelo usado na primeira medição será registrado no próprio `results/results.json`
  (`Model` por linha) e na metodologia da página, que lê o que a série traz.
- As credenciais dos cenários S5 (`admin@exemplo.com` / `senha-trilha-2026`) são fixadas no
  prompt congelado porque o teste escondido precisa de um contrato; são fictícias e só
  existem dentro das fixtures temporárias.
