# Plano: 014-observabilidade

## Superfície pública

```go
// Config
type Observability struct {
    Health   string        // caminho base; padrão "/_trilha/health"; Off desliga
    Metrics  string        // caminho; vazio = desligado; típico "/_trilha/metrics"
    Token    string        // TRILHA_OBS_TOKEN; exigido para detalhe e métricas fora de dev
    Trusted  []string      // CIDRs que dispensam o token (rede de raspagem)
    Details  string        // "" automático (dev sim, prod só autorizado), Off nunca
    Timeout  time.Duration // prazo por verificação (2 s)
    CacheFor time.Duration // validade do resultado (1 s); NoTimeout desliga o cache
}

func (a *App) Check(name string, fn func(context.Context) error) // prontidão
func (a *App) Health(ctx context.Context) HealthReport           // programático
func (a *App) Metrics() *Metrics

func (m *Metrics) Counter(name, help string, labels ...string) *Counter
func (m *Metrics) Gauge(name, help string, labels ...string) *Gauge
func (m *Metrics) Histogram(name, help string, buckets []float64, labels ...string) *Histogram
func (c *Counter) Inc() / Add(float64) / With(values ...string) *Counter
func (c *Ctx) Log() *slog.Logger  // request_id + trace_id
func (c *Ctx) TraceID() string    // W3C traceparent, "" quando ausente
```

## Arquivos

| Arquivo | Papel |
|---|---|
| `observability.go` | `Observability`, registro dos endpoints no mux, porteiro (token em tempo constante + CIDR), cabeçalhos `no-store`/`noindex` |
| `health.go` | `Check`, corredor com prazo e cache, `HealthReport`, resposta `application/health+json` mínima ou detalhada |
| `metrics.go` | registro (`Metrics`), séries com rótulos e teto de cardinalidade, formato de texto do Prometheus, coletores de runtime |
| `trace.go` | `traceparent` (W3C), `Ctx.TraceID`, `Ctx.Log` |
| `serve.go` | instrumentação: in-flight, duração, contador por rota; `trace_id` no log |
| `events.go` | `trilha_security_events_total{kind}` |
| `cmd/trilha/audit.go` | dois itens novos |
| `site/internal/docs/content/{aprender,referencia}/observabilidade.md` | documentação |
| `examples/blog/app/setup.go` + `examples/blog/blog_test.go` | uso real e teste de integração |
| `bench/bench_test.go` | custo com e sem métricas |

## Decisões

1. **Endpoints fora do `wrap`.** São registrados direto no mux, com o seu próprio caminho
   curto: não passam por CSRF, layouts, limitador nem middleware de aplicação. Isso evita
   429 numa sonda de vida (FR-005) e mantém o custo perto de zero.
2. **`live` não executa verificação.** Prontidão que falha tira o pod do balanceador;
   vivacidade que falha **mata** o processo. Misturar as duas derruba o serviço inteiro
   quando o banco pisca. `live` só responde que o processo aceita conexões.
3. **Cache do resultado.** Um alvo anônimo que dispara 10 mil sondas por segundo não pode
   virar 10 mil `SELECT 1`. O resultado vale 1 s por padrão (SC-5).
4. **Rótulo de rota é o padrão, não o caminho.** Caminho concreto traz identificador de
   usuário, token em query e cardinalidade infinita: três problemas de uma vez.
5. **Formato Prometheus escrito à mão.** São ~120 linhas de texto; trazer `client_golang`
   violaria o princípio II e arrastaria dezenas de módulos para dentro de todo projeto Trilha.
6. **`fail` genérico para anônimo.** ASVS V7.4.1: a causa vai para o log e para a visão
   autorizada.

## Constitution Check

| Princípio | Como a feature respeita |
|---|---|
| I. Convenção sobre configuração | não inventa convenção de pasta; os endereços são fixos e previsíveis, ligados em `setup.go` |
| II. Só biblioteca padrão | `sync/atomic`, `runtime`, `crypto/subtle`, `net/netip`; nada mais |
| III. Geração explícita | o gerador não muda; nada é descoberto por reflexão |
| IV. Contrato de handler | `Check` recebe `context.Context` e devolve `error`, no mesmo espírito |
| V. Dev rápido, prod um binário | endpoints embutidos, sem arquivo externo |
| VI. Teste primeiro | testes de `health`, `metrics`, porteiro e formato antes do código |
| VII. Segurança por padrão | desligado por padrão, autenticado quando ligado, sem vazamento de detalhe, sem cache, sem indexação, com prazo e teto |

## Complexity Tracking

Nenhuma violação. O ponto de atenção é o custo por requisição (spec 012): a instrumentação
fica atrás de um ponteiro nulo quando as métricas estão desligadas, e a leitura da série
usa `RWMutex` em leitura com chave pré-montada.
