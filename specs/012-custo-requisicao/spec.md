# Feature Specification: Reduzir o custo fixo por requisição

**Feature Branch**: `012-custo-requisicao` | **Created**: 2026-09-05 | **Status**: Draft
**Input**: achado dos benchmarks da spec 011, registrado a pedido do usuário.

## Achado que originou esta spec

Os benchmarks da spec 011 (`bench/`, Apple M2, Go 1.25) mostraram duas coisas opostas:

- **O `h` é rápido**: a página de exemplo (20 itens, layout) renderiza em 19,4 µs contra
  29,4 µs de `html/template` — cerca de **um terço mais rápido**, ainda que com mais
  alocações (482 × 270), porque o DSL monta a árvore em Go em vez de interpretar um
  template.
- **O framework cobra um pedágio fixo**: qualquer rota, mesmo devolvendo `"ok"`, custa
  **~3 µs e ~50 alocações a mais** que o `net/http` puro. Ele aparece igual nos quatro
  cenários que não são a página:

| Cenário | Stdlib | Trilha | Diferença |
|---|---|---|---|
| JSON (20 itens) | 4,2 µs · 10 allocs | 7,6 µs · 51 allocs | +3,4 µs · +41 allocs |
| Estático (1,4 KB) | 1,4 µs · 16 allocs | 4,3 µs · 60 allocs | +2,9 µs · +44 allocs |
| 200 rotas + parâmetro | 0,72 µs · 11 allocs | 4,0 µs · 51 allocs | +3,3 µs · +40 allocs |
| 5 middlewares | 0,64 µs · 10 allocs | 4,1 µs · 51 allocs | +3,4 µs · +41 allocs |

O número está documentado com honestidade em
[Desempenho e comparação](https://emersonjoe.github.io/trilha/referencia/desempenho): em um
app real, uma consulta ao banco custa de 100 µs a alguns ms e a rede custa mais, então 3 µs
não aparecem. Ainda assim, é custo pago por **toda** requisição, inclusive as de arquivo
estático e as de *health check*, e a leitura do código mostra que boa parte dele é
desperdício, não segurança.

## Onde o custo nasce (leitura do código, a confirmar com profile)

| Origem | O que acontece hoje | Por que é desperdício |
|---|---|---|
| `Security.csp()` em `security.go` | a política é **remontada a cada requisição**: mapa `seen`, slice `parts`, `sort.Strings`, `strings.Join` e `ReplaceAll` | a política só muda quando a `Config` muda; o nonce é a única parte variável |
| `Ctx.Nonce()` chamado por `applySecurity` | 16 bytes de `crypto/rand` + base64 em **toda** requisição | rota de API e arquivo estático nunca emitem HTML com script inline; não precisam de nonce |
| `newCtx` | `crypto/rand` (8 bytes) + `hex` para o id, e `map[string]any{}` sempre alocado | o mapa costuma ficar vazio; o id não é segredo (é identificador de log) |
| `logRequest` | `slog.Info` com 6 atributos e `time.Since(...).Round(...).String()` | formata mesmo quando o handler descarta a saída ou o nível está desligado |
| `applySecurity` | 6 a 7 `Header().Set` com `textproto.CanonicalMIMEHeaderKey` | os valores são constantes entre requisições |
| `Ctx` e `responseWriter` | uma alocação cada, por requisição | candidatos a `sync.Pool` |

## User Scenarios & Testing

### US1 - Mesmo comportamento, metade do custo (P1)
Nenhuma mudança de API pública, nenhum padrão de segurança afrouxado, `make test` verde.
**Acceptance**: `make bench` mostra, na máquina de referência, **≤ 2 µs e ≤ 25 alocações**
de diferença para o stdlib nos cenários JSON, estático, roteamento e middlewares (hoje
~3,4 µs e ~41); `bench/RESULTS.md` regravado; a tabela da página de desempenho atualizada.

### US2 - Nonce só onde faz sentido (P1)
Rotas que respondem JSON ou arquivo estático recebem a CSP sem `'nonce-…'` (precomputada
uma vez); páginas continuam com nonce por requisição, imprevisível, como hoje.
**Acceptance**: teste de segurança: `GET /api/x` tem CSP sem `nonce-`; `GET /` (página)
tem `'nonce-'` e o valor bate com `c.Nonce()` e com o `<script>` injetado em dev; dois
pedidos à mesma página têm nonces diferentes.

### US3 - Nada some do log (P2)
O log continua com os mesmos campos; só deixa de ser formatado quando o `slog` diz que o
nível está desabilitado, e a duração vira `slog.Duration` (sem string intermediária).
**Acceptance**: teste com um `slog.Handler` de nível `Error` não registra nada e não
formata; com nível `Info`, a linha tem `method`, `path`, `status`, `bytes`, `dur` e
`request_id`.

## Requirements

- **FR-001** Precompilar a CSP na `applyConfig` (duas variantes: com e sem nonce), guardando
  prefixo e sufixo em volta do `{nonce}` para que a requisição faça uma concatenação.
- **FR-002** Gerar o nonce sob demanda; `Ctx.Nonce()` continua público e válido em páginas.
- **FR-003** Id de requisição sem `crypto/rand` (não é segredo): `math/rand/v2` ou contador
  com prefixo aleatório por processo, documentado no doc comment.
- **FR-004** `Ctx.values` alocado no primeiro `Set`; `Get` num mapa nil devolve `nil`.
- **FR-005** `logRequest` guardado por `Enabled` e sem `String()` na duração.
- **FR-006** `sync.Pool` para `Ctx`/`responseWriter` **apenas se** o profile mostrar ganho
  real e se ficar provado que nada escapa depois do handler (é a mudança mais arriscada:
  fica por último e pode ser descartada).
- **FR-007** Benchmarks e `RESULTS.md` regravados; página de desempenho e CHANGELOG.

## Fora do escopo

Trocar o `http.ServeMux`, remover o log de requisição por padrão, ou tornar opcional
qualquer cabeçalho de segurança. Desempenho não compra afrouxamento de padrão seguro
(princípio VII da constituição).
