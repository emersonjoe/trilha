# Spec 129 — O nome que o documento dá e o identificador Go que o cliente escreve

- **Issues**: [#166](https://github.com/emersonjoe/trilha/issues/166) (nome de tag colide com
  nome de schema e o gerado não compila) e
  [#177](https://github.com/emersonjoe/trilha/issues/177) (nome de tag com acento vira
  identificador Go não-ASCII) — as issues são a fonte do escopo, com reprodução e proposta;
  aqui fica só a decisão.
- **Branch**: `feat/mig-s02-client-nomes`
- **Versão**: 0.108.0

## Por quê

As duas issues são a mesma pergunta feita em dois lugares: **como um nome escrito no OpenAPI
vira um identificador Go**. Hoje o `trilha client` responde com `exportName`, que junta as
palavras em PascalCase e não pergunta mais nada — nem se o nome já é de outro tipo do arquivo,
nem se as letras cabem no alfabeto que o resto do repositório usa.

O primeiro caso quebra na hora: a tag `auditoria` e o schema `Auditoria` dão o mesmo nome Go,
o arquivo sai com `type Auditoria` duas vezes e não compila — e o comando termina dizendo
`client.go written, package api.`, então o erro só aparece no `go build` de quem chamou. O
contorno no Verba foi renomear o schema na API (`AuditoriaLista`), isto é, mudar o servidor
por causa do gerador.

O segundo caso é pior por não quebrar: `dados públicos (MCP)` vira `DadosPúblicosMCP`, que é
Go válido e compila. A conta chega depois, em cada página que escreve
`c.VerificaçãoDeAssinaturas().Verificar(ctx, code)` num teclado sem acento morto, em cada
`grep` que depende de o `Público` da tela estar em NFC e não em NFD, e no contraste com a
convenção do próprio repositório, onde todo identificador exportado é ASCII. O contorno no
Acervo era renomear as tags na FastAPI — trocar o rótulo que aparece no `/docs` da API para
agradar o gerador.

Nos dois, quem cede é o documento. Devia ser o gerador: o nome do schema e o rótulo da tag são
dados, e o identificador Go é invenção nossa.

## O que muda

**1. Todo identificador que o gerador escreve é ASCII.** `exportName` passa a rebaixar a letra
acentuada para a sua base antes de montar o nome — a mesma regra para tag, schema, campo,
constante de enum, parâmetro de rota:

| no documento | antes | agora |
|---|---|---|
| tag `dados públicos (MCP)` | `DadosPúblicosMCP` | `DadosPublicosMCP` |
| tag `verificação de assinaturas` | `VerificaçãoDeAssinaturas` | `VerificacaoDeAssinaturas` |
| tag `formulário externo` | `FormulárioExterno` | `FormularioExterno` |
| schema `Endereço`, campo `número` | `Endereço.número`¹ | `Endereco.Numero` |

¹ o campo com acento nem exportado ficava: a primeira letra era um byte de uma sequência
UTF-8, e `strings.ToUpper(p[:1])` não tem o que subir — o cliente saía com um campo que o
`encoding/json` de fora não enxerga.

A tabela de transliteração cobre o latim acentuado (Latin-1 e Latin Extended-A: `áàâãäå`,
`ç`, `ñ`, `ü`, `ø`, `ß`→`ss`, `æ`→`ae`, `œ`→`oe`, …). Letra de outro alfabeto não tem base
latina e é descartada, como já acontecia com a pontuação.

**2. Quando o nome da tag já é de um tipo do documento, quem cede é o grupo.** O nome do
schema veio do documento; o do grupo é invenção do gerador, então é ele que ganha o sufixo
`API` — e `API2`, `API3`, se o sufixo também estiver tomado:

```go
// Auditoria is a schema of the API.
type Auditoria struct{ … }

// AuditoriaAPI is the Auditoria part of the API.
// Auditoria is already a type of the document, so the group carries the API suffix.
type AuditoriaAPI struct{ c *Client }

// Auditoria returns the Auditoria part of the API.
func (c *Client) Auditoria() *AuditoriaAPI { return &AuditoriaAPI{c: c} }

func (g *AuditoriaAPI) Listar(ctx context.Context, p AuditoriaListarParams) (Auditoria, error)
```

O sufixo fica **no tipo do grupo e em mais nada**: o acessório continua `c.Auditoria()`, os
tipos que o gerador inventa para a operação continuam `AuditoriaListarParams`, e o corte do
nome da tag no nome do método (spec 128) continua valendo sobre `Auditoria`. Quem escreve a
chamada não vê o sufixo; ele existe para o compilador.

**3. O documento continua no comentário, e a invenção aparece no relatório.** O tipo do grupo
ganha uma linha de comentário com o rótulo original quando o identificador não é o rótulo
(`// The document's tag is "dados públicos (MCP)".`), e o relatório do comando — que hoje não
diz uma palavra sobre os nomes que acabou de inventar — ganha uma linha por tag renomeada:

```
  tag "dados públicos (MCP)": the group is DadosPublicosMCP
  tag "auditoria": Auditoria is already a type of the document — the group is AuditoriaAPI
  internal/api/client.go written, package api.
```

Cliente já gerado que não tenha tag com acento nem colisão não muda um byte: as três regras só
entram onde hoje há um nome não-ASCII ou uma redeclaração.

## Fora de escopo

- **Tag em alfabeto não latino** (cirílico, grego, CJK): a letra é descartada e o nome cai no
  `X` que `exportName` já usava para nome vazio. Transliterar de verdade pede tabela por
  idioma, e nenhuma das duas migrações tem esse caso.
- **Colisão entre dois nomes que o gerador inventa** (um `…Params` com o mesmo nome de um
  `…Response`): o `Params` passa a desviar do nome de um grupo e do nome de um schema, mas a
  colisão entre dois tipos inline continua como está — é outra pergunta, sem caso real ainda.
- **Duas tags com o mesmo nome Go** (`user-data` e `user data`): continuam virando um grupo só,
  que compila e é o que o leitor esperaria.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `strings`/`unicode`/`utf8`; no teste, `go/ast` e um `go build` em `t.TempDir()` |
| VI — teste primeiro | `TestTagQueColideComSchema` e `TestIdentificadorDoDocumentoEASCII` falham antes |
| VII — segurança por padrão | não toca em entrada de usuário em tempo de execução; o gerador lê um documento que quem roda o comando escolheu |

## Tarefas

- [x] T001 Teste que falha: tag `auditoria` + schema `Auditoria` → o gerado **compila** (`go
      build` sobre a saída), nenhum tipo declarado duas vezes, `AuditoriaAPI` é o grupo e
      `Auditoria` é o schema
- [x] T002 Teste que falha: as tags da #177 e um schema `Endereço`/`número` → todo
      identificador do arquivo é ASCII e exportado onde precisa ser
- [x] T003 Implementação: transliteração em `exportName`; nome do tipo do grupo separado do
      nome da tag; sufixo `API` em colisão; comentário e notas do relatório
- [x] T004 Goldens (`make golden`) e clientes gerados commitados conferidos
- [x] T005 Referência do `trilha client` nas duas locales (`en/reference/cli.md`,
      `pt/referencia/cli.md`) com a regra de nomes
- [x] T006 `CHANGELOG.md`, `version` em `cmd/trilha/main.go`, linha da versão no `ROADMAP.md`
- [x] T007 `make test` verde e `verifica-trilha.sh --sem-testes`

## Aceitação

- **SC-001** O documento mínimo da #166 (tag `auditoria`, schema `Auditoria`) gera um arquivo
  que passa no `go build`, com `type Auditoria struct` uma única vez.
- **SC-002** `c.Auditoria()` continua sendo o caminho da chamada, e devolve `*AuditoriaAPI`.
- **SC-003** O documento da #177 gera `DadosPublicosMCP`, `VerificacaoDeAssinaturas` e
  `FormularioExterno`, e nenhum identificador do arquivo tem byte fora do ASCII.
- **SC-004** O relatório do comando nomeia cada tag cujo identificador ele inventou.
- **SC-005** O golden de `internal/client/api/client.go` e os clientes commitados dos exemplos
  não mudam: o documento deles não tem acento em nome nem colisão.
