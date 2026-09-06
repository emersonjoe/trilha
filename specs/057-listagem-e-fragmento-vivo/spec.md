# Spec 057 — A lista e o fragmento que se mexe sozinho

Issues: [#63](https://github.com/emersonjoe/trilha/issues/63) (`trilha.ListParams` +
`ui.DataTable`) e [#64](https://github.com/emersonjoe/trilha/issues/64) (`ui.Poll`,
`ui.Live`, `ui.On`). A issue é a fonte do escopo; aqui fica só a decisão.

## Por que as duas juntas

As duas caem na mesma emenda: o fragmento. A `#63` é a tabela que se redesenha quando
alguém ordena, filtra ou vira a página; a `#64` é o pedaço que se redesenha quando o
relógio bate ou o servidor avisa. Nos dois casos o servidor é o mesmo handler, a
resposta é o mesmo `c.Fragment()`, e o que muda é quem pediu. Separar as duas seria
escrever duas vezes a mesma decisão sobre quem manda o `Trilha-Fragment` e o que
acontece quando o JavaScript não está lá.

## Decisões

1. **`ListParams` é embutida, e o `Bind` a completa.** O `bindStruct` já achata struct
   aninhada; o que falta é o depois — aplicar limite e default e guardar a query crua
   para o `Href` preservar o resto. É um caso especial de três linhas no `bindStruct`,
   explícito, no lugar de um segundo caminho de leitura.
2. **`Href` devolve só a query (`?page=2&q=nota`).** URL relativa com query resolve
   contra o caminho atual em qualquer browser: não há caminho para guardar, não há
   caminho para errar, e o link continua certo se a rota for montada em outro prefixo.
3. **O repositório nunca recebe coluna que não foi declarada.** `Restrict(cols…)` zera
   um `Sort` desconhecido e escreve um aviso no log do request. Ordenar por coluna
   inventada é uma URL torta, não um 500 — e `ui.DataTable` chama `Restrict` sozinho
   com as colunas marcadas `Sort: true`.
4. **`ui.DataTable` não traz JS novo.** O cabeçalho ordenável são links de verdade e o
   filtro é `<form method=get>`; quando o `ID` do estado está preenchido, os dois
   ganham `ui.Swap` e trocam só a tabela, com a navegação normal como fallback.
5. **A linha clicável é por posição (`RowHref func(int) string`)**, não por `T`. O
   `ListState` fica não-genérico como na issue; o índice é o que dá para carregar sem
   arrastar o parâmetro de tipo para dentro do estado e para a seleção em massa.
6. **`Poll`, `Live` e `On` são atributos, e o script é à parte.** `ui.LiveScript(c)`
   carrega `ui.live.js`, como `NavigateScript` carrega `ui.nav.js`: quem não usa não
   baixa. O `ui.Head` continua com o mesmo orçamento.
7. **O evento carrega o nome, nunca o HTML.** O cliente refaz o GET do fragmento, então
   autorização e render continuam na rota da página e a conexão SSE não vira canal de
   dados. `Stream.Notify(name)` é só o `Send(name, "")` com nome.
8. **O servidor manda no relógio.** `Trilha-Poll: stop` encerra, `Trilha-Poll: 30s`
   muda o intervalo, `Retry-After` é respeitado no erro. O cliente pausa em
   `document.hidden` e sobe o recuo até 60 s, voltando ao intervalo no primeiro 200.
9. **Nada de bus no framework.** A rota `/events` é do app (princípio II). O que entra
   é o protocolo do cliente e o `Notify` de conveniência.

## Critérios de aceitação

- SC-001 `Bind` lê `ListParams` embutida numa struct com filtros próprios, em GET.
- SC-002 `PerPage` acima de 200 cai para 200; `Page` < 1 vira 1; ausente vira default.
- SC-003 `Href("page","3")` preserva `q`, `status` e o que mais veio na query.
- SC-004 `Href("q","")` remove o parâmetro em vez de mandar vazio.
- SC-005 `Offset`, `Limit`, `Asc` e `TotalPages` batem com a página pedida.
- SC-006 `Restrict` zera `Sort` de coluna não declarada e avisa no log; não erra.
- SC-007 `ui.DataTable` marca `aria-sort` na coluna ordenada e só nela.
- SC-008 O link do cabeçalho inverte a direção da coluna atual e volta para a página 1.
- SC-009 Com `ID` preenchido, cabeçalho, filtro e paginação levam `data-trilha-target`.
- SC-010 Lista vazia mostra o `Empty` e nenhuma linha; `Total` 0 não renderiza paginação.
- SC-011 `RowHref` põe o link na linha; `Select` põe a coluna de caixas e a barra de ação.
- SC-012 `ui.Poll("6s", src)` e `ui.On(nome, src)` rendem os atributos esperados.
- SC-013 `c.PollEvery(d)` e `c.PollStop()` escrevem `Trilha-Poll` e o `Vary` continua.
- SC-014 `Stream.Notify(nome)` manda um evento nomeado sem dado.
- SC-015 `ui.live.js` está no embed, no `Files` e sai no `trilha ui`.
- SC-016 `trilha audit` avisa `ui.Live` em rota sem `Auth`.
- SC-017 `examples/blog` tem uma listagem com `DataTable` e um fragmento com `Poll`.
- SC-018 `make test` verde, `api/current.txt` regravado, docs nas duas línguas.
