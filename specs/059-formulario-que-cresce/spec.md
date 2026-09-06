# Spec 059 — O formulário que cresce

Issue: [#69](https://github.com/emersonjoe/trilha/issues/69) (`Bind` de listas de structs
e mapas, e `ui.SchemaForm` a partir de esquema em dados). A issue é a fonte do escopo;
aqui fica só a decisão.

## Por que sozinha

A `#69` já é duas metades que se seguram: o `Bind` que entende `itens[0].nome` e o
formulário desenhado a partir de dados, que só existe porque a validação passou a ser uma
coisa só. A `#67` (combobox e dropzone) é a vizinha mais próxima, e fica para a próxima —
um combobox é mais um tipo de campo do `SchemaForm`, e é melhor que o motor exista antes
do widget.

## Decisões

1. **O nome é a chave do erro.** `itens[1].qtd` é o `name` do input, o índice do
   `FieldErrors` e o que o `ui.Errors` procura. Não há tradução no meio: quem lê o HTML
   sabe onde a mensagem vai cair, e o `ui.InvalidIf` continua funcionando sem saber que
   existe lista.
2. **Índice esparso é compactado, na ordem numérica.** O formulário pode mandar `[0]`,
   `[3]` e `[7]` — remover linha no cliente não renumera nada — e o Go recebe uma fatia
   de três posições. A chave do erro é a **posição depois de compactar**, porque é essa a
   linha que o servidor vai redesenhar.
3. **O índice não aloca.** O limite vem antes da alocação: `maxitems` na tag quando existe,
   e um teto do framework (`MaxItems = 1000`) quando não existe. `itens[999999999]` é uma
   chave que não bate com nada, não um `make` de um bilhão.
4. **A chave do mapa é literal.** `perm[docs]` dá a chave `docs`, com `]` proibido e o
   resto — ponto, espaço, acento — aceito como veio. Mapa é `map[string]T` com o mesmo `T`
   que um campo comum aceita; qualquer outra chave é erro de programação, e o `Bind`
   diz isso em vez de ignorar em silêncio.
5. **`BindJSON` fala a mesma língua.** Um `items[1].qty` do formulário e um
   `items[1].qty` do JSON são a mesma chave de erro, então a mesma tela trata os dois
   sem um mapa de tradução.
6. **`BindSchema` não inventa um segundo validador.** O esquema em dados vira as mesmas
   regras da spec 027 — `required`, `min`, `max`, `oneof`, `pattern` — e passa pelo mesmo
   motor. Um esquema que pede uma regra que não existe é erro na hora de ler o esquema,
   não na hora de validar o valor de alguém.
7. **O valor do esquema é texto.** `BindSchema` devolve `map[string]string`: o esquema
   veio do banco, então não há tipo Go para preencher, e converter para `any` só
   empurraria a conversão para o app. `number` e `date` são validados como texto e
   entregues como texto.
8. **`display` não é campo.** Aparece na tela, não entra no `Bind`, não recebe erro. É a
   instrução no meio do formulário, e tratá-la como campo faria um `required` de um
   parágrafo.
9. **`SchemaForm` desenha, não decide.** Recebe esquema, valores e erros e devolve os
   campos; o `<form>`, o botão e o CSRF são do app, como em todo formulário do kit.
10. **`file` e `signature` são o campo, não o comportamento.** `file` sai como
    `<input type=file>` e continua sendo lido por `c.File`; `signature` sai como campo de
    texto. O desenho da assinatura é do app.
11. **Nada de `ui.FieldList`.** Nomear a linha é um `fmt.Sprintf`, e o botão de adicionar
    é uma decisão do app (de onde vêm os valores da linha nova). Um componente que só
    concatena string não paga o próprio nome.

## Critérios de aceitação

- SC-001 `Bind` preenche `[]Item` a partir de `itens[0].nome`, `itens[0].qtd`, na ordem.
- SC-002 Índice esparso é compactado; a chave do erro usa a posição depois de compactar.
- SC-003 `maxitems` na tag corta antes de alocar; sem tag, o teto é `MaxItems`.
- SC-004 `Bind` preenche `map[string]int` a partir de `perm[docs]=2`.
- SC-005 Chave de mapa com `]` é recusada; chave com ponto e espaço passa inteira.
- SC-006 `FieldErrors` de um item traz `itens[1].qtd`, e `ui.Errors` acha o campo.
- SC-007 `required` dentro do item dispara para a linha que não foi preenchida.
- SC-008 `BindJSON` de `{"items":[…]}` erra com a mesma chave `items[1].qty`.
- SC-009 O fuzz do `Bind` cobre `itens[-1]`, `itens[9999999999]`, `itens[0][0]`, `perm[`
  e chave vazia: nunca pânico, nunca alocação proporcional ao índice.
- SC-010 `trilha.BindSchema` devolve valores e `FieldErrors` pelo motor da spec 027.
- SC-011 Esquema com regra desconhecida é erro do esquema, não 422 do usuário.
- SC-012 `display` não aparece nos valores nem nos erros.
- SC-013 `ui.SchemaForm` desenha `text`, `textarea`, `number`, `date`, `datetime`,
  `select`, `checkbox`, `file`, `signature` e `display`, com rótulo, ajuda e erro.
- SC-014 `select` do esquema respeita `Options` e valida com `oneof`.
- SC-015 `trilha openapi` descreve `[]Item` no corpo do formulário.
- SC-016 `examples/cadastro` ganha uma lista de dependentes e um esquema vindo de JSON.
- SC-017 `make test` verde, `api/current.txt` regravado, docs nas duas línguas.
