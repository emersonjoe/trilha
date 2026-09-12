# Spec 133 — As sondas de saúde antes do `AllowedHosts`

- **Issue**: [#173](https://github.com/emersonjoe/trilha/issues/173) — a issue é a fonte do
  escopo (a tabela de sondas por infraestrutura e a alternativa descartada estão lá); aqui
  fica só a decisão.
- **Branch**: `feat/health-antes-do-host`
- **Versão**: 0.112.0

## Por quê

Com `Config.AllowedHosts` definido e `Env: Prod`, `checkHost` roda antes de tudo em
`serveHTTP`, inclusive antes dos endpoints de observabilidade. Toda sonda de infraestrutura
— `HEALTHCHECK` do Docker, `healthcheck.path` do Traefik, `livenessProbe`/`readinessProbe`
do kubelet, target health de um ALB/NLB — endereça o processo por IP, nunca pelo nome
público da lista. O `Host` que chega é esse IP, `checkHost` recusa com 400 antes de a sonda
existir, e o container/pod nunca fica saudável.

Quem segue o aviso `hosts unset` do `trilha check` e define `AllowedHosts` descobre isso em
produção, não em desenvolvimento — a sonda funciona sem a variável e para de funcionar
assim que ela é definida. O contorno (mandar um `Host` da lista em cada sonda) é
conhecimento que cada deploy precisa redescobrir, e cada tentativa recusada vira um evento
`kind=host` no log de segurança, escondendo o evento real que o `AllowedHosts` existe para
denunciar.

## O que muda

`App.serveHTTP` responde as sondas de saúde do próprio kit — `Observability.Health` e seus
dois sufixos, isto é `/_trilha/health`, `/_trilha/health/live` e `/_trilha/health/ready` (ou
o prefixo configurado) — **antes** de `checkHost`, com qualquer `Host`, inclusive um fora da
lista de `AllowedHosts`. A resposta não muda de forma: mesmo corpo, mesmo código HTTP, mesma
autorização de detalhe (bearer ou rede confiável) que já valiam antes desta mudança. Nenhum
outro caminho — rotas do app, CORS, o endpoint de métricas — muda de lugar; `checkHost`
continua na frente deles.

```go
// Antes de checkHost: a sonda chega por IP e nunca reflete o Host de volta
// (sem link, sem cookie, sem redirect, sem cache por Host), então um Host
// forjado não vira ataque por esta porta, e um container que não pode ser
// sondado é pior do que uma sonda respondendo "pass"/"fail".
GET /_trilha/health/live  com Host: 10.0.0.7  (AllowedHosts: app.example.com, Prod) → 200
GET /                     com o mesmo Host                                          → 400
```

`trilha check`: quando `AllowedHosts` está definido, o aviso `hosts ok` ganha uma linha
lembrando que a sonda do container/proxy deve apontar para `/_trilha/health/live` ou
`/ready` (isentos da lista), ou mandar um `Host` da lista se apontar para uma rota do app.

## Fora de escopo

- **Rotas de saúde escritas pelo próprio app** (`app/healthz/route.go`, um liveness "só
  deste processo"). É rota do app como outra qualquer: continua sob `checkHost`, e quem quer
  a isenção troca pela sonda embutida ou manda um `Host` da lista.
- **O endpoint de métricas** (`Observability.Metrics`). Não é sonda de infraestrutura, não
  aparece na issue como alvo da isenção, e continua atrás de `checkHost`.
- **Aceitar CIDR/IP em `AllowedHosts`** e **liberar loopback em produção**: alternativas
  descartadas na issue — resolvem só o `HEALTHCHECK` do Docker, não a sonda que chega pelo
  IP do container/pod.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `net/http` apenas; nenhuma dependência nova |
| VI — teste primeiro | `TestHealthProbeAnswersBeforeAllowedHosts` falha antes da mudança |
| VII — segurança por padrão | a isenção é só a superfície que já responde pass/fail sem detalhe a ninguém anônimo; a proteção continua onde há reflexo de Host (rotas, links, cache) |

## Tarefas

- [x] T001 Teste que falha em `health_test.go`: `Env: Prod`, `AllowedHosts:
      []string{"app.example.com"}`, `GET /_trilha/health/live` com `Host: 10.0.0.7` → 200;
      `GET /` com o mesmo `Host` → 400; a forma da resposta da sonda não muda
- [x] T002 `serveHTTP` responde a sonda de saúde antes de `checkHost`; `TestHostNaBorda`
      (`host_test.go`) ajustado para o novo contrato (a sonda passa, a rota e as métricas
      continuam recusadas)
- [x] T003 `trilha check`: linha nova no hint de `hosts ok` sobre apontar a sonda do
      orquestrador para o caminho isento
- [x] T004 Documentação em `en/` e `pt/` (`reference/security`, `cookbook/production-checklist`)
      e `SECURITY-MODEL.md`; comentário em `examples/cookbook/production.go`
- [x] T005 `CHANGELOG.md`, `version` em `cmd/trilha/main.go`, linha da versão no
      `ROADMAP.md`
- [x] T006 `make test` verde

## Aceitação

- **SC-001** `GET /_trilha/health`, `/live` e `/ready` respondem sem consultar
  `AllowedHosts`, com qualquer `Host`, em `Prod`.
- **SC-002** Qualquer outro caminho — uma rota do app, o endpoint de métricas — continua
  recusado com 400 e o evento `kind=host` quando o `Host` está fora da lista.
- **SC-003** O corpo, o código HTTP e a autorização de detalhe da sonda são os mesmos de
  antes da mudança; só a posição no `serveHTTP` muda.
