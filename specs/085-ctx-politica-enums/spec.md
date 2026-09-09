# Spec 085 — trilha ctx: política por rota e enums registrados

- **Issue**: [#124](https://github.com/emersonjoe/trilha/issues/124) — a issue é a fonte do escopo.
- **Branch**: `085-ctx-politica-enums`
- **Versão**: 0.66.0

## Por quê

O `trilha ctx` existe para responder o que alguém aprenderia abrindo trinta arquivos. Duas
coisas ficaram de fora dele, cada uma por uma spec, e as duas pelo mesmo motivo — o `ctx` lê
código, e as duas moram numa declaração de nível de pacote ligada ao lugar onde é usada.

**Qual módulo e qual nível uma rota exige.** Hoje isso se descobre abrindo o `middleware.go` da
pasta e seguindo uma variável:

```go
var ver = acesso.Auth.RequirePolicy(acesso.Policy, "docs", "ver")

func Middleware(c *trilha.Ctx, next trilha.Next) error { return ver(c, next) }
```

Quem vai escrever a próxima tela precisa saber isso antes de escrever a primeira linha, e hoje
paga cinco arquivos por rota.

**Quais enums existem.** O `trilha.Enum` é o que impede duas telas de inventarem dois "status"
diferentes — e quem não sabe que um existe inventa o segundo. O registro é de tempo de execução
(`trilha.RegisterEnum` no `Setup`), então o `ctx` precisa reconhecer a declaração e casá-la com
o nome registrado.

É um trabalho só: **achar uma declaração de pacote e ligá-la ao uso**. Quem escrever um escreve
noventa por cento do outro.

## O que muda

No `trilha ctx`, no Markdown e no `--json` (que é o que o MCP e o agente leem):

```
## /documentos/{id}
  GET · page · app/documentos/id_/page.go
  exige: docs · ver          ← novo
```

```json
{ "pattern": "/documentos/{id}",
  "policy": { "module": "docs", "level": "ver", "from": "app/documentos/middleware.go" } }
```

```
## Enums
  documento.status — rascunho (Rascunho), enviado (Enviado), aprovado (Aprovado · success)
```

A política é **herdada como o middleware é herdado**: declarada em `app/docs/middleware.go`, ela
vale para tudo abaixo. Onde uma pasta declara a sua, a de baixo ganha; onde uma rota guarda um
método só (`MiddlewarePOST`), aparece por método, porque uma pasta legível por um nível e
gravável por outro é o caso que a matriz existe para expressar.

A leitura é estática e resolve uma indireção: a chamada dentro do `Middleware`, ou a var de
pacote que ele devolve. Duas indireções não; e o que não for reconhecido não vira palpite —
some da saída, que é o que mantém `ctx` uma fonte em que se confia.

Ao fechar, `LookupEnum` e `RegisteredEnums` são exportados: agora têm para quem servir, que era
a condição registrada na 0.46.0.

## Fora de escopo

- **Validar a política ou o enum.** O `trilha audit` já avisa do módulo que rota nenhuma exige.
  Aqui é contar o que existe, não julgar.
- **Resolver a política através de mais de uma indireção**, ou de outro pacote. Um
  `RequirePolicy` montado por uma função que recebe o módulo por parâmetro precisa de um
  compilador, e um `ctx` que chuta é pior que um `ctx` que cala.
- **Enum declarado fora do `app/`.** O registro está no `Setup`, e é lá e nos pacotes que ele
  importa do próprio projeto que se procura.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `go/ast`, `go/parser`, `go/token` — o mesmo que o `provided` já usa |
| VI — teste primeiro | herança, método único, enum com e sem rótulo, e o projeto que não usa nem um nem outro |
| Convenção nova | o `examples/local-login` já tem política e passa a aparecer no `ctx`; o `examples/blog` ganha o enum |

## Tarefas

- [x] T001 Teste que falha: a rota guardada por `RequirePolicy` mostra módulo e nível
- [x] T002 Leitura da política, com herança e por método
- [x] T003 Teste que falha: os enums registrados aparecem com valores e rótulos
- [x] T004 Leitura dos enums; `LookupEnum` e `RegisteredEnums` exportados
- [x] T005 Markdown e JSON, e o teste de que quem não usa não paga
- [x] T006 Documentação: referência do `cli` en + pt
- [x] T007 `CHANGELOG.md`, `version`, `ROADMAP.md`, `make test`, `scripts/release.sh 0.66.0 --issues "124"`

## Aceitação

- **SC-001** Uma rota abaixo de uma pasta com `RequirePolicy` mostra o módulo e o nível, e diz
  de qual arquivo veio.
- **SC-002** Uma pasta que declara a própria política sobrepõe a de cima; um `MiddlewarePOST`
  aparece só no método dele.
- **SC-003** Os enums registrados aparecem com valores, rótulos e tom.
- **SC-004** Um projeto sem política e sem enum tem exatamente a saída de antes.
- **SC-005** O que a leitura não reconhece não aparece — nunca um palpite.
