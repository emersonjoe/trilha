# Spec NNN — <nome>

> **Quando usar esta forma.** Mudança pequena: um pacote, sem convenção nova em `app/`, sem
> mudança incompatível na API pública, com plano que cabe em uma tela. Se a mudança inventa
> uma convenção, mexe em mais de um pacote ou precisa de justificativa em *Complexity
> Tracking*, use a forma completa (`spec.md` + `plan.md` + `tasks.md`).

- **Issue**: #NN — a issue é a fonte do escopo; **aponte para ela, não a reescreva** aqui.
- **Branch**: `NNN-nome`
- **Versão**: X.Y.Z

## Por quê

Um a três parágrafos: qual problema de quem escreve o app isto resolve, e o que a pessoa faz
hoje sem isto. Sem solução ainda.

## O que muda

O contrato, do jeito que a documentação vai contar: símbolos novos ou alterados com
assinatura, cabeçalhos, arquivos gerados, comportamento em caso de erro. Nada de detalhe de
implementação que não apareça de fora.

```go
// exemplo do uso final
```

## Fora de escopo

O que alguém vai perguntar "e isto?" e a resposta é "não agora" — com o motivo em meia linha.

## Constitution Check

Só os princípios que esta mudança toca (os demais não se aplicam; se todos se aplicam, a
mudança não é pequena e pede a forma completa).

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | |
| VI — teste primeiro | |
| VII — segurança por padrão | |

## Tarefas

Teste antes do código, em ordem de execução. Uma rodada de `make test` por bloco, não por
arquivo.

- [ ] T001 Teste que falha: <o que ele prova>
- [ ] T002 Implementação
- [ ] T003 Uso em `examples/<app>`
- [ ] T004 Documentação nas duas locales (`en/` e `pt/`) + referência
- [ ] T005 `CHANGELOG.md`, `version` em `cmd/trilha/main.go`, item do `ROADMAP.md`
- [ ] T006 `make test` verde e `scripts/release.sh X.Y.Z --issues "NN"`

## Aceitação

O que precisa ser verdade para fechar, verificável por teste ou por comando:

- **SC-001**
- **SC-002**
