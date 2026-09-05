# Contrato: API pública de `trilha`

```go
package trilha

type Env string // "dev" | "prod"

type Config struct {
    Addr         string      // ":3000"
    Env          Env         // TRILHA_ENV ou "prod"
    MaxBodyBytes int64       // 1 << 20
    Logger       *slog.Logger
    Public       fs.FS       // nil = sem estáticos
    CSRFForAPI   bool        // exigir CSRF também em route.go
}

type App struct { /* ... */ }
func New(cfg Config) *App
func (a *App) Values() map[string]any          // globais definidos por Setup
func (a *App) Handler() http.Handler
func (a *App) ListenAndServe() error             // graceful shutdown em SIGINT/SIGTERM

// Registro (usado só pelo trilha_gen.go)
type Route struct {
    Pattern     string
    Page        PageFunc
    Methods     map[string]HandlerFunc
    Layouts     []LayoutFunc     // de dentro para fora
    Middlewares []MiddlewareFunc // de fora para dentro
}
func (a *App) Register(r Route)
func (a *App) SetNotFound(f PageFunc)
func (a *App) SetErrorPage(f ErrorPageFunc)

type Next func() error
type PageFunc func(*Ctx) (h.Node, error)
type LayoutFunc func(*Ctx, h.Node) (h.Node, error)
type HandlerFunc func(*Ctx) error
type MiddlewareFunc func(*Ctx, Next) error
type ErrorPageFunc func(*Ctx, error) (h.Node, error)

var ErrNotFound = errors.New("trilha: not found")
type RedirectError struct{ URL string; Code int }
type HTTPError struct{ Code int; Message string }
func Redirect(url string) error            // 303
func RedirectCode(url string, code int) error
func Errorf(code int, format string, a ...any) error

type Ctx struct { /* ... */ }
func (c *Ctx) Request() *http.Request
func (c *Ctx) Writer() http.ResponseWriter
func (c *Ctx) Context() context.Context
func (c *Ctx) App() *App
func (c *Ctx) Env() Env
func (c *Ctx) Param(name string) string
func (c *Ctx) Query(name string) string
func (c *Ctx) Form(name string) string          // faz ParseForm (com limite) sob demanda
func (c *Ctx) BindJSON(v any) error             // 400 em JSON inválido
func (c *Ctx) Header(k, v string)
func (c *Ctx) Status(code int)                   // status da próxima escrita
func (c *Ctx) JSON(code int, v any) error
func (c *Ctx) Text(code int, s string) error
func (c *Ctx) HTML(code int, n h.Node) error     // sem layouts
func (c *Ctx) Redirect(url string) error         // atalho para return Redirect(url)
func (c *Ctx) Cookie(name string) (*http.Cookie, error)
func (c *Ctx) SetCookie(ck *http.Cookie)
func (c *Ctx) Set(key string, v any)
func (c *Ctx) Get(key string) any
func (c *Ctx) Title() string
func (c *Ctx) SetTitle(t string)
func (c *Ctx) CSRFToken() string                 // cria cookie se não existir
func (c *Ctx) RequestID() string
```
