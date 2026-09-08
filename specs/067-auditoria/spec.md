# Spec 067 — A trilha de auditoria

- **Issue**: #104 — a issue é a fonte do escopo; aponte para ela, não a reescreva aqui.
- **Branch**: `067-auditoria`
- **Versão**: 0.49.0

## Por quê

Toda aplicação interna acaba precisando de "quem fez o quê". O framework já tem metade do dado
— request id, IP atrás dos proxies confiáveis, sessão, gabarito da rota. Falta a frase de
negócio e um lugar para ela.

O que o iniciante faz hoje é `slog.Info("excluiu", "id", id)` e perde o ator, o IP e o request
id; ou cria uma tabela e esquece de gravar em metade dos handlers. As duas falhas são a mesma:
a informação estava ali e ninguém a juntou.

## O que muda

`c.Audit(ação, alvo, campos...)` em uma linha, e o resto vem da requisição. O contrato está na
referência de observabilidade, nas duas línguas. O que vale registrar:

**Sink que falha não derruba a resposta.** É a decisão mais importante aqui, e a issue já
chegou com ela. O documento foi excluído de qualquer jeito; recusar-se a responder agora
perderia a trilha **e** confundiria quem fez — duas falhas em vez de uma. O erro vai alto para
o log.

**O `auth` marca o ator, não a aplicação.** Havia cinco `c.Set(ctxKey, u)` espalhados pelo
pacote; viraram um `remember`, que marca sessão e ator juntos. Cinco lugares fazendo duas
coisas é onde a quinta esquece uma delas — e foi assim que este refactor achou o próprio bug:
o regex que fez a troca substituiu também a chamada dentro do `remember`, e a recursão
infinita apareceu na primeira execução da suíte.

**Anônimo é registrado, não descartado.** Trilha que some com a ação anônima tem um buraco
exatamente onde alguém vai procurar.

**`Route` é o gabarito, não o caminho.** O id concreto já está no `Target`; o gabarito é o que
deixa uma consulta agrupar mil exclusões numa linha.

## Fora de escopo

- **`audit.SQL(db, tabela)`** — a issue pede DDL para SQLite e Postgres. O framework não tem
  nenhuma dependência de banco hoje, e passar a embarcar DDL de dois dialetos é uma decisão
  maior que esta spec; a interface de um método existe justamente para a aplicação escrever
  seu `INSERT`, e o exemplo mostra um.
- **`ui.AuditTable`** — depende do `c.CSV` da [#109](https://github.com/emersonjoe/trilha/issues/109),
  que não existe. Uma tabela de auditoria sem o botão que a issue descreve seria metade da
  peça, e a metade que sobra é um `DataTable` comum.
- **`trilha ctx` listando as ações auditadas** — mesma dívida de scanner da
  [#124](https://github.com/emersonjoe/trilha/issues/124), e ela vai junto.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `time` e `log/slog`. Nenhum banco entra. |
| IV — superfície pequena | Uma interface de um método; nada de tabela nem de dialeto. |
| VI — teste primeiro | O preenchimento automático, o anônimo, o sink que falha, o log sem sink e o ator vindo da sessão. |
| VII — segurança por padrão | O `trilha audit` avisa quando há `c.Audit` num projeto sem nenhuma rota guardada. |

## Tarefas

- [x] T001 `audit.go`: `Fields`, `Actor`, `AuditRecord`, `AuditSink`, `AuditFunc`, `c.Audit`,
      `c.SetActor`, `c.Actor` e `Config.Audit`.
- [x] T002 `auth`: um `remember` marca sessão e ator, no lugar de cinco `c.Set`.
- [x] T003 Testes do núcleo e do laço com a sessão.
- [x] T004 `trilha audit`: aviso de `Audit` sem rota guardada.
- [x] T005 `examples/local-login` grava a mudança de permissão, com teste.
- [x] T006 Referência de observabilidade nas duas línguas; receita sincronizada.
- [ ] T007 `CHANGELOG.md`, `version`, `make test` verde e release.

## Aceitação

- **SC-001** Uma linha na aplicação rende ator, IP, request id e gabarito da rota.
- **SC-002** Sem sessão, o registro sai com `via: anonymous` — e sai.
- **SC-003** Sink que devolve erro deixa a resposta intacta e o erro no log.
- **SC-004** Sem `Config.Audit`, o registro vai para o log com `kind=audit`.
