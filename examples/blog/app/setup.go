package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/cache"
	"github.com/emersonjoe/trilha/examples/blog/internal/icones"
	"github.com/emersonjoe/trilha/examples/blog/internal/posts"
)

// Config runs before trilha.New: the place to change fields New derives
// from (Logger, Secret, RateLimit, TrustedProxies) or to read your own config.
// Devolver erro é opcional; é o que permite falhar onde a configuração é
// lida, que é a operação que mais falha na subida.
func Config(cfg *trilha.Config) error {
	// The layout links assets through c.Asset, which puts the content hash in
	// the URL: an address that changes with the file can be cached forever,
	// and a deploy never leaves anyone with new HTML and old CSS.
	cfg.StaticCacheControl = "public, max-age=31536000, immutable"
	// As sondas /_trilha/health/live e /ready já existem sem configuração. O
	// endereço de métricas é opt-in: aqui ele só aparece quando o ambiente
	// traz TRILHA_METRICS, e quem raspa precisa de TRILHA_OBS_TOKEN.
	cfg.Observability.CacheFor = 2 * time.Second
	// Os ícones não moram em public/: quem os gera escreve em internal/icones.
	// A montagem liga o prefixo de URL à árvore de disco, sem mexer em nenhuma
	// das duas.
	cfg.Mounts = map[string]fs.FS{"/icones/": icones.FS()}
	// O log de requisição fica com o que alguém vai ler: arquivo estático
	// servido com 200 é a maior parte do volume e não diz nada.
	cfg.LogRequest = func(c *trilha.Ctx, status int, _ time.Duration) bool {
		return status >= 400 || !strings.HasPrefix(c.Request().URL.Path, "/icones/")
	}
	// Aqui é onde a configuração do próprio app é lida — e onde ela falha.
	if v := os.Getenv("BLOG_CACHE_SEGUNDOS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("BLOG_CACHE_SEGUNDOS: %w", err)
		}
		cfg.Observability.CacheFor = time.Duration(n) * time.Second
	}
	return nil
}

// Setup runs once before the server starts.
func Setup(a *trilha.App) error {
	// O app fala português; as mensagens de validação também.
	trilha.UseValidationPTBR()
	// O cache é do app, não do framework: quem o cria decide o teto, o nome
	// que aparece em /metrics e quem enxerga a variável. Aqui ele vive no
	// pacote que produz as listas, ao lado das escritas que o derrubam.
	posts.Cache = cache.New(cache.Options{Name: "posts", MaxEntries: 500, Metrics: a.Metrics()})
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
