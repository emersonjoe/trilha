# Spec 117 — Uso por chave: quem chamou, onde, quando parou

- **Issue**: [#151](https://github.com/emersonjoe/trilha/issues/151) — a issue é a fonte do escopo.
- **Branch**: `117-key-usage`
- **Versão**: 0.96.0

## Por quê

É a primeira pergunta de quem entregou uma chave a um parceiro: **eles estão usando? onde? quando
pararam?** Medido no Acervo: `GET /api-usage` agrega uma tabela `api_usage` e a tela do
desenvolvedor mostra chamadas por chave, por rota e o último uso.

O Trilha já sabe **qual** chave fez a requisição (`Keys.User`, `Key.LastUsed`) e já conta por chave
para limitar (`RateLimit`). O que falta é guardar por rota e mostrar.

## O que muda

```go
var Chaves = auth.APIKeys(auth.KeyOptions{
	Usage:     auth.UsageMemory(),  // ou uma tabela atrás da mesma interface
	UsageKeep: 400 * 24 * time.Hour,
})

func main() { … Chaves.Setup(a) … }   // flush no relógio e no Shutdown

uso, _ := Chaves.Usage(c.Context(), auth.UsageQuery{Key: id, Since: trinta})
// uso.Total, uso.Errors, uso.Last, uso.ByRoute, uso.ByDay
```

- **Contar não pode custar a requisição.** O incremento é em memória, agregado por
  `(chave, método, rota, dia)`, e vai para o store em lote — o mesmo trato do `touch`, que já
  escreve no máximo uma vez por minuto.
- **Nada muda sem o store.** `Usage` nulo é o comportamento de hoje, byte por byte.
- **A rota é o padrão, não o caminho.** `/documentos/{id}` e não `/documentos/8f2c…`: um contador
  por caminho concreto é um contador com uma linha por requisição.
- **Chave revogada mantém o histórico.** A pergunta "quem estava usando isto?" chega depois da
  revogação, não antes.
- **`ui.APIUsage`**: as barras por dia desenhadas no servidor e a tabela por rota. A
  `ui.APIKeysTable` ganha a coluna de chamadas quando as linhas trazem uma.

## Fora de escopo

- **`trilha audit` com "chave sem uso há 90 dias".** O comando lê código, não banco: ele roda numa
  máquina que não é a de produção e não tem o store. O que entra no lugar é `Keys.Idle`, que
  responde a mesma pergunta onde o dado está — e a tela marca a linha.
- **`auth.UsageSQL(db)`.** Store é interface mais memória; o SQL fica no projeto, como no resto.
- **Cota e cobrança.** O `RateLimit` já limita; cota mensal é outra issue.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `sync`, `time`, `sort` |
| VI — teste primeiro | mil requisições concorrentes sem perder conta, flush no Shutdown, revogada com histórico, retenção |
| Store é interface | `UsageStore` + `UsageMemory` |

## Tarefas

- [x] T001 Teste que falha: contagem concorrente, por rota, retenção, revogada, ociosa
- [x] T002 `UsageStore`, `UsageMemory`, `KeyOptions.Usage`, `Keys.Usage`, `Flush`, `Setup`, `Idle`
- [x] T003 `ui.APIUsage` e a coluna de chamadas na `ui.APIKeysTable`
- [x] T004 A receita `api-keys` passa a contar e a mostrar
- [x] T005 Documentação (en + pt), superfície de API e catálogo do kit
- [x] T006 `CHANGELOG.md`, `version`, `ROADMAP.md`, `make test`, `scripts/release.sh 0.96.0`

## Aceitação

- **SC-001** Mil requisições concorrentes com a mesma chave contam mil, e nenhuma delas esperou o
  store.
- **SC-002** O relatório separa por rota e por dia, com o último uso e os erros.
- **SC-003** `Shutdown` grava o que estava em memória.
- **SC-004** Uma chave revogada continua tendo histórico.
- **SC-005** `UsageKeep` apaga o que passou do prazo, e só isso.
- **SC-006** `ui.APIUsage` desenha as barras sem JavaScript e some com dignidade quando não há uso.
