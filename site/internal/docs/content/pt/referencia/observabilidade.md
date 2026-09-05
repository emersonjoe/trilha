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
	Status        string        // "pass" | "fail"
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
portão de inicialização). Quem decide o que revelar é o endpoint.

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
