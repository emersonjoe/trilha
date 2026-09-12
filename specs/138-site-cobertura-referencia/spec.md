# Spec 138 — site: prova de cobertura da referência

> Mudança pequena: um pacote (`site/internal/docs`) mais o conteúdo do site, sem convenção
> nova em `app/`, sem mudança na API pública.

- **Issue**: [#188](https://github.com/emersonjoe/trilha/issues/188) — 17 componentes do `ui`
  fora do catálogo e ~100 símbolos de `api/current.txt` sem página, e o teste que impede a
  deriva. A issue é a fonte do escopo; este documento não a repete.
- **Branch**: `feat/site-cobertura-referencia`
- **Versão**: 0.117.0

## Por quê

`api/current.txt` é a promessa escrita: um teste falha quando um símbolo exportado entra ou
sai da superfície pública, e o diff dele entra na revisão. O site não tem esse guarda. A cada
release a API cresce, o capítulo novo é escrito, e o símbolo que não ganhou parágrafo nenhum
fica sem nada — ninguém percebe, porque nada quebra. Quem lê a referência para saber se dá
para expirar uma solicitação de aprovação, ou para achar o nome do cabeçalho que o webhook
assina, não encontra nem a linha da tabela; vai ler o `godoc`, ou o código, ou perguntar.

Não é bug de um capítulo, é deriva: 102 símbolos de `api/current.txt` hoje não aparecem como
código em página nenhuma, e 18 funções do kit `ui` não estão no catálogo de
`reference/ui.md`, inclusive as que têm capítulo próprio — e o catálogo é onde se procura.
Sem um teste, a lista só cresce.

## O que muda

Dois testes em `site/internal/docs`, no tom de `TestNoExternalDeps` (falham listando o que
falta, com o que fazer no fim):

- `TestReferenceCoversAPI`: todo símbolo de `api/current.txt` — `const`, `var`, `func`,
  `type` e método, sem os campos de struct, que a linha do tipo carrega — aparece **como
  código** (bloco cercado ou trecho entre backticks) em pelo menos uma página `en/` e em
  pelo menos uma página `pt/`. Escrever como código é como as páginas nomeiam um símbolo; a
  mesma palavra numa frase ("pending", "inbox") não é algo que o leitor possa chamar. O
  `pkg h` fica fora — os elementos são gerados de uma tabela e `reference/h.md` documenta o
  DSL, não uma página por tag. Um símbolo que o leitor nunca escreve entra em
  `documentedElsewhere` com o motivo ao lado; o teste também reclama de uma entrada que não
  existe mais na superfície pública, para a lista de exceções não virar lixo.
- `TestUIKitCatalogue`: toda `func` exportada de `ui/` aparece em `reference/ui.md` (en) e
  `referencia/ui.md` (pt), com o link para o capítulo quando ele existe.

E o conteúdo que fecha a lista de hoje: os 18 componentes no catálogo do `ui` e os 102
símbolos na página do seu pacote — a maioria é uma linha numa tabela que já existe.
`trilha.Limiter` e `App.RunShutdown` ganham parágrafo, porque os dois têm comportamento que
uma linha de tabela não conta (o balde por chave que o app usa fora do middleware, e o
desligamento executado à mão em teste).

A lista de exceções fecha com **uma** entrada: `trilha.CompileErrorPage`, a página de erro de
compilação que o `internal/dev` desenha e que quem escreve um app nunca chama. Todo o resto
ganhou linha — inclusive o que só existe para um store de terceiros implementar
(`blob.Presigner`, `auth.UsageStore`, `trilha.LinkStore`), porque é exatamente quem vai
escrever esse store que precisa de documentação.

### A lista que o teste imprime na `main` de hoje

```
--- FAIL: TestReferenceCoversAPI
    the reference fell behind the code: 204 symbols.
    - ai.Choice (type)                          - trilha.DeadlineKind (type)
    - ai.DefaultBaseURL (const)                 - trilha.EnumValue (type)
    - ai.DefaultServeHistory (const)            - trilha.ErrFrozen (const)
    - ai.DefaultServeInput (const)              - trilha.ErrNoLink (var)
    - ai.FunctionCall (type)                    - trilha.ErrNoVersion (var)
    - ai.FunctionDef (type)                     - trilha.ErrSearchKind (const)
    - ai.Request.MarshalJSON (method)           - trilha.ErrSecretShort (const)
    - ai.Tool.Def (method)                      - trilha.ErrUnknownKind (var)
    - ai.ToolCall (type)                        - trilha.Hint.Unwrap (method)
    - ai.ToolDef (type)                         - trilha.IslandRuntime (const)
    - approval.Approvals.Expire (method)        - trilha.KindAuto (const)
    - approval.Approvals.MayDecide (method)     - trilha.Limiter (type)
    - approval.Assignee (type)                  - trilha.LinkStore (type)
    - approval.ErrNotYours (var)                - trilha.MinSecretLen (const)
    - approval.ErrUnknown (var)                 - trilha.Problem.MarshalJSON (method)
    - approval.Expired (const)                  - trilha.ProblemMediaType (const)
    - approval.Rejected (const)                 - trilha.ReportPath (const)
    - approval.Withdrawn (const)                - trilha.RouteKind (type)
    - auth.Auth.Provider (method)               - trilha.SchemaOption (type)
    - auth.Claims (type)                        - trilha.SchemaTypes (var)
    - auth.ErrUnknownKey (var)                  - trilha.SearchGroup (type)
    - auth.Keys.Flush (method)                  - trilha.SearchMemory (func)
    - auth.Policy.LevelNames (method)           - trilha.SearchResult (type)
    - auth.Policy.LevelOf (method)              - trilha.Secret.LogValue (method)
    - auth.Policy.ModuleNames (method)          - trilha.Secret.MarshalJSON (method)
    - auth.Policy.RolesSorted (method)          - trilha.Secret.UnmarshalJSON (method)
    - auth.UsageReport (type)                   - trilha.SnippetPart (type)
    - auth.UsageRow (type)                      - trilha.StatusFail (const)
    - auth.UsageStore (type)                    - trilha.StatusPass (const)
    - blob.Disk (type)                          - trilha.Stream.Flush (method)
    - blob.Disk.Keys (method)                   - trilha.TestClient.PostJSON (method)
    - blob.Files.PutBytes (method)              - trilha.VersionMemory (func)
    - blob.Memory.Keys (method)                 - trilha.VersionRecord (type)
    - blob.NewDisk (func)                       - trilha.VersionStore (type)
    - blob.NewMemory (func)                     - ui.APIKeyRow (type)
    - blob.Presigner (type)                     - ui.APIKeysOpts (type)
    - blob.S3 (type)                            - ui.APIUsageData (type)
    - blob.S3.Keys (method)                     - ui.APIUsageDay (type)
    - blob.S3.Presign (method)                  - ui.APIUsageOpts (type)
    - task.Running (const)                      - ui.APIUsageRoute (type)
    - trilha.App.RunShutdown (method)           - ui.FieldOpt (type)
    - trilha.App.SetErrorPage (method)          - ui.FlashFadeMs (const)
    - trilha.App.SetNotFound (method)           - ui.FormatOpt (type)
    - trilha.App.SetRootLayout (method)         - ui.SearchBoxOpts (type)
    - trilha.AuditSink (type)                   - ui.SearchResultsOpts (type)
    - trilha.Bag (type)                         - ui.TimeOnly (func)
    - trilha.BusinessCalendar (type)            - webhook.ErrUnknownEvent (var)
    - trilha.CSRFCookie (const)                 - webhook.HeaderEvent (const)
    - trilha.CSRFField (const)                  - webhook.HeaderSignature (const)
    - trilha.CSRFHeader (const)                 - webhook.HeaderTimestamp (const)
    - trilha.ConnectionMemory (func)            - webhook.SecretCookie (const)

--- FAIL: TestUIKitCatalogue
    the kit catalogue fell behind the code: 36 symbols.
    - ui.Bars             - ui.InboxBadge       - ui.SparklineTitle   - ui.TaskTable
    - ui.ChartTitle       - ui.PageHeader       - ui.Stat             - ui.TimeOnly
    - ui.Donut            - ui.PolicyGrid       - ui.StatHint         - ui.WebhooksPanel
    - ui.EmptyError       - ui.Shell            - ui.Status
    - ui.Inbox            - ui.Sparkline        - ui.TaskProgress
```

São 102 símbolos e 18 funções (204 e 36 contando as duas locales, que é como o teste conta —
a linha diz qual das duas falta; hoje nenhum símbolo está só numa delas). A lista bate com o
levantamento da issue, feito à mão contra a 0.106.0: o que sobra são os símbolos que entraram
depois, e o que não está nela são os que a issue listou e alguma página já menciona.

## Fora de escopo

- **Um símbolo por página, gerado do `godoc`** — é o `godoc` de novo, e a referência do site
  existia para ser outra coisa: capítulo por assunto, com o exemplo do uso. O teste exige
  menção, não página.
- **Cobertura de campo de struct.** `type Config struct, Addr string` não é conferido campo a
  campo: a linha do tipo carrega o tipo, e exigir cada campo transformaria o teste numa
  tabela de 1.100 linhas que ninguém lê.
- **`llms-full.txt`** continua saindo do conteúdo, então se cobre sozinho quando o conteúdo
  cobre.
- **Conferir que a menção está na página *certa*.** `ui.Bars` documentado em
  `reference/charts.md` conta para `TestReferenceCoversAPI`; quem exige o lugar é o
  `TestUIKitCatalogue`, só para o catálogo do kit, que é o caso em que "onde se procura"
  tem uma resposta única.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | Os testes usam `os`, `sort`, `strconv`, `strings`, `testing`. Nada novo em runtime nem na CLI. |
| IV — API pública pequena e estável | Nenhum símbolo novo; `api/current.txt` não muda. O teste é o que passa a defender o outro lado da promessa. |
| VI — teste primeiro | `TestReferenceCoversAPI` e `TestUIKitCatalogue` entram falhando com a lista acima, e o conteúdo entra depois. |
| Idioma | Todo o conteúdo novo sai em `en/` e em `pt/` no mesmo commit, que é o que o teste passa a exigir por símbolo. |

## Tarefas

- [x] T001 Teste que falha: `site/internal/docs/coverage_test.go` — a lista acima
- [x] T002 Catálogo do `ui`: as 18 funções em `en/reference/ui.md` e `pt/referencia/ui.md`
- [x] T003 Os 102 símbolos na página do seu pacote, en e pt; `Limiter` e `App.RunShutdown`
      com parágrafo; `documentedElsewhere` para o que o leitor nunca escreve
- [x] T004 `CHANGELOG.md`, `version` em `cmd/trilha/main.go`, linha do `ROADMAP.md`
- [x] T005 `make test` verde

## Aceitação

- **SC-001** `go test ./site/internal/docs/` falha na `main` de hoje com a lista acima e
  passa nesta branch.
- **SC-002** Todo símbolo de `api/current.txt` fora do `pkg h` aparece como código em uma
  página `en/` e em uma `pt/`, ou está em `documentedElsewhere` com o motivo.
- **SC-003** Toda `func` exportada de `ui/` está no catálogo de `reference/ui.md` nas duas
  locales.
- **SC-004** `make test` verde.
