---
title: CLI
description: Os comandos de trilha e suas opções.
---

```text
trilha new <dir> [--module caminho] [--lang en|pt] [--trilha-dir ../trilha] [--no-tidy]
trilha gen [--check] [--package nome]
trilha generate page|route <url> | component <Nome> [--force] [--dir caminho]
trilha dev [--addr :3000]
trilha build [-o bin/<nome>]
trilha export [-o out] [--base /prefixo]
trilha openapi [-o arquivo] [--title T] [--version V] [--server URL] [--check]
trilha routes
trilha audit [--no-vuln]
trilha ui [--force] [--css-only|--js-only]
trilha version
```

| Comando | O que faz |
|---|---|
| `new` | cria um projeto com `go.mod`, layout, página inicial, 404, uma rota de API, `public/style.css` e `.gitignore`; roda `go mod tidy` e `gen` |
| `gen` | varre `app/` e escreve `trilha_gen.go`; falha com uma linha por convenção violada |
| `generate` | grava um esqueleto — página, rota de API ou componente — na pasta que a convenção pede |
| `dev` | `gen` + `go build` + executa o app em uma porta interna + proxy em `--addr` + recarga por SSE + inspetor de rotas em `/_trilha/routes` |
| `build` | `gen` + `go build -trimpath -ldflags="-s -w"` com `CGO_ENABLED=0` |
| `export` | `gen` + `go build` + executa com `TRILHA_EXPORT` para gerar HTML estático |
| `openapi` | escreve o documento OpenAPI 3.1 das rotas de API (`-o -` na saída padrão) |
| `routes` | imprime `MÉTODOS PADRÃO ORIGEM` para cada rota |
| `audit` | checklist de segurança antes de publicar (veja [Segurança](/pt/referencia/seguranca)) |

Os comandos rodam na pasta que contém `app/`. O caminho de import do projeto vem do
`go.mod` mais próximo, mais a subpasta, então um app pode viver dentro de um módulo maior.

## Idioma

As mensagens da CLI seguem `TRILHA_LANG`, depois `LC_ALL`, `LC_MESSAGES` e `LANG`: um
valor começando com `pt` (em qualquer caixa) seleciona português; qualquer outro, inclusive
variável indefinida, seleciona inglês. As mensagens do runtime, do scanner e do gerador (as
que acabam no seu código e nos seus logs) são sempre em inglês.

`trilha new --lang en|pt` escolhe o idioma dos textos gerados (página inicial, 404,
`<html lang>`); o padrão é o idioma da CLI.

## trilha dev

Além do proxy e da recarga, o supervisor serve o inspetor de rotas em `/_trilha/routes`: a
tabela de rotas em ordem de precedência com layouts e middlewares de cada uma, e uma caixa que
responde qual padrão atenderia um caminho. A página é do supervisor, não do app, então ela não
existe no binário que o `trilha build` produz — veja
[Dev e produção](/pt/aprender/dev-e-producao#o-inspetor-de-rotas).

## trilha generate

A convenção é o que custa lembrar: que `/blog/{slug}` mora em `app/blog/slug_/`, que uma pasta
catch-all termina em `__`, que um grupo termina em `-`. O `generate` recebe a URL e faz a
tradução:

```bash
trilha generate page /blog/{slug}     # app/blog/slug_/page.go
trilha generate route /api/itens/{id} # app/api/itens/id_/route.go
trilha generate component Aviso       # internal/components/aviso.go
```

A página e a rota saem compilando, com `c.Param` já lendo cada parâmetro, e o `trilha_gen.go`
é regerado no fim — a URL responde antes de você abrir o editor. Um componente é uma função
que devolve `h.Node`, então compõe como qualquer outra; `--dir` põe em outro lugar
(`internal/icones`, por exemplo).

O nome do pacote é o que já está declarado na pasta, quando existe; senão vem do nome da pasta
(`slug_` → `slug`, `relatorio.csv` → `relatoriocsv`, `type` → `type_`).

Um arquivo existente não é sobrescrito sem `--force`, e o `--force` não cobre a única recusa
que é convenção: uma pasta responde ou uma página ou uma rota, nunca as duas.

## trilha ui

Grava ou atualiza o kit de interface em `public/`: `ui.theme.css` (só criado; é o seu
tema), `ui.css` e `ui.js` (atualizados; se editados localmente, só com `--force`).
`--css-only` e `--js-only` limitam o que é tocado. `trilha new` roda o mesmo passo. Veja
[Interface com ui](/pt/aprender/interface-com-ui).

## trilha openapi

Lê `app/`, deduz o documento a partir dos handlers e escreve `openapi.json`. `-o -` escreve na
saída padrão; `--title`, `--version` e `--server` preenchem o que o código não tem como saber
(o padrão é o nome do módulo, `0.0.0` e nenhum servidor). `--check` compara com o arquivo no
disco e sai com `1` quando divergem — a mesma linha que o `gen --check` é, pelo mesmo motivo:

```yaml
- run: trilha openapi --check
```

O que é deduzido e as diretivas `openapi:` estão em [APIs](/pt/aprender/api#documento-openapi).

## trilha gen --check

Gera em memória, compara com o `trilha_gen.go` commitado e sai com `1` mostrando as linhas
que divergem — uma linha no CI, e uma pasta nova em `app/` sem `trilha gen` depois deixa de
ser um 404 que ninguém explica:

```yaml
- run: trilha gen --check
```

O `trilha audit` faz a mesma comparação como aviso, e ainda compara a versão da CLI com a da
biblioteca no `go.mod`: uma CLI mais nova escreve código que a biblioteca pode ainda não ter,
e o erro aparece dentro de código gerado — o pior lugar para procurar.

## Arquivo gerado

`trilha_gen.go` é determinístico (mesma árvore, mesmos bytes), tem o cabeçalho
`// Code generated by trilha. DO NOT EDIT.`, mais a diretiva `//go:generate trilha gen`
(para `go generate ./...` funcionar sem ninguém precisar saber o nome da ferramenta), e deve
ser commitado: `go build ./...` funciona
sem a CLI instalada. Ele define `newApp() *trilha.App` e `main()`; se outro arquivo do
pacote já tem `func main()`, o gerador omite o dele (veja [App](/pt/referencia/app)).

### Um app dentro de um binário que já existe

O arquivo gerado adota o pacote que a pasta declara, então um app Trilha pode ser um pacote
comum, importável, dentro de um servidor `net/http` que você já roda:

```go
// internal/crm/crm.go — package crm, escrito à mão
// internal/crm/app/…    — as rotas
// internal/crm/trilha_gen.go — package crm, func NewApp() *trilha.App

mux.Handle("/", crm.NewApp().Handler())
```

A precedência, do mais explícito ao menos: `--package <nome>`; o pacote declarado pelos `.go`
escritos à mão da pasta; o pacote declarado por um `trilha_gen.go` que já esteja lá; `main`.
O terceiro passo é o que faz a bandeira valer uma vez só — o arquivo gerado lembra a escolha,
e o `trilha gen --check` do CI não precisa dela.

Fora do `package main` o construtor é exportado (`NewApp`, porque quem chama mora em outro
pacote) e nenhum `func main()` é escrito. O `trilha dev` e o `trilha build` recusam um app
assim e dizem quem o roda: não há binário aqui, o hospedeiro é que tem um.

## Códigos de saída

`0` sucesso; `1` erro de geração, compilação ou execução; `2` uso incorreto.
