# Spec 060 — Uma ilha, um runtime

- **Issue**: sem issue própria; sai do risco registrado na
  [#70](https://github.com/emersonjoe/trilha/issues/70#issuecomment-5588255671) e do pedido do
  mantenedor de fechá-lo antes que cobrasse juros.
- **Branch**: `060-uma-ilha-um-runtime`
- **Versão**: 0.42.0

## Por quê

O canal ilha→servidor da 0.41.0 nasceu duplicado: uma implementação completa no script inline
do `island.go`, para a página sem o kit, e outra no `ui.js`, para a ilha que chega dentro de um
fragmento trocado. As duas fazem a mesma coisa — token do elemento, JSON de ida e volta, 422
com os campos — e a razão de existirem duas é que nenhuma das duas alcança os dois casos.

Elas **já tinham divergido no nascimento**. O `swap` do `ui.js` passa pelo `swap` do kit, que
restaura o foco, devolve o cursor ao campo em uso, hidrata o que chegou e faz a transição. O
`swap` do loader inline faz `outerHTML` na mão e não faz nada disso. Quem escreve a ilha não
tem como saber qual das duas montou a dele, então o comportamento dependia de a página ter ou
não o kit — e isso não está escrito em lugar nenhum, porque não era para ser verdade.

Havia até um teste, `TestIslandChannelIsTheSameOnBothSides`, cujo trabalho era comparar as duas
cópias texto por texto. Um teste assim é um sintoma: existe para segurar o que a estrutura
deveria tornar impossível, e não segurou.

## O que muda

**O runtime de ilha vira um arquivo do kit**, `public/ui.island.js`, e o `Ctx.Island` o liga
por `<script src>` em vez de embutir script inline:

```html
<div data-trilha-island="/editor.js?v=…" data-trilha-csrf="…">…</div>
<script data-trilha-islands src="/ui.island.js?v=…" defer></script>
```

Isso resolve os dois casos com uma implementação só:

- **Página sem o kit** — o `<script src>` é o próprio runtime; nada mais é preciso.
- **Ilha que chega num fragmento** — um `<script>` escrito por `outerHTML` não roda, então o
  `ui.js` recria *aquela tag* (pela marca `data-trilha-islands`), e só enquanto o runtime não
  estiver carregado. Depois disso ele ouve o `trilha:swap` e monta sozinho.

O `island.swap` passa a usar o `window.ui.swap` quando o kit está na página e a fazer a
substituição direta quando não está — um caminho, uma decisão explícita, em vez de dois
comportamentos que ninguém escolheu.

De quebra: **não há mais script inline de ilha**, então `script-src 'self'` basta e o nonce
deixa de ser necessário para essa parte; e o runtime é cacheado com hash, em vez de viajar em
toda resposta que tenha ilha.

`trilha.IslandRuntime` é a constante com o nome do arquivo. `trilha audit` (e portanto o
`check`) reprova com crítico quando o projeto usa `c.Island` e o arquivo não está em `public/`
— a falha é silenciosa de outro jeito: o recuo aparece e nada acontece.

## Fora de escopo

- **Servir o runtime a partir do framework**, sem cópia em `public/` — mudaria o modelo do kit,
  que é "copiado para o projeto e editável", por causa de um arquivo só.
- **Fazer o mesmo com `ui.nav.js` e `ui.upload.js`** — eles já são arquivos; o problema era
  só da ilha.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | Nenhuma dependência; o runtime é JavaScript do kit. |
| IV — superfície pequena | Um símbolo novo (`IslandRuntime`), nenhum removido. |
| VI — teste primeiro | O teste que comparava as duas cópias vira o teste que prova que só existe uma; o do canal passa a ler o arquivo. |
| VII — segurança por padrão | Um script inline a menos na página; a CSP padrão continua sem `unsafe-inline`, agora com uma exigência a menos. |

## Migração

Projeto que já usa ilha precisa de `trilha ui` uma vez, para escrever o `public/ui.island.js`.
Quem esquecer ouve do `trilha check`, com a linha do conserto.

## Tarefas

- [x] T001 `ui/assets/ui.island.js`: a implementação única, legível.
- [x] T002 `island.go`: `<script src>` no lugar do inline; `IslandRuntime`.
- [x] T003 `ui.js`: remover a cópia (−4,1 KB) e recriar a tag que chega no fragmento.
- [x] T004 Testes: o do canal lê o arquivo; o das duas cópias vira o de uma só.
- [x] T005 `trilha audit`: crítico quando usa ilha sem o arquivo, nas duas línguas.
- [x] T006 Documentação nas duas locales e `examples/blog` com o arquivo novo.
- [ ] T007 `CHANGELOG.md`, `version`, `make test` verde e release.

## Aceitação

- **SC-001** Uma página com ilha não tem script inline de ilha, e liga o runtime uma vez.
- **SC-002** A ilha monta na página com o kit, na página sem o kit, e quando chega num
  fragmento numa página que não tinha nenhuma.
- **SC-003** `ui.js` não contém mais nada do canal, e um teste falha se voltar.
- **SC-004** `trilha check` reprova o projeto que usa ilha sem `public/ui.island.js`.
