# Spec 128 — O nome do método que o corte da tag deixou minúsculo

- **Issue**: [#169](https://github.com/emersonjoe/trilha/issues/169) — a issue é a fonte do
  escopo (reprodução mínima e proposta estão lá); aqui fica só a decisão.
- **Branch**: `feat/mig-s01-client-prefixo-tag`
- **Versão**: 0.107.0

## Por quê

O `methodName` do `trilha client` corta do início do nome PascalCase o nome do grupo, para
que `Documents.ListDocuments` vire `Documents.List`. O corte é `strings.TrimPrefix` no
literal: ele não pergunta se o que sobrou começa uma palavra.

Na migração do Verba, a tag `config` e o `operationId` `configurar_regra` deram
`ConfigurarRegra`, que começa com `Config` por acaso — a palavra é outra. O que sobrou foi
`urarRegra`, e o método nasceu **não exportado**:

```go
// urarRegra is PUT /api/regras/{regra_key}: configurar Regra
func (g *Config) urarRegra(ctx context.Context, regraKey string, body RegraConfigRequest) (RegraConfigurada, error)
```

O detalhe que faz este bug custar caro é que **o arquivo gerado compila**. Um método não
exportado é Go válido; quem paga é o pacote de fora, no `go build` de quem chamou — ou
ninguém, se ninguém chamar aquela rota, e aí a operação simplesmente não existe para o
cliente. No Verba o contorno foi um `internal/api/regras.go` escrito à mão ao lado do
`client.go` gerado, só para reexportar `g.urarRegra` de dentro do pacote.

## O que muda

O corte do prefixo só acontece em **fronteira de palavra**: o que sobra tem de começar com
maiúscula. `ConfigurarRegra` na tag `config` fica inteiro; `ConfigReset` na mesma tag
continua virando `Reset`.

| tag | operationId | antes | agora |
|---|---|---|---|
| `config` | `configurar_regra` | `urarRegra` | `ConfigurarRegra` |
| `config` | `config_reset` | `Reset` | `Reset` |
| `documents` | `list_documents` | `List` | `List` |

**Não recapitalizar o resto** foi a decisão, contra a alternativa de emitir `UrarRegra`: esse
nome não está no documento nem na cabeça de quem lê a API, e um cliente gerado vale pelo
nome reconhecível. Quando o corte não tem uma palavra para tirar, não há redundância para
resolver — a tag é apenas um pedaço da primeira palavra — e o certo é não cortar.

O corte do **sufixo** (`gName` no fim, e o rabo `path_method` que a FastAPI escreve) fica
como está: `TrimSuffix` é sensível a maiúsculas, então um casamento sempre cai no início de
uma palavra do PascalCase, e o que sobra é o começo do nome, que já era maiúsculo.

## Fora de escopo

- **Avisar sobre o nome inventado.** A issue observa que nada avisa, como em
  [#166](https://github.com/emersonjoe/trilha/issues/166). Depois desta mudança não sobra
  caso a avisar: nome não exportado deixou de ser possível, e é o teste que garante isso.
  O aviso que a #166 pede é de outra natureza (colisão entre tag e schema) e continua lá.
- **A colisão de nomes entre tag e schema** (#166), que é a outra queixa da mesma migração.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `unicode/utf8` e `unicode`; `go/parser` só no teste |
| VI — teste primeiro | `TestPrefixoDaTagSoCaiEmFronteiraDePalavra` falha antes da mudança |
| VII — segurança por padrão | não toca em entrada de usuário nem em superfície exposta |

## Tarefas

- [x] T001 Teste que falha: o OpenAPI mínimo da issue gera `ConfigurarRegra` exportado, e
      nenhum método do arquivo gerado é não exportado (`go/parser` sobre a saída)
- [x] T002 `trimWordPrefix` em `internal/client/gen.go`, usado no lugar do `strings.TrimPrefix`
- [x] T003 Golden de `internal/client` conferido (`make golden`)
- [x] T004 `CHANGELOG.md`, `version` em `cmd/trilha/main.go`, linha da versão no `ROADMAP.md`
- [x] T005 `make test` verde

## Aceitação

- **SC-001** Tag `config` + `operationId` `configurar_regra` dá o método `ConfigurarRegra`.
- **SC-002** Nenhum método do cliente gerado é não exportado, verificado no AST e não por
  `strings.Contains`.
- **SC-003** O corte em fronteira de palavra continua: `config_reset` na tag `config` é
  `Reset`, e o golden de `internal/client/api/client.go` não muda.
