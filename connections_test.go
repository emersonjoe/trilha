package trilha

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// connApp is an app whose audit trail and log are readable by the test.
func connApp(env Env) (*App, *[]AuditRecord, *bytes.Buffer) {
	var trail []AuditRecord
	logs := &bytes.Buffer{}
	a := New(Config{Env: env, Logger: slog.New(slog.NewTextHandler(logs, nil)),
		Audit: AuditFunc(func(r AuditRecord) error { trail = append(trail, r); return nil })})
	return a, &trail, logs
}

func connCtx(a *App) *Ctx {
	req := httptest.NewRequest("POST", "/admin/conexoes", nil)
	rec := httptest.NewRecorder()
	c := newCtx(a, &responseWriter{ResponseWriter: rec, status: http.StatusOK}, req, kindPage)
	c.SetActor(Actor{Subject: "ana", Tenant: "acme"})
	return c
}

func conexoes(opts ...ConnectionsOpts) *Connections {
	o := ConnectionsOpts{}
	if len(opts) > 0 {
		o = opts[0]
	}
	if o.Kinds == nil {
		o.Kinds = []ConnectionKind{
			{Key: "api", Label: "API HTTP", Auth: []string{"none", "bearer", "header", "basic"}, Test: TestHTTP("GET", "/health")},
		}
	}
	return NewConnections(o)
}

// #153 — a URL de uma conexão é o endereço de um terceiro: http/https, com
// host, e em produção nunca a própria máquina nem a rede interna.
func TestValidateExternalURL(t *testing.T) {
	ok := []string{"https://api.exemplo.com", "http://api.exemplo.com:8080/v1", "https://10.0.0.1.nip.io"}
	for _, u := range ok {
		if err := ValidateExternalURL(u, Prod); err != nil {
			t.Fatalf("%s: %v", u, err)
		}
	}
	bad := map[string]string{
		"":                          "required",
		"api.exemplo.com":           "http",
		"ftp://api.exemplo.com":     "http",
		"https://":                  "host",
		"https://user:pw@x.com":     "credentials",
		"http://127.0.0.1:8801":     "private",
		"http://localhost/api":      "private",
		"http://10.1.2.3":           "private",
		"http://192.168.0.5":        "private",
		"http://[::1]:80":           "private",
		"http://169.254.169.254":    "private",
		"http://metadata.internal.": "private",
	}
	for u, want := range bad {
		err := ValidateExternalURL(u, Prod)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("%q: err = %v, quer %q", u, err, want)
		}
	}
	// Em desenvolvimento a rede privada é onde o serviço de verdade roda.
	if err := ValidateExternalURL("http://127.0.0.1:8801", Dev); err != nil {
		t.Fatalf("dev: %v", err)
	}
	if err := ValidateExternalURL("ftp://x", Dev); err == nil {
		t.Fatal("dev ainda exige http")
	}
}

func TestConnectionsSaveValidaEAudita(t *testing.T) {
	a, trail, logs := connApp(Prod)
	cx := conexoes()
	c := connCtx(a)

	_, err := cx.Save(c, Connection{Kind: "api", Name: "", URL: "http://127.0.0.1", Auth: "oauth"})
	var fe FieldErrors
	if !errors.As(err, &fe) {
		t.Fatalf("err = %v", err)
	}
	for _, f := range []string{"name", "url", "auth"} {
		if !fe.Has(f) {
			t.Fatalf("faltou o campo %q em %v", f, fe)
		}
	}
	if _, err := cx.Save(c, Connection{Kind: "smtp", Name: "x", URL: "https://x.com"}); err == nil {
		t.Fatal("tipo não declarado passou")
	}
	if len(*trail) != 0 {
		t.Fatalf("auditou o que não gravou: %+v", *trail)
	}

	saved, err := cx.Save(c, Connection{Kind: "api", Name: "Acervo", URL: "https://api.acervo.com/", Auth: "bearer", Secret: "tok-secreto-123"})
	if err != nil {
		t.Fatal(err)
	}
	if saved.ID == "" || saved.Tenant != "acme" || saved.CreatedAt.IsZero() {
		t.Fatalf("%+v", saved)
	}
	if len(*trail) != 1 || (*trail)[0].Action != "connection.save" || (*trail)[0].Target != saved.ID {
		t.Fatalf("auditoria: %+v", *trail)
	}
	if s := fmt.Sprintf("%v %+v", (*trail)[0], saved); strings.Contains(s, "tok-secreto-123") {
		t.Fatalf("o segredo vazou: %s", s)
	}
	if strings.Contains(logs.String(), "tok-secreto-123") {
		t.Fatalf("o segredo está no log: %s", logs.String())
	}
	raw, _ := json.Marshal(saved)
	if strings.Contains(string(raw), "tok-secreto-123") {
		t.Fatalf("o segredo está no JSON: %s", raw)
	}
	// A lista é do tenant de quem pergunta.
	list, _ := cx.List(c)
	if len(list) != 1 || list[0].Name != "Acervo" {
		t.Fatalf("lista = %+v", list)
	}
	other := connCtx(a)
	other.SetActor(Actor{Subject: "bia", Tenant: "outra"})
	if list, _ := cx.List(other); len(list) != 0 {
		t.Fatalf("outro tenant vê %+v", list)
	}
	if _, err := cx.Get(other, saved.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("outro tenant lê a conexão: %v", err)
	}
}

// Segredo vazio no update mantém o anterior; com valor, troca. É a metade da
// tela que toda aplicação escreve à mão, e que a metade delas escreve errado.
func TestConnectionsSaveMantemOSegredo(t *testing.T) {
	a, _, _ := connApp(Prod)
	cx := conexoes()
	c := connCtx(a)
	saved, _ := cx.Save(c, Connection{Kind: "api", Name: "A", URL: "https://a.com", Auth: "bearer", Secret: "primeiro"})

	saved.Name, saved.Secret = "A renomeada", ""
	again, err := cx.Save(c, saved)
	if err != nil {
		t.Fatal(err)
	}
	if again.Secret.Reveal() != "primeiro" || again.Name != "A renomeada" {
		t.Fatalf("%+v", again)
	}
	again.Secret = "segundo"
	if again, _ = cx.Save(c, again); again.Secret.Reveal() != "segundo" {
		t.Fatalf("%+v", again)
	}
	// O que o store guarda é o que Get devolve — inclusive o segredo trocado.
	got, _ := cx.Get(c, saved.ID)
	if got.Secret.Reveal() != "segundo" || got.UpdatedAt.Before(got.CreatedAt) {
		t.Fatalf("%+v", got)
	}
	// Apagar apaga, e audita.
	if err := cx.Delete(c, saved.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := cx.Get(c, saved.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("ainda está lá")
	}
}

// URL privada: recusada em produção, aceita em desenvolvimento com aviso.
func TestConnectionsURLPrivadaPorAmbiente(t *testing.T) {
	a, _, _ := connApp(Prod)
	if _, err := conexoes().Save(connCtx(a), Connection{Kind: "api", Name: "L", URL: "http://127.0.0.1:8801", Auth: "none"}); err == nil {
		t.Fatal("prod aceitou 127.0.0.1")
	}
	a, _, logs := connApp(Dev)
	if _, err := conexoes().Save(connCtx(a), Connection{Kind: "api", Name: "L", URL: "http://127.0.0.1:8801", Auth: "none"}); err != nil {
		t.Fatalf("dev recusou: %v", err)
	}
	if !strings.Contains(logs.String(), "private") {
		t.Fatalf("dev não avisou: %s", logs.String())
	}
}

// Client monta a autenticação, tem prazo e só fala com o host da conexão.
func TestConnectionsClient(t *testing.T) {
	var got http.Header
	var gotPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, gotPath = r.Header.Clone(), r.URL.Path
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "http://example.com/steal", http.StatusFound)
			return
		}
		w.WriteHeader(204)
	}))
	defer upstream.Close()

	a, _, _ := connApp(Dev)
	cx := conexoes(ConnectionsOpts{Timeout: 3 * time.Second})
	c := connCtx(a)
	cases := []struct {
		conn Connection
		want func(h http.Header) bool
	}{
		{Connection{Kind: "api", Name: "b", URL: upstream.URL, Auth: "bearer", Secret: "tok"},
			func(h http.Header) bool { return h.Get("Authorization") == "Bearer tok" }},
		{Connection{Kind: "api", Name: "h", URL: upstream.URL, Auth: "header", Header: "X-Api-Key", Secret: "k1", Headers: map[string]string{"X-Tenant": "acme"}},
			func(h http.Header) bool { return h.Get("X-Api-Key") == "k1" && h.Get("X-Tenant") == "acme" }},
		{Connection{Kind: "api", Name: "u", URL: upstream.URL, Auth: "basic", Username: "ana", Secret: "pw"},
			func(h http.Header) bool {
				u, p, ok := (&http.Request{Header: h}).BasicAuth()
				return ok && u == "ana" && p == "pw"
			}},
		{Connection{Kind: "api", Name: "n", URL: upstream.URL, Auth: "none", Secret: "ignored"},
			func(h http.Header) bool { return h.Get("Authorization") == "" }},
	}
	for _, tc := range cases {
		saved, err := cx.Save(c, tc.conn)
		if err != nil {
			t.Fatalf("%s: %v", tc.conn.Name, err)
		}
		client, err := cx.Client(c, saved.ID)
		if err != nil {
			t.Fatal(err)
		}
		if client.Timeout == 0 {
			t.Fatal("cliente sem prazo")
		}
		resp, err := client.Get(upstream.URL + "/x")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if !tc.want(got) {
			t.Fatalf("%s: cabeçalhos %v", tc.conn.Name, got)
		}
	}
	// Outro host é recusado antes de sair — o segredo não viaja para onde a
	// conexão não aponta, nem por redirect.
	saved, _ := cx.Save(c, Connection{Kind: "api", Name: "r", URL: upstream.URL, Auth: "bearer", Secret: "tok"})
	client, _ := cx.Client(c, saved.ID)
	if _, err := client.Get("http://example.com/other"); err == nil || !strings.Contains(err.Error(), "host") {
		t.Fatalf("outro host: %v", err)
	}
	if _, err := client.Get(upstream.URL + "/redirect"); err == nil || !strings.Contains(err.Error(), "host") {
		t.Fatalf("redirect para fora: %v", err)
	}
	if gotPath != "/redirect" {
		t.Fatalf("path = %s", gotPath)
	}
}

// Test roda o teste do tipo com prazo, guarda o resultado e audita.
func TestConnectionsTest(t *testing.T) {
	state := "ok"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path != "/health":
			w.WriteHeader(404)
		case r.Header.Get("Authorization") != "Bearer tok":
			w.WriteHeader(401)
		case state == "slow":
			time.Sleep(300 * time.Millisecond)
		default:
			w.WriteHeader(200)
		}
	}))
	defer upstream.Close()
	a, trail, _ := connApp(Dev)
	cx := conexoes(ConnectionsOpts{Timeout: 100 * time.Millisecond})
	c := connCtx(a)
	saved, _ := cx.Save(c, Connection{Kind: "api", Name: "A", URL: upstream.URL, Auth: "bearer", Secret: "tok"})

	res, err := cx.Test(c, saved.ID)
	if err != nil || !res.OK || res.At.IsZero() {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	got, _ := cx.Get(c, saved.ID)
	if got.LastTest == nil || !got.LastTest.OK {
		t.Fatalf("não guardou: %+v", got.LastTest)
	}
	if n := len(*trail); n != 2 || (*trail)[1].Action != "connection.test" || (*trail)[1].Fields["ok"] != true {
		t.Fatalf("auditoria: %+v", *trail)
	}

	saved.Secret = "errado"
	cx.Save(c, saved)
	if res, _ := cx.Test(c, saved.ID); res.OK || !strings.Contains(res.Message, "401") {
		t.Fatalf("%+v", res)
	}
	state = "slow"
	saved.Secret = "tok"
	cx.Save(c, saved)
	if res, _ := cx.Test(c, saved.ID); res.OK || res.Message == "" {
		t.Fatalf("prazo: %+v", res)
	}
	// Um tipo sem teste responde que não há como testar, em vez de "ok".
	cx2 := conexoes(ConnectionsOpts{Kinds: []ConnectionKind{{Key: "x", Label: "X", Auth: []string{"none"}}}})
	s2, _ := cx2.Save(c, Connection{Kind: "x", Name: "x", URL: "https://x.com", Auth: "none"})
	if _, err := cx2.Test(c, s2.ID); err == nil {
		t.Fatal("tipo sem Test testou")
	}
}

// Um teste próprio recebe a conexão e o cliente já autenticado.
func TestConnectionKindTestProprio(t *testing.T) {
	var seen string
	kinds := []ConnectionKind{{Key: "mcp", Label: "MCP", Auth: []string{"bearer"},
		Test: func(ctx context.Context, conn Connection) error {
			req, _ := http.NewRequestWithContext(ctx, "POST", conn.URL, nil)
			resp, err := conn.Client(time.Second).Do(req)
			if err != nil {
				return err
			}
			resp.Body.Close()
			seen = resp.Header.Get("X-Seen")
			return nil
		}}}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Seen", r.Header.Get("Authorization"))
	}))
	defer upstream.Close()
	a, _, _ := connApp(Dev)
	cx := conexoes(ConnectionsOpts{Kinds: kinds})
	c := connCtx(a)
	saved, _ := cx.Save(c, Connection{Kind: "mcp", Name: "m", URL: upstream.URL, Auth: "bearer", Secret: "abc"})
	if res, err := cx.Test(c, saved.ID); err != nil || !res.OK || seen != "Bearer abc" {
		t.Fatalf("res = %+v, err = %v, seen = %q", res, err, seen)
	}
}
