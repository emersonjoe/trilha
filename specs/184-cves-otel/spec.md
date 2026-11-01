# Spec 184 — As dependências do módulo otel sem vulnerabilidade conhecida

**Feature Branch**: `184-cves-otel` | **Created**: 2026-11-01 | **Status**: Entregue (0.163.0)

- **Issue**: #321 — a fonte do escopo.
- **Branch**: `184-cves-otel`
- **Versão**: 0.163.0

## Por quê

O módulo opcional `otel/` fixava o SDK do OpenTelemetry 1.28.0 e, por ele, o gRPC 1.64.0 e o
`x/net` 0.26.0, com avisos de severidade crítica e alta no OSV. Ninguém viu porque o
`make security` roda o `govulncheck ./...` na raiz, e o `./...` não entra em módulo aninhado.
Quem instala o exportador recebia as falhas junto.

## Jornada e risco

Jornada: quem escreve o app liga o `otel.Install` e vê os traces no Grafana/Tempo. Pior
impacto da troca de SDK: os spans somem em silêncio. O SDK novo passou a usar o caminho da
URL como está, e um `Endpoint: "http://coletor:4318"` postaria em `/`. Além disso, o resource
padrão traz outro schema de semconv, e o `service.name` cairia para `unknown_service`. Os dois
foram pegos pelos testes do módulo e corrigidos aqui.

## O que muda

- `otel/`: OpenTelemetry 1.45.0, gRPC 1.83.2, `x/net` 0.62.0, `x/text` 0.43.0, `x/sys` 0.49.0.
  O piso do módulo sobe de Go 1.22 para **Go 1.26**: as correções do `x/net` só existem
  com ele. O núcleo continua em Go 1.22.
- `Options.Endpoint` sem caminho (ou com `/`) envia para `/v1/traces`; com caminho, usa o
  caminho escrito. É o contrato que a documentação já prometia.
- O resource do serviço é montado sem schema, e o merge com o resource padrão não falha mais
  a cada troca de SDK.
- `sqltest/`: `x/sys` 0.44.0 (piso Go 1.25; o job do CI já usa 1.25).
- `make security` também varre o `otel/`; o CI roda o `otel/` num job próprio em Go 1.26.

## Fora de escopo

- Publicar o `otel/` como módulo com tag própria (`otel/vX.Y.Z`): ele ainda depende do núcleo
  por `replace`, o que é outra discussão.
- Subir o SDK para a 1.47: a 1.45.0 já corrige, e mudar menos reduz o risco.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | O núcleo não muda de dependência; o SDK continua isolado no módulo opcional. |
| VI — teste primeiro | A tabela de caminhos do `TestOTLPExportReachesTheCollector` e o `service.name` do `TestSpanCarriesRouteStatusAndTenant` reprovaram com o SDK novo antes das correções. O govulncheck do `otel/` reprova na `main` (exit 3) e passa aqui. |
| VII — segurança por padrão | Remove as dependências vulneráveis e fecha o ponto cego do gate. |

## Segurança e privacidade

- Ativos e fronteiras de confiança afetados: o cliente OTLP/HTTP do app até o coletor (saída
  do processo) e as bibliotecas HTTP/2 e gRPC que ele carrega.
- Controles OWASP ASVS 5.0 nível 2 aplicáveis: V15.2 (componentes sem vulnerabilidade
  conhecida, inventário de dependências).
- Riscos OWASP Top 10:2025 considerados: A03 — falhas na cadeia de suprimentos de software.
- Segredos, dados pessoais, logs, limites e exceções: nenhum dado novo sai do processo; os
  atributos do span não mudam. Exceção registrada: o piso do `otel/` passa a ser Go 1.26.

## Tarefas

- [x] T001 Teste que falha: tabela de caminhos do `Endpoint` (sem caminho, `/`, caminho
  próprio) e o `service.name` com o SDK novo
- [x] T002 Atualização das dependências; caminho padrão e resource sem schema em `otel.go`
- [x] T003 `make security` varre o `otel/`; job `otel` em Go 1.26 no CI
- [x] T004 Documentação nas duas locales (`learn/observability`, `aprender/observabilidade`)
- [x] T005 `CHANGELOG.md`, `version` em `cmd/trilha/main.go`, item do `ROADMAP.md`
- [x] T006 Suíte inteira e os números em *Evidências*

## Aceitação

- **SC-001** `govulncheck` no `otel/`, com o toolchain do gate (`go1.26.10`): "No
  vulnerabilities found".
- **SC-002** Um `Endpoint: "http://coletor:4318"` entrega o span em `/v1/traces`; um
  `Endpoint` com caminho entrega no caminho dele.
- **SC-003** O span chega com `service.name` igual ao `Options.Service`.

## Evidências

| Nível | Comando | Passaram | Falharam | Pularam |
|---|---|---|---|---|
| Módulo otel | `make test-otel` (go1.26.10) | 10 | 0 | 0 |
| Unidade + integração | `make test` | 1646 | 0 | 2 |
| Stores SQL | `make test-sql` (SQLite) | ok | 0 | — |
| Segurança | `make security` (race + govulncheck na raiz e no `otel/`) | ok, "No vulnerabilities found" ×2 | 0 | — |

Os dois pulados são `TestS3AoVivo` e `TestOIDCAoVivo`, que dependem de serviço externo.

Navegador não se aplica: nada no HTML servido mudou.
