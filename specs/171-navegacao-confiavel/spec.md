# Spec 171 — Navegação no cliente confiável

**Feature Branch**: `171-navegacao-confiavel` | **Created**: 2026-10-06 | **Status**: Entregue (0.152.0)

- **Issues**: #290 (scripts da região trocada), #294 (segundo clique), #297 (anunciador de
  rota) — as issues são a fonte do escopo e dos critérios de aceitação.
- **Versão**: 0.152.0

Forma curta: um pacote (`ui`, os dois arquivos de troca do kit e uma função de atributo), sem
convenção nova em `app/` e sem quebra de API.

## Por quê

Ligar `ui.Navigate` na moldura (o que o Acervo precisa para parar de "recarregar tudo") hoje
quebra três coisas que a navegação completa dava de graça: a página de destino chega morta
(ilha, `ui.Defer`, `ui.Poll`, script da página — #290); dois cliques seguidos num paginador
deixam a barra numa página e o conteúdo noutra (#294); e o leitor de tela não fica sabendo que
a página mudou (#297), o que um órgão público sujeito ao eMAG e à LBI não pode aceitar.

## Jornada e risco

"Navegar e listar" de quem usa o app: abrir um item da lista, paginar, voltar. Pior impacto: a
tela mostra outra coisa que a barra (decisão tomada sobre o dado errado), um componente morto
até o F5, ou uma pessoa cega sem saber onde está.

## O que muda

- **Scripts que a troca traz rodam** (#290), nos dois caminhos (`ui.Swap` e `ui.Navigate`):
  cada `<script src>` da mesma origem, uma vez por URL por documento, recriado com `type`,
  `async`, `nonce`, `integrity`, `crossorigin`, `referrerpolicy`. Inline nunca roda (e avisa
  no console); outra origem também não. Exposto como `window.ui.activate(root)`, que substitui o
  caso especial do runtime de ilha (#82).
- **`trilha:before-swap`** no `document`, com `detail: {target, id, url}`, com o elemento antigo
  ainda conectado, nos dois caminhos (`window.ui.beforeSwap` para quem troca à mão).
- **Um pedido por alvo, com intenção** (#294): leitura nova aborta leitura velha; escrita nunca
  é abortada nem sai duas vezes; leitura que chega com escrita no ar é descartada. `ask` e o
  `fetchInto` do `ui.nav.js` resolvem `"swapped" | "skipped" | "navigate"` e só `"swapped"`
  mexe no histórico — o que também tira a entrada extra que o clique abortado do `ui.nav.js`
  empurrava. Marcas de espera por pedido: só o mais novo de um alvo as tira.
- **Anunciador de rota e foco no título** (#297): depois de uma navegação no cliente (e no
  Voltar/Avançar), o foco vai ao primeiro `h1` da região e `#trilha-route-announcer`
  (`.ui-sr`, `aria-live="assertive"`, `aria-atomic`, criado no cliente) recebe o `<title>` novo,
  ou o `h1` quando o título não mudou; calado quando o `h1` focado já diz o mesmo.
  `ui.NavigateFocus("h1" | "region" | "none")` escolhe outro destino.

```go
h.Main(h.ID("conteudo"), ui.Navigate(""), ui.NavigateFocus("region"), children)
```

**Mudança de comportamento**: o foco depois de `ui.Navigate` passa da região ao `h1`. Quem
dependia do anterior marca a região com `ui.NavigateFocus("region")`.

## Fora de escopo

- Mesclar o `<head>` por página: o kit não tem head por página (#290, "fora desta issue").
- Seguir `Trilha-Location`, form dentro da região e URL declarada pelo servidor: #291–#293, na
  spec seguinte, que usa o resultado de três estados criado aqui.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | nada novo em `go.mod`; JavaScript do kit, sem dependência |
| IV — API pública | `ui.NavigateFocus` aditivo; `api/current.txt` e catálogo do `uidoc` atualizados |
| VI — teste primeiro | os seis cenários novos reprovaram no Chromium contra o kit antigo, cada um pelo defeito da issue ("page 2" depois do clique em 3; ilha sem `data-trilha-mounted`; foco em `section#regiao`; nenhum `before-swap`) |
| VII — segurança por padrão | script inline vindo de resposta nunca executa (cenário de regressão) |

## Segurança e privacidade

- **Fronteira**: HTML de resposta do próprio servidor entrando no documento. Só arquivos da
  mesma origem rodam, que é o que a CSP `script-src 'self'` já permitia numa carga completa;
  nada novo a liberar. Inline continua inerte (ASVS V3.2.2, Top 10 A05:2025 Injection).
- **Integridade**: `integrity` e `crossorigin` copiados para o script recriado.
- **Dados**: nenhum novo; o anunciador lê o `<title>` e o `h1`, texto da própria página.
- **Exceções**: nenhuma.

## Tarefas

- [x] T001 Cenários que reprovam: `TestUIClientNavRunsRegionScripts`, `TestUIBeforeSwapEvent`,
  `TestUIClientNavAnnounces`, `TestUIClientNavFocusRegion`, `TestUISwapNewestClickWins`,
  `TestUISwapPostOnce`; `TestUIClientNavFocus` passa a exigir o `h1`; fixture `/fluxos/nav/*`
  e `/fluxos/lenta`.
- [x] T002 `ui.js` (`activate`, `beforeSwap`, `ask` de três estados, marcas por pedido),
  `ui.nav.js` (rotina comum, anunciador, foco), `ui.NavigateFocus`.
- [x] T003 `examples/blog`: cópias do kit e goldens (o hash do `ui.js` mudou).
- [x] T004 Guia de interatividade e referência nas duas locales; catálogo `uitest/JORNADAS.md`.
- [x] T005 Orçamentos: `ui.js` 32 KB e `ui.nav.js` 6 KB, com o motivo no teste.
- [x] T006 CHANGELOG, ROADMAP, versão; suíte inteira com números.

## Aceitação

- **SC-001** — Navegar no cliente para uma página com ilha, `Defer` e arquivo de script deixa
  tudo funcionando sem F5; o mesmo arquivo não roda duas vezes; o inline não roda.
- **SC-002** — Clicar "2" e depois "3" mostra a 3, com a 3 na barra e uma entrada no histórico;
  dois cliques em salvar gravam uma vez.
- **SC-003** — Depois de navegar, o leitor de tela ouve o título novo e o foco está no título
  da página; Voltar anuncia também.
- **SC-004** — Tudo isso em Chromium, Firefox e WebKit.

## Evidências

| Nível | Comando | Passaram | Falharam | Pularam |
|---|---|---|---|---|
| Unidade + integração | `make test` | 1587 | 0 | 2 |
| Navegador, 19 cenários × 3 motores + 5 do módulo | `UITEST_REQUIRED=1 UITEST_BROWSERS=all make test-ui` | 24 | 0 | 0 |

Pulos: `TestS3AoVivo` e `TestOIDCAoVivo` (serviço real, por desenho). Sem teste automatizado: a
escolha entre `assertive` e `polite` com NVDA e VoiceOver de verdade — o cenário prova o
atributo e o texto, não o que cada leitor fala; fica como verificação manual na #297.
