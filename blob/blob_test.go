package blob

import (
	"context"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha"
)

// upload monta o que o c.File entregaria, sem passar por uma requisição.
func upload(t *testing.T, nome, ctype, conteudo string) *trilha.Upload {
	t.Helper()
	dir := t.TempDir()
	caminho := filepath.Join(dir, "x")
	if err := os.WriteFile(caminho, []byte(conteudo), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(caminho)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return &trilha.Upload{Name: nome, MIME: ctype, Size: int64(len(conteudo)), File: multipartFile{f}}
}

type multipartFile struct{ *os.File }

func (multipartFile) private() {}

var _ multipart.File = multipartFile{}

// A chave é o conteúdo, não o nome. É isso que faz travessia de caminho ser
// impossível por construção, e não uma checagem que alguém pode esquecer.
func TestChaveVemDoConteudoENaoDoNome(t *testing.T) {
	f := New(NewMemory())
	up := upload(t, "../../etc/passwd", "text/plain", "oi")
	ref, err := f.Put(context.Background(), up)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(ref.Key, "..") || strings.Contains(ref.Key, "passwd") {
		t.Fatalf("chave = %q", ref.Key)
	}
	// Dois níveis, o digest e a extensão do tipo que foi farejado.
	if !strings.HasPrefix(ref.Key, ref.SHA256[:2]+"/"+ref.SHA256[2:4]+"/") || !strings.HasSuffix(ref.Key, ".txt") {
		t.Fatalf("formato = %q", ref.Key)
	}
	// E o nome original continua lá, para mostrar.
	if ref.Name != "../../etc/passwd" {
		t.Fatalf("nome = %q", ref.Name)
	}
}

// O mesmo conteúdo duas vezes é um arquivo só. Deduplicação de graça — e o que
// isso significa para o Delete é decisão da aplicação, não deste pacote.
func TestMesmoConteudoMesmaChave(t *testing.T) {
	f := New(NewMemory())
	a, err := f.Put(context.Background(), upload(t, "um.txt", "text/plain", "igual"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := f.Put(context.Background(), upload(t, "outro.txt", "text/plain", "igual"))
	if err != nil {
		t.Fatal(err)
	}
	if a.Key != b.Key {
		t.Fatalf("%q != %q", a.Key, b.Key)
	}
}

// Uma chave que não saiu daqui não passa. É a porta pela qual uma travessia
// entraria, e ela está fechada antes de qualquer store ver a string.
func TestChaveDeForaEhRecusada(t *testing.T) {
	f := New(NewMemory())
	for _, ruim := range []string{
		"../etc/passwd", "/etc/passwd", "ab/cd/../../../x", "ab\\cd\\x",
		"AB/CD/maiuscula.txt", "curta",
	} {
		if _, err := f.Get(context.Background(), ruim); err == nil {
			t.Fatalf("aceitou %q", ruim)
		}
		if err := f.Delete(context.Background(), ruim); err == nil {
			t.Fatalf("apagou com %q", ruim)
		}
	}
}

// Guardar, ler de volta, apagar. E apagar duas vezes não é erro: duas
// requisições apagando a mesma coisa não deviam precisar de um lock.
func TestGuardaLeApaga(t *testing.T) {
	for nome, store := range map[string]Store{"memória": NewMemory(), "disco": NewDisk(t.TempDir())} {
		t.Run(nome, func(t *testing.T) {
			f := New(store)
			ctx := context.Background()
			ref, err := f.Put(ctx, upload(t, "nota.txt", "text/plain", "conteúdo"))
			if err != nil {
				t.Fatal(err)
			}
			r, err := f.Get(ctx, ref.Key)
			if err != nil {
				t.Fatal(err)
			}
			b := make([]byte, 32)
			n, _ := r.Read(b)
			r.Close()
			if string(b[:n]) != "conteúdo" {
				t.Fatalf("voltou %q", b[:n])
			}
			if info, err := f.Stat(ctx, ref.Key); err != nil || info.Size != ref.Size {
				t.Fatalf("%v %+v", err, info)
			}
			if err := f.Delete(ctx, ref.Key); err != nil {
				t.Fatal(err)
			}
			if err := f.Delete(ctx, ref.Key); err != nil {
				t.Fatalf("apagar de novo devia ser silêncio: %v", err)
			}
			if _, err := f.Get(ctx, ref.Key); err == nil {
				t.Fatal("continuou lá")
			}
		})
	}
}

// A varredura de órfãos: o que está no storage e não está na lista de quem
// aponta para ele. É a linha apagada com o objeto ficando para trás.
func TestOrfaos(t *testing.T) {
	f := New(NewMemory())
	ctx := context.Background()
	usado, _ := f.Put(ctx, upload(t, "a.txt", "text/plain", "usado"))
	orfao, _ := f.Put(ctx, upload(t, "b.txt", "text/plain", "esquecido"))

	got, err := f.Orphans(ctx, func(yield func(string) bool) { yield(usado.Key) })
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != orfao.Key {
		t.Fatalf("órfãos = %v", got)
	}
}

// Um arquivo escrito pela metade não vira uma chave que existe e não abre.
func TestDiscoNaoDeixaParcialVirarChave(t *testing.T) {
	dir := t.TempDir()
	d := NewDisk(dir)
	os.MkdirAll(filepath.Join(dir, "ab", "cd"), 0o700)
	os.WriteFile(filepath.Join(dir, "ab", "cd", ".partial-123"), []byte("metade"), 0o600)

	var keys []string
	if err := d.Keys(context.Background(), func(k string) error {
		keys = append(keys, k)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(keys) != 0 {
		t.Fatalf("o parcial virou chave: %v", keys)
	}
}

// O Serve entrega pelo caminho do Ctx, com o nome que a aplicação escolheu e o
// tipo que ficou guardado — nunca o que o cliente disse no upload.
func TestServeEntregaComNomeETipo(t *testing.T) {
	f := New(NewMemory())
	ref, _ := f.Put(context.Background(), upload(t, "nota fiscal.txt", "text/plain", "linha"))

	app := trilha.New(trilha.Config{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	app.Register(trilha.Route{Pattern: "/arquivo", Kind: trilha.KindAPI,
		Methods: map[string]trilha.HandlerFunc{"GET": func(c *trilha.Ctx) error {
			return f.Serve(c, ref.Key, ServeOpts{Name: ref.Name})
		}}})
	rec := httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/arquivo", nil))

	if rec.Code != 200 || rec.Body.String() != "linha" {
		t.Fatalf("%d %q", rec.Code, rec.Body.String())
	}
	if d := rec.Header().Get("Content-Disposition"); !strings.Contains(d, "nota") {
		t.Fatalf("Content-Disposition = %q", d)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Fatalf("Content-Type = %q", ct)
	}
}

// Chave que não existe é 404 e não 500: pedir um arquivo apagado é uma coisa
// que acontece, não uma falha do servidor.
func TestServeDeChaveQueNaoExisteEh404(t *testing.T) {
	f := New(NewMemory())
	app := trilha.New(trilha.Config{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	app.Register(trilha.Route{Pattern: "/arquivo", Kind: trilha.KindAPI,
		Methods: map[string]trilha.HandlerFunc{"GET": func(c *trilha.Ctx) error {
			return f.Serve(c, "ab/cd/abcdef0123456789.txt", ServeOpts{})
		}}})
	rec := httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/arquivo", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("%d", rec.Code)
	}
}
