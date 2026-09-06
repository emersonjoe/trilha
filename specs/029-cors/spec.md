# Spec 029 — CORS configurável

- **Issue**: [#29](https://github.com/emersonjoe/trilha/issues/29) (ROADMAP, Fase 3, item 12)
- **Branch**: `029-cors`
- **Versão**: 0.20.0

## Por quê

Um app de outra origem que chama a API do Trilha hoje é barrado pelo navegador sem que o
servidor tenha dito nada: o `OPTIONS` do preflight cai no roteador, volta 405, e a mensagem
que o desenvolvedor vê ("blocked by CORS policy") não aponta para lugar nenhum. Cada projeto
acaba escrevendo o próprio middleware — e o middleware caseiro erra sempre nos mesmos dois
pontos: devolve `Access-Control-Allow-Origin: *` junto com credenciais (o navegador recusa, e
quando não recusa é porque alguém trocou o `*` por eco cego da origem, que é pior) e esquece
o `Vary: Origin`, o que faz um cache intermediário servir a resposta de uma origem para
outra.

## O que muda

`Config.CORS`. Vazio, nada acontece — quem não configura não paga nem um cabeçalho.

```go
trilha.Config{
	CORS: trilha.CORS{
		Origins:     []string{"https://app.exemplo.com"},
		Credentials: true,
		MaxAge:      10 * time.Minute,
	},
}
```

### Superfície

| Símbolo | Papel |
|---|---|
| `CORS.Origins []string` | origens exatas (`https://app.exemplo.com`) ou `"*"` sozinho; vazio = CORS desligado |
| `CORS.Methods []string` | padrão `GET, HEAD, POST, PUT, PATCH, DELETE` |
| `CORS.Headers []string` | o que o cliente pode mandar; padrão `Content-Type, Authorization, X-CSRF-Token, Trilha-Fragment` |
| `CORS.Expose []string` | o que o script da outra origem pode ler na resposta; padrão nenhum |
| `CORS.Credentials bool` | libera cookie e `Authorization`; incompatível com `*` |
| `CORS.MaxAge time.Duration` | quanto o navegador guarda o preflight; zero omite o cabeçalho |

## Decisões

- **Combinação insegura é pânico na configuração, não 500 em produção.** `*` com
  `Credentials`, `*` misturado com outras origens e origem malformada (com caminho, com barra
  no fim, sem esquema) derrubam o `New`, junto com o resto do boot. É o mesmo caminho que
  `AddRule` já toma para nome de regra repetido: erro de quem escreve o app aparece na
  primeira execução, não na primeira requisição de fora.
- **O preflight é respondido antes do roteador.** O gancho fica no `serveHTTP`, antes do
  `mux`, para o `OPTIONS` valer em qualquer rota — inclusive nos arquivos estáticos, que não
  passam por `wrap`.
- **Origem não listada não é bloqueada em requisição simples.** O preflight recusado devolve
  403, porque ali o navegador está perguntando e merece resposta; a requisição simples segue
  para o handler **sem** os cabeçalhos de CORS, e é o navegador que esconde a resposta do
  script. Barrar no servidor quebraria o cliente que não é navegador e a requisição de mesma
  origem que manda `Origin` mesmo assim.
- **`Vary: Origin` sempre que a política existe.** Sem ele, um cache na frente do app serve
  a resposta liberada para uma origem a quem não devia.
- **`Credentials` nunca ecoa `*`.** Com credencial ligada, o cabeçalho leva a origem exata
  que veio — o que só acontece se ela estiver na lista.

## Requisitos

- **FR-001** `CORS` zerado não muda nada: nenhum cabeçalho novo, `OPTIONS` continua como era.
- **FR-002** Preflight (`OPTIONS` + `Access-Control-Request-Method`) de origem e método
  permitidos devolve 204 com `Allow-Origin`, `Allow-Methods`, `Allow-Headers`, `Max-Age`
  (quando configurado) e `Vary`.
- **FR-003** Preflight de origem não listada ou de método não permitido devolve 403 sem
  `Allow-Origin`, e não chega ao roteador.
- **FR-004** Requisição simples de origem permitida ganha `Allow-Origin` (e
  `Allow-Credentials` / `Expose-Headers` quando configurados); de origem não listada, roda
  igual e não ganha nenhum deles.
- **FR-005** `Origins: ["*"]` sem credencial responde `*`; com `Credentials` entra em pânico
  no `New`.
- **FR-006** `*` junto de outra origem, e origem com caminho, barra final ou sem esquema,
  entram em pânico no `New`.
- **FR-007** Zero dependência externa; `go test -race` limpo.

## Fora de escopo

- **Origem por padrão (`*.exemplo.com`) e origem decidida em runtime.** Lista exata resolve o
  caso real e não tem como errar; casamento por padrão é onde os CVEs de CORS moram
  (`exemplo.com.atacante.net`). Quem precisar escreve o próprio middleware.
- **Private Network Access (`Access-Control-Allow-Private-Network`).** Rascunho ainda.
- **CORS por rota.** A política é do app; rota que precisa de outra política é sinal de que
  são dois apps.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| I — SSR primeiro | nada muda no HTML; é cabeçalho de resposta |
| II — só biblioteca padrão | `net/http`, `net/url`, `strings`, `time` |
| III — coerência com Go | struct de configuração como `RateLimit`; sem opções variádicas |
| IV — convenção nova tem teste e uso no exemplo | `examples/blog` libera uma origem e o teste de integração cobre preflight |
| VI — teste primeiro | `cors_test.go` vermelho antes de `cors.go` |
| VII — compatibilidade | campo novo com zero valor inerte; nada existente muda |

## Tarefas

- [x] T001 `cors_test.go` vermelho: preflight ok, origem não listada, método não permitido,
      requisição simples (listada e não listada), `*` com e sem credencial, origem malformada,
      app sem CORS.
- [x] T002 `cors.go` + `Config.CORS` + política no `applyConfig` + gancho no `serveHTTP`.
- [x] T003 `examples/blog`: origem liberada no `Config` e teste de integração do preflight.
- [x] T004 Documentação nas duas locales: `learn/security` (a seção do CORS) e
      `reference/app` (linha do `CORS` no bloco e na tabela).
- [x] T005 `CHANGELOG.md` (0.20.0), `version` em `cmd/trilha/main.go`, ROADMAP item 12.
- [x] T006 `make test` verde e `make release VERSION=0.20.0 ISSUES="29"`.

## Aceitação

- **SC-001** Preflight de `https://app.exemplo.com` para `PUT` volta 204 com os quatro
  cabeçalhos e `Vary: Origin`.
- **SC-002** O mesmo preflight vindo de `https://atacante.net` volta 403 e sem `Allow-Origin`.
- **SC-003** `GET` de origem liberada volta 200 com `Allow-Origin` igual à origem; de origem
  não liberada, volta 200 com o corpo de sempre e sem `Allow-Origin`.
- **SC-004** `New` com `Origins: ["*"], Credentials: true` entra em pânico com mensagem que
  nomeia o campo.
- **SC-005** A suíte inteira continua verde com `CORS` zerado em todos os outros testes.
