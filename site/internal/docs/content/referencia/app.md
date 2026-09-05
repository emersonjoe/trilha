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
	CSRFForAPI   bool         // exigir CSRF também em route.go
	BasePath     string       // prefixo de URL; TRILHA_BASE_PATH
	Security     Security     // cabeçalhos (veja Segurança)
	TrustedProxies []string   // CIDRs; TRILHA_TRUSTED_PROXIES
	RateLimit    RateLimit    // limite global por cliente
	Secret, PreviousSecret []byte // TRILHA_SECRET, TRILHA_SECRET_PREVIOUS
	Timeouts     Timeouts     // limites do http.Server (trilha.NoTimeout desliga um)
	StaticCacheControl string // Cache-Control dos estáticos em prod ("public, max-age=3600")
	StaticHeaders func(name string, hdr http.Header) // cabeçalhos por arquivo estático
	OnSecurityEvent func(SecurityEvent)
}
```

`trilha.ConfigFromEnv()` lê as variáveis; `trilha.PublicFS(embutido, "public")` escolhe
entre a cópia embutida (prod) e a pasta no disco (dev).

### Onde configurar

O arquivo gerado faz `cfg := trilha.ConfigFromEnv()`, chama `app.Config(&cfg)` se
`app/setup.go` exportar `func Config(cfg *trilha.Config)`, e então `trilha.New(cfg)` e
`app.Setup(a)`. Você pode mexer em qualquer campo em qualquer um dos dois; a diferença é
só *quando* o valor é lido:

| Campos | Lidos em | `Config` | `Setup` (via `a.Config()`) |
|---|---|---|---|
| `Security`, `Public`, `MaxBodyBytes`, `CSRFForAPI`, `BasePath`, `OnSecurityEvent`, `StaticCacheControl`, `StaticHeaders` | a cada requisição | ✓ | ✓ |
| `Logger`, `Secret`/`PreviousSecret`, `RateLimit`, `TrustedProxies` | derivados em `New` e **reaplicados** ao começar a servir (`ListenAndServe`, `Handler`, `Export`) | ✓ | ✓ |
| `Addr`, `Timeouts` | `ListenAndServe` | ✓ | ✓ |
| `Env` | `New` (chave efêmera em dev) e por requisição | ✓ | parcial |

Use `Config` quando quiser montar a configuração a partir do seu próprio pacote (arquivo,
Vault, flags) em vez do ambiente.

### Timeouts

Zero significa "padrão"; `trilha.NoTimeout` desliga o limite (uploads grandes em rede
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
cfg.StaticCacheControl = "public, max-age=31536000, immutable" // assets com ?v= ou hash
cfg.StaticHeaders = func(name string, h http.Header) {
	if name == "robots.txt" { h.Set("Cache-Control", "no-store") }
	h.Set("Cross-Origin-Resource-Policy", "same-origin")
}
```

## App

| Método | Descrição |
|---|---|
| `New(cfg) *App` | cria a aplicação |
| `Register(Route)` | registra uma rota (chamado pelo arquivo gerado) |
| `SetRootLayout`, `SetNotFound`, `SetErrorPage` | ligam os arquivos da raiz |
| `Values() map[string]any` | valores globais definidos em `Setup` |
| `Logger() *slog.Logger` | o logger |
| `Env() Env` | ambiente |
| `Handler() http.Handler` | o mux raiz, para testes e para embutir em outro servidor |
| `ListenAndServe() error` | serve com desligamento gracioso em SIGINT/SIGTERM |
| `Routes() map[string][]string` | padrões registrados e seus métodos |
| `AddExportPath(paths...)` | caminhos extras para `Export` |
| `ExportPaths() []string` | o que `Export` vai renderizar |
| `Export(dir) error` | escreve o site estático |
| `BasePath() string` | prefixo de URL |
| `Security() *Security` | cabeçalhos, ajustáveis em `Setup` |
| `Config() *Config` | a configuração inteira, ajustável em `Setup` (veja "Onde configurar") |

`trilha.Run(a)` é o que o `main` gerado chama: exporta se `TRILHA_EXPORT` estiver definido,
senão serve. `trilha.Fatal(err)` registra e encerra, ignorando `http.ErrServerClosed`.

## Testar um app

O arquivo gerado define `newApp()`. Um teste no pacote `main` do projeto pode usá-lo:

```go
func TestHome(t *testing.T) {
	rec := httptest.NewRecorder()
	newApp().Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != 200 {
		t.Fatal(rec.Code)
	}
}
```
