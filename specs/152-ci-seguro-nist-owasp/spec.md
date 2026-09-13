# Spec 152 — CI self-hosted seguro e baseline NIST/OWASP

- **Issue**: [#245](https://github.com/emersonjoe/trilha/issues/245) — a issue é a fonte do escopo.
- **Branch**: `152-ci-seguro-nist-owasp`
- **Versão**: 0.131.0

## Por quê

O CI Linux dependia somente da capacidade hospedada pelo GitHub, enquanto os requisitos de
desenvolvimento seguro estavam espalhados entre implementação e documentação. Reusar a conta
compartilhada do runner da VPS também entregaria acesso ao Docker — e portanto root efetivo —
ao código executado por qualquer repositório.

## O que muda

1. Um runner `eoslab` exclusivo do repositório roda com usuário sem `sudo` e sem `docker`.
2. Somente `push` revisado na `main` e execução manual usam a VPS; pull requests continuam em
   `ubuntu-latest`, e o deploy privilegiado do Pages continua hospedado pelo GitHub.
3. Actions são fixadas por commit, o token recebe permissões mínimas e caches do job são
   efêmeros. `make security` reúne vet, detector de corrida e `govulncheck` sob Go 1.25.13,
   baixado automaticamente pelo Go 1.22+; o script de release executa esse gate.
4. NIST SSDF 1.1, OWASP ASVS 5.0 nível 2 e OWASP Top 10:2025 viram baseline verificável na
   constituição, templates, política e documentação bilíngue, sem alegação de certificação.

## Fora de escopo

- Executar código não revisado de pull request na VPS persistente.
- Dar Docker, sudo ou segredos de produção ao runner.
- Afirmar certificação ou conformidade integral de uma implantação.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | nenhuma dependência entra no runtime ou CLI; o scanner é ferramenta de CI |
| VI — teste primeiro | `make test`, corrida, fuzz e vulnerabilidades continuam gates separados e verificáveis |
| VII — segurança por padrão | menor privilégio, fronteira de PR, pinagem imutável, matriz ASVS e resposta a incidente |
| Idioma | política, baseline e página Learn são atualizadas em inglês e pt-BR |

## Tarefas

- [x] T001 Registrar runner dedicado e provar ausência de grupos privilegiados.
- [x] T002 Endurecer CI e Pages, mantendo PR e deploy privilegiado fora da VPS.
- [x] T003 Criar `make security` e fixar dependências executáveis por versão/commit.
- [x] T004 Atualizar constituição, templates, política, baseline e página Learn nas duas locales.
- [x] T005 Executar gates e revisar o diff; publicar `0.131.0` pela política de release.

## Aceitação

- [x] GitHub reporta `eoslab` online e o serviço usa uma conta exclusiva sem `sudo`/`docker`.
- [x] PRs escolhem `ubuntu-latest`; `main`/manual escolhem `RUNNER_CI` com fallback seguro.
- [x] `make test`, `make security`, testes do site e `git diff --check` passam.
- [x] A baseline exige impacto, controles e evidência e declara que alinhamento não é certificação.
