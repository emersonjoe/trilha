# Spec 022 — Ilhas: o pedaço interativo de uma página estática

- **Issue**: [#22](https://github.com/emersonjoe/trilha/issues/22) (ROADMAP, Fase 1, item 3)
- **Branch**: `022-ilhas`
- **Versão**: 0.13.0

## Por quê

A 0.10.0 (spec 018) fechou o degrau em que o servidor tem a resposta: filtrar uma lista,
salvar um formulário, trocar um pedaço da página. Sobrou o degrau em que ele não tem — um
editor com prévia ao vivo, um canvas, um mapa que arrasta. Aí o estado é do cliente e não
existe ida e volta a fazer; hoje isso obriga a escrever `<script>` à mão, passar dado por
atributo na unha e brigar com a CSP sem `unsafe-inline`. É o último buraco da Fase 1, e é o
que empurra projeto para SPA.

## O que muda

```go
func (c *Ctx) Island(src string, props any, children ...h.Node) h.Node
```

```html
<div data-trilha-island="/editor.js?v=9c1f" data-trilha-props="{&quot;ppm&quot;:200}">…</div>
```

- `src` é um módulo em `public/`, endereçado pelo `Asset` (hash do conteúdo na URL); a
  **exportação padrão** é a montagem, chamada uma vez por elemento com `(el, props)`.
- `props` é o que o `encoding/json` serializa, ou `nil`. Vai escapado num atributo e volta
  pelo `JSON.parse`: é **dado**, nunca marcação. O que não serializa avisa uma vez
  (`warnOnce`, como o `Asset`) e mantém só o conteúdo de origem.
- Os filhos são o conteúdo de origem, renderizados pelo servidor: script bloqueado, a
  caminho ou 404, a página é a de sempre.
- O carregador é **um** script inline por resposta, com o nonce da requisição — é o que faz
  a CSP padrão aceitá-lo sem `unsafe-inline`. Ele monta cada ilha uma vez só e reescaneia no
  `trilha:swap`, para que ilha que chega dentro de um fragmento não nasça morta.

## Fora de escopo

- **Runtime de ilha em arquivo próprio** (`/_trilha/island.js` ou dentro do `ui.js`). O
  primeiro cria rota nova no framework; o segundo amarra ilha ao kit `ui`, que é opcional. O
  carregador inline com nonce não precisa de nenhum dos dois.
- **`<script type="application/json">` para as props.** Bloco de dado inline levanta questão
  de CSP em navegador que trata todo `<script>` como script; atributo escapado pelo `h` não
  levanta nenhuma.
- **`MutationObserver`** para montar ilha inserida por qualquer caminho: o único caminho que
  o repositório tem é a troca de fragmento, e ela já avisa por evento.
- **Empacotador, hidratação global, ilha em `route.go`.** A ilha é um nó de página.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| I — SSR primeiro, aprimoramento progressivo | o conteúdo de origem é o caminho normal; o módulo só acrescenta |
| II — só biblioteca padrão | `encoding/json` e o `h`; nenhum arquivo novo embutido, nenhuma rota nova |
| IV — convenção nova tem teste, uso no exemplo e integração | `examples/blog` ganha a ilha do editor em `/blog/novo` e `TestIlhaDoEditorDegradaSemScript` |
| VI — teste primeiro | os quatro testes do `island_test.go` foram escritos antes e falharam por símbolo inexistente |
| VII — compatibilidade | API nova; nada existente muda de assinatura ou de saída |

## Tarefas

- [x] T001 `island_test.go` vermelho: marcação, um carregador por resposta, props que não
      escapam, props inválidas mantendo o conteúdo de origem
- [x] T002 `island.go` (`Ctx.Island`, carregador com nonce) e o campo por requisição no `Ctx`
- [x] T003 `examples/blog`: `public/ilha-editor.js` e a ilha em `/blog/novo`, com teste
- [x] T004 Documentação nas duas locales: capítulo de interatividade e referência do `Ctx`
- [x] T005 `CHANGELOG`, `version`, `ROADMAP`, `make release VERSION=0.13.0`

## Aceitação

- **SC-001** A página com ilha traz o módulo com hash, as props escapadas e o conteúdo de
  origem; sem JavaScript, o formulário do exemplo continua publicando.
- **SC-002** Duas ilhas na mesma resposta produzem **um** carregador.
- **SC-003** Props com `</script>`, aspas e `<img onerror=…>` voltam do `JSON.parse` como a
  mesma string, e a resposta não ganha nenhuma tag a mais.
- **SC-004** Props que o `encoding/json` recusa não derrubam a página: o conteúdo de origem
  fica, e o log avisa uma vez por ilha.
