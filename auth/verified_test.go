package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/emersonjoe/trilha"
)

// #164 — o provedor manda `email` e `email_verified: false` quando a pessoa
// digitou aquele endereço e não provou que é dela. Uma lista de permitidos por
// e-mail que não vê a claim aceita o endereço de qualquer um que saiba
// escrevê-lo.
func TestEmailNaoVerificadoChegaAoOnLogin(t *testing.T) {
	var visto *User
	idp, _, b := setup(t, Options{OnLogin: func(c *trilha.Ctx, u *User) error {
		visto = u
		return nil
	}})
	idp.claims = map[string]any{"email": "ana@exemplo.com", "email_verified": false}

	b.login(idp, "")
	if visto == nil {
		t.Fatal("o OnLogin não rodou")
	}
	if visto.EmailVerified {
		t.Fatal("email_verified: false chegou como verificado")
	}
	if visto.Email != "ana@exemplo.com" {
		t.Fatalf("e-mail = %q", visto.Email)
	}
}

// Com a claim verdadeira o campo é verdadeiro — nas duas formas, porque
// provedor manda das duas e quem recusa e-mail não verificado não pode
// depender de qual.
func TestEmailVerificadoNasDuasFormas(t *testing.T) {
	for _, forma := range []struct {
		nome  string
		valor any
	}{
		{"booleano", true},
		{"string", "true"},
	} {
		t.Run(forma.nome, func(t *testing.T) {
			var visto *User
			idp, _, b := setup(t, Options{OnLogin: func(c *trilha.Ctx, u *User) error {
				visto = u
				return nil
			}})
			idp.claims = map[string]any{"email": "ana@exemplo.com", "email_verified": forma.valor}

			b.login(idp, "")
			if visto == nil || !visto.EmailVerified {
				t.Fatalf("email_verified=%v não chegou como verificado: %+v", forma.valor, visto)
			}
		})
	}
}

// Com a opção ligada, a recusa acontece antes do OnLogin: a função do app não
// chega a ver um e-mail que não devia existir.
func TestRequireVerifiedEmailRecusaAntesDoOnLogin(t *testing.T) {
	rodou := false
	idp, _, b := setup(t, Options{
		RequireVerifiedEmail: true,
		OnLogin: func(c *trilha.Ctx, u *User) error {
			rodou = true
			return nil
		},
	})
	idp.claims = map[string]any{"email": "ana@exemplo.com", "email_verified": false}

	rec := b.login(idp, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("retorno → %d, queria 401", rec.Code)
	}
	if rodou {
		t.Fatal("o OnLogin rodou com um e-mail que o provedor não garantiu")
	}
	if _, ok := b.cookies["trilha_session"]; ok {
		t.Fatal("saiu sessão de um login recusado")
	}
}

// A mesma opção ligada não atrapalha quem tem o e-mail verificado.
func TestRequireVerifiedEmailDeixaPassarOVerificado(t *testing.T) {
	idp, _, b := setup(t, Options{RequireVerifiedEmail: true})
	idp.claims = map[string]any{"email": "ana@exemplo.com", "email_verified": true}

	b.login(idp, "")
	if rec := b.get("/admin", nil); rec.Code != 200 {
		t.Fatalf("/admin → %d", rec.Code)
	}
}

// O Callback cai para preferred_username quando não há email. Um nome de
// usuário não é um e-mail que alguém garantiu, então ele nunca chega
// verificado — e com a opção ligada não entra.
func TestPreferredUsernameNuncaEhVerificado(t *testing.T) {
	var visto *User
	idp, _, b := setup(t, Options{OnLogin: func(c *trilha.Ctx, u *User) error {
		visto = u
		return nil
	}})
	// email_verified true no token, e nenhum email: a claim fala de um e-mail
	// que não veio.
	idp.claims = map[string]any{"preferred_username": "ana", "email_verified": true}

	b.login(idp, "")
	if visto == nil {
		t.Fatal("o OnLogin não rodou")
	}
	if visto.Email != "ana" {
		t.Fatalf("e-mail = %q, queria o preferred_username", visto.Email)
	}
	if visto.EmailVerified {
		t.Fatal("um preferred_username chegou como e-mail verificado")
	}

	idp2, _, b2 := setup(t, Options{RequireVerifiedEmail: true})
	idp2.claims = map[string]any{"preferred_username": "ana", "email_verified": true}
	if rec := b2.login(idp2, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("retorno → %d, queria 401", rec.Code)
	}
}

// Sessão gravada antes deste campo existir carrega com false, que é o valor
// seguro: o JSON não tem a chave e o zero de bool é o que queremos.
func TestSessaoAntigaCarregaComoNaoVerificada(t *testing.T) {
	var u User
	if err := json.Unmarshal([]byte(`{"sub":"u-1","email":"ana@exemplo.com"}`), &u); err != nil {
		t.Fatal(err)
	}
	if u.EmailVerified {
		t.Fatal("sessão sem a chave carregou como verificada")
	}
}

// #165 — o Store em banco. lojaContexto implementa as duas interfaces e conta
// por qual delas foi chamada.
type lojaContexto struct {
	*MemoryStore
	viaContexto int
	falha       error
	ctxRecebido func(canceled bool)
}

func novaLojaContexto() *lojaContexto {
	return &lojaContexto{MemoryStore: NewMemoryStore()}
}

func (l *lojaContexto) SaveContext(ctx context.Context, id string, u *User, ttl time.Duration) error {
	l.viaContexto++
	return l.MemoryStore.Save(id, u, ttl)
}

func (l *lojaContexto) LoadContext(ctx context.Context, id string) (*User, error) {
	l.viaContexto++
	if l.ctxRecebido != nil {
		l.ctxRecebido(ctx.Err() != nil)
	}
	if l.falha != nil {
		return nil, l.falha
	}
	u, ok := l.MemoryStore.Load(id)
	if !ok {
		return nil, ErrNoSession
	}
	return u, nil
}

func (l *lojaContexto) DeleteContext(ctx context.Context, id string) error {
	l.viaContexto++
	return l.MemoryStore.Delete(id)
}

// Um Store que implementa StoreContext é chamado por ela, com o contexto da
// requisição.
func TestStoreContextEhUsadoQuandoExiste(t *testing.T) {
	loja := novaLojaContexto()
	idp, _, b := setup(t, Options{Store: loja})
	idp.claims = map[string]any{"email": "ana@exemplo.com"}

	b.login(idp, "")
	if loja.viaContexto == 0 {
		t.Fatal("o Store implementa StoreContext e foi chamado pela interface antiga")
	}
	antes := loja.viaContexto
	if rec := b.get("/admin", nil); rec.Code != 200 {
		t.Fatalf("/admin → %d", rec.Code)
	}
	if loja.viaContexto == antes {
		t.Fatal("a leitura da sessão não passou por LoadContext")
	}
}

// O contexto que chega é o da requisição: um request já cancelado chega
// cancelado no store, que é o que deixa a consulta ser abortada.
func TestStoreContextRecebeOContextoDaRequisicao(t *testing.T) {
	loja := novaLojaContexto()
	idp, _, b := setup(t, Options{Store: loja})
	idp.claims = map[string]any{"email": "ana@exemplo.com"}
	b.login(idp, "")

	visto := false
	loja.ctxRecebido = func(canceled bool) { visto = visto || canceled }

	// Uma requisição cujo contexto já morreu é o que o servidor entrega
	// quando o cliente desliga no meio: o store tem de receber esse contexto,
	// e não um context.Background() com um prazo inventado.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest("GET", "/admin", nil).WithContext(ctx)
	req.Header.Set("Accept", "text/html")
	for k, v := range b.cookies {
		req.AddCookie(&http.Cookie{Name: k, Value: v})
	}
	b.app.Handler().ServeHTTP(httptest.NewRecorder(), req)
	if !visto {
		t.Fatal("o cancelamento da requisição não chegou ao Store")
	}
}

// O banco fora do ar não é "não existe sessão". Mandar a pessoa para o login
// vira um laço de login e some com o incidente de todo log.
func TestStoreQueFalhaDa503ENaoRedirect(t *testing.T) {
	loja := novaLojaContexto()
	idp, _, b := setup(t, Options{Store: loja})
	idp.claims = map[string]any{"email": "ana@exemplo.com"}
	b.login(idp, "")

	loja.falha = errors.New("dial tcp: connection refused")
	rec := b.get("/admin", nil)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("/admin com o store fora do ar → %d, queria 503", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "" {
		t.Fatalf("virou redirect para %q", loc)
	}
}

// E a sessão que de fato não existe continua sendo o login.
func TestSemSessaoContinuaIndoParaOLogin(t *testing.T) {
	loja := novaLojaContexto()
	_, _, b := setup(t, Options{Store: loja})
	rec := b.get("/admin", nil)
	if rec.Code != http.StatusFound || !strings.Contains(rec.Header().Get("Location"), "/entrar") {
		t.Fatalf("anônimo → %d %q", rec.Code, rec.Header().Get("Location"))
	}
}

// Um Store que só tem a interface de hoje continua funcionando inteiro.
func TestStoreAntigoContinuaFuncionando(t *testing.T) {
	idp, a, b := setup(t, Options{Store: NewMemoryStore()})
	idp.claims = map[string]any{"email": "ana@exemplo.com"}
	b.login(idp, "")
	if rec := b.get("/admin", nil); rec.Code != 200 {
		t.Fatalf("/admin → %d", rec.Code)
	}
	if _, ok := a.opts.Store.(StoreContext); ok {
		t.Fatal("o MemoryStore passou a implementar StoreContext; esta spec o deixa como está")
	}
}
