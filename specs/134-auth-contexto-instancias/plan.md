# Plano — spec 134

## Onde cada coisa entra

| Arquivo | O que muda |
|---|---|
| `auth/session.go` | `SessionListerContext`; `listSessions(ctx, subject)` como único ponto de escolha; `Sessions` e `LogoutOthers` por ele; `User.Audience`; `write` carimba o público; `Session` recusa sessão de outro público |
| `auth/flow.go` | `Options.Audience` |
| `auth/middleware.go` | `a.ctxKey()`; `remember` vira método e grava o slot da instância e o compartilhado; `User` confere o dono do que achou |
| `auth/tenant.go` | comentário: o slot compartilhado é o da sessão por onde a requisição passou |
| `auth/apikeys.go` | nada de comportamento; o `Keys` continua no slot compartilhado, com o porquê escrito |
| `auth/local_test.go` | `TestSessionsContextLevaOPrazoEOErro` (SC-001 a SC-003) e `TestDoisPublicosNoMesmoProcesso` (SC-004, SC-005), ao lado do `localApp` e do `newBrowser` que os dois usam |
| `api/current.txt` | regravado por `make api` |
| `site/internal/docs/content/en/reference/auth.md`, `.../pt/referencia/auth.md` | `SessionListerContext` na seção da lista de sessões; `Audience` em Options e uma subseção "Two publics in one process" |
| `CHANGELOG.md`, `cmd/trilha/main.go`, `ROADMAP.md` | a 0.113.0 |

## Ordem

A #176 primeiro: ela é uma interface nova e um helper, sem efeito sobre o resto do pacote, e
serve de referência de forma para a segunda. Depois a #180, que toca o caminho por onde passa
toda requisição autenticada — e por isso entra com o teste da travessia já escrito e com a
suíte inteira do `auth` rodando atrás.

`examples/blog` não implementa `SessionLister` nem cria dois `Auth` (o exemplo tem um público
só), então não há rota nova a registrar nem `trilha_gen.go` a regerar; o teste de integração
das duas mudanças é a suíte do pacote `auth`, que sobe um `App` de verdade com `httptest`.

## Riscos

1. **Sessão existente recusada.** Só acontece para quem liga o `Audience` — o campo é novo e o
   zero vale o comportamento de hoje. Fica no `CHANGELOG` na entrada da opção, não escondido.
2. **Um `Auth` com `Audience` deixa de enxergar o chamador por chave de API no `User(c)`.** É o
   resultado certo (a chave não é sessão daquele público) e é o que a decisão 5 da spec registra;
   `Keys.User(c)` não muda.
3. **`auth.Tenant(c)` com dois `Auth` na mesma requisição** responde pelo último `remember` que
   rodou. Na prática só um guarda roda por rota; o doc comment passa a dizer isso em vez de
   deixar a pergunta aberta.
