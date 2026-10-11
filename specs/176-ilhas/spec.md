# Spec 176 — Ilhas: redirect, CSRF, imports versionados, tipos entre pacotes e TypeScript

**Feature Branch**: `176-ilhas` | **Created**: 2026-10-10 | **Status**: Entregue (0.156.0)

- **Issues**: #298, #299, #300, #301, #302 — a fonte do escopo e dos critérios.
- **Versão**: 0.156.0

Forma curta: cinco ajustes na mesma fronteira (a ilha e o servidor), cada um pequeno e com
teste próprio; spec única porque compartilham fixture, documentação e release.

## Por quê

A ilha que sai do contador e vira um jogo, um editor ou um motor esbarra em cinco cantos: o
`island.post` que termina em `c.Redirect` recebe o HTML do destino (#302); toda `route.go`
chamada pela ilha precisa do mesmo `MiddlewarePOST` com `RequireCSRF`, e a que esquece fica
aberta a outro site na sessão do usuário (#300); o módulo importado por caminho relativo não é
versionado e roda velho depois do deploy (#298); o `islands.d.ts` não enxerga tipo de outro
pacote e obriga a duplicar o struct (#299); e não há caminho para escrever a ilha em
TypeScript sem bundler (#301).

## Jornada e risco

"Interação rica" + "segurança percebida". Pior impacto: POST forjado aceito numa `route.go`
com cookie de sessão (#300); a ilha nova rodando com o módulo velho do cache e quebrando para
todos até o cache expirar (#298); a ação da ilha "funcionando" no servidor sem a tela mudar
(#302).

## O que muda

- **#302** — o runtime (`ui.island.js`) manda `Trilha-Island: 1`; o `c.Redirect` desse pedido
  responde `204` + `Trilha-Location`, como para fragmento, e o `island.post` navega. O flash
  fica no cookie para a página de destino. Outros clientes continuam recebendo `303`.
- **#300** — pedido com corpo a uma `route.go` que chega com cookie e sem `Authorization`
  (`browserCredentials`) é conferido como POST de página, em `serve.go` e no `RequireCSRF`.
  Bearer e pedido sem cookie (webhook) seguem isentos; `Config.CSRFForAPI` segue valendo.
- **#298** — a página com ilhas leva um import map (`<script type="importmap" nonce>`, logo
  após `<head>`) que leva cada módulo de `public/` alcançado por import `./`, `../` ou `/` à
  URL com `?v=`. A busca lê os módulos (BFS, cache por caminho+versão); import nu fica de fora.
- **#299** — o scanner do `islands.d.ts` resolve o import do pacote da página: pacote do mesmo
  módulo (achado pelo `go.mod`, nome do pacote pela cláusula) é lido e os tipos entram; pacote
  de fora vira `unknown` com uma nota que diz o campo e o módulo.
- **#301** — `trilha islands build [--check]`: `islands/tsconfig.json`, `tsc` do projeto
  (`TRILHA_TSC`, `node_modules/.bin/tsc`, `PATH`) com `--rootDir islands --outDir public
  --listEmittedFiles --noEmitOnError`; cabeçalho `Code generated` em `.js`/`.d.ts`; source map
  deslocado uma linha e com `sources` relativo a `public/`; `--check` compila em pasta
  temporária e reprova diferente, faltando e órfão com cabeçalho. Portão `islands` no
  `trilha check`, pulado sem o tsconfig.

## Decisões

1. **Cabeçalho próprio, não `Accept`.** O `Accept: application/json` também vem de clientes
   de API que querem o `303`; o `Trilha-Island` só o runtime manda.
2. **Cookie sem `Authorization` = navegador.** É o critério que separa a página do cliente de
   API sem configuração; quem chama a API com cookie e sem token de propósito liga
   `CSRFForAPI` ou passa a mandar o token (mudança de comportamento anotada no CHANGELOG).
3. **Import map, não reescrever o módulo.** Reescrever o `import` no servidor mudaria o
   arquivo servido e o hash; o mapa é padrão do navegador e não toca no módulo. Limite: a ilha
   que chega por fragmento numa página sem ilhas fica sem mapa (o módulo dela é versionado, os
   imports não) — o navegador só aceita import map antes do primeiro módulo.
4. **Sem `go/packages`** no `islands.d.ts`: o scanner lê os pacotes do módulo com `go/parser`
   (princípio II, zero dependências na CLI).
5. **Saída do `islands build` commitada.** O `go build` não tem node; o `--check` mantém a
   disciplina do `gen --check`. Sem `tsc` o portão reprova (não dá para conferir) em vez de
   passar calado.
6. **O `examples/blog` não ganha `islands/`.** O exemplo roda sem node em toda a CI; a
   convenção é coberta por teste com `tsc` falso compilado em Go e conferida à mão com o
   TypeScript 5 real (build, `--check`, erro de tipo que não grava nada).

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `go/parser`, `encoding/json`, `os/exec`; nada novo no `go.mod` |
| IV — API pública | nenhum símbolo exportado novo; `api/current.txt` inalterado; `trilha islands build` novo na CLI |
| VI — teste primeiro | `TestIslandRedirectBecomesLocation`, `TestRouteWithCookiesNeedsCSRF`, `TestIslandImportsAreVersioned`, `TestPropsFromOtherPackagesOfTheModule` e `TestIslandsBuild*` falhavam antes; `TestUIIslandPostRedirects` reprova com o runtime sem o cabeçalho (conferido) |
| VII — segurança por padrão | CSRF passa a valer onde o navegador manda cookie, sem opt-in |

## Segurança e privacidade

- **Fronteira de confiança**: navegador → `route.go`. O pedido com cookie de sessão e sem
  `Authorization` agora exige o token de duplo envio (ASVS V3.5.1, Top 10 A01:2025). Bearer e
  webhook sem cookie: inalterados, testados.
- **Import map**: só caminhos de `public/` resolvidos pelo servidor; JSON escapado; `nonce` da
  CSP. Nenhuma URL vinda do pedido entra no mapa.
- **`islands build`**: executa o compilador escolhido pelo projeto (`TRILHA_TSC`/`node_modules`/
  `PATH`), só por comando explícito; nada roda no `go build` nem no servidor. Só marca arquivos
  que o `tsc` listou dentro da pasta de saída.
- **Exceções**: nenhuma.

## Tarefas

- [x] T001 Testes primeiro: redirect da ilha, CSRF com cookie, import versionado, tipos entre
  pacotes (`testdata/islandsmod`), `islands build` com `tsc` falso.
- [x] T002 `render.go`, `csrf.go`, `serve.go`, `importmap.go`, `island.go`, `ui.island.js`.
- [x] T003 `internal/islands`: imports do módulo, nome do pacote pela cláusula, nota de tipo externo.
- [x] T004 `cmd/trilha/islandsbuild.go`, `case "islands"`, portão `islands`, mensagens en/pt.
- [x] T005 Cenários `TestUIIslandImportIsVersioned` e `TestUIIslandPostRedirects`, casos no
  `JORNADAS.md`, fixture `/fluxos/ilha/salto`.
- [x] T006 Documentação nas duas locales (ctx, api, ilha-react, cli, o-teto), `SECURITY-MODEL`,
  cópia do kit no `examples/blog`.
- [x] T007 CHANGELOG, ROADMAP, versão; suíte inteira com números.
- [x] T008 `make security` reprovou com avisos da biblioteca padrão publicados depois da
  0.155.1 (GO-2026-6599 e outros, sem correção na linha 1.25): `SECURITY_GO_VERSION` passa a
  `go1.26.9`, com nota no CHANGELOG. Não é efeito desta spec; é o portão pegando o mundo.

## Aceitação

- **SC-001** Clicar no botão da ilha que posta numa rota com `c.Redirect` leva o navegador ao
  destino, com o flash.
- **SC-002** POST com cookie e sem token numa `route.go` responde 403; com o token, com bearer
  ou sem cookie, passa.
- **SC-003** O módulo importado por `./motor.js` é buscado com `?v=`, sem erro no console.
- **SC-004** `islands.d.ts` tem `Card` de `internal/api` com os campos; `uuid.UUID` é `unknown`
  com nota.
- **SC-005** `trilha islands build --check` passa logo após o build, reprova saída editada,
  faltando e órfã; erro de tipo reprova com a mensagem do `tsc` e não grava nada.

## Evidências

| Nível | Comando | Passaram | Falharam | Pularam |
|---|---|---|---|---|
| Unidade + integração | `make test` | 1600 | 0 | 2 |
| Navegador, 39 cenários × 3 motores + 5 do módulo | `UITEST_REQUIRED=1 UITEST_BROWSERS=all make test-ui` | 44 | 0 | 0 |

Pulos: `TestS3AoVivo`, `TestOIDCAoVivo` (serviço real). Sem teste automatizado: o `islands
build` com o `tsc` real (a CI não instala TypeScript; conferido à mão com o TypeScript 5) e a
ilha que chega por fragmento numa página sem ilhas (limite documentado, decisão 3).
