package trilha

import (
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func assetApp(t *testing.T, env Env, files fstest.MapFS) *App {
	t.Helper()
	return New(Config{Env: env, Public: files, Logger: quiet(),
		Secret: []byte("0123456789abcdef0123456789abcdef")})
}

// FR-001/FR-002: a versão vem do conteúdo, então é estável entre chamadas e
// entre processos — uma publicação que não mudou nada não invalida cache.
func TestAssetVersionComesFromContent(t *testing.T) {
	files := fstest.MapFS{"site.css": &fstest.MapFile{Data: []byte("body{}")}}
	a := assetApp(t, Prod, files)
	got := a.Asset("/site.css")
	if !strings.HasPrefix(got, "/site.css?v=") {
		t.Fatalf("sem versão: %q", got)
	}
	if again := a.Asset("/site.css"); again != got {
		t.Fatalf("versão instável: %q depois %q", got, again)
	}
	outro := assetApp(t, Prod, fstest.MapFS{"site.css": &fstest.MapFile{Data: []byte("body{}")}})
	if outro.Asset("/site.css") != got {
		t.Fatal("mesmo conteúdo deveria dar a mesma versão em outro processo")
	}
	diff := assetApp(t, Prod, fstest.MapFS{"site.css": &fstest.MapFile{Data: []byte("body{color:red}")}})
	if diff.Asset("/site.css") == got {
		t.Fatal("conteúdo diferente com a mesma versão")
	}
}

// O prefixo de BasePath vem junto: Asset substitui Base()+caminho.
func TestAssetCarriesBasePath(t *testing.T) {
	a := New(Config{Env: Prod, BasePath: "/trilha", Logger: quiet(),
		Public: fstest.MapFS{"site.css": &fstest.MapFile{Data: []byte("x")}}})
	if got := a.Asset("/site.css"); !strings.HasPrefix(got, "/trilha/site.css?v=") {
		t.Fatalf("%q", got)
	}
}

// FR-004: um erro de digitação não pode derrubar a página.
func TestAssetUnknownPathStaysUnversioned(t *testing.T) {
	var buf strings.Builder
	a := New(Config{Env: Prod, Logger: slog.New(slog.NewTextHandler(&buf, nil)),
		Public: fstest.MapFS{"site.css": &fstest.MapFile{Data: []byte("x")}}})
	for _, p := range []string{"/nao-existe.css", "/../fora.css", "/"} {
		if got := a.Asset(p); got != p {
			t.Errorf("%q → %q, esperado o caminho intacto", p, got)
		}
	}
	if !strings.Contains(buf.String(), "nao-existe.css") {
		t.Fatalf("o aviso precisa dizer qual caminho: %s", buf.String())
	}
	n := strings.Count(buf.String(), "nao-existe.css")
	a.Asset("/nao-existe.css")
	if strings.Count(buf.String(), "nao-existe.css") != n {
		t.Fatal("o aviso deve sair uma vez por caminho, não a cada renderização")
	}
	// Sem Public configurado, o caminho também sai intacto.
	b := New(Config{Env: Prod, Logger: quiet()})
	if got := b.Asset("/site.css"); got != "/site.css" {
		t.Fatalf("%q", got)
	}
}

// FR-002: em dev, editar o arquivo muda a versão sem reiniciar o servidor.
func TestAssetRereadsInDev(t *testing.T) {
	f := &fstest.MapFile{Data: []byte("body{}"), ModTime: time.Now()}
	files := fstest.MapFS{"site.css": f}
	dev := assetApp(t, Dev, files)
	antes := dev.Asset("/site.css")
	f.Data = []byte("body{color:red}")
	f.ModTime = f.ModTime.Add(time.Second)
	if depois := dev.Asset("/site.css"); depois == antes {
		t.Fatal("dev precisa perceber a alteração")
	}

	// Em prod o arquivo é lido uma vez: o binário e o public/ andam juntos.
	g := &fstest.MapFile{Data: []byte("body{}"), ModTime: time.Now()}
	prod := assetApp(t, Prod, fstest.MapFS{"site.css": g})
	antes = prod.Asset("/site.css")
	g.Data = []byte("body{color:red}")
	g.ModTime = g.ModTime.Add(time.Second)
	if prod.Asset("/site.css") != antes {
		t.Fatal("prod não deveria reler a cada renderização")
	}
}

// FR-003: só o v correto libera o cache de um ano.
func TestStaticCacheControlDependsOnVersion(t *testing.T) {
	files := fstest.MapFS{"site.css": &fstest.MapFile{Data: []byte("body{}")}}
	a := New(Config{Env: Prod, Public: files, Logger: quiet(),
		StaticCacheControl: "public, max-age=600"})
	v := strings.TrimPrefix(a.Asset("/site.css"), "/site.css?v=")

	cases := []struct{ url, want string }{
		{"/site.css?v=" + v, "public, max-age=31536000, immutable"},
		{"/site.css?v=deadbeef", "public, max-age=600"},
		{"/site.css", "public, max-age=600"},
	}
	for _, tc := range cases {
		rec := httptest.NewRecorder()
		a.Handler().ServeHTTP(rec, httptest.NewRequest("GET", tc.url, nil))
		if rec.Code != 200 {
			t.Fatalf("%s → %d", tc.url, rec.Code)
		}
		if got := rec.Header().Get("Cache-Control"); got != tc.want {
			t.Errorf("%s → %q, esperado %q", tc.url, got, tc.want)
		}
	}
	// Em dev nada é imutável, senão o navegador segura o CSS enquanto se edita.
	d := New(Config{Env: Dev, Public: files, Logger: quiet()})
	rec := httptest.NewRecorder()
	d.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/site.css?v="+v, nil))
	if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("dev → %q", got)
	}
}

// Ctx.Asset é o que o layout usa; precisa dar o mesmo que App.Asset.
func TestCtxAsset(t *testing.T) {
	files := fstest.MapFS{"site.css": &fstest.MapFile{Data: []byte("body{}")}}
	a := New(Config{Env: Prod, Public: files, Logger: quiet()})
	var got string
	a.Register(Route{Pattern: "/", Methods: map[string]HandlerFunc{
		"GET": func(c *Ctx) error { got = c.Asset("/site.css"); return c.Text(200, "ok") }}})
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	if want := a.Asset("/site.css"); got != want {
		t.Fatalf("Ctx.Asset %q, App.Asset %q", got, want)
	}
}

// Issue #26: o arquivo estático ganha ETag da mesma impressão digital que já
// vai no "?v=" — data de modificação sozinha invalida tudo num container novo,
// onde o clone reescreve arquivos idênticos com a hora de agora.
func TestStaticETagAnswers304(t *testing.T) {
	files := fstest.MapFS{"site.css": &fstest.MapFile{Data: []byte("body{}")}}
	a := New(Config{Env: Prod, Public: files, Logger: quiet()})

	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/site.css", nil))
	tag := rec.Header().Get("ETag")
	if rec.Code != 200 || tag == "" {
		t.Fatalf("%d %q", rec.Code, tag)
	}
	if want := `"` + strings.TrimPrefix(a.Asset("/site.css"), "/site.css?v=") + `"`; tag != want {
		t.Fatalf("ETag %s, esperado %s (a mesma versão da URL)", tag, want)
	}

	again := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/site.css", nil)
	req.Header.Set("If-None-Match", tag)
	a.Handler().ServeHTTP(again, req)
	if again.Code != 304 || again.Body.Len() != 0 {
		t.Fatalf("revalidação: %d, %d bytes", again.Code, again.Body.Len())
	}

	// Conteúdo diferente, etiqueta diferente: ninguém fica com o CSS velho.
	other := New(Config{Env: Prod, Logger: quiet(),
		Public: fstest.MapFS{"site.css": &fstest.MapFile{Data: []byte("body{color:red}")}}})
	rec2 := httptest.NewRecorder()
	other.Handler().ServeHTTP(rec2, httptest.NewRequest("GET", "/site.css", nil))
	if rec2.Header().Get("ETag") == tag {
		t.Fatal("dois conteúdos, uma etiqueta")
	}
}
