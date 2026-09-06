---
title: Problemas comuns
description: Erros que aparecem nos primeiros minutos e o que cada um significa.
---

## `zsh: command not found: trilha`

O `go install` colocou o binário em `~/go/bin` (ou no que `go env GOPATH` mostrar mais
`/bin`), e essa pasta não está no seu `PATH`. Adicione ao `~/.zshrc` ou `~/.bashrc` e abra
um terminal novo:

```bash
export PATH="$HOME/go/bin:$PATH"
```

## `verifying module ... 404 Not Found` no `go install`

O módulo está em um repositório privado, ou acabou de ficar público e o proxy ainda não o
conhece. O banco de checksums `sum.golang.org` só consegue verificar módulos públicos. Para
um módulo privado, diga ao Go para não verificar:

```bash
go env -w GOPRIVATE=github.com/sua-org/*
```

Para um módulo recém-publicado, prefira instalar por tag (`@v0.1.0`) em vez de `@latest`.

## `app/ directory not found`

Os comandos da CLI rodam na raiz do projeto, a pasta que contém `app/`. Se o app fica dentro
de um módulo maior (como `examples/blog` no repositório do Trilha), rode a CLI dentro dessa
subpasta: o caminho de import é calculado a partir do `go.mod` mais próximo.

## `E_NO_PAGE_FUNC` ou `E_NO_METHOD`

O arquivo existe, mas a função esperada não está exportada com o nome certo. `page.go`
precisa de `Page`; `route.go` precisa de pelo menos um de `GET`, `POST`, `PUT`, `PATCH`,
`DELETE`; `layout.go` de `Layout`; `middleware.go` de `Middleware`. Assinatura errada é
erro de compilação no `trilha_gen.go`, apontando o pacote.

## `E_UNUSED_METHOD_MIDDLEWARE`

Um `MiddlewarePOST` (ou `GET`, `PUT`, `PATCH`, `DELETE`) num `middleware.go` que não alcança
nenhuma rota com aquele método na sua pasta ou abaixo dela. Em geral o método mudou de lugar
e a regra ficou, ou o nome tem um erro de digitação. Apague ou dê à rota o método que ela
deveria guardar — uma permissão que não guarda nada é pior que permissão nenhuma, porque
parece proteção.

## `E_DUPLICATE_ROUTE`

Duas pastas geram a mesma URL, quase sempre por causa de um grupo de rota. `app/eventos/`
e `app/organizador-/eventos/` respondem os dois em `/eventos`. Renomeie uma.

## `E_HIDDEN_ROUTE`

Um `page.go` ou um `route.go` dentro de pasta cujo nome começa com ponto. O scanner pula
essas pastas, então a rota nunca responderia — antes ela sumia sem uma palavra, e o único
sintoma era um 404. Renomeie a pasta sem o ponto na frente ou, se ela deve mesmo ficar fora
do roteamento, comece o nome com `_`. A única pasta com ponto que **é** roteada é a
`.well-known` (veja [convenções](/pt/referencia/convencoes#pastas)).

## `E_UNROUTABLE_METHOD`

`func HEAD`, `func TRACE` ou `func CONNECT` num `route.go`. O roteador não tira esses de um
arquivo, então a função compilava e não respondia nada: a requisição caía no 405 que o
fallback escreve antes de qualquer middleware. HEAD não está faltando — desde o Go 1.22 o
roteador responde com o handler do `GET`, então escreva a resposta lá. Já o `OPTIONS` é um
handler como os outros, e a rota que só precisa do preflight pode declarar `var CORS` em vez
de escrevê-lo (veja
[convenções](/pt/referencia/convencoes#origem-cruzada-numa-rota-so)).

## O preflight responde 405

A rota não serve `OPTIONS`. Ou declare `var CORS = trilha.CORS{...}` no `route.go` dela — aí
o framework responde o preflight a partir da política — ou escreva `func OPTIONS` à mão. O
`Config.CORS` também responde, mas para o app inteiro: use quando todas as rotas dividem a
política, não para abrir três caminhos.

## Formulário responde 403

Faltou `trilha.CSRFInput(c)` dentro do `<form>`, ou a página do formulário foi aberta antes
de o cookie existir (por exemplo, um `curl` direto no `POST`). Abra a página com `GET`
primeiro, como um navegador faria, ou mande o token em `X-CSRF-Token`.

## O `trilha dev` diz que não há binário aqui

A pasta declara um pacote diferente de `main`, então o `trilha gen` escreveu um pacote
importável, com `NewApp()` e sem `func main()` — um app feito para ser montado por um binário
hospedeiro (`mux.Handle("/", crm.NewApp().Handler())`). Rode o hospedeiro, não esta pasta. Se
o pacote foi engano, corrija no arquivo escrito à mão e gere de novo; o arquivo gerado segue
o que a pasta declara. Veja
[CLI](/pt/referencia/cli#um-app-dentro-de-um-binario-que-ja-existe).

## A porta 3000 está ocupada

```bash
trilha dev --addr :3001
```

## O navegador não recarrega

O script de recarga só é injetado quando a resposta é HTML e passa pelo layout. Uma página
que devolve `c.Text(...)` ou `c.JSON(...)` não recebe o script. Verifique também se algum
proxy (nginx, extensão) está bloqueando `/_trilha/events`, que é uma conexão SSE.

## Mudei `public/` e nada aconteceu em produção

Em produção `public/` está embutido no binário. Rode `trilha build` de novo. Em
desenvolvimento a pasta é lida do disco e a mudança aparece na hora.

## A CLI fala inglês (ou português) e eu quero o outro idioma

A CLI segue `TRILHA_LANG`, depois `LC_ALL`, `LC_MESSAGES` e `LANG`. Defina
`TRILHA_LANG=pt` ou `TRILHA_LANG=en` para forçar um idioma; qualquer valor que não comece
com `pt` significa inglês.
