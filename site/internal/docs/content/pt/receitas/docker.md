---
title: Docker
description: Um binário estático numa imagem distroless, os assets já dentro dele, as variáveis que ele precisa e uma sonda de saúde que o orquestrador consegue usar.
---

Um app Trilha é um binário só, com as páginas compiladas dentro e, se você usou `//go:embed`,
os arquivos estáticos também. Isso deixa a imagem pequena o bastante para que a parte
interessante seja o que você tira dela.

## O Dockerfile

```dockerfile
FROM golang:1.22 AS build
WORKDIR /src
# Dependências primeiro: esta camada fica em cache até o go.mod mudar.
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# O arquivo gerado é commitado, mas gerar de novo no build é como você
# descobre que alguém esqueceu de rodar o trilha gen.
RUN go run ./cmd/trilha gen
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /app ./

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /app /app
# A porta é documentação; a plataforma decide o que publica.
EXPOSE 3000
USER nonroot
ENTRYPOINT ["/app"]
```

Duas linhas carregam o peso. `CGO_ENABLED=0` faz um binário estático, que é o que permite o
segundo estágio ser `distroless/static` — sem shell, sem gerenciador de pacotes, nada para
explorar que não seja o seu código. `nonroot` quer dizer que um bug no seu app é um bug rodando
como uid 65532.

:::atencao
`CGO_ENABLED=0` e os drivers de SQLite que precisam de cgo são mutuamente exclusivos. Ou você
usa um driver em Go puro (`modernc.org/sqlite`), ou constrói sobre `debian:bookworm-slim` e
aceita a imagem maior.
:::

## O endereço

`trilha.ConfigFromEnv` já lê `PORT` e `ADDR`, então a plataforma que entrega uma porta é
obedecida sem código — o padrão é `:3000`, e `:3000` significa todas as interfaces, que é o que
um contêiner precisa. A imagem quebrada mais comum é a que mandaram escutar em `127.0.0.1`:
dentro do contêiner isso é o próprio contêiner, e nada de fora chega nele.

## As variáveis

| Variável | O que é |
|---|---|
| `TRILHA_ENV` | `prod` — desliga o recarregamento de dev, a página de erro com fonte, o log verboso |
| `TRILHA_SECRET` | a chave de cookies e CSRF; pelo menos 32 bytes, vinda do cofre da plataforma |
| `PORT` ou `ADDR` | onde escutar; `:3000` por padrão |
| `DATABASE_URL` | o DSN do seu pool |
| `TRILHA_BASE_PATH` | só quando o app não está na raiz do domínio |

Um segredo assado na imagem é um segredo no registry, e em todo cache de camada que já baixou
ela. Rodar a chave é `TRILHA_SECRET_PREVIOUS` com o valor antigo por um deploy ou dois, para
que sessões assinadas com a chave velha continuem valendo enquanto expiram.

## Compose

```yaml
services:
  app:
    build: .
    environment:
      TRILHA_ENV: prod
      DATABASE_URL: postgres://app:app@db:5432/app?sslmode=disable
    env_file: [.env]           # TRILHA_SECRET mora aqui, não neste arquivo
    ports: ["8080:3000"]
    depends_on:
      db: { condition: service_healthy }
    healthcheck:
      test: ["CMD", "/app", "-health"]
      interval: 10s
  db:
    image: postgres:16-alpine
    environment: { POSTGRES_PASSWORD: app, POSTGRES_USER: app, POSTGRES_DB: app }
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U app"]
      interval: 5s
    volumes: [pgdata:/var/lib/postgresql/data]
volumes:
  pgdata:
```

Uma imagem distroless não tem `curl` nem shell, então a checagem de saúde não pode ser um
comando de shell contra uma URL. Duas saídas: uma flag `-health` no seu próprio binário, que
pede `/_trilha/health/ready` e sai com o status, ou a sonda do próprio orquestrador — que é o
que o Kubernetes faz, e não precisa de nada dentro da imagem:

```yaml
livenessProbe:
  httpGet: { path: /_trilha/health/live, port: 3000 }
readinessProbe:
  httpGet: { path: /_trilha/health/ready, port: 3000 }
  periodSeconds: 5
```

`live` diz que o processo está de pé; `ready` diz que ele consegue servir, e é o que fica
vermelho quando o banco sumiu. Ligar os dois trocados é como um contêiner que perdeu o banco
passa a ser reiniciado para sempre em vez de ser tirado do balanceador.

:::nota
Existe uma resposta menor que um contêiner. `trilha export` escreve um site estático quando o
app não tem rota dinâmica, e `trilha build` escreve o binário se tudo que você precisa é copiar
um arquivo para uma máquina e rodar sob o systemd. Nem tudo precisa de orquestrador.
:::
