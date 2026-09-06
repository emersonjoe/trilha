package trilha

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/emersonjoe/trilha/h"
)

// segredoDoServidor mora fora da raiz servida. Se ele aparecer numa resposta,
// algum caminho atravessou para fora de Public — que é o bug que este alvo
// procura.
const segredoDoServidor = "segredo-que-nao-pode-vazar"

func fuzzRouteApp(f *testing.F) http.Handler {
	f.Helper()
	dir := f.TempDir()
	pub := filepath.Join(dir, "public")
	if err := os.MkdirAll(filepath.Join(pub, "sub"), 0o755); err != nil {
		f.Fatal(err)
	}
	for name, data := range map[string]string{
		filepath.Join(pub, "site.css"):    "body{}",
		filepath.Join(pub, "sub", "a.js"): "console.log(1)",
		filepath.Join(dir, "segredo.txt"): segredoDoServidor,
	} {
		if err := os.WriteFile(name, []byte(data), 0o644); err != nil {
			f.Fatal(err)
		}
	}
	a := New(Config{Env: Prod, Logger: quiet(), Public: os.DirFS(pub),
		Secret: []byte("0123456789abcdef0123456789abcdef")})
	a.Register(Route{Pattern: "/", Layouts: []LayoutFunc{rootLayout}, Page: func(c *Ctx) (h.Node, error) {
		return h.P(h.Text("início")), nil
	}})
	a.Register(Route{Pattern: "/blog/{slug}", Layouts: []LayoutFunc{rootLayout}, Page: func(c *Ctx) (h.Node, error) {
		return h.P(h.Text(c.Param("slug"))), nil
	}})
	a.Register(Route{Pattern: "/arquivos/{path...}", Methods: map[string]HandlerFunc{
		"GET": func(c *Ctx) error { return c.Text(http.StatusOK, c.Param("path")) },
	}})
	a.Register(Route{Pattern: "/api/itens/{id}", Methods: map[string]HandlerFunc{
		"GET": func(c *Ctx) error { return c.JSON(http.StatusOK, map[string]string{"id": c.Param("id")}) },
	}})
	return a.Handler()
}

// FuzzRouteMatch: o alvo da requisição é a primeira coisa que um terceiro
// escolhe. Nenhum deles pode derrubar o app nem sair da raiz servida.
func FuzzRouteMatch(f *testing.F) {
	handler := fuzzRouteApp(f)
	for _, alvo := range []string{
		"/", "//", "/.", "/..", "/blog/ola", "/blog/", "/api/itens/7", "/arquivos/a/b",
		"/site.css", "/sub/a.js", "/../segredo.txt", "/arquivos/../../segredo.txt",
		"/%2e%2e/%2e%2e/segredo.txt", "/site.css?v=1", "/site.css/../../segredo.txt",
		"/_trilha/health", "/blog/" + strings.Repeat("a", 300), "/%00", "/\\..\\segredo.txt",
	} {
		f.Add(alvo)
	}
	f.Fuzz(func(t *testing.T, alvo string) {
		req, err := http.NewRequest(http.MethodGet, "http://exemplo.test"+alvo, nil)
		if err != nil {
			return // um alvo assim nem chega a um handler: o servidor recusa antes
		}
		req.RequestURI = alvo
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code < 100 || rec.Code >= 500 {
			t.Fatalf("%q: status = %d\n%s", alvo, rec.Code, rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), segredoDoServidor) {
			t.Fatalf("%q serviu um arquivo de fora da raiz", alvo)
		}
	})
}

// FuzzParseTraceparent: o cabeçalho vem de quem chama, e o que sai daqui vai
// para o log. Ou é um identificador hexadecimal que veio da entrada, ou é nada.
func FuzzParseTraceparent(f *testing.F) {
	for _, v := range []string{
		"", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
		"00-00000000000000000000000000000000-00f067aa0ba902b7-01",
		"00-4BF92F3577B34DA6A3CE929D0E0E4736-00f067aa0ba902b7-01",
		"00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01-extra",
		"00-4bf92f3577b34da6a3ce929d0e0e473g-00f067aa0ba902b7-01",
		"trace_id=\"; DROP TABLE\"", strings.Repeat("0", 55),
	} {
		f.Add(v)
	}
	f.Fuzz(func(t *testing.T, v string) {
		id := parseTraceparent(v)
		if id == "" {
			return
		}
		if len(id) != 32 {
			t.Fatalf("%q: id com %d caracteres", v, len(id))
		}
		if !hexOnly(id) {
			t.Fatalf("%q: id fora do alfabeto hexadecimal minúsculo: %q", v, id)
		}
		if !strings.Contains(v, id) {
			t.Fatalf("%q: id %q não veio da entrada", v, id)
		}
		if strings.Trim(id, "0") == "" {
			t.Fatalf("%q: id zerado aceito", v)
		}
	})
}

// FuzzSignedVerify: um cookie assinado é a única prova de identidade que o
// framework emite. O alvo afirma o que "válido" quer dizer — só passa o que
// alguma chave produziria, e só enquanto não vencer.
func FuzzSignedVerify(f *testing.F) {
	atual := []byte("0123456789abcdef0123456789abcdef")
	anterior := []byte("fedcba9876543210fedcba9876543210")
	s := NewSigner(atual, anterior)
	agora := time.Unix(1_700_000_000, 0)
	valido, _ := s.Sign("ana", agora.Add(time.Hour))
	comAnterior, _ := NewSigner(anterior).Sign("bia", agora.Add(time.Hour))
	vencido, _ := s.Sign("ana", agora.Add(-time.Hour))
	for _, tok := range []string{
		"", "|", "||", valido, comAnterior, vencido, valido + "x", strings.ToUpper(valido),
		strings.Replace(valido, "|", "|9999999999|", 1), "YW5h|9999999999|nao-e-mac",
	} {
		f.Add(tok, agora.Unix())
	}
	f.Fuzz(func(t *testing.T, token string, quando int64) {
		agora := time.Unix(quando, 0)
		valor, ok := s.Verify(token, agora)
		if !ok {
			return
		}
		partes := strings.Split(token, "|")
		if len(partes) != 3 {
			t.Fatalf("aceito com %d partes: %q", len(partes), token)
		}
		exp, err := strconv.ParseInt(partes[1], 10, 64)
		if err != nil {
			t.Fatalf("aceito com validade ilegível: %q", token)
		}
		if agora.Unix() > exp {
			t.Fatalf("aceito depois de vencer: %q em %d", token, quando)
		}
		for _, k := range [][]byte{atual, anterior} {
			if refeito, err := NewSigner(k).Sign(valor, time.Unix(exp, 0)); err == nil && refeito == token {
				return
			}
		}
		t.Fatalf("aceito um token que nenhuma chave produziria: %q (valor %q)", token, valor)
	})
}

// fuzzEntrada tem uma regra de cada tipo: se o Bind não devolve erro, todas
// valem.
type fuzzEntrada struct {
	Nome  string   `form:"nome" json:"nome" validate:"required,max=10"`
	Idade int      `form:"idade" json:"idade" validate:"min=0,max=150"`
	Tipo  string   `form:"tipo" json:"tipo" validate:"oneof=livro disco"`
	Tags  []string `form:"tag" json:"tags" validate:"max=3"`
}

func (e fuzzEntrada) conferir(t *testing.T, corpo string) {
	t.Helper()
	if e.Nome == "" || utf8.RuneCountInString(e.Nome) > 10 {
		t.Fatalf("%q: nome %q passou pelo required/max", corpo, e.Nome)
	}
	if e.Idade < 0 || e.Idade > 150 {
		t.Fatalf("%q: idade %d passou pelo min/max", corpo, e.Idade)
	}
	if e.Tipo != "" && e.Tipo != "livro" && e.Tipo != "disco" {
		t.Fatalf("%q: tipo %q passou pelo oneof", corpo, e.Tipo)
	}
	if len(e.Tags) > 3 {
		t.Fatalf("%q: %d tags passaram pelo max", corpo, len(e.Tags))
	}
}

// bindFuzzApp responde 200 com o que ligou, para o alvo conferir de fora.
func bindFuzzApp(bind func(c *Ctx, v any) error) http.Handler {
	a := New(Config{Logger: quiet()})
	a.Register(Route{Pattern: "/api/x", Methods: map[string]HandlerFunc{
		"POST": func(c *Ctx) error {
			var in fuzzEntrada
			if err := bind(c, &in); err != nil {
				return err
			}
			return c.JSON(http.StatusOK, in)
		},
	}})
	return a.Handler()
}

func bindFuzzPost(t *testing.T, handler http.Handler, contentType, corpo string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/x", strings.NewReader(corpo))
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code >= 500 {
		t.Fatalf("%q: status = %d\n%s", corpo, rec.Code, rec.Body.String())
	}
	if rec.Code != http.StatusOK {
		return // recusou, que é metade do contrato
	}
	var out fuzzEntrada
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("%q: resposta ilegível: %v", corpo, err)
	}
	out.conferir(t, corpo)
}

// FuzzBindForm: corpo de formulário é entrada de terceiro. Ou o Bind recusa,
// ou o que ele entrega respeita as regras de validate — nunca as duas coisas.
func FuzzBindForm(f *testing.F) {
	handler := bindFuzzApp(func(c *Ctx, v any) error { return c.Bind(v) })
	for _, corpo := range []string{
		"", "nome=ana", "nome=ana&idade=30&tipo=livro&tag=a&tag=b",
		"nome=" + strings.Repeat("a", 40), "idade=-1", "idade=999999999999999999999",
		"tipo=outro", "tag=a&tag=b&tag=c&tag=d", "nome=%C3%A1", "nome=a&nome=b", "%zz=1",
		"nome=ana&idade=trinta", ";", "&&&", "nome=a\x00b",
	} {
		f.Add(corpo)
	}
	f.Fuzz(func(t *testing.T, corpo string) {
		bindFuzzPost(t, handler, "application/x-www-form-urlencoded", corpo)
	})
}

// FuzzBindJSON: idem para JSON, incluindo corpo truncado e tipo trocado.
func FuzzBindJSON(f *testing.F) {
	handler := bindFuzzApp(func(c *Ctx, v any) error { return c.BindJSON(v) })
	for _, corpo := range []string{
		"", "{}", `{"nome":"ana"}`, `{"nome":"ana","idade":30,"tipo":"livro","tags":["a"]}`,
		`{"nome":"` + strings.Repeat("a", 40) + `"}`, `{"idade":"trinta"}`, `{"idade":1e400}`,
		`{"tags":["a","b","c","d"]}`, `{"nome":null}`, `{"nome":"ana"`, `[1,2,3]`, "null",
		`{"desconhecido":1}`, `{"nome":"\ud800"}`,
	} {
		f.Add(corpo)
	}
	f.Fuzz(func(t *testing.T, corpo string) {
		bindFuzzPost(t, handler, "application/json", corpo)
	})
}
