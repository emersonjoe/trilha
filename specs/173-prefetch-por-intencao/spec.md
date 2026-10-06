# Spec 173 — Prefetch por intenção

**Feature Branch**: `173-prefetch-por-intencao` | **Created**: 2026-10-06 | **Status**: Entregue (0.154.0)

- **Issue**: #295 — a fonte do escopo e dos critérios.
- **Versão**: 0.154.0

Forma curta: um comportamento opt-in do `ui.nav.js`, três atributos do kit e um método de `Ctx`.

## Por quê

Com a navegação no cliente, a troca ainda espera a ida e volta inteira à vista: a 150–300 ms o
limiar de 120 ms passa e a região esmaece em quase toda navegação. O app que vem do Next.js
(o Acervo) compara com o `<Link>` que antecipa por padrão. O intervalo entre apontar e clicar é
quase a ida e volta inteira.

## Jornada e risco

"Navegar e listar". Riscos do lado oposto: tráfego e carga por movimento de mouse; um GET com
efeito colateral (trilha de acesso) disparado sem clique; conteúdo velho no clique.

## O que muda

- `ui.Prefetch()` (região ou link), `ui.NoPrefetch()` (link), `ui.PrefetchTTL(ms)`.
- Gatilho: ponteiro parado 80 ms (cancelado ao sair ou ao apertar), foco **de teclado**
  (`:focus-visible`), toque. Nunca por viewport, `NoNavigate`, download, outra origem, `target`,
  `saveData` ou `2g`.
- Cache em memória por endereço (sem `#`), TTL padrão 10 s, oito entradas; o clique consome a
  entrada, em voo ou pronta; redirect ou não-200 não é guardado.
- `trilha:swap` ganha `detail.prefetched`.
- `c.IsPrefetch()`: `Sec-Purpose` contendo `prefetch` (o do navegador) ou `Purpose: prefetch`.

## Decisões

1. **`Purpose`, não `Sec-Purpose`, no pedido do kit.** Fato verificado: cabeçalhos com prefixo
   `Sec-` são *forbidden request headers* do Fetch; o `fetch` não os envia. O kit manda o antigo
   `Purpose: prefetch`, e `IsPrefetch` aceita os dois — inclusive a especulação do próprio
   navegador.
2. **O foco do clique não é intenção.** O clique do mouse dá foco ao link antes do `click`; sem
   `:focus-visible`, isso disparava um prefetch que o próprio clique consumia, e o GET do clique
   ia marcado como prefetch (achado pelo cenário de TTL).
3. **O clique consome sem pedir de novo, então `IsPrefetch` não basta para trilha de acesso.** A
   issue propunha `IsPrefetch` para a trilha de acesso da LGPD; mas o servidor nunca fica sabendo
   do clique que usou a resposta antecipada. A documentação (`IsPrefetch`, `Audit`, guia) diz o
   que vale: `IsPrefetch` para efeito que pode ser pulado (contador), `ui.NoPrefetch` nos links
   de uma leitura que precisa ser registrada sempre. É a mesma regra do Turbo.
4. **Fora**: a contagem de prefetches no `trilha dev` (a issue diz "pode") e o retrato do Voltar.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | nada novo |
| IV — API pública | `ui.Prefetch`, `ui.NoPrefetch`, `ui.PrefetchTTL`, `Ctx.IsPrefetch`; aditivo |
| VI — teste primeiro | `TestIsPrefetch` não compilava; os cenários de antecipação reprovaram — depois que o `WaitJS` passou a aguardar `Promise` (antes, um `fetch` na condição era "verdadeiro" e o cenário passava sem provar nada) |
| VII — segurança por padrão | opt-in, só mesma origem, sem viewport; nada antecipado em economia de dados |

## Segurança e privacidade

- **Fronteira**: GET de mesma origem com `credentials: "same-origin"`, o mesmo que o clique faria.
- **Privacidade**: uma leitura antecipada não é uma leitura (decisão 3); documentado onde o dev
  escreve a trilha (`Audit`).
- **Disponibilidade**: TTL curto, oito entradas, só intenção (ASVS V2.4 anti-automação não se
  aplica; o custo é medível por `detail.prefetched` e por `IsPrefetch` no log da app).

## Tarefas

- [x] T001 `TestIsPrefetch`; cinco cenários (`TestUIPrefetch*`); `Session.Hover`, `Session.Sweep`
  e `WaitJS` que aguarda `Promise` no `uitest`.
- [x] T002 `Ctx.IsPrefetch`; `ui.Prefetch`/`NoPrefetch`/`PrefetchTTL`; prefetch no `ui.nav.js`.
- [x] T003 Catálogo, guia de interatividade e referências nas duas locales; `Audit` e
  `IsPrefetch` documentam a decisão 3; `api/current.txt`, `uidoc`, cópia do kit no `blog`.
- [x] T004 `ui.nav.js` 12 KB, com o motivo no teste.
- [x] T005 CHANGELOG, ROADMAP, versão; suíte inteira com números.

## Aceitação

- **SC-001** Apontar para um link antecipa uma vez; o clique não pede de novo.
- **SC-002** Passar por cima, `NoPrefetch` e economia de dados não pedem nada.
- **SC-003** Depois do TTL o clique pede; prefetch redirecionado não é guardado.
- **SC-004** Nos três motores.

## Evidências

| Nível | Comando | Passaram | Falharam | Pularam |
|---|---|---|---|---|
| Unidade + integração | `make test` | 1591 | 0 | 2 |
| Navegador, 32 cenários × 3 motores + 5 do módulo | `UITEST_REQUIRED=1 UITEST_BROWSERS=all make test-ui` | 37 | 0 | 0 |

Pulos: `TestS3AoVivo`, `TestOIDCAoVivo` (serviço real). Sem teste automatizado: toque
(`touchstart`) — os três motores headless dirigidos aqui não emulam toque sem um contexto
`hasTouch`; o caminho é o mesmo `prefetch(a)` do foco e do ponteiro.
