# Spec 170 — Padrão de qualidade e E2E em três motores

**Feature Branch**: `170-padrao-de-qualidade` | **Created**: 2026-10-05 | **Status**: Entregue (0.151.0)

- **Origem**: pedido direto do mantenedor — adotar o padrão de testes (as sete práticas de E2E
  da IBM mais a pirâmide) antes de atacar as issues #290–#297, que mexem na navegação do
  cliente e precisam de cenário de navegador para provar a correção.
- **Versão**: 0.151.0

Forma curta: nada muda no framework nem na CLI. Mudam o módulo de teste `uitest/`, a CI, a
constituição e a documentação de testes.

## Por quê

O `uitest/` (spec 164) dirigia só o Chrome, por CDP. Um defeito que só o Safari ou o Firefox
mostram — e a navegação no cliente das #290–#297 é exatamente o tipo de código em que os motores
divergem — passava verde. Os jobs `ui` e `sql` da CI continuavam opcionais ("até o M3 fechar"),
com o M3 fechado desde a 0.146.0: um cenário vermelho não reprovava nada. E a `main` está com a
CI vermelha desde 26/09 (0.148.0) sem ninguém ver: o `gofmt` do Go 1.22 recusa uma linha em
branco em `ui/patterns_test.go`, e o `make vet` local não olhava a pasta `ui/`.

Os cenários também não estavam escritos como casos de teste: quem quisesse saber quais jornadas
estão protegidas, com que dados e o que se espera, lia Go.

## Jornada e risco

A jornada é a de quem mantém o framework (e de quem testa o próprio app com `uitest`): "a CI
verde quer dizer que as telas funcionam para quem usa". Pior impacto: uma release que quebra
login, formulário ou navegação num motor inteiro de navegador sem nenhum sinal.

## O que muda

- **`uitest` em Playwright** (`github.com/mxschmitt/playwright-go` v0.6201.1, Playwright 1.62.1),
  no lugar do chromedp. A API de `Session` é a mesma; os cenários não mudaram uma linha.
  - `UITEST_BROWSERS`: `chromium` (padrão), uma lista (`firefox,webkit`) ou `all`. Nome
    desconhecido é erro, nunca pulo.
  - `RunWith` roda o cenário em cada motor, todos mesmo depois de um falhar, e reprova com os
    relatórios juntos; o relatório ganha `browser:` e vira `report/<cenário>-<motor>.txt`.
  - `Session.Browser`, `Browsers()`, `AllBrowsers`, `InstallCommand(...)`, `RequireBrowsers(t)`
    (era `RequireBrowser`, que devolvia o caminho do Chrome), `Close()` para o `TestMain`.
  - Sai `UITEST_CHROME`/`BrowserPath`: o navegador é o que o Playwright instala, com versão
    casada com o driver. Sem ele, pula com o comando de instalação; `UITEST_REQUIRED=1` reprova.
- **`make test-ui-install`** baixa o driver e os navegadores (`WITH_DEPS=1` para as bibliotecas
  do sistema no Linux).
- **CI**: o job `ui` vira matriz `chromium`/`firefox`/`webkit`, `fail-fast: false`, e deixa de
  ser `continue-on-error`; o `sql` também. A `main` volta ao verde (`ui/patterns_test.go`), e o
  `make vet`/`make security`/`make fmt` passam a olhar `ui/` como a CI.
- **Catálogo de jornadas** `uitest/JORNADAS.md`: pirâmide do repositório, tabela de jornadas
  críticas com pior impacto, e um caso por cenário (jornada, risco, pré-condições, dados,
  passos, resultado esperado). `TestCatalogoDeJornadas` reprova cenário sem caso, caso sem
  cenário, campo faltando e cenário fora da tabela de jornadas.
- **Constituição 1.6.0**: o padrão vira parte do princípio VI; templates de spec e plano pedem
  jornada e risco, o nível do teste e a tabela de evidências.

## Decisões

1. **Playwright, não WebDriver BiDi.** É a única via que dirige os três motores do mesmo jeito,
   inclusive WebKit no Linux da CI (o `safaridriver` só existe no macOS).
2. **Um contexto novo por execução, navegador compartilhado por motor.** O contexto é um perfil
   isolado (cookies, storage, cache); relançar o navegador a cada cenário triplicaria o tempo
   com três motores. O "navegador novo" da segunda tentativa vira "contexto novo".
3. **O driver é por `PLAYWRIGHT_DRIVER_PATH`**: é o que deixa o teste do pulo apontar para um
   diretório vazio sem depender do que a máquina tem instalado.
4. **O catálogo é pt-BR** (como specs): é documento de teste do repositório, não página pública.

## Fora de escopo

- Cenários novos: a cobertura das #290–#297 entra nas specs delas, já nos três motores.
- Cache dos navegadores na CI: o download leva menos que a suíte; revisitar se pesar.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | o Playwright fica em `uitest/go.mod`; raiz e CLI intocadas (`TestNoExternalDeps`) |
| VI — teste primeiro | `TestBrowsersFromEnv` e `TestCatalogoDeJornadas` antes do código; o guarda reprovou com um caso renomeado |
| VII — segurança por padrão | cenários de CSP e segredo passam a valer nos três motores |

## Segurança e privacidade

- **Cadeia de suprimentos** (SSDF PW.4, Top 10 A03:2025): `playwright-go` fixado por versão e
  `go.sum`; ele baixa o `playwright-core` do npm e os navegadores da CDN da Microsoft, só em
  máquina de teste e na CI, nunca no build do app nem no binário.
- **Dados**: sintéticos (`example.com`, senhas literais de teste); servidor e contexto novos por
  execução, temporários removidos no `TestMain`.
- **Exceções**: nenhuma.

## Tarefas

- [x] T001 Testes que falham: `TestBrowsersFromEnv`, `TestNoBrowserSkipsUnlessRequired` (driver
  vazio), relatório com motor; `TestCatalogoDeJornadas`.
- [x] T002 `uitest.go` em Playwright, `report.go` com motor.
- [x] T003 `JORNADAS.md` com os 13 casos.
- [x] T004 `Makefile` (`test-ui-install`, `ui/` no gofmt), CI em matriz bloqueante, `ui/patterns_test.go`.
- [x] T005 Constituição 1.6.0, templates, `CLAUDE.md`, guia de testes nas duas locales, READMEs.
- [x] T006 CHANGELOG, ROADMAP, versão; suíte inteira com números.

## Aceitação

- **SC-001** — Os 13 cenários de navegador passam em Chromium, Firefox e WebKit
  (`UITEST_REQUIRED=1 UITEST_BROWSERS=all make test-ui`).
- **SC-002** — Um cenário sem caso no catálogo reprova o `make test-ui`.
- **SC-003** — Um navegador que não instalou reprova o job da CI em vez de passar verde.
- **SC-004** — `make test` verde, com o gofmt que a CI do Go 1.22 aplica.

## Evidências

| Nível | Comando | Passaram | Falharam | Pularam |
|---|---|---|---|---|
| Unidade + integração (raiz, CLI, exemplo) | `make test` | 1587 | 0 | 2 |
| Navegador, 13 cenários × 3 motores + 5 do próprio módulo | `UITEST_REQUIRED=1 UITEST_BROWSERS=all make test-ui` (~65 s) | 18 | 0 | 0 |
| Stores SQL | `make test-sql` (SQLite) | 3 | 0 | 0 |

Os dois pulos são `TestS3AoVivo` e `TestOIDCAoVivo`, que só rodam com credenciais de um serviço
de verdade nas variáveis de ambiente (por desenho, desde as specs que os criaram). Os testes
contam também subtestes. O PostgreSQL do `sqltest` roda na CI (`SQLTEST_POSTGRES=1`), não nesta
máquina.
