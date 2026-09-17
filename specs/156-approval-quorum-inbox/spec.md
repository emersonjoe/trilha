# Feature Specification: Quórum no `approval` e ação por linha no `ui.Inbox`

**Feature Branch**: `156-approval-quorum-inbox` | **Created**: 2026-09-17 | **Status**: Draft
**Input**: Issue [#250](https://github.com/emersonjoe/trilha/issues/250) — a issue é a fonte
do escopo; este documento não recopia a lista de implementação dela, só o contrato.

## Contexto

O `approval` + `ui.Inbox` da spec 147 modelam uma pessoa, uma decisão, agora. A fila do
Acervo (#250) tem três formas que não cabem nisso: comitês que decidem por quórum (a tarefa
continua pendente até a N-ésima pessoa votar), etapas com formulário — que não são uma
decisão de aprovar/rejeitar — precisando conviver com aprovações na mesma caixa, e um campo
`display` do `ui.SchemaForm` que descarta o rótulo.

## User Scenarios & Testing

### US1 - Fila decidida por comitê (P1)

Uma aplicação abre um `approval.Request` com `Quorum: 3`. Três pessoas diferentes chamam
`Decide`; a tarefa permanece `Pending` depois da primeira e da segunda, e só fecha
(`State`/`Decided` gravados, `On` executado) depois da terceira. A mesma pessoa chamando
`Decide` duas vezes não conta dois votos.

**Independent Test**: `TestQuorumFechaSoNoUltimoVoto` em `approval/approval_test.go` —
`Quorum: 3`, três `Decide` de atores distintos, checa `Pending` após os dois primeiros e o
estado final após o terceiro; um quarto `Decide` do primeiro ator devolve `ErrAlreadyVoted`.

**Acceptance Scenarios**:

1. **Given** um request com `Quorum: 3` e nenhum voto, **When** a primeira pessoa decide,
   **Then** o registro continua `Pending` e ganha um `Vote` em `Votes`.
2. **Given** um request com dois votos de `Quorum: 3`, **When** a mesma pessoa do primeiro
   voto chama `Decide` de novo, **Then** o pacote devolve `ErrAlreadyVoted` e `Votes` não
   cresce.
3. **Given** um request com `Quorum: 3` e dois votos, **When** a terceira pessoa decide,
   **Then** `State`/`Decided`/`By` são gravados e o `On` do `Kind` roda, como hoje.
4. **Given** um request com `Quorum` zero ou não definido, **When** uma pessoa decide,
   **Then** o comportamento é o de hoje — fecha no primeiro voto (regressão).

### US2 - Linha do inbox com uma ação que não é aprovar/rejeitar (P1)

Uma aplicação com etapas de formulário e aprovações na mesma caixa preenche
`ui.InboxRow.Action` com o nó que aquela linha precisa (um link para a etapa, ou os dois
botões de aprovação). `ui.Inbox` desenha esse nó na célula de ação em vez do `decideForm`
padrão.

**Independent Test**: `TestInboxRowComAction` em `ui/inbox_test.go` — uma linha com `Action`
preenchida e outra sem, checando que a primeira desenha o nó dado e a segunda continua com o
`decideForm` de hoje quando `InboxOpts.Decide != ""`.

**Acceptance Scenarios**:

1. **Given** uma `InboxRow` com `Action` preenchida, **When** `ui.Inbox` renderiza,
   **Then** a última célula da linha é o nó de `Action`, não os dois botões.
2. **Given** uma `InboxRow` sem `Action` e `InboxOpts.Decide` preenchido, **When** a linha é
   `pending`, **Then** a célula desenha o `decideForm` de hoje (regressão).
3. **Given** uma `InboxRow` com `Progress: "1/3"`, **When** `ui.Inbox` renderiza,
   **Then** o texto aparece ao lado do `Status` da coluna State, sem mudar `Kind`.

### US3 - Campo `display` do `SchemaForm` com rótulo (P2)

Uma etapa de formulário usa um campo `{Type: "display", Label: "Endereço do imóvel", Text:
"Rua das Flores, 12"}` para mostrar um valor já apurado. `ui.SchemaForm` desenha o rótulo
junto do valor.

**Independent Test**: `TestSchemaFieldDisplayComLabel` em `ui/schema_test.go`.

**Acceptance Scenarios**:

1. **Given** um `SchemaField{Type: "display", Label: "X", Text: "Y"}`, **When**
   `SchemaForm` renderiza, **Then** a saída contém `X` e `Y`.
2. **Given** um `SchemaField{Type: "display", Text: "Y"}` sem `Label`, **When**
   `SchemaForm` renderiza, **Then** a saída é a de hoje — só `Y` (regressão).

### Edge Cases

- `Quorum: 1` e `Quorum: 0` se comportam de forma idêntica (decisão única, como hoje).
- Um `Vote` cujo `State` diverge dos anteriores (duas aprovações e uma rejeição) ainda fecha
  no N-ésimo voto; qual `State` fica gravado é o da decisão que completa o quórum — a
  aplicação que quer maioria/veto lê `Votes` antes de chamar `Decide` de novo (fora de
  escopo desta spec, ver abaixo).
- `InboxRow.Action` preenchida em uma linha `pending` some com os dois botões padrão mesmo
  quando `InboxOpts.Decide` está preenchido — `Action` sempre vence.

## Requirements

### Security and privacy impact

- **Assets and trust boundaries**: `approval` continua sendo a fronteira que decide quem
  pode votar; nada muda em `MayDecide`. `Vote.By` é o mesmo dado pessoal que `Record.By` já
  guarda hoje — nenhum campo sensível novo.
- **ASVS 5.0 Level 2 controls**: V4 (controle de acesso) — `Decide` continua chamando
  `MayDecide` antes de gravar qualquer voto, quórum ou não; V1 (arquitetura) — o quórum é
  estado do servidor (`approval`), `ui.Inbox` só desenha o `Progress` que a aplicação manda.
- **OWASP Top 10:2025 risks**: A01 (controle de acesso quebrado) mitigado pelo `MayDecide`
  existente e por `ErrAlreadyVoted` (voto duplicado nunca conta duas vezes); nenhum código
  novo de template/SQL, sem risco de injeção novo.
- **Secrets and personal data**: nenhum segredo novo; `Vote.By` é auditado do mesmo jeito
  que `rec.By` já é (`c.Audit("approval.decide", …)`).
- **Exceptions**: nenhuma.

### Functional Requirements

- **FR-001**: `approval.Request` MUST aceitar um campo `Quorum int`; zero ou um preserva o
  comportamento de hoje (fecha no primeiro voto).
- **FR-002**: `approval.Record` MUST acumular um `Vote` por decidente distinto em `Votes`
  enquanto `len(Votes) < Quorum`, mantendo `State == Pending`.
- **FR-003**: `approval.Approvals.Decide` MUST devolver `ErrAlreadyVoted` quando o ator que
  chama já tem um voto em `Votes` para aquele registro.
- **FR-004**: Ao acumular o N-ésimo voto (`len(Votes) == Quorum`), `Decide` MUST gravar
  `State`/`Decided`/`By` e rodar o `On` do `Kind`, como acontece hoje no voto único.
- **FR-005**: `ui.InboxRow` MUST aceitar um campo `Action h.Node`; quando preenchido,
  `ui.Inbox` MUST desenhar esse nó na célula de ação em vez do `decideForm` padrão.
- **FR-006**: `ui.InboxRow` MUST aceitar um campo `Progress string`; quando não vazio,
  `ui.Inbox` MUST desenhá-lo ao lado do `Status` da coluna State.
- **FR-007**: `ui.SchemaForm`, no campo `Type: "display"`, MUST desenhar `f.Label` quando
  preenchido, mantendo o comportamento de hoje quando `Label` está vazio.
- **FR-008**: `ui/inbox.go` `decideForm` MUST usar `ui.Submit` em vez de
  `ui.Button(..., h.Type("submit"))` para os dois botões de decisão.

### Key Entities

- **Vote**: um voto dentro de um `Record` com quórum — quem (`By`), o quê (`State`:
  approved/rejected), por quê (`Reason`) e quando (`At`).
- **InboxRow.Action / Progress**: extensões de uma linha do inbox que já existe — a célula
  de ação configurável e o texto de progresso do quórum.

## Success Criteria

### Measurable Outcomes

- **SC-001**: Uma fila com `Quorum: N` só fecha no N-ésimo voto de N pessoas distintas;
  nenhum voto duplicado da mesma pessoa altera o estado.
- **SC-002**: Uma aplicação com etapas de formulário e aprovações na mesma caixa desenha as
  duas formas de ação na mesma tabela, sem duas instâncias de `ui.Inbox`.
- **SC-003**: `make test` verde, incluindo os testes de regressão do comportamento de hoje
  (quórum 0/1, `InboxRow` sem `Action`, campo `display` sem `Label`).

## Assumptions

- A política de empate/maioria entre votos divergentes que fecham o quórum é da aplicação
  (lê `Votes` antes de decidir de novo, ou decide no próprio `On`); este pacote só sabe
  contar votos e fechar no N-ésimo, não arbitrar entre eles.
- `ui.DeadlineList` (mesma falta de ação por linha, #237) e um método
  `Activate`/maioria automática em `approval` ficam fora desta spec — trabalho de spec
  própria, como a #250 já separa.
