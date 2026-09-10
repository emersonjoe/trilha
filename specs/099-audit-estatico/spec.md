# Spec 099 — o `audit` cresce com o que dá para ver sem rodar

- **Issue**: [#118](https://github.com/emersonjoe/trilha/issues/118) — a issue é a fonte do escopo.
- **Branch**: `099-audit-estatico`
- **Versão**: 0.79.0

## Por quê

A 0.78.0 fez o `trilha dev` contar o que só o navegador vê. Sobraram na tabela da issue as linhas
que **não precisam de navegador nenhum**: dá para ver no código, antes de subir, e por isso são do
`trilha audit` — o comando que a CI já roda.

Três delas, e as três têm a mesma cara: funcionam. Ninguém erra, ninguém vê erro, e o problema só
aparece quando alguém de fora percebe antes de você.

## O que muda

O `audit` ganha três verificações:

- **fluxo de eventos em rota sem middleware.** Uma rota que abre `c.Stream()` e não tem nenhum
  middleware acima manda tudo para quem conectar. É a linha *"`ui.Live` numa rota sem `Require`:
  todo mundo recebe tudo"*;
- **`c.Audit` em rota sem middleware.** A trilha registra o ator anônimo — a linha existe, e não
  diz quem fez. Uma trilha que não nomeia ninguém é um arquivo de log caro;
- **`string` que vem de fora sem `max=`.** Um campo com `form:`/`json:` e `validate:` sem limite
  de tamanho aceita o que couber no corpo. O banco recusa em produção, e a mensagem que a pessoa
  vê é a do driver.

Cada uma imprime as rotas (ou os campos) que a disparam, porque um aviso que não diz onde é um
aviso que ninguém age.

## Fora de escopo

- **Decidir se o middleware que está lá é de autenticação.** O `audit` vê que há uma cadeia, não o
  que ela faz; dizer o contrário seria adivinhar. A regra impressa é a que ele aplica: *nenhum
  middleware acima*.
- **`oneof` e `len` como limites.** Já são limites, e o aviso os respeita — mas não se tenta
  inferir tamanho de um `email` ou de um `uuid`.
- **A pasta `docs/errors/` gerada e o cenário do bench.** Continuam na issue.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | expressão regular e o `scan`, como o resto do `audit` |
| VI — teste primeiro | um projeto sintético por verificação, com o caso que deve calar |
| Uma fonte | as mensagens entram no `msgs` en + pt, como as outras |

## Tarefas

- [x] T001 Teste que falha: as três verificações, com o caso que dispara e o que não
- [x] T002 As verificações no `runAudit`, com as rotas e os campos no aviso
- [x] T003 Mensagens en + pt
- [x] T004 Documentação (en + pt)
- [x] T005 `CHANGELOG.md`, `version`, `ROADMAP.md`, `make test`, `scripts/release.sh 0.79.0`

## Aceitação

- **SC-001** Rota com `c.Stream()` e sem middleware vira aviso, com o padrão da rota.
- **SC-002** A mesma rota com um middleware acima não vira aviso.
- **SC-003** Rota com `c.Audit` e sem middleware vira aviso.
- **SC-004** `Nome string` com `form:` e `validate:"required"` vira aviso; com `max=80`,
  `oneof=` ou `len=`, não.
- **SC-005** Um projeto que não faz nada disso não ganha ruído nenhum.
