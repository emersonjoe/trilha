# Spec 051 — `trilha generate` com contrato: `--methods`, `--bind`, `--form` e `generate test`

- **Issue**: [#49](https://github.com/emersonjoe/trilha/issues/49) (ROADMAP, Fase 5, item 27)
- **Branch**: `051-generate-com-contrato`
- **Versão**: 0.39.0

## Por quê

O `trilha generate` da 0.27.0 acerta o lugar do arquivo e erra o resto: o esqueleto é
genérico. Quem chamou o comando ainda escreve a struct, o `Bind`, a validação, a resposta e o
teste — e é aí que um agente erra assinatura e gasta token de saída, que é o token caro.

O comando já sabe a URL. Com ela vêm os parâmetros do caminho, e com duas flags vêm o
contrato: quais métodos e qual tipo entra. Tudo isso o gerador consegue escrever certo na
primeira vez, porque é convenção, não decisão de produto.

## O que muda

### `trilha generate route <url> [--methods GET,POST] [--bind Tipo]`

- `--methods`: um handler por método, na assinatura certa, com `c.Param("id")` já escrito
  para cada parâmetro do caminho. Sem a flag, o `GET` de hoje. A lista é validada contra os
  métodos que o scanner reconhece; um método repetido ou desconhecido é erro antes de
  escrever arquivo.
- `--bind Tipo`: os métodos com corpo (`POST`, `PUT`, `PATCH`) ganham `c.BindJSON(&in)` e o
  422 da 0.21.0 sai de graça, porque devolver o erro do `Bind` já é a resposta certa.
  - Se o tipo existe no projeto, ele é importado pelo caminho onde está.
  - Se não existe, a struct nasce no pacote da rota com tags `json` e `validate` de exemplo.
  - Um tipo que existe em mais de um pacote é erro: o comando diz onde achou e pede o nome
    qualificado (`posts.Comment`).

### `trilha generate page <url> [--form Tipo] [--layout app/layout.go]`

- `--form Tipo`: a página vira a ida e volta completa de formulário do kit `ui` —
  `trilha.CSRFInput`, um `ui.Field` por campo com `ui.InvalidIf`/`ui.Errors`, `POST` que
  reexibe a página com 422 e `trilha.FieldErrors` quando o `Bind` recusa, e redireciona
  quando aceita. O tipo segue a mesma regra do `--bind`, com tags `form` no lugar de `json`.
- `--layout <arquivo>`: grava o esqueleto do layout naquele caminho, se ainda não houver um.
  O caminho tem que ser um `layout.go` numa pasta ancestral da página, dentro de `app/` —
  qualquer outro o scanner nunca aplicaria, e pedir por ele é engano que custa uma rodada.

### `trilha generate test <url>`

Escreve `page_test.go` ou `route_test.go` ao lado da rota, no pacote dela, com um caso por
método usando `trilha.TestRoute` e `trilha.TestPage`. A rota é encontrada pelo mesmo scanner
que gera o `trilha_gen.go`, então o teste conhece os métodos que existem de verdade, não os
que a flag pediu.

- Um parâmetro do caminho vira o próprio nome no alvo: `/blog/{slug}` é exercitado em
  `/blog/slug`.
- Método com corpo cujo tipo de `Bind` o gerador resolve manda um corpo montado das tags
  (`min` de string vira texto com aquele tamanho, número vira número, `oneof` vira o primeiro
  valor da lista) — o teste passa pela validação em vez de esbarrar nela.
- Método com corpo cujo tipo não é resolvível manda `{}` e afirma o status que o esqueleto
  responde.

### `--lang en|pt`

Como no `new`: os comentários do esqueleto saem no idioma pedido (padrão, o da CLI).
Identificadores, nomes de campo e mensagens de erro continuam em inglês, como manda a
constituição.

## Superfície

| Símbolo | Papel |
|---|---|
| `trilha generate route --methods --bind` | contrato da rota de API |
| `trilha generate page --form --layout` | contrato da página de formulário |
| `trilha generate test <url>` | o teste ao lado da rota |
| `scaffold.GenOptions.{Methods,Bind,Form,Layout,Module,Lang}` | as opções novas |

Nada novo em `package trilha`.

## Fora de escopo

- **Inferência de tipo com o compilador.** O tipo do `--bind` é procurado por parsing, como o
  `internal/openapi` faz desde a 031. Tipo que vem de dependência externa não é achado, e o
  comando diz isso em vez de adivinhar.
- **Teste verde sobre rota escrita à mão.** A promessa de nascer verde vale para o esqueleto
  que este comando escreveu. Numa rota que já existia, `generate test` é ponto de partida:
  ele acerta métodos, alvo e auxiliares, e o corpo do caso pode precisar de edição.
- **`--methods` numa página.** Página que responde `POST` é o `--form`; qualquer outro método
  numa página é rota, e o comando manda usar `route`.
- **Gerar o `openapi:` de cada resposta.** O documento sai do handler, e o handler que este
  comando escreve já é lido pelo `internal/openapi` sem anotação.
- **Mexer no esqueleto de hoje.** Sem flag nenhuma, byte por byte, a saída é a da 0.38.0.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| I — roteamento por arquivos é a fonte | o `generate test` acha a rota pelo scanner, não por argumento |
| II — só biblioteca padrão | `go/ast`, `go/parser`, `go/format`, `text/template` |
| III — gerador determinístico | cada combinação de flags tem golden; ordem de campos e imports é a do arquivo lido |
| IV — convenção nova pede scanner + exemplo + integração | nenhuma convenção nova em `app/`; o e2e prova o par gerado passando no `check` |
| VI — teste primeiro | golden e teste de recusa antes do template |
| IX — API pública pequena | três flags e um `kind`; `package trilha` não muda |

## Aceitação

- **SC-001** `trilha generate route /api/posts/{id}/comments --methods GET,POST` escreve um
  `GET` e um `POST` com `c.Param("id")` nos dois, e bate com o golden.
- **SC-002** `--bind Comment` com o tipo ausente declara a struct no pacote da rota com tags
  `json` e `validate`; com o tipo presente em `internal/posts`, importa
  `<módulo>/internal/posts` e usa `posts.Comment`. Dois goldens.
- **SC-003** `--bind` de um nome que existe em dois pacotes falha nomeando os dois, sem
  escrever arquivo.
- **SC-004** `trilha generate page /contato --form Contact` escreve a página com `CSRFInput`,
  um campo por campo do tipo, `POST` com 422 e redirect; bate com o golden.
- **SC-005** `--layout app/layout.go` grava o layout que falta; `--layout app/x/layout.go`
  para uma página em `app/y/` é recusado por não ser ancestral, e `--layout app/layout.txt`
  por não ser um `layout.go`.
- **SC-006** `trilha generate test /api/posts` escreve `route_test.go` no pacote da rota com
  um caso por método existente; `generate test` numa URL que nenhuma rota responde falha
  dizendo isso.
- **SC-007** e2e: num projeto novo, `generate route --methods GET,POST --bind Item`,
  `generate page --form Contact` e os dois `generate test` correspondentes deixam
  `trilha check` verde, sem edição nenhuma.
- **SC-008** `--lang pt` troca os comentários do esqueleto e não troca identificador nenhum
  (golden do par `--form Contact` nos dois idiomas).
- **SC-009** Sem flags, a saída dos três `kind`s de hoje é idêntica à da 0.38.0 (os testes da
  036 continuam verdes sem mudança).
- **SC-010** `make golden` regrava os goldens do `internal/scaffold`; o `usage` da CLI e o
  `AGENTS.md` nomeiam as flags novas, nos dois idiomas.
