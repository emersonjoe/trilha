# Spec 177 — Entrega rápida: gzip no runtime, poll que não troca o que não mudou, pool do upstream

**Feature Branch**: `177-entrega-rapida` | **Created**: 2026-10-18 | **Status**: Entregue (0.157.0)

- **Issues**: #305, #306, #307 — a fonte do escopo e dos critérios.
- **Versão**: 0.157.0

Forma curta: três ajustes no caminho entre o servidor e o navegador, cada um com teste
próprio; spec única porque saíram da mesma medição (o Verba migrando do Next.js) e
compartilham release.

## Por quê

O app que migra do Next.js perde três coisas que o Next fazia sem ninguém pedir: a resposta
comprimida (#305 — o kit e o SSR saíam crus, ~148 KB de CSS/JS que viram ~42 KB), o
re-render que não acontece quando o dado não mudou (#306 — o poll trocava o fragmento a cada
tique, remontando ilhas e perdendo rolagem) e a conexão reaproveitada com a API (#307 — o
`http.DefaultTransport` guarda 2 conexões ociosas por host).

## Jornada e risco

"Navegar e listar" e "interação rica". Pior impacto: SSE retido pelo buffer do gzip (o
assistente e o `ui.Live` param de chegar ao vivo); BREACH lendo o token CSRF numa página
comprimida; a ilha de um fragmento vivo morta depois da primeira troca (achado no caminho:
o `ui.live.js` disparava `trilha:hydrate` e o `ui.island.js` só ouvia `trilha:swap`).

## O que muda

- **#305** — `Config.Compression{Disabled, MinSize}`, ligado no valor zero. Wrapper no
  `serveHTTP` que decide no primeiro byte, no `Flush` ou no fim: comprime texto para quem
  manda `Accept-Encoding: gzip`, nunca `text/event-stream`, resposta já codificada, tipo já
  comprimido, `HEAD`, 1xx/204/206/304, `Range` ou corpo abaixo de 1 KiB. `Vary:
  Accept-Encoding` somado, `ETag` forte vira fraco. Estático versionado comprimido uma vez
  por versão e servido da memória (teto de 4 MiB por arquivo e 64 MiB no total).
  `c.CSRFToken()` passa a devolver o token mascarado (`m.` + pad ‖ segredo⊕pad), novo a
  cada resposta; a checagem desmascara os dois lados e aceita o valor cru.
- **#306** — `ui.live.js` compara a resposta com a anterior (sem nonce e sem token
  mascarado) e, no primeiro tique, o DOM da página com o fragmento (sem o que a ilha
  desenhou); igual, não troca. Depois de uma falha troca sempre. `ui.island.js` monta as
  ilhas também no `trilha:hydrate`.
- **#307** — `trilha.UpstreamTransport()`: clone do default com `MaxIdleConnsPerHost` 64,
  `MaxIdleConns` 256, `IdleConnTimeout` 90 s, compartilhado; é o transport do `Upstream` sem
  `Transport` e o recomendado para o cliente de API do app.

## Decisões

- **gzip ligado por padrão, com o token mascarado.** Desligado por padrão, todo app migrado
  seguiria sem compressão; ligado sem a máscara, a página com token e texto refletido ficaria
  exposta ao BREACH. A máscara é o que torna o padrão seguro.
- **Brotli fora.** Sairia da biblioteca padrão (princípio II).
- **Comparar texto e DOM "nu", não morphing.** Morphing resolveria também o "mudou pouco",
  mas é dependência ou muito código no kit; a igualdade cobre o caso dominante.
- **Orçamento do `ui.live.js` de 6 para 8 KiB.** O arquivo estava em 6085 bytes; a
  igualdade não cabia em 59. Com o gzip da #305 ele trafega com uns 2,8 KB.
- **`trilha:hydrate` no `ui.island.js`, não `trilha:swap` no `ui.live.js`.** O `trilha:swap`
  acordaria navegação, gravador e árvore para um fragmento que não é navegação.

## Fora de escopo

- Brotli e arquivos `.br`/`.gz` pré-comprimidos em disco — o cache em memória cobre o kit.
- Compressão de corpo de requisição.
- Morphing de DOM no poll.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `compress/gzip`, `net/http`; nada novo no `go.mod` |
| VI — teste primeiro | `compress_test.go` (SC-001…006), `TestUpstreamReusesConnectionsUnderLoad` (20 vs 56 conexões), `TestUIPollKeepsUnchangedFragment` reprovando com o `ui.live.js` e o `ui.island.js` anteriores |
| VII — segurança por padrão | token CSRF mascarado por resposta (BREACH); SSE nunca comprimido; nada de `Content-Encoding` sobre resposta já codificada |

## Segurança e privacidade

- Ativos e fronteiras de confiança afetados: o token CSRF no HTML (cliente ↔ servidor); a
  conexão app ↔ API (upstream).
- Controles OWASP ASVS 5.0 nível 2 aplicáveis: V3 (sessão: o segredo do cookie não muda,
  só a forma escrita no HTML), V4/V50 (CSRF: a checagem segue em tempo constante sobre o
  segredo desmascarado), V13 (configuração: compressão desligável).
- Riscos OWASP Top 10:2025 considerados: A02 (falha criptográfica — BREACH sobre resposta
  comprimida, mitigado pela máscara), A05 (configuração — `Disabled` para quem comprime no
  proxy), A04 (design inseguro — SSE e respostas já codificadas fora da compressão).
- Segredos, dados pessoais, logs, limites e exceções: segredo do app numa página (chave de
  API mostrada uma vez) não é conhecido pelo framework; a doc de segurança manda servi-lo em
  resposta própria ou com `no-store`. Cache de estáticos limitado a 4 MiB/arquivo e 64 MiB.

## Tarefas

- [x] T001 Testes da compressão (página, JSON, CSS; o que nunca comprime; SSE ao vivo; cache
  e 304 do estático; `Disabled`; token mascarado por resposta) — `compress_test.go`
- [x] T002 `compress.go`, `Config.Compression`, wrapper no `serveHTTP`, atalho no `static.go`
- [x] T003 Máscara do token (`csrf.go`), testes que afirmavam "token = cookie" passam a
  afirmar "token desmascara para o cookie"; snapshot reconhece o token mascarado
- [x] T004 Cenário `TestUIPollKeepsUnchangedFragment` + caso em `uitest/JORNADAS.md`,
  reprovando antes da correção
- [x] T005 `ui.live.js` (igualdade) e `ui.island.js` (`trilha:hydrate`)
- [x] T006 Teste de conexões do upstream + `UpstreamTransport`
- [x] T007 Docs en/pt (app, segurança, upstreams, vivo), `api/current.txt`, CHANGELOG,
  ROADMAP, versão

## Fechamento

- `make test`: 1608 testes passaram, 0 falharam, 2 pularam (`go test -json ./...`, Go 1.24).
- `make security`: gofmt, vet e `-race` verdes em go1.26.9 (41 pacotes); o `govulncheck`
  não rodou nesta máquina (`vuln.go.dev` bloqueado pela rede). Nenhuma dependência nova:
  `go.mod`/`go.sum` iguais aos da 0.156.0.
- `uitest` (Chromium): 45 cenários passaram, 0 falharam. `TestUIPollKeepsUnchangedFragment`
  reprova com o `ui.live.js` anterior (fragmento trocado) e com o `ui.island.js` anterior
  (ilha do fragmento trocado no fallback).
- Upstream: 3 rajadas de 20 requisições abrem 20 conexões com `UpstreamTransport`, 56 com
  `http.DefaultTransport`.
