---
title: Saúde e observabilidade
description: Sondas de vida e prontidão, métricas no formato Prometheus e correlação de log, com o cuidado de não transformar monitoração em vazamento.
---

Um app em produção precisa responder três perguntas para quem o opera: *está de pé?*,
*pode receber tráfego?* e *o que está acontecendo?*. O Trilha responde às três sem
dependência nenhuma, e responde de um jeito que não entrega o mapa da sua infraestrutura
para quem passar na rua.

A referência aqui é dupla, como no capítulo de segurança: o **NIST SP 800-53r5** (AU-2 e
AU-3 para o conteúdo do registro, AU-9 para proteger essa informação, SI-4 para monitoração
e SC-5 contra negação de serviço) e o **OWASP** (Top 10 2021 A09, API Security 2023 API8 e
o capítulo V7 do ASVS).

## As duas sondas

Sem configurar nada, todo app Trilha já responde:

| Endereço | Pergunta | Executa verificações? |
|---|---|---|
| `/_trilha/health/live` | o processo consegue atender? | não |
| `/_trilha/health/ready` | pode receber tráfego? | sim |
| `/_trilha/health` | igual a `ready` | sim |

A separação não é burocracia. No Kubernetes, uma *readiness* que falha tira o pod do
balanceador; uma *liveness* que falha **mata o processo**. Se as duas rodassem a mesma
verificação de banco, uma oscilação de rede reiniciaria a frota inteira em vez de esperar
o banco voltar. Por isso `live` nunca toca em dependência alguma.

```go
// app/setup.go
func Setup(a *trilha.App) error {
	a.Check("banco", func(ctx context.Context) error {
		return db.PingContext(ctx)
	})
	a.Check("fila", func(ctx context.Context) error {
		return fila.Ping(ctx)
	})
	return nil
}
```

Cada verificação roda com prazo (2 s por padrão) e em paralelo; um `panic` dentro dela
vira falha, não derruba o processo. O resultado fica em cache por 1 s: uma sonda por
segundo — ou dez mil por segundo, vindas de alguém mal-intencionado — não viram dez mil
`SELECT 1` no seu banco.

## O que o anônimo vê

Em produção, sem autorização, a resposta é exatamente esta:

```json
{"status":"fail"}
```

Nome da verificação, mensagem de erro, hostname e versão ficam de fora de propósito
(ASVS V7.4.1). Saber que existe um Postgres chamado `financeiro` e que ele está fora do ar
é meio caminho para quem está sondando o alvo. A causa vai para o log, onde já existe
controle de acesso, e para quem se autentica:

```
curl -H "Authorization: Bearer $TRILHA_OBS_TOKEN" https://app/_trilha/health
```

```json
{"status":"fail","checks":[{"name":"banco","status":"fail","duration_ms":2001.4,
 "error":"prazo esgotado: context deadline exceeded"}],"uptime_seconds":8134.2}
```

Em `dev` o detalhe é aberto — lá o alvo é você mesmo.

## Métricas

O endereço de métricas **não existe** até você pedir. Isso é deliberado: um `/metrics`
público é a má configuração descrita no API8 do OWASP, e ele conta ao visitante quantas
rotas você tem, quais têm erro e a que horas o tráfego cai.

```go
func Config(cfg *trilha.Config) {
	cfg.Observability.Metrics = "/_trilha/metrics"   // ou TRILHA_METRICS
	// TRILHA_OBS_TOKEN (32+ bytes) autoriza a raspagem;
	// alternativa: Trusted com o CIDR do coletor.
	cfg.Observability.Trusted = []string{"10.42.0.0/16"}
}
```

A saída é o formato de texto do Prometheus, então Prometheus, VictoriaMetrics, Grafana
Alloy e OpenTelemetry Collector leem sem tradutor:

```
trilha_requests_total{method="GET",route="/blog/{slug}",status="200"} 1841
trilha_request_duration_seconds_bucket{method="GET",route="/blog/{slug}",le="0.05"} 1802
trilha_requests_in_flight 3
trilha_security_events_total{kind="csrf"} 2
trilha_panics_total 0
go_goroutines 14
```

Repare no rótulo `route`: é o **padrão registrado**, `/blog/{slug}`, nunca o caminho
concreto `/blog/como-fiz-x`. Caminho concreto é entrada do usuário — traz identificador,
às vezes traz token na query string, e faz o número de séries crescer sem limite até a
memória acabar. O que não casa com rota registrada (estático, 404) cai num único rótulo
`other`, e cada métrica tem teto de séries (mil por padrão).

Métrica sua entra no mesmo lugar:

```go
posts.Publicados = a.Metrics().Counter("blog_posts_total", "Posts publicados.")
lentidao := a.Metrics().Histogram("blog_render_seconds", "Tempo de render.", nil, "template")
lentidao.With("post").Observe(dur.Seconds())
```

## Achar uma requisição no log

Todo log de requisição já sai com `request_id`, e o mesmo valor volta no cabeçalho
`X-Request-ID`. Quando o cliente manda `traceparent` (W3C Trace Context — é o que um
gateway, um Istio ou um SDK de OpenTelemetry mandam), o `trace_id` entra junto:

```go
func GET(c *trilha.Ctx) error {
	c.Log().Info("consultando fornecedor", "cnpj", cnpj)  // request_id + trace_id
	return c.JSON(200, resp)
}
```

O Trilha **propaga** o contexto e o coloca no log; ele não exporta spans nem amostra
traços. Rastreamento distribuído completo é trabalho de um coletor, e prendê-lo ao core
custaria dezenas de dependências.

O que nunca entra no log, por decisão de projeto: corpo da requisição, cookies, cabeçalho
`Authorization` e query string (ASVS V7.1.1). É lá que segredo viaja.

## Custo

Com o endereço de métricas desligado, a instrumentação não roda: é uma comparação de
ponteiro. Ligada, ela custa **zero alocações** por requisição (duas buscas em mapa com
chave montada na pilha e alguns incrementos atômicos); a diferença de tempo fica dentro
do ruído da máquina de referência. Os números estão em
[Desempenho](/pt/referencia/desempenho).

## Desafio

Faça o `/_trilha/health/ready` do seu app verificar o banco **e** um serviço externo, com
prazo de 500 ms para o externo; exponha as métricas só para a rede `10.0.0.0/8`; e conte,
numa métrica sua, quantas vezes o serviço externo falhou.

:::solucao
```go
// app/setup.go
func Config(cfg *trilha.Config) {
	cfg.Observability.Metrics = "/_trilha/metrics"
	cfg.Observability.Trusted = []string{"10.0.0.0/8"}
}

func Setup(a *trilha.App) error {
	falhas := a.Metrics().Counter("integracao_falhas_total", "Falhas na consulta ao parceiro.", "servico")

	a.Check("banco", func(ctx context.Context) error { return db.PingContext(ctx) })

	a.Check("parceiro", func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		defer cancel()
		if err := parceiro.Ping(ctx); err != nil {
			falhas.With("parceiro").Inc()
			return err
		}
		return nil
	})
	return nil
}
```

O prazo curto do parceiro convive com o prazo geral (`Observability.Timeout`): vale o que
vencer primeiro. E como o contador é criado no `Setup`, ele aparece na raspagem desde a
primeira requisição, com valor zero — o que é melhor do que sumir do painel até a primeira
falha.
:::
