---
title: App e Config
description: O que o arquivo gerado monta e o que você pode ajustar em setup.go.
---

## Config

```go
type Config struct {
	Addr         string       // ":3000"; PORT/ADDR no ambiente
	Env          Env          // Dev | Prod; TRILHA_ENV
	MaxBodyBytes int64        // 1 MiB
	Logger       *slog.Logger // slog.Default()
	Public       fs.FS        // arquivos estáticos; nil desliga
	Mounts       map[string]fs.FS // árvores estáticas por prefixo de URL, antes de Public
	CSRFForAPI   bool         // exigir CSRF também em route.go
	CSRF         CSRF         // nomes do cookie, do campo e do cabeçalho do token
	BasePath     string       // prefixo de URL; TRILHA_BASE_PATH
	Security     Security     // cabeçalhos (veja Segurança)
	TrustedProxies []string   // CIDRs; TRILHA_TRUSTED_PROXIES
	RateLimit    RateLimit    // limite global por cliente
	Secret, PreviousSecret []byte // TRILHA_SECRET, TRILHA_SECRET_PREVIOUS
	Timeouts     Timeouts     // limites do http.Server (trilha.NoTimeout desliga um)
	StaticCacheControl string // Cache-Control dos estáticos em prod ("public, max-age=3600")
	StaticHeaders func(name string, hdr http.Header) // cabeçalhos por arquivo estático
	LogRequest   func(c *Ctx, status int, dur time.Duration) bool // nil loga todas
	OnSecurityEvent func(SecurityEvent)
	DevReload    string       // trilha.Off desliga o script de recarga em dev; TRILHA_DEV_RELOAD=off
	Observability Observability // sondas de saúde e o endereço de métricas
	CORS         CORS         // origens que podem chamar o app (zero = desligado)
}
```

`trilha.ConfigFromEnv()` lê as variáveis; `trilha.PublicFS(embutido, "public")` escolhe
entre a cópia embutida (prod) e a pasta no disco (dev).

### Onde configurar

O arquivo gerado faz `cfg := trilha.ConfigFromEnv()`, chama `app.Config(&cfg)` se
`app/setup.go` exportar `func Config(cfg *trilha.Config)`, e então `trilha.New(cfg)` e
`app.Setup(a)`. `Config` também pode ser escrita como `func Config(cfg *trilha.Config) error`,
e aí o arquivo gerado interrompe a subida com a sua mensagem — ler a configuração do próprio
app é a operação que mais falha ao subir, e ela precisa poder falhar onde acontece. Você pode mexer em qualquer campo em qualquer um dos dois; a diferença é
só *quando* o valor é lido:

| Campos | Lidos em | `Config` | `Setup` (via `a.Config()`) |
|---|---|---|---|
| `Security`, `Public`, `MaxBodyBytes`, `CSRFForAPI`, `BasePath`, `OnSecurityEvent`, `StaticCacheControl`, `StaticHeaders` | a cada requisição | ✓ | ✓ |
| `Logger`, `Secret`/`PreviousSecret`, `RateLimit`, `TrustedProxies`, `CORS` | derivados em `New` e **reaplicados** ao começar a servir (`ListenAndServe`, `Handler`, `Export`) | ✓ | ✓ |
| `Addr`, `Timeouts` | `ListenAndServe` | ✓ | ✓ |
| `Env` | `New` (chave efêmera em dev) e por requisição | ✓ | parcial |

Use `Config` quando quiser montar a configuração a partir do seu próprio pacote (arquivo,
Vault, flags) em vez do ambiente.

### Nomes do CSRF

O token anda com três nomes, e cada um deles é um padrão, não uma regra:

| Campo | Padrão |
|---|---|
| `CSRF.Cookie` | `trilha_csrf` |
| `CSRF.Field` | `_csrf` |
| `CSRF.Header` | `X-CSRF-Token` |

```go
cfg.CSRF = trilha.CSRF{Cookie: "billing_csrf", Field: "_billing_csrf", Header: "X-Billing-CSRF"}
```

Troque quando o app não estiver sozinho na página: montado dentro de um servidor que já
escreve `_csrf`, dois campos escondidos com o mesmo nome chegam ao handler e o navegador
manda o cookie que quiser. Campo vazio fica com o padrão, então trocar um é uma linha. O nome
posto aqui é o que `CSRFInput`, `CSRFToken`, a verificação, a lista do CORS e o cliente de
teste usam — não existe um segundo lugar para manter em dia.

### CORS

`CORS` fica desligado enquanto `Origins` estiver vazio: nenhum cabeçalho novo, e o `OPTIONS`
continua chegando ao roteador.

| Campo | Papel |
|---|---|
| `Origins []string` | origens exatas (`https://app.exemplo.com`), ou a entrada única `"*"` |
| `Methods []string` | padrão `GET, HEAD, POST, PUT, PATCH, DELETE` |
| `Headers []string` | o que o cliente pode mandar; padrão `Content-Type, Authorization, X-CSRF-Token, Trilha-Fragment` |
| `Expose []string` | cabeçalhos de resposta que o script da outra origem pode ler |
| `Credentials bool` | libera cookie e `Authorization`; incompatível com `"*"` |
| `MaxAge time.Duration` | quanto o navegador guarda o preflight; zero omite o cabeçalho |

Política insegura ou malformada entra em pânico no `New` (`"*"` com `Credentials`, `"*"`
misturado com outras origens, origem com caminho, barra no fim ou sem esquema). O porquê
está em [Segurança](/pt/aprender/seguranca).

### Timeouts

`Timeouts.Shutdown` (5 s) é quanto `ListenAndServe` espera as requisições em andamento após
`SIGINT`/`SIGTERM`. Zero significa "padrão"; `trilha.NoTimeout` desliga o limite (uploads grandes em rede
lenta, long polling). `Write` vale para a resposta inteira: em vez de desligá-lo
globalmente, uma rota que transmite deve usar `c.Stream()` (SSE) ou `c.NoWriteDeadline()`.

```go
func Config(cfg *trilha.Config) {
	cfg.Timeouts.Read = trilha.NoTimeout // uploads de 32 MB do celular
}
```

### Estáticos

`StaticCacheControl` troca o `Cache-Control` de produção (dev sempre manda `no-cache`).
`StaticHeaders(nome, cabeçalhos)` roda depois, por arquivo, e pode mudar qualquer cabeçalho:

```go
cfg.StaticCacheControl = "public, max-age=31536000, immutable" // seguro com c.Asset
cfg.StaticHeaders = func(name string, h http.Header) {
	if name == "robots.txt" { h.Set("Cache-Control", "no-store") }
	h.Set("Cross-Origin-Resource-Policy", "same-origin")
}
```

### Árvores estáticas fora de `public/`

`Public` serve uma árvore só, na raiz, o que exige que as pastas no disco tenham o formato
das URLs. Quando não têm — um gerador de ícones que escreve em outro lugar, uma pasta
compartilhada com outro build — `Mounts` liga prefixo a árvore:

```go
cfg.Mounts = map[string]fs.FS{
	"/icones/": sub(embutidos, "static/publico/icons"),
	"/js/":     sub(embutidos, "static/js"),
}
```

As montagens são tentadas antes de `Public`, do prefixo mais longo para o mais curto; um
prefixo que casa sem ter o arquivo cai na próxima e depois em `Public`, então nenhuma
precisa ser exaustiva. `StaticCacheControl`, `StaticHeaders` e `Asset` tratam um arquivo
montado como qualquer outro, e o `name` que chega ao `StaticHeaders` é o da URL
(`icones/icon-192.png`), que é o que distingue uma montagem da outra.

### O log de requisição

Toda requisição casada por rota é logada. Num app que serve os próprios estáticos, a maior
parte desse volume diz "um arquivo foi servido com 200" — e log que ninguém lê não protege
ninguém. `LogRequest` decide, com a resposta já pronta:

```go
cfg.LogRequest = func(c *trilha.Ctx, status int, _ time.Duration) bool {
	return status >= 400 || !strings.HasPrefix(c.Request().URL.Path, "/js/")
}
```

Serve também para "não logar health check" e "amostrar 1% do tráfego". Arquivo servido por
`Public` ou por `Mounts` nunca passou por esse log.

### Versão no endereço (`Asset`)

```go
func (a *App) Asset(path string) string
func (c *Ctx) Asset(path string) string // idem, é o que o layout usa
```

`c.Asset("/site.css")` devolve `/site.css?v=8f3a1c92`, onde `v` é o hash FNV-1a do conteúdo
do arquivo em `Config.Public` (com o prefixo de `BasePath`, como `c.Base()`). Como o
endereço muda quando o arquivo muda, um deploy nunca deixa alguém com HTML novo e CSS
antigo — o navegador pede uma URL que ele nunca viu.

Um pedido cujo `v` confere recebe `public, max-age=31536000, immutable`, seja qual for o
`StaticCacheControl`; um `v` errado ou ausente cai na regra normal, e em `dev` nada é
imutável. O arquivo é lido uma vez em produção; em `dev` um `Stat` decide se relê, então
editar o CSS e atualizar a página basta.

Caminho que não existe em `Public` volta sem versão, com um aviso no log: um erro de
digitação no layout não derruba a página. `ui.Head` e os exemplos já usam `Asset`.

## App

| Método | Descrição |
|---|---|
| `New(cfg) *App` | cria a aplicação |
| `Register(Route)` | registra uma rota (chamado pelo arquivo gerado) |
| `SetRootLayout`, `SetNotFound`, `SetErrorPage` | ligam os arquivos da raiz |
| `trilha.Provide[T](a, v)` | guarda uma dependência sob o tipo dela (veja "Dependências") |
| `trilha.Use[T](b) T` | lê de volta, a partir de um `*Ctx` ou do `*App` |
| `Values() map[string]any` | valores globais definidos em `Setup`, por nome e sem tipo |
| `Logger() *slog.Logger` | o logger |
| `Env() Env` | ambiente |
| `Handler() http.Handler` | o mux raiz, para testes e para embutir em outro servidor |
| `ListenAndServe() error` | serve com desligamento gracioso em SIGINT/SIGTERM; depois roda os ganchos de `OnShutdown` |
| `OnShutdown(func(*App) error)` | registra o que fechar ao encerrar (pool, fila, flush); `setup.go` pode exportar `Shutdown`, que o arquivo gerado registra |
| `Routes() map[string][]string` | padrões registrados e seus métodos |
| `AddExportPath(paths...)` | caminhos extras para `Export` |
| `ExportPaths() []string` | o que `Export` vai renderizar |
| `Export(dir) error` | escreve o site estático |
| `BasePath() string` | prefixo de URL |
| `Security() *Security` | cabeçalhos, ajustáveis em `Setup` |
| `Config() *Config` | a configuração inteira, ajustável em `Setup` (veja "Onde configurar") |

`trilha.Run(a)` é o que o `main` gerado chama: exporta se `TRILHA_EXPORT` estiver definido,
senão serve. `trilha.Fatal(err)` registra e encerra, ignorando `http.ErrServerClosed`.

### Dependências

Uma página precisa do store, do pool, do mailer. Guardar isso em variáveis de pacote funciona
até o dia em que existem dois apps no mesmo processo — um hospedeiro que monta dois, ou um
teste que constrói o segundo — e aí os dois leem as mesmas globais, e o segundo teste a rodar
enxerga os dados do primeiro.

```go
func Setup(a *trilha.App) error {
	store := posts.New()
	trilha.Provide(a, store)
	return nil
}
```

```go
func Page(c *trilha.Ctx) (h.Node, error) {
	store := trilha.Use[*posts.Store](c)
	...
}
```

`Provide` guarda o valor sob o tipo dele; `Use[T]` lê de volta, e aceita tanto o `*Ctx` de um
handler quanto o próprio `*App` — que é o que `Setup` e um teste têm na mão. Um tipo que
ninguém proveu estoura na chamada, dizendo qual tipo é, em vez de aparecer depois como um nil
em outro lugar.

O tipo é a chave, então uma costura se declara escrevendo o tipo: `trilha.Provide[Mailer](a,
SMTPMailer{...})` guarda uma interface, e o handler que pede `Use[Mailer](c)` nunca fica
sabendo qual implementação recebeu. Sem o argumento de tipo a chave seria `SMTPMailer`, e o
handler estaria pedindo outra coisa.

`Values()` continua ali para cola por nome, e `c.Get`/`c.Set` são os valores por requisição
que um middleware deixa para trás — outra pergunta, respondida em
[Middleware](/pt/aprender/middleware).

### `main` próprio

Se algum arquivo do pacote `main` do projeto já declara `func main()`, o gerador omite o
dele e escreve só `newApp()`. Você fica com o controle do ciclo de vida:

```go
func main() {
	a := newApp()
	if err := migrar(a); err != nil { // entre o Setup e o servidor
		trilha.Fatal(err)
	}
	trilha.Run(a)
}
```

`public/` é opcional: o `//go:embed` só é gerado quando a pasta tem arquivos.

### Um app dentro de outro binário

Quando a pasta declara um pacote diferente de `main`, o arquivo gerado acompanha e exporta o
construtor:

```go
// internal/crm/trilha_gen.go → package crm, func NewApp() *trilha.App

mux := http.NewServeMux()
mux.HandleFunc("/legado", legado.Handler)
mux.Handle("/", crm.NewApp().Handler())
http.ListenAndServe(":8080", mux)
```

O `Handler()` devolve o `http.Handler` do app inteiro — roteamento, estáticos, middlewares e
páginas de erro — então o hospedeiro monta como monta qualquer handler. O `trilha gen` não
precisa de nada além do pacote que a pasta já declara; veja
[CLI](/pt/referencia/cli#um-app-dentro-de-um-binario-que-ja-existe).

## Testar um app

O arquivo gerado define `newApp()`, e o `package trilha` traz o cliente de teste, então um
teste no pacote `main` do projeto passa pelo app de verdade sem encanamento próprio:

```go
func TestHome(t *testing.T) {
	trilha.TestRequest(t, newApp(), "GET", "/").WantStatus(200).WantContains("<h1>")
}
```

| Símbolo | Papel |
|---|---|
| `TestingT` | `Helper()` e `Fatalf(...)`: o que os auxiliares usam de `*testing.T`, para o pacote nunca importar `testing` |
| `TestRequest(t, a *App, method, target string, opts ...TestOption) *TestResponse` | um pedido no app inteiro |
| `TestRoute(t, r Route, method, target string, opts ...TestOption) *TestResponse` | um `route.go`, com seus middlewares |
| `TestPage(t, r Route, target string, opts ...TestOption) *TestResponse` | uma página, com seus layouts; o `Node` vem preenchido |
| `NewTestClient(t, a *App) *TestClient` | o cliente com pote de cookies |
| `(*TestClient) Request / Get / PostForm / PostJSON` | os pedidos |
| `TestOption` | `WithApp`, `WithHeader`, `WithCookie`, `WithSigned`, `WithForm`, `WithJSON`, `WithBody`, `WithoutCSRF` |
| `TestResponse` | `Node`, `WantStatus`, `WantContains`, `WantHeader`, `JSON(&v)`, `Cookie(nome)`; embute o `*httptest.ResponseRecorder` |

Todo pedido leva o cookie do CSRF e, num método com corpo, o cabeçalho `X-CSRF-Token`
correspondente: cookie e token vêm do mesmo cliente, que é exatamente o que o duplo envio
pede de um navegador. O `WithoutCSRF()` é como um teste prova a recusa.

Nenhuma asserção devolve `error` — em teste, o valor de um erro é parar com a mensagem certa,
então a falha imprime o alvo, o status e o corpo. O que as asserções prontas não cobrem é um
`if` sobre o recorder embutido. Veja [Testes](/pt/aprender/testes) para a trilha inteira.
