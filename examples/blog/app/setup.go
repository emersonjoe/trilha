package app

import (
	"context"
	"errors"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/blog/internal/posts"
)

// Config runs before trilha.New: the place to change fields New derives
// from (Logger, Secret, RateLimit, TrustedProxies) or to read your own config.
func Config(cfg *trilha.Config) {
	// The layout links assets through c.Asset, which puts the content hash in
	// the URL: an address that changes with the file can be cached forever,
	// and a deploy never leaves anyone with new HTML and old CSS.
	cfg.StaticCacheControl = "public, max-age=31536000, immutable"
	// As sondas /_trilha/health/live e /ready já existem sem configuração. O
	// endereço de métricas é opt-in: aqui ele só aparece quando o ambiente
	// traz TRILHA_METRICS, e quem raspa precisa de TRILHA_OBS_TOKEN.
	cfg.Observability.CacheFor = 2 * time.Second
}

// Setup runs once before the server starts.
func Setup(a *trilha.App) error {
	posts.Seed()
	a.Values()["site"] = "Trilha Blog"
	// Prontidão: o app só serve depois que o "banco" (aqui, a memória de
	// posts) responde. A verificação tem prazo e o resultado fica em cache,
	// então a sonda não vira carga.
	a.Check("posts", func(ctx context.Context) error {
		if posts.Count() == 0 {
			return errors.New("nenhum post carregado")
		}
		return nil
	})
	// Métrica de domínio: aparece na raspagem junto com as do framework.
	posts.Published = a.Metrics().Counter("blog_posts_total", "Posts publicados desde o início do processo.")
	// Limite global brando; /api tem o seu próprio em app/api/middleware.go.
	a.Security().CSPExtra = map[string][]string{"img-src": {"https:"}}
	return nil
}
