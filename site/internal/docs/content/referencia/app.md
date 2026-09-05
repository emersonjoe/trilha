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
}
```

`trilha.ConfigFromEnv()` lê as variáveis; `trilha.PublicFS(embutido, "public")` escolhe
entre a cópia embutida (prod) e a pasta no disco (dev).

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
