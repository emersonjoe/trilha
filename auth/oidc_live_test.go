package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/emersonjoe/trilha"
)

// O fakeIDP do fake_test.go emite tokens com o mesmo código que este pacote
// depois valida: ele prova que somos coerentes conosco mesmos, e é ótimo para
// os casos de recusa (chave trocada, iss mentindo, alg none). O que ele não
// pode provar é que a gente fala OIDC — só um provedor de verdade prova isso.
//
// Este teste faz o login inteiro contra um Keycloak: descoberta, PKCE, code,
// troca no token endpoint, validação do id_token com a JWKS dele, e os papéis
// saindo de resource_access. Ele é pulado sem TRILHA_OIDC_TEST, porque teste
// que depende de contêiner falha por motivo errado na máquina de outra pessoa.
//
//	docker run -d --name trilha-keycloak -p 8080:8080 \
//	  -e KC_BOOTSTRAP_ADMIN_USERNAME=admin -e KC_BOOTSTRAP_ADMIN_PASSWORD=admin \
//	  quay.io/keycloak/keycloak:26.0 start-dev
//
//	TRILHA_OIDC_TEST=http://localhost:8080 go test ./auth/ -run TestOIDCAoVivo -v
func TestOIDCAoVivo(t *testing.T) {
	base := strings.TrimSuffix(os.Getenv("TRILHA_OIDC_TEST"), "/")
	if base == "" {
		t.Skip("sem TRILHA_OIDC_TEST: este teste quer um Keycloak de verdade (veja o comentário)")
	}
	kc := &keycloak{t: t, base: base, realm: "trilha", client: "trilha-app", secret: "segredo-do-cliente"}
	kc.provisiona()

	// A app: login, callback e uma página guardada. É o mesmo desenho de
	// qualquer app que usa auth.New — nada de especial para o teste.
	// O construtor de verdade, não um Provider montado à mão: é ele que liga o
	// tratamento de papéis específico do Keycloak, e testar sem ele seria
	// testar outro caminho de código.
	prov := Keycloak(base, kc.realm, kc.client, kc.secret, "")
	a := New(prov, Options{LoginPath: "/entrar", AfterLogin: "/painel"})

	app := trilha.New(trilha.Config{Env: trilha.Prod, Secret: []byte("0123456789abcdef0123456789abcdef"),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	app.Register(trilha.Route{Pattern: "/entrar", Kind: trilha.KindPage,
		Methods: map[string]trilha.HandlerFunc{"GET": a.Start}})
	app.Register(trilha.Route{Pattern: "/callback", Kind: trilha.KindPage,
		Methods: map[string]trilha.HandlerFunc{"GET": a.Callback}})
	app.Register(trilha.Route{Pattern: "/painel", Kind: trilha.KindPage,
		Methods: map[string]trilha.HandlerFunc{"GET": func(c *trilha.Ctx) error {
			u := a.User(c)
			return c.Text(200, fmt.Sprintf("%s|%s|%s", u.Subject, u.Email, strings.Join(u.Roles, ",")))
		}}, Middlewares: []trilha.MiddlewareFunc{a.Require()}})

	srv := servidorDeTeste(t, app)
	prov.RedirectURL = srv.URL + "/callback"
	kc.registraRedirect(prov.RedirectURL)

	// Um navegador: guarda cookies e segue redirecionamentos, como o de
	// verdade.
	jar, _ := cookiejar.New(nil)
	cl := &http.Client{Jar: jar, Timeout: 20 * time.Second}

	// 1. A app manda para o Keycloak; o Keycloak responde o formulário de
	//    login, e é dele que sai o endereço para onde a senha vai.
	res, err := cl.Get(srv.URL + "/entrar")
	if err != nil {
		t.Fatalf("entrar: %v", err)
	}
	corpo, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if !strings.Contains(res.Request.URL.Host, hostOf(base)) {
		t.Fatalf("não fomos parar no provedor: %s", res.Request.URL)
	}
	acao := formAction(t, string(corpo))

	// 2. A senha vai para o Keycloak, que devolve o code no nosso callback.
	res, err = cl.PostForm(acao, url.Values{"username": {"ana"}, "password": {"segredo-da-ana"}})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	fim, _ := io.ReadAll(res.Body)
	res.Body.Close()

	// 3. E o callback nos deixou logados: o painel responde com o que veio do
	//    id_token que este pacote validou contra a JWKS do Keycloak.
	if res.StatusCode != 200 {
		t.Fatalf("depois do login: %d %s", res.StatusCode, fim)
	}
	partes := strings.Split(string(fim), "|")
	if len(partes) != 3 || partes[0] == "" {
		t.Fatalf("o painel respondeu %q", fim)
	}
	if partes[1] != "ana@exemplo.com" {
		t.Fatalf("e-mail = %q", partes[1])
	}
	t.Logf("sub=%s email=%s papéis=%s", partes[0], partes[1], partes[2])
	// O papel de realm tem de chegar: é o caminho de código que o construtor
	// Keycloak liga, e é o que decide quem entra em cada tela.
	if !strings.Contains(partes[2], "analista") {
		t.Fatalf("o papel do provedor não chegou na sessão: %q", partes[2])
	}
}

// servidorDeTeste sobe a app. O endereço é configurável porque o padrão do
// httptest é 127.0.0.1, e há máquina em que o laço IPv4 não fecha conexão —
// WSL em modo espelhado é uma delas. TRILHA_OIDC_BIND='[::1]:0' resolve, sem
// este teste ter de saber nada sobre a rede de quem o roda.
func servidorDeTeste(t *testing.T, app *trilha.App) *httptest.Server {
	t.Helper()
	bind := os.Getenv("TRILHA_OIDC_BIND")
	if bind == "" {
		srv := httptest.NewServer(app.Handler())
		t.Cleanup(srv.Close)
		return srv
	}
	l, err := net.Listen("tcp", bind)
	if err != nil {
		t.Fatalf("ouvir em %s: %v", bind, err)
	}
	srv := &httptest.Server{Listener: l, Config: &http.Server{Handler: app.Handler()}}
	srv.Start()
	t.Cleanup(srv.Close)
	return srv
}

// ---- o provedor, preparado pela API de administração -----------------------

type keycloak struct {
	t                              *testing.T
	base, realm                    string
	client, secret                 string
	adminToken, clientUUID, userID string
}

func (k *keycloak) provisiona() {
	k.adminToken = k.token()
	k.cria("/admin/realms", map[string]any{"realm": k.realm, "enabled": true})
	k.clientUUID = k.criaCliente()
	k.userID = k.criaUsuario()
	k.criaPapel("analista")
}

// criaPapel cria um papel de realm e o dá à usuária. É o caminho que o
// tratamento específico de Keycloak lê, e sem ele o teste provaria só o login.
func (k *keycloak) criaPapel(nome string) {
	k.cria("/admin/realms/"+k.realm+"/roles", map[string]any{"name": nome})
	var papel struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	k.get("/admin/realms/"+k.realm+"/roles/"+nome, &papel)
	k.cria("/admin/realms/"+k.realm+"/users/"+k.userID+"/role-mappings/realm",
		[]map[string]any{{"id": papel.ID, "name": papel.Name}})
}

// token pega o token de administrador do realm master.
func (k *keycloak) token() string {
	res, err := http.PostForm(k.base+"/realms/master/protocol/openid-connect/token", url.Values{
		"grant_type": {"password"}, "client_id": {"admin-cli"},
		"username": {"admin"}, "password": {"admin"},
	})
	if err != nil {
		k.t.Fatalf("token de admin: %v", err)
	}
	defer res.Body.Close()
	var out struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil || out.AccessToken == "" {
		k.t.Fatalf("token de admin: %d %v", res.StatusCode, err)
	}
	return out.AccessToken
}

func (k *keycloak) cria(path string, body any) *http.Response {
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, k.base+path, strings.NewReader(string(b)))
	req.Header.Set("Authorization", "Bearer "+k.adminToken)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		k.t.Fatalf("POST %s: %v", path, err)
	}
	// 409 é "já existe", que serve: o contêiner pode estar de pé desde a
	// execução anterior.
	if res.StatusCode >= 400 && res.StatusCode != http.StatusConflict {
		msg, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		res.Body.Close()
		k.t.Fatalf("POST %s: %d %s", path, res.StatusCode, msg)
	}
	return res
}

func (k *keycloak) criaCliente() string {
	k.cria("/admin/realms/"+k.realm+"/clients", map[string]any{
		"clientId": k.client, "secret": k.secret, "enabled": true,
		"publicClient": false, "standardFlowEnabled": true,
		"redirectUris": []string{"http://127.0.0.1:*/callback", "http://localhost:*/callback"},
	})
	var lista []struct {
		ID string `json:"id"`
	}
	k.get("/admin/realms/"+k.realm+"/clients?clientId="+k.client, &lista)
	if len(lista) == 0 {
		k.t.Fatal("o cliente não apareceu depois de criado")
	}
	return lista[0].ID
}

func (k *keycloak) criaUsuario() string {
	// O perfil vai completo: sem sobrenome o Keycloak interrompe o login com a
	// tela de "complete seu cadastro", e o teste pararia numa página que não
	// tem nada a ver com o que ele quer provar.
	perfil := map[string]any{
		"username": "ana", "email": "ana@exemplo.com", "emailVerified": true,
		"firstName": "Ana", "lastName": "Lovelace", "enabled": true,
		"credentials": []map[string]any{{"type": "password", "value": "segredo-da-ana", "temporary": false}},
	}
	k.cria("/admin/realms/"+k.realm+"/users", perfil)
	// O contêiner pode estar de pé desde a execução anterior, com a usuária
	// criada pela metade: atualizar é mais barato que recriar.
	var achados []struct {
		ID string `json:"id"`
	}
	k.get("/admin/realms/"+k.realm+"/users?username=ana&exact=true", &achados)
	if len(achados) == 0 {
		k.t.Fatal("a usuária não apareceu depois de criada")
	}
	k.put("/admin/realms/"+k.realm+"/users/"+achados[0].ID, perfil)
	return achados[0].ID
}

func (k *keycloak) put(path string, body any) {
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPut, k.base+path, strings.NewReader(string(b)))
	req.Header.Set("Authorization", "Bearer "+k.adminToken)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		k.t.Fatalf("PUT %s: %v", path, err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		msg, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		k.t.Fatalf("PUT %s: %d %s", path, res.StatusCode, msg)
	}
}

func (k *keycloak) get(path string, out any) {
	req, _ := http.NewRequest(http.MethodGet, k.base+path, nil)
	req.Header.Set("Authorization", "Bearer "+k.adminToken)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		k.t.Fatalf("GET %s: %v", path, err)
	}
	defer res.Body.Close()
	if err := json.NewDecoder(res.Body).Decode(out); err != nil {
		k.t.Fatalf("GET %s: %v", path, err)
	}
}

// registraRedirect acrescenta o endereço que o servidor de teste acabou de
// ganhar: a porta só existe depois de ele subir.
func (k *keycloak) registraRedirect(uri string) {
	k.put("/admin/realms/"+k.realm+"/clients/"+k.clientUUID, map[string]any{
		// Só o endereço exato: o Keycloak recusa curinga em host IPv6, e o
		// exato é o que este teste precisa de qualquer forma.
		"redirectUris": []string{uri},
	})
}

var reForm = regexp.MustCompile(`(?i)<form[^>]+action="([^"]+)"`)

// formAction acha para onde o formulário de login do Keycloak posta. É a única
// parte deste teste que depende do HTML do provedor, e é por isso que ela falha
// com a página inteira à vista quando a versão dele muda.
func formAction(t *testing.T, html string) string {
	t.Helper()
	m := reForm.FindStringSubmatch(html)
	if m == nil {
		t.Fatalf("não achei o formulário de login na página do provedor:\n%s", primeiros(html, 800))
	}
	return strings.ReplaceAll(m[1], "&amp;", "&")
}

func primeiros(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func hostOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	return u.Hostname()
}
