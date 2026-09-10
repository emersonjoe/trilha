# Spec 106 — a receita `blob`: guardar arquivo sem inventar caminho

- **Issue**: [#116](https://github.com/emersonjoe/trilha/issues/116) — a issue é a fonte do escopo.
- **Branch**: `106-receita-blob`
- **Versão**: 0.86.0

## Por quê

Guardar arquivo que alguém mandou é a funcionalidade que mais parece simples e mais tem armadilha:
o nome que veio do cliente virando caminho, o mesmo arquivo ocupando espaço três vezes, o
`Content-Type` que o navegador executa, e a listagem que aponta para uma chave que já não existe.

O módulo `blob` resolve os quatro desde a Fase 6. A receita é o caminho até ele.

## O que muda

```
$ trilha add blob
  + internal/arquivos/arquivos.go     o store, e a tabela do que foi guardado
  + internal/arquivos/arquivos_test.go
  + app/arquivos/page.go              enviar, listar
  + app/arquivos/chave_/route.go      servir o que foi guardado
  + arquivos_test.go
  ~ app/setup.go                      trilha.Provide(a, arquivos.Novo())
```

O que a receita escreve, e por que:

- **A chave é o digest, e o nome do cliente nunca vira caminho.** É o módulo quem faz, e o
  arquivo diz isso onde o `Put` é chamado — para ninguém "melhorar" depois usando o nome.
- **Servir é `Files.Serve`**, com `Content-Disposition` e o tipo que o store guardou: um HTML que
  alguém mandou não é servido como HTML executável no seu domínio.
- **As regras do upload são declaradas** — tamanho e tipos — porque um upload sem limite é um
  disco cheio esperando o dia.
- **A tabela do que foi guardado é do app.** O blob guarda bytes; quem sabe que aquele arquivo é
  o anexo do pedido 12 é o app, e é essa tabela que a receita escreve junto.

## Fora de escopo

- **S3 e disco.** `blob.FromEnv()` decide por variável de ambiente; a receita escreve memória e diz
  a linha, como todas.
- **A varredura de órfãos.** `Files.Orphans` existe e é uma rotina, não uma tela — pertence à
  receita `tasks` de quem precisa dela.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | o módulo `blob` e o kit |
| VI — teste primeiro | a receita escreve dois testes, e o e2e da CI aplica ela |
| VII — segurança por padrão | chave por digest, tipo do store, limites declarados |

## Tarefas

- [x] T001 Teste que falha: `add blob` escreve os cinco e liga o `Provide`
- [x] T002 A receita: store, tela, rota que serve, testes
- [x] T003 O e2e da CI aplicando `blob` junto com as outras
- [x] T004 Documentação (en + pt)
- [x] T005 `CHANGELOG.md`, `version`, `ROADMAP.md`, `make test`, `scripts/release.sh 0.86.0`

## Aceitação

- **SC-001** `trilha add blob` num projeto novo passa `trilha check` sem uma edição.
- **SC-002** Um upload aparece na lista e é servido pela chave.
- **SC-003** O mesmo arquivo mandado duas vezes ocupa espaço uma vez.
- **SC-004** Um arquivo maior que o limite é recusado com a mensagem no campo.
