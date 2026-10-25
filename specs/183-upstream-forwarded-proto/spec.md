# Spec 183 — O upstream recebe o esquema que o app vê

**Feature Branch**: `183-upstream-forwarded-proto` | **Created**: 2026-10-26 | **Status**: Entregue (0.162.1)

- **Issue**: #316 — a fonte do escopo.
- **Branch**: `183-upstream-forwarded-proto`
- **Versão**: 0.162.1

## Por quê

Atrás de um proxy que termina o TLS, o `Config.Upstreams` mandava sempre
`X-Forwarded-Proto: http`, porque o `SetXForwarded` da biblioteca padrão só olha `in.TLS`. O
app sabia que a requisição era https (`isSecure`, com `TrustedProxies`); o upstream não.

## Jornada e risco

Jornada de quem põe uma API existente atrás do app (`Config.Upstreams`). Pior impacto: o
upstream decidir `Secure` de cookie, URL absoluta ou redirect como se fosse HTTP. Rebaixamento
fixo, não controlado por quem faz a requisição; coberto por teste de integração (app +
upstream de verdade em `httptest`).

## O que muda

O `X-Forwarded-Proto` que sai para o upstream é `https` quando o app vê https: TLS direto, ou
proxy confiável que mandou `X-Forwarded-Proto: https`. Cliente não confiável continua sem ser
ouvido.

## Fora de escopo

`X-Forwarded-Host` continua sendo o `Host` recebido; `X-Forwarded-For` continua o da biblioteca
padrão (acrescenta o peer à lista recebida, e cabe ao upstream confiar só no fim dela).

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | Nada novo. |
| VI — teste primeiro | `TestUpstreamForwardsTheSchemeTheAppSees` reprovou antes da correção. |
| VII — segurança por padrão | O cabeçalho do cliente só vale vindo de `TrustedProxies`, o mesmo critério de `isSecure`. |

## Segurança e privacidade

- Ativos e fronteiras: app → upstream (o esquema da requisição).
- OWASP ASVS 5.0 nível 2: V3.4 (atributos de cookie), V13 (configuração de proxy confiável).
- OWASP Top 10:2025: A02 (configuração), A04 (falha criptográfica por rebaixamento de esquema).
- Segredos, logs, limites: nenhum valor novo sai; o cabeçalho de cliente não confiável segue
  ignorado.

## Tarefas

- [x] T001 Teste que falha: proxy confiável com `https`, cliente não confiável com `https`,
  TLS direto, proxy confiável em `http`.
- [x] T002 Implementação no `Rewrite` de `upstream.go`.
- [x] T003 Referência de upstreams nas duas locales.
- [x] T004 `CHANGELOG.md`, `version`, `ROADMAP.md`.
- [x] T005 Suíte inteira e os números em *Evidências*.

## Aceitação

- **SC-001** Atrás de um proxy confiável que terminou o TLS, o upstream recebe `https`.
- **SC-002** Um cliente direto que manda `X-Forwarded-Proto: https` sem TLS não muda nada: o
  upstream recebe `http`.

## Evidências

| Nível | Comando | Passaram | Falharam | Pularam |
|---|---|---|---|---|
| Unidade + integração (testes e subtestes) | `make test` | 1646 | 0 | 2 |
| Segurança | `make security` (race + `govulncheck` em Go 1.26.10) | verde | 0 | — |

Sem cenário de navegador: nada que a página mostra muda.
