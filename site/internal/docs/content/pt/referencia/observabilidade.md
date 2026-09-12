---
title: Observabilidade
description: Config.Observability, endpoints de saúde, registro de métricas, variáveis de ambiente e o contrato de cada resposta.
---

## Config.Observability

| Campo | Padrão | O que faz |
|---|---|---|
| `Health string` | `/_trilha/health` | caminho base das sondas; `trilha.Off` remove |
| `Metrics string` | `""` (desligado) | caminho da raspagem; vazio não registra endereço **nem instrumenta requisições** |
| `Token string` | `TRILHA_OBS_TOKEN` | autoriza detalhe e métricas; **mínimo de 32 bytes**, comparado em tempo constante |
| `Trusted []string` | — | CIDRs (ou IPs) que dispensam o token |
| `Details string` | automático | `trilha.Off` nunca revela detalhe, nem para quem tem token; vazio = aberto em `dev`, autorizado em `prod` |
| `Timeout time.Duration` | 2 s | prazo de cada verificação; `trilha.NoTimeout` desliga |
| `CacheFor time.Duration` | 1 s | validade do resultado de prontidão; `trilha.NoTimeout` desliga o cache |

Variáveis lidas por `ConfigFromEnv`: `TRILHA_OBS_TOKEN`, `TRILHA_METRICS`,
`TRILHA_OBS_TRUSTED` (lista separada por vírgula).

## Endpoints

| Método e caminho | Resposta | Status |
|---|---|---|
| `GET /_trilha/health/live` | `application/health+json` | sempre 200 |
| `GET /_trilha/health/ready` | idem, roda as verificações | 200 ou 503 + `Retry-After: 5` |
| `GET /_trilha/health` | igual a `ready` | 200 ou 503 |
| `GET <Metrics>` | `text/plain; version=0.0.4` | 200, ou 401 sem autorização |

Todas saem com `Cache-Control: no-store`, `X-Robots-Tag: noindex` e
`X-Content-Type-Options: nosniff`. Outro método devolve 405 com `Allow: GET, HEAD`.

As sondas correm **fora** da cadeia de middleware: sem CSRF, sem layout, sem limite de
taxa (uma sonda de vida que tomasse 429 mataria um processo saudável) e registradas em
nível `Debug`, para não afogar o log de auditoria.

## Verificações de prontidão

```go
func (a *App) Check(name string, fn func(context.Context) error)
func (a *App) HealthReport(ctx context.Context) HealthReport
```

```go
type HealthReport struct {
	Status        string        // trilha.StatusPass | trilha.StatusFail
	Checks        []CheckResult
	UptimeSeconds float64
}

type CheckResult struct {
	Name       string
	Status     string
	DurationMS float64
	Error      string
}
```

`HealthReport` devolve tudo, sempre: é para o seu código (uma página de status interna, um
portão de inicialização). Quem decide o que revelar é o endpoint. Os dois valores são as
constantes `trilha.StatusPass` (`"pass"`) e `trilha.StatusFail` (`"fail"`), então uma página de
status compara com o nome do framework em vez de uma string que ela digitou.

## Registro de métricas

```go
func (a *App) Metrics() *Metrics

func (m *Metrics) Counter(name, help string, labels ...string) *Counter
func (m *Metrics) Gauge(name, help string, labels ...string) *Gauge
func (m *Metrics) Histogram(name, help string, buckets []float64, labels ...string) *Histogram
```

`MaxSeries` (mil por padrão) é o teto de combinações de rótulo por métrica; o excedente cai
numa série com todos os rótulos em `other` e um aviso no log, uma única vez.

| Tipo | Métodos |
|---|---|
| `*Counter` | `Inc()`, `Add(v)`, `With(valores...)` |
| `*Gauge` | `Set(v)`, `Add(v)`, `Inc()`, `Dec()`, `With(valores...)` |
| `*Histogram` | `Observe(v)`, `With(valores...)` |

Nome inválido (fora de `[a-zA-Z_:][a-zA-Z0-9_:]*`) ou número errado de valores de rótulo
causam `panic`: é erro de programação, aparece na primeira execução e não corrompe a saída.
Chamar `Counter` duas vezes com o mesmo nome devolve a mesma série.

`Histogram` com `buckets` nulo usa os padrões, em segundos: 0,001 0,005 0,01 0,025 0,05
0,1 0,25 0,5 1 2,5 5 10.

## Métricas do framework

| Métrica | Tipo | Rótulos |
|---|---|---|
| `trilha_requests_total` | contador | `method`, `route`, `status` |
| `trilha_request_duration_seconds` | histograma | `method`, `route` |
| `trilha_requests_in_flight` | medidor | — |
| `trilha_security_events_total` | contador | `kind` (`csrf`, `auth`, `body`, `rate`, `panic`) |
| `trilha_panics_total` | contador | — |
| `go_goroutines`, `go_memstats_alloc_bytes`, `go_memstats_sys_bytes` | medidores | — |
| `go_gc_cycles_total` | contador | — |
| `trilha_uptime_seconds` | medidor | — |
| `trilha_build_info` | medidor (sempre 1) | `version`, `go_version` |

`route` é o padrão registrado (`/blog/{slug}`). Estático, 404 e qualquer coisa fora do
roteador entram como `other`.

## Correlação

```go
func (c *Ctx) RequestID() string  // X-Request-ID do cliente, ou gerado
func (c *Ctx) TraceID() string    // W3C traceparent; "" quando ausente ou malformado
func (c *Ctx) Log() *slog.Logger  // logger com request_id e trace_id
```

Um `traceparent` fora do formato é descartado em silêncio: valor escolhido por terceiro não
entra no log como se fosse traço legítimo.

## O que a auditoria verifica

`trilha audit` acrescenta três itens: token curto demais (crítico), métricas configuradas
sem token nem rede confiável (crítico), `0.0.0.0/0` em `Trusted` (aviso) e ausência de
qualquer `a.Check(` no projeto (aviso).

## A trilha de auditoria

Toda aplicação interna acaba precisando de "quem fez o quê": quem excluiu o documento, quem
mudou a permissão, quem exportou a lista. O framework já tem metade — o request id, o IP do
cliente atrás de `TrustedProxies`, a sessão, o gabarito da rota. Falta a frase, e um lugar para
ela.

```go
func DELETE(c *trilha.Ctx) error {
	if err := docs.Excluir(c, c.Param("id")); err != nil {
		return err
	}
	c.Audit("documento.excluiu", c.Param("id"))
	return c.Redirect("/documentos")
}

c.Audit("permissao.alterou", papel, trilha.Fields{"modulo": "docs", "de": "ver", "para": "editar"})
```

O registro que sai, sem a aplicação montar mais nada:

```json
{"at":"2026-09-08T15:04:05Z","action":"documento.excluiu","target":"42",
 "actor":{"subject":"u_17","email":"ana@org.br","via":"session"},
 "ip":"10.0.0.7","request_id":"…","route":"/documentos/{id}"}
```

É por isso que cabe em uma linha: o ator, o endereço, o request id e a rota já são conhecidos —
e escrevê-los à mão é o que toda aplicação faz e o que toda aplicação esquece em metade dos
handlers.

### Para onde vai

`Config.Audit` é um `trilha.AuditSink`: um método, `Write(AuditRecord) error`, porque a decisão
que a aplicação de fato toma é *qual tabela*, não *qual formato*. Um erro que volta vai para o
log e a requisição segue — o documento foi apagado de todo jeito, e recusar a resposta perderia a
trilha *e* confundiria a pessoa.

```go
cfg.Audit = trilha.AuditFunc(func(r trilha.AuditRecord) error {
	_, err := db.Exec(`INSERT INTO auditoria (at, action, target, actor, ip) VALUES (?,?,?,?,?)`,
		r.At, r.Action, r.Target, r.Actor.Subject, r.IP)
	return err
})
```

Deixe nil e o registro vai para o logger da app com `kind=audit`, o que basta para um `grep` e
basta para publicar uma primeira versão.

**Sink que falha não derruba a resposta.** O erro vai para o log e a requisição segue. É
deliberado: o documento foi excluído de qualquer forma, e recusar-se a responder agora perderia
a trilha **e** confundiria quem fez — duas falhas em vez de uma.

### Quem é o ator

O `auth` marca: qualquer rota atrás de `Require`, `RequireRole` ou `RequirePolicy` atribui a
trilha sem a aplicação escrever uma linha, com `via: "session"`.

Aplicação que autentica do seu jeito chama `c.SetActor` uma vez no middleware, e tudo abaixo
fica atribuído:

```go
c.SetActor(trilha.Actor{Subject: chave.ID, Name: chave.Rotulo, Via: "api_key"})
```

Ninguém reconhecido é registrado como `anonymous` — e é registrado: trilha que descarta em
silêncio a ação anônima tem um buraco exatamente onde alguém iria procurar. O `trilha audit`
avisa quando o `c.Audit` é chamado num projeto em que nenhuma rota exige sessão.

### A tela

O `ui.AuditTable` é a tela que toda aplicação com `c.Audit` acaba escrevendo à mão: quem fez o
quê, em quê, quando e de onde.

```go
regs, total := auditoria.Buscar(q)
return ui.AuditTable(c, regs, ui.AuditOpts{
	Params:  q.ListParams,
	Total:   total,
	Actions: auditoria.Acoes(),
	Action:  q.Acao,
	Export:  "/auditoria/csv",
}), nil
```

Por baixo é um `ui.DataTable`, e é esse o ponto: o formulário de filtro, os links de ordem, a
paginação e a troca de fragmento são os mesmos de qualquer outra listagem. Uma trilha que se
comportasse diferente do resto do app seria uma segunda coisa para aprender.

**Ler a trilha é da aplicação.** O `Config.Audit` é uma interface de escrita com um método e
continua assim — o framework não tem banco, e a consulta desta tela (um período, um ator, uma
tabela que este app escolheu) não é algo que ele pudesse escrever. A do exemplo tem trinta
linhas sobre uma fatia.

`Fields` é detalhe e não coluna: cada ação carrega as suas chaves, e uma coluna por chave é uma
tabela que ganha coluna toda vez que alguém audita algo novo.

O `Export` aponta para uma rota que responde com `c.CSV`, e o botão leva o recorte que está na
tela — exportar ignorando o filtro na frente da pessoa é exportar a coisa errada, e ela só
descobre na planilha.

:::warning
A trilha é a lista das ações de todo mundo, então **lê-la é ato administrativo**. Ponha a tela
atrás do mesmo guarda do resto da administração, e ponha a exportação **dentro** da pasta
guardada — um download é outra resposta, não outra permissão. No
[`examples/local-login`](https://github.com/emersonjoe/trilha/tree/main/examples/local-login/app/auditoria)
o `/auditoria/csv` herda o middleware da pasta sem dizer uma palavra sobre isso; fora dela seria
o único endereço entregando a trilha inteira para qualquer um.
:::

### `Route` é o gabarito

`/documentos/{id}`, não `/documentos/42`. O id concreto já está no `Target`; o gabarito é o que
permite a uma consulta agrupar mil exclusões numa linha.

