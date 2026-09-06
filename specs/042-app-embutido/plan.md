# Plano — App embutido: pacote gerado, middleware por método e 4xx com a cara do app

## Fatos que decidem o desenho

1. **`findProject` já usa o diretório corrente como raiz** (`cmd/trilha/main.go:73`): quem roda
   `trilha gen` dentro de `internal/crm` gera ali, com o módulo certo (`go.mod` da raiz +
   caminho relativo). O que falta não é achar o lugar — é o pacote do arquivo escrito.

2. **`rootHasMain` já parseia os `.go` da raiz** (`internal/scan/scan.go:459`) ignorando
   `_test.go` e `trilha_gen.go`. Ler o nome do pacote é a mesma passada, no mesmo lugar: o
   `Result` ganha um campo, o gerador imprime esse campo, e ninguém precisa de bandeira para
   o caso comum.

3. **A bandeira precisa sobreviver ao CI.** `trilha gen --check`, `dev` e `build` regeneram sem
   as bandeiras do primeiro `gen`; se `--package` só existisse na linha de comando, o `--check`
   acusaria o arquivo como velho todo dia. Por isso a precedência cai no próprio
   `trilha_gen.go`: o arquivo gerado é onde a escolha fica guardada, e não há estado novo em
   lugar nenhum.

4. **`wrap` já é por (rota, método)** (`serve.go:37` e `serve.go:58`): `Register` chama
   `a.wrap(&route, kind, fn)` uma vez por método e uma vez para o `Page`. A cadeia por método
   entra aí, sem tocar no `run`, que continua sendo a única coisa que sabe encadear.

5. **O CSRF roda dentro do `final`**, depois de todos os middlewares (`serve.go:92`). Um
   `MiddlewarePOST` de permissão, então, responde 403 antes de o token ser conferido — que é a
   mesma ordem dos middlewares de rota de hoje, e a ordem certa: quem não pode agir não deveria
   depender de ter trazido um token válido.

6. **`renderErrorPage` fixa `c.status = 500`** (`render.go:195`). Receber o código em vez de
   assumi-lo é a mudança inteira: os dois ramos que já passam por `wrapRoot` continuam iguais,
   e o terceiro ramo do `switch` — o que hoje escreve `simplePage` sem passar pelo app — deixa
   de existir como caminho próprio e vira o fallback que já existia dentro do `renderErrorPage`.

7. **`statusOf` já é a classificação oficial** (`errors.go:48`) e é o que o `handleError` usa
   para decidir o status. Exportá-la é preferível a um `c.ErrorStatus()`: o `error.go` recebe o
   erro, não o status, e a mesma função responde em teste, fora de um `Ctx`.

## Fases

### Fase 1 — pacote do arquivo gerado (#51)

`scan.Result.Package` (`"main"` quando nada diz o contrário) preenchido por um `rootPackage`
irmão do `rootHasMain`; `Result.HasMain` continua como está, mas o gerador só considera
`func main()` quando o pacote é `main`. O template troca `package main` fixo por `{{.Package}}`
e o nome do construtor por `newApp`/`NewApp` conforme o pacote. `render`/`generate` em
`cmd/trilha/main.go` passam a aceitar o `--package` do `cmdGen`, aplicado sobre o `Result`
antes do `gen.Generate`. `cmdDev`/`cmdBuild` recusam pacote ≠ `main` com mensagem traduzida.

Golden novo (`testdata/apps/embedded` → `embedded.go.golden`) com um `.go` à mão declarando
`package crm`, para o caso de detecção ficar preso no teste e não na cabeça de quem leu a spec.

### Fase 2 — middleware por método (#56)

`scan.Route.MiddlewaresByMethod map[string][]Ref`, herdado pela subárvore junto do
`Middlewares`. `walk` passa a acumular um segundo mapa; a validação de `middleware.go` aceita
`Middleware` ou qualquer `MiddlewareX`. Depois do walk, uma verificação cruza cada
`MiddlewareX` com as rotas da subárvore e emite `E_UNUSED_METHOD_MIDDLEWARE` quando nenhuma
serve aquele método. O gerador emite o mapa; `Register` compõe `Middlewares` + o do método na
hora de montar cada `wrap`.

### Fase 3 — `error.go` para todo status (#53)

`trilha.StatusOf` exportada (`statusOf` vira uma chamada dela). `renderErrorPage(c, cause, code)`
com o `simplePage` de fallback escolhendo título e detalhe pelo código — 5xx mantém o texto e o
stack em `Dev`, o resto mantém `http.StatusText` e a mensagem do `HTTPError`, que é exatamente o
que o ramo removido escrevia. `handleError` fica com dois ramos: 404 e todo o resto.

### Fase 4 — documentação e release

`learn/middleware`, `learn/troubleshooting`, `reference/conventions`, `reference/errors`,
`reference/app`, `reference/cli` e `cookbook/migration` nas duas locales; CHANGELOG, ROADMAP,
`version` e `make release VERSION=0.32.0 ISSUES="51 53 56"`.

## Constitution Check

- **I (convenção antes de configuração)**: `MiddlewarePOST` é nome de função, não registro; o
  pacote do arquivo gerado é lido do diretório, não configurado. A única bandeira nova existe
  para o diretório que ainda não tem `.go` nenhum, e some do uso depois da primeira vez.
- **II (só a biblioteca padrão)**: nenhuma dependência nova.
- **III (o erro diz o conserto)**: `E_UNUSED_METHOD_MIDDLEWARE` nomeia o arquivo, o método e a
  subárvore vazia; `dev`/`build` em pacote embutido dizem que quem roda é o hospedeiro.
- **IV (nada quebra em silêncio)**: `Route` ganha campo, `ErrorPageFunc` não muda de assinatura,
  `newApp` continua `newApp` em `package main`. O 4xx muda de aparência — está no CHANGELOG
  como mudança de comportamento, com a página interna preservada para quem não tem `error.go`.
- **V (o teste prova a convenção)**: golden do app embutido, teste de ordem da cadeia por
  método, teste do 403 passando pelo `error.go` com layout.
