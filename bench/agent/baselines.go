package main

// The hidden tests of the baseline side. They run in the baseline fixture's
// package main, against the handler() http.Handler every baseline exposes —
// the stdlib counterpart of the trilha fixtures' newApp(). Same contracts as
// the trilha side, stdlib tools only.

const baselineCommentsTest = `package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBenchBaselineComments(t *testing.T) {
	srv := httptest.NewServer(handler())
	defer srv.Close()

	post := func(body string) *http.Response {
		res, err := http.Post(srv.URL+"/api/posts/ola-trilha/comments", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		return res
	}

	res := post(` + "`" + `{"author":"Ana","body":"Primeiro!"}` + "`" + `)
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("POST = %d, want 201", res.StatusCode)
	}
	var criado map[string]any
	if err := json.NewDecoder(res.Body).Decode(&criado); err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if criado["author"] != "Ana" || criado["body"] != "Primeiro!" {
		t.Fatalf("resposta = %v, want the comment echoed", criado)
	}
	if _, ok := criado["created"]; !ok {
		t.Fatalf("sem created na resposta: %v", criado)
	}

	res = post(` + "`" + `{"author":"","body":""}` + "`" + `)
	if res.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("inválido = %d, want 422", res.StatusCode)
	}
	res.Body.Close()

	res = post(` + "`" + `{"author":"Ana","body":"` + "`" + ` + strings.Repeat("x", 501) + ` + "`" + `"}` + "`" + `)
	if res.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("corpo de 501 = %d, want 422", res.StatusCode)
	}
	res.Body.Close()

	res, err := http.Post(srv.URL+"/api/posts/nao-existe/comments", "application/json",
		strings.NewReader(` + "`" + `{"author":"Ana","body":"x"}` + "`" + `))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("post inexistente = %d, want 404", res.StatusCode)
	}
	res.Body.Close()

	res, err = http.Get(srv.URL + "/api/posts/ola-trilha/comments")
	if err != nil {
		t.Fatal(err)
	}
	var lista []map[string]any
	if err := json.NewDecoder(res.Body).Decode(&lista); err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	aqui := false
	for _, c := range lista {
		if c["author"] == "Ana" && c["body"] == "Primeiro!" {
			aqui = true
		}
	}
	if !aqui {
		t.Fatalf("GET = %v, want the comment by Ana in it", lista)
	}
}
`

const baselineContactFormTest = `package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestBenchBaselineContato(t *testing.T) {
	srv := httptest.NewServer(handler())
	defer srv.Close()

	res, err := http.Get(srv.URL + "/contato")
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /contato = %d", res.StatusCode)
	}
	// "Preços" is a link of the layout: the page must be inside it.
	for _, quero := range []string{"<form", "Preços"} {
		if !strings.Contains(string(b), quero) {
			t.Fatalf("a página não tem %q", quero)
		}
	}

	post := func(v url.Values) *http.Response {
		res, err := http.PostForm(srv.URL+"/contato", v)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	if res = post(url.Values{"nome": {"Ana"}, "email": {"ana@example.com"}, "mensagem": {"Olá"}}); res.StatusCode != http.StatusOK && res.StatusCode != http.StatusSeeOther {
		t.Fatalf("POST válido = %d, want 200 or 303", res.StatusCode)
	}
	res.Body.Close()
	if res = post(url.Values{"nome": {""}, "email": {"nao-e-email"}, "mensagem": {""}}); res.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("POST inválido = %d, want 422", res.StatusCode)
	}
	res.Body.Close()
}
`

const baselinePaginationTest = `package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

var hrefPost = regexp.MustCompile(` + "`" + `href="/blog/[^"]+"` + "`" + `)

func quantos(body string) int { return len(hrefPost.FindAllString(body, -1)) }

func TestBenchBaselinePaginacao(t *testing.T) {
	srv := httptest.NewServer(handler())
	defer srv.Close()

	// The fixture seeds 12 posts: 5, 5, 2.
	pagina := func(q string) string {
		res, err := http.Get(srv.URL + "/blog" + q)
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Fatalf("GET /blog%s = %d", q, res.StatusCode)
		}
		return string(b)
	}
	p1 := pagina("")
	if n := quantos(p1); n != 5 {
		t.Fatalf("page 1 lists %d posts, want 5", n)
	}
	if !strings.Contains(p1, "page=2") || strings.Contains(p1, "page=0") {
		t.Fatal("page 1 links are wrong")
	}
	p2 := pagina("?page=2")
	if n := quantos(p2); n != 5 {
		t.Fatalf("page 2 lists %d posts, want 5", n)
	}
	if !strings.Contains(p2, "page=1") || !strings.Contains(p2, "page=3") {
		t.Fatal("page 2 lost its neighbours")
	}
	p3 := pagina("?page=3")
	if n := quantos(p3); n != 2 {
		t.Fatalf("page 3 lists %d posts, want 2", n)
	}
}
`

const baselineGenerateCRUDTest = `package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestBenchBaselineGenerateCRUD(t *testing.T) {
	srv := httptest.NewServer(handler())
	defer srv.Close()

	res, err := http.Get(srv.URL + "/categorias")
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /categorias = %d", res.StatusCode)
	}
	res.Body.Close()

	res, err = http.PostForm(srv.URL+"/categorias/new", url.Values{"nome": {"Contratos"}})
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusSeeOther {
		t.Fatalf("POST /categorias/new = %d, want 303", res.StatusCode)
	}
	res.Body.Close()

	res, err = http.Get(srv.URL + "/categorias")
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if !strings.Contains(string(b), "Contratos") {
		t.Fatal("a listagem não mostra o que foi criado")
	}
}
`

const baselineS5LoginTest = `package main

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestBenchBaselineS5Login(t *testing.T) {
	t.Setenv("ADMIN_EMAIL", "admin@exemplo.com")
	t.Setenv("ADMIN_PASSWORD", "senha-trilha-2026")
	srv := httptest.NewServer(handler())
	defer srv.Close()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	cliente := &http.Client{
		Jar: jar,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	entrar := func(email, senha string) *http.Response {
		res, err := cliente.PostForm(srv.URL+"/entrar", url.Values{"email": {email}, "password": {senha}})
		if err != nil {
			t.Fatal(err)
		}
		return res
	}

	res, err := cliente.Get(srv.URL + "/entrar")
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /entrar = %d", res.StatusCode)
	}
	res.Body.Close()

	// A wrong password is refused, and no session is opened.
	res = entrar("admin@exemplo.com", "errada")
	if res.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("senha errada = %d, want 422", res.StatusCode)
	}
	res.Body.Close()
	if u, _ := url.Parse(srv.URL); len(jar.Cookies(u)) > 0 {
		t.Fatal("a wrong password opened a session")
	}

	// The right one opens it.
	res = entrar("admin@exemplo.com", "senha-trilha-2026")
	if res.StatusCode != http.StatusSeeOther {
		t.Fatalf("senha certa = %d, want 303", res.StatusCode)
	}
	res.Body.Close()
	u, _ := url.Parse(srv.URL)
	if len(jar.Cookies(u)) == 0 {
		t.Fatal("o login não deixou cookie de sessão")
	}

	// And signing out closes it.
	res, err = cliente.PostForm(srv.URL+"/sair", url.Values{})
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusSeeOther {
		t.Fatalf("POST /sair = %d, want 303", res.StatusCode)
	}
	if _, err := io.Copy(io.Discard, res.Body); err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if len(jar.Cookies(u)) > 0 {
		t.Fatal("o logout não encerrou a sessão")
	}
}
`

const baselineS6CRUDTest = `package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestBenchBaselineS6CRUD(t *testing.T) {
	srv := httptest.NewServer(handler())
	defer srv.Close()

	post := func(path, body string) *http.Response {
		res, err := http.Post(srv.URL+path, "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		return res
	}

	res := post("/api/produtos", ` + "`" + `{"nome":"Teclado","preco":129.9}` + "`" + `)
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("POST = %d, want 201", res.StatusCode)
	}
	res.Body.Close()
	res = post("/api/produtos", ` + "`" + `{"nome":"","preco":-1}` + "`" + `)
	if res.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("inválido = %d, want 422", res.StatusCode)
	}
	res.Body.Close()

	res, err := http.Get(srv.URL + "/api/produtos")
	if err != nil {
		t.Fatal(err)
	}
	var lista []map[string]any
	if err := json.NewDecoder(res.Body).Decode(&lista); err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	aqui := false
	for _, p := range lista {
		if p["nome"] == "Teclado" {
			aqui = true
		}
	}
	if !aqui {
		t.Fatalf("GET /api/produtos = %v, want the Teclado in it", lista)
	}

	// The screen: form, per-field error on invalid, redirect on valid.
	res, err = http.Get(srv.URL + "/produtos")
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK || !strings.Contains(string(b), "<form") {
		t.Fatalf("GET /produtos = %d, want a form on it", res.StatusCode)
	}
	res, err = http.PostForm(srv.URL+"/produtos", url.Values{"nome": {""}, "preco": {"-1"}})
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("POST inválido = %d, want 422", res.StatusCode)
	}
	res.Body.Close()
	res, err = http.PostForm(srv.URL+"/produtos", url.Values{"nome": {"Monitor"}, "preco": {"899.00"}})
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusSeeOther {
		t.Fatalf("POST válido = %d, want 303", res.StatusCode)
	}
	res.Body.Close()
	res, err = http.Get(srv.URL + "/produtos")
	if err != nil {
		t.Fatal(err)
	}
	b, err = io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if !strings.Contains(string(b), "Monitor") {
		t.Fatal("a listagem não mostra o que foi criado")
	}
}
`

const baselineS7TelaTest = `package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func relatorio(t *testing.T, srv, q string) string {
	t.Helper()
	res, err := http.Get(srv + "/relatorios" + q)
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /relatorios%s = %d", q, res.StatusCode)
	}
	return string(b)
}

func TestBenchBaselineS7Tela(t *testing.T) {
	srv := httptest.NewServer(handler())
	defer srv.Close()

	// The columns the task named, and the chart drawing the totals in SVG.
	body := relatorio(t, srv.URL, "")
	for _, quero := range []string{"<table", "Cliente", "Estado", "Total", "<svg", "SP", "RJ", "MG"} {
		if !strings.Contains(body, quero) {
			t.Fatalf("a tela não mostra %q", quero)
		}
	}

	// Twelve sales, five per page: page 1 has Ana's block and none of Carla's;
	// page 3 is the other way around; page 2 points at both neighbours.
	if !strings.Contains(body, "Ana Souza") || strings.Contains(body, "Carla Dias") {
		t.Fatal("a página 1 não é a primeira metade da lista")
	}
	if !strings.Contains(body, "page=2") {
		t.Fatal("a página 1 não aponta para a 2")
	}
	p2 := relatorio(t, srv.URL, "?page=2")
	if !strings.Contains(p2, "page=1") || !strings.Contains(p2, "page=3") {
		t.Fatal("a página 2 perdeu os vizinhos")
	}
	p3 := relatorio(t, srv.URL, "?page=3")
	if !strings.Contains(p3, "Carla Dias") || strings.Contains(p3, "Ana Souza") {
		t.Fatal("a página 3 não é o fim da lista")
	}

	// The search reads the URL and reaches the table.
	filtrada := relatorio(t, srv.URL, "?q=Bruno")
	if strings.Contains(filtrada, "Ana Souza") {
		t.Fatal("o filtro ?q deixou passar o que não era dele")
	}
	if !strings.Contains(filtrada, "Bruno Lima") {
		t.Fatal("o filtro escondeu o que devia mostrar")
	}
}
`
