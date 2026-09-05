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

## `pasta app/ não encontrada`

Os comandos da CLI rodam na raiz do projeto, a pasta que contém `app/`. Se o app fica dentro
de um módulo maior (como `examples/blog` no repositório do Trilha), rode a CLI dentro dessa
subpasta: o caminho de import é calculado a partir do `go.mod` mais próximo.

## `E_NO_PAGE_FUNC` ou `E_NO_METHOD`

O arquivo existe, mas a função esperada não está exportada com o nome certo. `page.go`
precisa de `Page`; `route.go` precisa de pelo menos um de `GET`, `POST`, `PUT`, `PATCH`,
`DELETE`; `layout.go` de `Layout`; `middleware.go` de `Middleware`. Assinatura errada é
erro de compilação no `trilha_gen.go`, apontando o pacote.

## `E_DUPLICATE_ROUTE`

Duas pastas geram a mesma URL, quase sempre por causa de um grupo de rota. `app/eventos/`
e `app/organizador-/eventos/` respondem os dois em `/eventos`. Renomeie uma.

## Formulário responde 403

Faltou `trilha.CSRFInput(c)` dentro do `<form>`, ou a página do formulário foi aberta antes
de o cookie existir (por exemplo, um `curl` direto no `POST`). Abra a página com `GET`
primeiro, como um navegador faria, ou mande o token em `X-CSRF-Token`.

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
