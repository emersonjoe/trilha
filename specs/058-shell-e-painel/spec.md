# Spec 058 — O shell e o painel

Issues: [#65](https://github.com/emersonjoe/trilha/issues/65) (`ui.Shell` e
`trilha new --template app`) e [#68](https://github.com/emersonjoe/trilha/issues/68)
(`ui.Stat`, `ui.Bars`, `ui.Sparkline`, `ui.Donut`). A issue é a fonte do escopo; aqui
fica só a decisão.

## Por que as duas juntas

A `#65` pede o ponto de partida de um app de gestão e, no meio da lista do que ele
precisa, escreve `ui.Stat` para o dashboard — que é a primeira das quatro funções da
`#68`. A primeira tela que o template `app` gera é um painel: contadores e barras.
Fazer a `#65` sem a `#68` seria escrever o `Stat` duas vezes, uma delas para jogar
fora; fazer a `#68` sem a `#65` seria entregar quatro gráficos sem nenhuma tela que os
use. As duas caem no mesmo lugar: o que um app de gestão vê ao abrir.

## Decisões

1. **`ui.Shell` é composição, não peça nova.** `Sidebar`, `Nav`, `NavLink`, `Header`,
   `Brand` e `Menu` continuam existindo e exportados; o `Shell` os monta na ordem que
   todo mundo monta. Quem quer outro arranjo continua chamando as peças.
2. **O item ativo é o prefixo mais longo que casa com o caminho.** `/docs` e
   `/docs/novo` no mesmo menu: abrir `/docs/novo` marca o segundo, nunca os dois. O
   caminho vem de `c.Request().URL.Path`, e `NavLink(href, label, current)` continua
   recebendo o booleano pronto para quem calcula por conta própria.
3. **`When: false` some da renderização, e a documentação repete que menu é cosmético.**
   Esconder o item não protege a rota; quem protege é o `middleware.go`. O template
   gerado põe os dois no mesmo arquivo para que a frase tenha onde ser lida.
4. **O recolher não traz arquivo novo.** A classe entra no `<html>` antes do primeiro
   paint pelo mesmo script inline do tema, e o clique são poucas linhas no `ui.js` que
   já é baixado. O drawer do mobile é o `<dialog>` nativo.
5. **Gráfico é SVG gerado no servidor, sem JS e sem eixo.** `role="img"` com `<title>`
   e `<desc>`, `viewBox` para escalar, cor por `var(--…)` do tema, e uma tabela
   escondida em `.ui-sr` ao lado — que é a acessibilidade e o que sai na impressão.
   Interatividade nenhuma: o tooltip é o `<title>` nativo do SVG.
6. **O número chega formatado.** O Trilha não tem locale; `ui.Stat` recebe a string
   pronta e `ui.Datum` recebe `float64` só para calcular proporção.
7. **Série degenerada não quebra.** Vazia, total zero, um ponto só, valor negativo:
   cada um tem um desenho definido (nada, barras zeradas, uma linha reta, barra
   ignorada com o valor ainda no rótulo) e um teste. Nenhum caso entra em `NaN` no
   atributo.
8. **O golden é o SVG inteiro.** Sem `rand`, sem `time`, sem mapa iterado: a mesma
   série desenha o mesmo arquivo, e o diff do golden é a revisão do desenho.
9. **`trilha new --template app`, com o `blog` continuando padrão.** Os templates viram
   três pastas: `base` (o que todo projeto tem), `blog` e `app`. Nome de template
   desconhecido é erro com a lista, não um projeto pela metade.
10. **O template `app` sai verde no `trilha check`, sem edição.** Login com
    `auth.Sessions`, `middleware.go` com as exceções públicas, `error.go` por
    `trilha.StatusOf`, `setup.go` com as dependências, painel com `Stat`/`Bars`, uma
    entidade com `ui.DataTable` + formulário + exclusão confirmada, e um teste por rota.
11. **A régua do #45 fica fora.** Rodar os quatro cenários sobre o template novo custa
    hora de máquina e chamada paga; a linha de base já está velha desde a v0.35.0 e a
    decisão de gastar é do dono do repositório, não desta spec.

## Critérios de aceitação

- SC-001 `ui.Shell` marca `aria-current="page"` no item de prefixo mais longo e só nele.
- SC-002 Grupo ou item com `When: false` não aparece no HTML.
- SC-003 O botão de recolher tem `aria-expanded` e o `<html>` recebe a classe antes do paint.
- SC-004 `ui.PageHeader` renderiza título, o `Back{Href,Label}` e as ações à direita.
- SC-005 `ui.UserMenu` vira `MenuTrigger` + `Menu` com os itens passados.
- SC-006 `ui.Stat(label, value, hints…)` renderiza rótulo, valor e dica, sem SVG.
- SC-007 `ui.Bars` desenha uma barra por `Datum`, proporcional ao maior valor.
- SC-008 `ui.Sparkline` desenha um `polyline` com os pontos na ordem dada.
- SC-009 `ui.Donut` desenha um arco por fatia e a legenda em `<ul>` fora do SVG.
- SC-010 Cada gráfico tem `role="img"`, `<title>` e a tabela `.ui-sr` com os números.
- SC-011 Série vazia, total zero, um ponto e valor negativo não geram `NaN` nem panic.
- SC-012 Golden do SVG das quatro funções, estável entre execuções.
- SC-013 Nenhuma cor literal: só `var(--…)` declarada no `ui.theme.css`.
- SC-014 `trilha new --template app` escreve o app de gestão; `--template blog` e a
  ausência da flag escrevem o de hoje; nome desconhecido é erro com a lista.
- SC-015 e2e: o projeto do template `app` compila, `trilha check` passa e
  `go test ./...` passa sem nenhuma edição.
- SC-016 `--lang pt` traduz os textos do template `app` no mesmo commit.
- SC-017 `examples/orcamento` mostra orçado × realizado com `ui.Bars`.
- SC-018 `make test` verde, `api/current.txt` regravado, docs nas duas línguas.
