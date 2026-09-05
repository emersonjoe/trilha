# Feature Specification: Health check, observabilidade e métricas por padrão

**Feature Branch**: `014-observabilidade` | **Created**: 2026-09-05 | **Status**: Implementada (v0.6.0)
**Input**: "implement modulo de health check e observabilidade e metricas default - usando
nist e owasp"

## Contexto e referências

Um framework que já entrega segurança por padrão (princípio VII) precisa entregar também a
capacidade de **saber que está de pé e o que está acontecendo**. As duas famílias de norma
citadas dizem coisas diferentes e complementares:

| Fonte | O que exige | Como esta feature responde |
|---|---|---|
| NIST SP 800-53r5 **AU-2/AU-3** | registrar eventos com quem, quando, onde, o quê e resultado | log de requisição já traz método, rota, status, duração, IP, `request_id`; ganha `trace_id` (W3C Trace Context) |
| NIST SP 800-53r5 **AU-8** | marca de tempo confiável | logs em UTC, duração medida por relógio monotônico |
| NIST SP 800-53r5 **AU-9** | proteger a informação de auditoria contra acesso não autorizado | `/metrics` e o detalhe do health exigem token ou rede confiável |
| NIST SP 800-53r5 **SI-4** | monitoração do sistema | métricas de requisição, erros, eventos de segurança e runtime |
| NIST SP 800-53r5 **SC-5** | proteção contra negação de serviço | health com cache e prazo por verificação; teto de cardinalidade nas métricas |
| NIST SP 800-92 | logs úteis e não redundantes | sondas de saúde saem em `Debug`, não afogam o registro de auditoria |
| OWASP Top 10 2021 **A09** | falha de registro e monitoração | métricas de `4xx`, `5xx`, pânico e eventos de segurança prontas para alarme |
| OWASP API Security 2023 **API8** | má configuração, endpoint de monitoração exposto | `/metrics` **desligado por padrão**; ligado, é autenticado |
| OWASP ASVS **V7.1.1** | não registrar credencial nem segredo | nada de corpo, cookie, `Authorization` ou query string nos logs e nas métricas |
| OWASP ASVS **V7.4.1** | erro genérico para fora, detalhe no log | health público responde só `pass`/`fail`; a causa vai para o log e para a visão autenticada |

## User Scenarios & Testing

### US1 - Sonda de vida e de prontidão (P1)
Como quem opera o app em contêiner, quero dois endereços estáveis para o orquestrador:
`/_trilha/health/live` responde 200 enquanto o processo consegue atender, e
`/_trilha/health/ready` responde 200 só quando todas as verificações registradas passam,
503 quando alguma falha.
**Acceptance**: com uma verificação que falha, `ready` responde 503 e `live` continua 200;
o corpo segue `application/health+json` com `{"status":"pass"|"fail"}`; a resposta tem
`Cache-Control: no-store` e `X-Robots-Tag: noindex`.

### US2 - Detalhe só para quem pode ver (P1)
Como responsável pela segurança, não quero que um visitante anônimo descubra que o app tem
um Postgres chamado `financeiro` que está fora do ar.
**Acceptance**: em produção, sem token, o corpo não contém nome de verificação, mensagem de
erro, versão nem hostname. Com `Authorization: Bearer <TRILHA_OBS_TOKEN>` (comparação em
tempo constante) ou vindo de uma rede em `Observability.Trusted`, o corpo traz `checks` com
nome, estado, duração e o erro. Em `dev` o detalhe é aberto.

### US3 - Métricas no formato que as ferramentas leem (P1)
Como quem já tem Prometheus, quero raspar o app sem instalar biblioteca no meu projeto.
**Acceptance**: com `Observability.Metrics = "/_trilha/metrics"`, o endereço responde no
formato de exposição de texto do Prometheus (`text/plain; version=0.0.4`) com
`trilha_requests_total`, `trilha_request_duration_seconds` (histograma), `trilha_requests_in_flight`,
`trilha_security_events_total`, `trilha_panics_total`, métricas de runtime do Go e
`trilha_build_info`. Sem configurar nada, o endereço **não existe** (404).

### US4 - Métrica da aplicação sem dependência (P2)
Como pessoa desenvolvedora, quero contar coisas do meu domínio.
**Acceptance**: `a.Metrics().Counter("blog_posts_total", "ajuda").Inc()` e as variantes
`Gauge` e `Histogram` aparecem na raspagem; nomes inválidos causam pânico no `Setup` (erro
de programação, não de runtime), nunca corrompem a saída.

### US5 - Correlação de requisição (P2)
Como quem investiga um incidente, quero achar todas as linhas de uma mesma requisição.
**Acceptance**: `c.Log()` devolve um `*slog.Logger` já com `request_id` e, quando o cliente
mandou `traceparent`, com `trace_id`; o mesmo `trace_id` aparece no log de requisição e o
`X-Request-ID` volta no cabeçalho.

### US6 - Auditoria avisa sobre má configuração (P2)
**Acceptance**: `trilha audit` marca como crítico métricas expostas sem token e sem rede
confiável em produção, e como aviso o detalhe do health forçado para todos.

## Requirements

- **FR-001** Nenhuma dependência externa; apenas biblioteca padrão (princípio II).
- **FR-002** Desligado é o padrão seguro: métricas só existem quando configuradas; o detalhe
  do health exige autorização fora de `dev`.
- **FR-003** Rótulo de métrica nunca carrega dado do usuário: a dimensão de rota é o *padrão*
  registrado (`/blog/{slug}`), nunca o caminho concreto; o que não casa com uma rota vira
  `other`. Teto de séries por métrica (padrão 1000), excedente vai para `other` e emite um
  aviso no log, uma vez.
- **FR-004** Verificação de prontidão tem prazo (padrão 2 s) e resultado em cache (padrão 1 s),
  para que a sonda não vire vetor de amplificação contra o banco.
- **FR-005** As respostas de observabilidade não entram em cache, não são indexadas e não
  contam para o limitador global (uma sonda de vida não pode receber 429).
- **FR-006** As sondas saem no log em nível `Debug`; requisições normais continuam em `Info`.
- **FR-007** O custo por requisição com métricas ligadas fica abaixo de 1 µs e 1 alocação;
  com métricas desligadas, zero (medido em `bench/`, coerente com a spec 012).
- **FR-008** Toda a superfície nova é documentada em pt-BR (capítulo + referência) e usada em
  `examples/blog`.

## Fora de escopo

- Rastreamento distribuído completo (exportador OTLP, amostragem): o Trilha só **propaga** o
  contexto W3C e o coloca no log. Um adaptador OpenTelemetry pode virar módulo separado.
- Alertas, painéis e retenção: são do Prometheus/Grafana de quem opera.
- Autenticação forte no endpoint (mTLS, OIDC): quem precisa disso põe o app atrás do proxy e
  usa `Trusted`.
