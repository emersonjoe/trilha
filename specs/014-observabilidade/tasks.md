# Tasks: 014-observabilidade

- [ ] T001 Testes primeiro: `health_test.go`, `metrics_test.go`, `observability_test.go`
      (sonda, 503, detalhe negado/autorizado, formato de texto, teto de cardinalidade)
- [ ] T002 `metrics.go`: registro, contador/medidor/histograma com rótulos, teto, formato
      de exposição, coletores de runtime e `trilha_build_info`
- [ ] T003 `health.go`: `Check`, corredor com prazo e cache, `HealthReport`, resposta
      `application/health+json`
- [ ] T004 `observability.go`: `Observability` no `Config`, registro dos endpoints, porteiro
      (token em tempo constante, CIDRs), cabeçalhos, `TRILHA_OBS_TOKEN` em `ConfigFromEnv`
- [ ] T005 `trace.go` + `serve.go` + `events.go`: `traceparent`, `Ctx.TraceID`, `Ctx.Log`,
      instrumentação das requisições, sondas em `Debug`, evento de segurança contado
- [ ] T006 `cmd/trilha/audit.go`: métricas sem proteção (crítico), detalhe aberto (aviso)
- [ ] T007 `examples/blog`: verificação registrada, métrica de domínio, testes de integração
- [ ] T008 `bench/`: custo com e sem métricas; registrar em `bench/RESULTS.md`
- [ ] T009 Documentação: capítulo "Saúde e observabilidade", referência, README, CHANGELOG,
      versão 0.6.0
