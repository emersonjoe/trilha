# Tasks: Modelo de ameaças escrito e validação de `Host`

**Input**: [spec.md](./spec.md), [plan.md](./plan.md) · uma rodada de `make test` por bloco.

## Bloco 1 — casamento de host (SC-001, SC-002)

- [ ] T001 Teste que falha em `host_test.go`: lista vazia aceita tudo; `exemplo.com` casa
      `exemplo.com:8443` e `EXEMPLO.com.`; `*.exemplo.com` casa `app.exemplo.com` e recusa
      `exemplo.com` e `a.b.exemplo.com`; `Host` vazio recusa; em `Dev`, `localhost:3000`,
      `127.0.0.1` e `[::1]` passam fora da lista.
- [ ] T002 `host.go`: `Config.AllowedHosts` e a função de casamento (normaliza porta, caixa e
      ponto final; curinga de um rótulo só).

## Bloco 2 — borda (SC-002, SC-003)

- [ ] T003 Teste que falha em `host_test.go`: requisição com `Host` de fora responde 400, não
      chega ao handler nem ao `/_trilha/health`, e emite `SecurityEvent{Kind: "host"}`;
      `TRILHA_ALLOWED_HOSTS=a,b` preenche o campo pelo `ConfigFromEnv`.
- [ ] T004 A conferência como primeira linha de `serveHTTP`, o evento sem `*Ctx` em `events.go`
      e `TRILHA_ALLOWED_HOSTS` no `ConfigFromEnv`.

## Bloco 3 — auditoria (SC-005)

- [ ] T005 Teste em `cmd/trilha/audit_test.go`: projeto sem `AllowedHosts` ganha aviso; com o
      campo no fonte ou com a variável de ambiente, item ok.
- [ ] T006 Item no `runAudit` e as mensagens nas duas línguas do `i18n.go`.

## Bloco 4 — documento e referência (SC-004)

- [ ] T007 `SECURITY-MODEL.md`: ativos, fronteiras, agentes, tabela STRIDE com o controle de
      cada ameaça e a lista do que fica aberto — cada linha conferida contra o código.
- [ ] T008 `docs/pt-BR/SECURITY-MODEL.md` (tradução) e o link nos dois `SECURITY.md`.
- [ ] T009 Site: `Config.AllowedHosts` na referência de segurança e uma seção curta no capítulo
      de segurança, nas duas locales, ligando ao modelo de ameaças.

## Bloco 5 — fechamento

- [ ] T010 `CHANGELOG.md` (0.25.0), `version` em `cmd/trilha/main.go`, ROADMAP (§10 e §20).
- [ ] T011 `make test` verde e `make release VERSION=0.25.0 ISSUES="34"`.
