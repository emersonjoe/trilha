---
title: Desenvolvimento e produção
description: O que trilha dev faz por baixo, como publicar um binário e como configurar por variáveis de ambiente.
---

## `trilha dev`

O comando escuta em `:3000` e roda o seu app em uma porta interna, encaminhando as
requisições. A cada arquivo salvo:

1. regenera `trilha_gen.go` se a árvore de `app/` mudou;
2. recompila com `go build`;
3. sobe o processo novo, espera ele responder e só então derruba o antigo;
4. avisa o navegador por um evento (SSE), que recarrega.

Mudanças só em `public/` pulam os passos 1 a 3. Um erro de compilação vira uma página com a
saída do `go build`; corrija e ela some. O processo do app roda com `TRILHA_ENV=dev`, o que
liga stack traces nas páginas de erro e desliga o cache de estáticos.

## `trilha build`

```bash
trilha build            # → bin/agenda
TRILHA_ENV=prod PORT=8080 ./bin/agenda
```

O binário é estático (`CGO_ENABLED=0`), tem `public/` embutido e não precisa da CLI nem de
nenhum arquivo ao lado. Um `Dockerfile` cabe em quatro linhas:

```text
FROM golang:1.25 AS build
WORKDIR /src
COPY . .
RUN go run github.com/emersonjoe/trilha/cmd/trilha@latest build -o /app

FROM gcr.io/distroless/static
COPY --from=build /app /app
ENV PORT=8080
CMD ["/app"]
```

## Variáveis de ambiente

| Variável | Efeito |
|---|---|
| `PORT` ou `ADDR` | porta ou endereço de escuta (padrão `:3000`) |
| `TRILHA_ENV` | `dev` ou `prod` (padrão `prod`) |
| `TRILHA_BASE_PATH` | prefixo de URL quando o app vive em um subcaminho; use `c.Base()` nos links |
| `TRILHA_EXPORT` | pasta de saída: em vez de servir, exporta o site estático e sai |
| `TRILHA_DEV_RELOAD` | `off` desliga a injeção do script de recarga em dev (testes de snapshot, comparação de HTML); stack traces e `no-cache` continuam |

Outras configurações (limite de corpo, logger, CSRF em APIs) ficam em `trilha.Config`, que o
arquivo gerado monta com `trilha.ConfigFromEnv()`.

## Inicialização com `setup.go`

Abrir um banco, carregar um cache, validar variáveis: tudo isso vai em `app/setup.go`:

```go
package app

import "github.com/emersonjoe/trilha"

func Setup(a *trilha.App) error {
	db, err := sql.Open("pgx", os.Getenv("DATABASE_URL"))
	if err != nil {
		return err // aborta a subida com a mensagem no terminal
	}
	a.Values()["db"] = db
	return nil
}
```

O idiomático em Go é guardar o pool em uma variável do seu próprio pacote
(`internal/banco`), importado pelas páginas. `a.Values()` existe para colagem rápida.

## `trilha export`

Se todas as páginas são estáticas (um blog, uma documentação), exporte HTML e publique em
qualquer hospedagem:

```bash
trilha export -o out --base /agenda
```

Páginas com parâmetro entram quando `Setup` as declara com `a.AddExportPath("/eventos/x")`.
O site que você está lendo foi gerado assim.

## Assets e cache

Publicar HTML novo com CSS velho é o bug que ninguém consegue reproduzir dez minutos
depois. A causa é sempre a mesma: o endereço do arquivo não mudou quando o conteúdo mudou,
e alguma camada de cache — o navegador, um CDN, o GitHub Pages — ainda tem a versão antiga.

O `Asset` põe o hash do conteúdo na URL:

```go
h.Link(h.Rel("stylesheet"), h.Href(c.Asset("/style.css"))) // /style.css?v=8f3a1c92
```

Com isso, um cache longo passa a ser seguro:

```go
cfg.StaticCacheControl = "public, max-age=31536000, immutable"
```

Quem pede a URL versionada certa recebe o cache de um ano; quem pede `/style.css` sem
versão cai na regra normal. Em `dev` nada é imutável e o hash acompanha o arquivo, então
salvar o CSS e atualizar a página basta. O `trilha export` não precisa de nenhuma opção: o
HTML exportado sai com as mesmas URLs, porque é o mesmo layout que o gera.

`trilha audit` avisa quando encontra `immutable` num projeto que não usa `Asset` — é a
combinação que congela um arquivo por um ano no endereço errado.

## Segurança por padrão

Cabeçalhos `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY` e `Referrer-Policy`
em toda resposta; corpo limitado; CSRF em formulários; estáticos sem *path traversal*;
logs com método, caminho, status e duração, nunca com corpo ou cookies. Erros em produção
mostram uma página opaca e vão para o log com o `request_id` que aparece no cabeçalho
`X-Request-ID`.

## Desafio

Publique a agenda em um servidor com `systemd` e faça o serviço reiniciar sozinho se cair.

:::solucao
```text
[Unit]
Description=agenda
After=network.target

[Service]
ExecStart=/opt/agenda/bin/agenda
Environment=PORT=8080 TRILHA_ENV=prod
Restart=always
User=agenda

[Install]
WantedBy=multi-user.target
```
:::
