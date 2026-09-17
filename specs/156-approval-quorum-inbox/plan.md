# Implementation Plan: Quórum no `approval` e ação por linha no `ui.Inbox` (156)

**Branch**: `156-approval-quorum-inbox` (artefatos na branch de sessão
`claude/awesome-goodall-nltffh` — ver nota em spec.md) | **Spec**: spec.md

## Constitution Check

| Princípio | Como a feature respeita |
|---|---|
| II — só biblioteca padrão | `Vote` usa `time`; `ErrAlreadyVoted` usa `errors` — ambos já importados em `approval/`. `ui/inbox.go` e `ui/schema.go` não ganham import novo. |
| VI — teste primeiro | `approval/approval_test.go` ganha o teste de quórum antes da mudança em `Decide`; `ui/inbox_test.go` e `ui/schema_test.go` ganham os testes de `Action`/`Progress`/`display` antes da mudança em `ui/`. |
| VII — segurança por padrão | `MayDecide` continua sendo a única porta antes de qualquer voto contar; `ErrAlreadyVoted` fecha o caso de um voto duplicado inflando o quórum. |

## Design

### Arquivos

```
approval/
  approval.go       // Request.Quorum, Record.Votes, Vote, ErrAlreadyVoted, Decide
  approval_test.go  // TestQuorumFechaSoNoUltimoVoto e regressão Quorum 0/1
ui/
  inbox.go           // InboxRow.Action, InboxRow.Progress, decideForm usa Submit
  inbox_test.go      // TestInboxRowComAction, TestInboxRowProgress
  schema.go          // schemaField: display desenha Label
  schema_test.go      // TestSchemaFieldDisplayComLabel
internal/recipes/approvals.go   // linhas() do recipe passa Progress quando o Record tem Votes
CHANGELOG.md, cmd/trilha/main.go (version), ROADMAP.md
```

### `approval.Decide` — quórum

```go
type Vote struct {
    By     string
    State  string
    Reason string
    At     time.Time
}

// em Record:
Votes []Vote

var ErrAlreadyVoted = errors.New("approval: this person already voted")
```

`Decide` (mesma assinatura) passa a:

1. Carregar `rec`, checar `rec.State == Pending` e `MayDecide` — igual a hoje.
2. Se `rec.Quorum` (novo campo em `Record`, copiado de `Request.Quorum` em `Open`) `<= 1`:
   caminho de hoje, sem tocar em `Votes`.
3. Senão: se `actorOf(c)` já aparece em `rec.Votes[i].By`, devolve `ErrAlreadyVoted` sem
   gravar. Caso contrário, acrescenta um `Vote{By, State, Reason, At: a.now()}` a
   `rec.Votes`, salva e retorna sem rodar `On` enquanto `len(rec.Votes) < rec.Quorum`.
4. Quando o voto que acabou de entrar fecha o quórum (`len(rec.Votes) == rec.Quorum`),
   grava `State`/`Decided`/`By` (do voto que fechou) e roda `On`, como o caminho de hoje.

`Open` grava `rec.Quorum = r.Quorum` (campo novo em `Record`, espelhando `Request.Quorum`,
necessário porque `Decide` só tem o `Record` guardado, não o `Request` original).

### `ui.Inbox` — `Action` e `Progress`

Em `Inbox`, a célula final passa de

```go
if o.Decide != "" && r.State == "pending" {
    cells = append(cells, h.Td(decideForm(r, o, reason, w)))
} else {
    cells = append(cells, h.Td(decidedCell(r)))
}
```

para: `r.Action != nil` desenha `h.Td(r.Action)` antes de checar `o.Decide`. A coluna State
ganha `Status(...)` seguido de `Muted(h.Text(r.Progress))` quando `r.Progress != ""`.
`decideForm` troca os dois `Button(Sm(), h.Type("submit"), ...)`/`Button(Outline(), Sm(),
h.Type("submit"), ...)` por `Submit(Sm(), ...)`/`Submit(Outline(), Sm(), ...)`.

### `ui.SchemaForm` — `display`

```go
if f.Type == "display" {
    if f.Label == "" {
        return h.Div(h.Class("ui-field ui-field-display"), h.P(h.Text(f.Text)))
    }
    return h.Div(h.Class("ui-field ui-field-display"),
        h.Label_(h.Class("ui-label"), h.Text(f.Label)), h.P(h.Text(f.Text)))
}
```

(nome exato do construtor de `<label>` em `h` confirmado durante a implementação — o
pacote `ui` já usa `Label` como nome de componente próprio, então o `<label>` cru do `h`
pode ter outro nome; ver `h/elements.go`.)

## Riscos

- `Record.Quorum` é campo novo que precisa ser persistido pelo `Store` de memória e por
  qualquer `Store` de aplicação (interface, campos por valor) — como `Record` já é copiado
  por valor entre `Save`/`Get`, nenhum `Store` precisa de mudança de schema além de guardar
  o campo (a implementação de memória do próprio pacote é a única no repositório).
- Nenhum risco de compatibilidade: `Quorum`, `Votes`, `Action`, `Progress` são campos novos
  com zero-value que reproduz o comportamento de hoje; nenhuma assinatura de função muda.
