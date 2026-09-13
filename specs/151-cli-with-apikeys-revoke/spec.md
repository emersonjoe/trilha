# Spec 151 — trilha new --with e o Revoke do apikeys

- **Issues**: #212, #216 — a issue é a fonte do escopo; aponte para ela, não a reescreva aqui.
- **Branch**: `151-cli-with-apikeys-revoke`
- **Versão**: 0.130.0

## Por quê

`trilha new --template app` escreve `app/layout.go` e `app/middleware.go` importando
`internal/sessao`, que só a receita `login` grava. `--with` substitui a lista inteira de
receitas do template, então `--with ""` ou qualquer `--with` sem `login` produz um projeto
que não compila — e `--with` com um nome de receita que não existe é aceito calado, o que faz
o erro de digitação passar por comando bem-sucedido (#212).

Separadamente, o botão "Revoke" que `ui.APIKeysTable` desenha para a receita `api-keys` só
leva os campos `id` e o CSRF; a receita despacha o único POST da tela pelo campo `action`, e
sem `action=revoke` o clique de verdade cai no ramo que cria uma chave, sem nome — 422. Um
teste que já manda `action` à mão não vê isso, porque forja exatamente o campo que falta
(#216).

## O que muda

- `internal/scaffold`: `Needs(tmpl string) []string` diz que receitas do módulo `recipes` um
  template exige para compilar — `"app"` responde `["login"]`, `"blog"` responde `nil`.
- `cmd/trilha new`: antes de escrever qualquer arquivo, resolve a lista de receitas a partir
  de `--with` (ou do padrão do template), inclui as de `scaffold.Needs` que não estiverem lá
  — avisando em stderr — e recusa qualquer nome que `recipes.Get` não reconheça, com a lista
  dos nomes válidos na mensagem. Um `--with` com nome desconhecido para antes de tocar o
  disco.
- `ui.APIKeysTable`: o formulário do botão "Revoke" passa a levar
  `<input type="hidden" name="action" value="revoke">`, no mesmo padrão que
  `ui/webhooks.go` já usa para `subscribe`/`revoke`/`retry`/`ping`. A receita `api-keys`
  continua despachando por `action`; o widget é quem estava incompleto.

```go
// depois de --with "" ou --with audit num --template app: login entra sozinho,
// com um aviso, e o projeto compila.
$ trilha new demo --template app --with "" 2>&1 | grep template
template "app" needs recipe "login"; adding it automatically

// nome desconhecido para antes de escrever
$ trilha new demo --with bogus
error: unknown recipe "bogus"; valid recipes: api-keys, approvals, audit, blob, ...
```

## Fora de escopo

- Um `--without` para tirar uma receita do padrão do template sem reescrever a lista inteira:
  a issue #212 aceita a opção mais simples (forçar `Needs`) e não pede o flag novo.
- Mover o despacho da receita `api-keys` de `action` para `id != ""`: a issue já diz que o
  widget é o lado mais robusto de corrigir, e é o lado que este spec escolhe.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | Nenhuma dependência nova; só `flag`, `slices`, `strings` já em uso. |
| VI — teste primeiro | Teste do scaffold (`Needs`), e2e da CLI (`--with` força `login` e recusa nome desconhecido) e teste de integração da receita `api-keys` escritos antes da implementação. |
| VII — segurança por padrão | Não altera CSRF, escape ou limites; o campo `action` novo é um `hidden` como os demais do formulário. |

## Tarefas

- [x] T001 Teste que falha: `internal/scaffold` `TestNeeds` (app → login, blog → nada)
- [x] T002 Teste que falha: e2e `cmd/trilha` — `--with ""` num `--template app` inclui login e
      `go vet ./...` passa; `--with` com nome desconhecido recusa antes de escrever, com a
      lista de nomes válidos
- [x] T003 Teste que falha: `ui.APIKeysTable` — o formulário de revogar leva `action=revoke`
- [x] T004 Teste que falha: e2e `cmd/trilha` (`TestAddE2E`) — o formulário que a tela de
      verdade desenha (extraído da própria resposta HTTP, não forjado) revoga e não cria uma
      segunda chave
- [x] T005 Implementação: `scaffold.Needs`, `cmd/trilha/new.go` (força `Needs`, valida nomes
      antes de escrever), `ui/apikeys.go` (`action=revoke`)
- [x] T006 `reference/cli.md` (en + pt), `CHANGELOG.md`, `version`, `ROADMAP.md`
- [x] T007 `make test` verde

## Aceitação

- **SC-001**: `trilha new x --template app --with ""` produz um projeto onde `go vet ./...`
  passa, sem edição manual.
- **SC-002**: `trilha new x --with bogus` sai com erro antes de escrever qualquer arquivo, e a
  mensagem lista os nomes de receita válidos.
- **SC-003**: um teste que posta exatamente os campos que `ui.APIKeysTable` desenhou para o
  botão Revoke revoga a chave e não cria uma segunda.
