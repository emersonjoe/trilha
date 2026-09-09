# Spec 073 — A árvore, e o campo que escolhe um nó dela

- **Issue**: #101 — a issue é a fonte do escopo; aponte para ela, não a reescreva aqui.
- **Branch**: `073-tree`
- **Versão**: 0.55.0

## Por quê

Hierarquia com milhares de nós é o componente que as pessoas vão buscar no npm. Expandir, buscar
e teclado são cada um fácil e juntos são trezentas linhas de React — e é exatamente o tipo de
coisa que o iniciante não faz sozinho. O kit tinha `ui.Combobox` para lista plana e nada para
hierarquia.

## O que muda

`ui.Tree`, `ui.TreeItems`/`ui.TreeNodes`, `ui.TreePicker` e o `ui.tree.js`, que é opcional como o
`ui.nav.js`. O contrato está na referência do `ui`, nas duas línguas. O que vale registrar:

**Um nó é `<details>`, e é essa a história inteira do sem-JavaScript.** O script não desenha
nada: ele busca os filhos na primeira abertura, no lugar de pedir uma página inteira. Nó cujos
filhos já vieram do servidor não pede nada — é assim que o caminho até o nó atual chega aberto e
completo no primeiro desenho, inclusive depois de um 422 trazer o formulário de volta.

**O seletor posta um radio.** Nada de `<input hidden>` para manter em sincronia, nada de texto
para resolver no servidor: quem não tem script navega pelos mesmos `<details>` e marca o mesmo
radio, e o formulário manda o mesmo campo. Foi a decisão que fez o resto encolher.

**Um papel por elemento.** A primeira versão emitia `role="tree"` e `role="group"` no mesmo
`div` — dois atributos `role`, que não é uma promessa mais forte e sim uma inválida. Árvore de
radios é campo e se anuncia como grupo de escolhas; árvore de links é navegação. A escolha
acontece uma vez.

**`aria-label` vazio não sai.** Nomear o campo com nada é pior que não nomear: esconde o que o
`<label>` em volta diria.

**O que chega ao servidor não veio da árvore, veio de uma requisição.** O exemplo confere o
código contra a hierarquia com uma regra própria, e o aviso está na referência.

## O que isto consertou de caminho

O `AddRule` explodia com "already registered" quando o `Setup` rodava duas vezes — e a doc dele
manda registrar regra **no `Setup`**, que é exatamente o que uma suíte de testes faz uma vez por
teste. É o mesmo tropeço que o `RegisterEnum` teve na spec 064, e agora tem a mesma saída: a
mesma função registrando de novo é aceita (comparação por ponteiro), e duas funções diferentes
sob um nome continuam sendo pânico — esse é o bug que a guarda existe para pegar.

## Fora de escopo

- **Expandir tudo sem script.** O `*` é do teclado, e teclado é script. Sem JavaScript, quem
  quiser tudo aberto abre; a alternativa seria um link por nó recarregando a página.
- **Rolagem virtual.** Milhares de nós numa lista plana precisariam disso; numa árvore que abre
  um ramo por vez, não — e o custo seria um segundo caminho de renderização em JavaScript, que é
  o oposto do que este componente faz.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | Nada novo; `<details>`, radios e um fetch. |
| III — rota no exemplo | `/setores/nos` e `/setores/busca` no `cadastro`, com o seletor no formulário e e2e. |
| IV — superfície pequena | Duas funções de desenho, duas de fonte, um script opcional. |
| V — inglês no código, pt-BR junto | Referência do `ui` nas duas línguas no mesmo commit. |
| VI — teste primeiro | Render (papéis, folha × ramo, caminho aberto, radio, aria vazio, caminho da busca), e2e (árvore sem netos, fonte, busca com ancestralidade, código inventado recusado, escolha marcada no 422) e o navegador para o que teste nenhum prova: filhos na primeira abertura, neto na segunda, busca e volta, setas, `*`, e o valor no formulário. |
| VII — segurança por padrão | O aviso de conferir o valor contra a hierarquia, e o exemplo fazendo isso com uma regra de validação. |

## Tarefas

1. `ui/tree.go`: `Tree`, `TreeNode`, `TreeItems`, `TreeNodes`, `TreePicker`, `TreeScript`. ✅
2. `ui/assets/ui.tree.js` + CSS (teto do `ui.css` para 34 KB, com o motivo no teste). ✅
3. `examples/cadastro`: setores, as duas rotas-fonte, o campo no formulário e a regra. ✅
4. `AddRule` idempotente para a mesma função. ✅
5. Referência do `ui` en e pt; `api/current.txt`, catálogo, cópias do kit. ✅
