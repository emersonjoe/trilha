package webhook

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/emersonjoe/trilha"
)

// A URL é do parceiro, mas quem digita trabalha aqui. Um formulário que aceita
// http://169.254.169.254/ transforma o próprio servidor na coisa que busca as
// credenciais da máquina e entrega para quem cadastrou o endereço.
func TestEnderecoQueVoltaParaDentroEhRecusado(t *testing.T) {
	h := New(Options{Env: trilha.Prod, Logger: quieto()})
	proibidos := map[string]string{
		"a própria máquina":       "https://127.0.0.1/hook",
		"localhost pelo nome":     "https://localhost/hook",
		"loopback em IPv6":        "https://[::1]/hook",
		"metadados da nuvem":      "https://169.254.169.254/latest/meta-data/",
		"rede privada 10/8":       "https://10.0.0.5/hook",
		"rede privada 192.168/16": "https://192.168.1.10/hook",
		"rede privada 172.16/12":  "https://172.20.0.3/hook",
		"CGNAT":                   "https://100.90.1.2/hook",
		"não especificado":        "https://0.0.0.0/hook",
	}
	for nome, url := range proibidos {
		err := h.check(url)
		if err == nil {
			t.Fatalf("%s passou: %s", nome, url)
		}
		// A mensagem tem de dizer o que aconteceu para quem digitou a URL —
		// "inválida" não é acionável, "resolve para a própria máquina" é.
		if !strings.Contains(err.Error(), "webhook:") || len(err.Error()) < 40 {
			t.Fatalf("%s: a mensagem não explica: %v", nome, err)
		}
	}
}

// E um endereço público passa. Sem isto, o teste acima passaria com uma função
// que recusa tudo.
func TestEnderecoPublicoPassa(t *testing.T) {
	h := publico(t, net.ParseIP("93.184.216.34"))
	if err := h.check("https://parceiro.exemplo/hook"); err != nil {
		t.Fatalf("recusou um endereço público: %v", err)
	}
}

// O caso que uma checagem descuidada deixa passar: um nome que responde com um
// endereço público **e** com o loopback. Olhar só o primeiro é o bug, e é por
// isso que a checagem olha todos.
func TestNomeComUmEnderecoBomEUmRuimEhRecusado(t *testing.T) {
	h := publico(t, net.ParseIP("93.184.216.34"), net.ParseIP("127.0.0.1"))
	err := h.check("https://parceiro.exemplo/hook")
	if err == nil {
		t.Fatal("passou olhando só o primeiro endereço")
	}
	if !strings.Contains(err.Error(), "127.0.0.1") {
		t.Fatalf("a mensagem não diz qual endereço: %v", err)
	}
}

// Nome que não resolve é recusa, e não uma entrega que sai para lugar nenhum.
func TestNomeQueNaoResolveEhRecusado(t *testing.T) {
	h := New(Options{Env: trilha.Prod, Logger: quieto()})
	h.resolve = func(string) ([]net.IP, error) { return nil, errors.New("sem DNS") }
	if err := h.check("https://parceiro.exemplo/hook"); err == nil {
		t.Fatal("passou sem resolver")
	}
}

// publico é um Hooks cujo DNS responde o que o teste mandar — para a suíte não
// depender de rede, e para o caso de vários endereços ser testável.
func publico(t *testing.T, ips ...net.IP) *Hooks {
	t.Helper()
	h := New(Options{Env: trilha.Prod, Logger: quieto()})
	h.resolve = func(string) ([]net.IP, error) { return ips, nil }
	return h
}

// http:// carrega a assinatura e o payload pela rede de outra pessoa. Em dev
// passa, porque é onde o parceiro é um servidor de teste; fora dele, não.
func TestHTTPSoEmDev(t *testing.T) {
	dev := publico(t, net.ParseIP("93.184.216.34"))
	dev.env = trilha.Dev
	if err := dev.check("http://parceiro.exemplo/hook"); err != nil {
		t.Fatalf("dev recusou http: %v", err)
	}
	prod := publico(t, net.ParseIP("93.184.216.34"))
	err := prod.check("http://parceiro.exemplo/hook")
	if err == nil {
		t.Fatal("produção aceitou http")
	}
	if !strings.Contains(err.Error(), "https") {
		t.Fatalf("a mensagem não diz o que fazer: %v", err)
	}
	// O valor zero de Env é Prod, que é o lado seguro para um campo que
	// alguém esquece de preencher.
	zero := publico(t, net.ParseIP("93.184.216.34"))
	if err := zero.check("http://parceiro.exemplo/hook"); err == nil {
		t.Fatal("o valor zero de Env deixou passar http")
	}
}

func TestEsquemaEstranhoEhRecusado(t *testing.T) {
	h := publico(t, net.ParseIP("93.184.216.34"))
	h.env = trilha.Dev
	for _, u := range []string{"file:///etc/passwd", "gopher://exemplo.com/", "ftp://exemplo.com/", "/sem-esquema", "https://"} {
		if err := h.check(u); err == nil {
			t.Fatalf("%q passou", u)
		}
	}
}

// A checagem acontece no cadastro e outra vez na entrega. Um nome que resolvia
// para o parceiro quando foi cadastrado e resolve para 169.254.169.254 agora é
// o ataque; uma checagem só seria o acidente.
func TestOEnderecoEhConferidoDeNovoNaEntrega(t *testing.T) {
	r := &relogio{now: time.Now()}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	// Cadastra com a checagem desligada, que é o equivalente a um nome que
	// respondia outra coisa naquele momento.
	h := motor(t, r, Options{})
	if _, err := h.Subscribe(nil, Subscription{URL: srv.URL}); err != nil {
		t.Fatal(err)
	}
	// Agora a checagem volta a valer, e o endereço é o loopback do httptest.
	h.allow = false
	h.env = trilha.Prod

	h.Emit(nil, "fluxo.concluido", map[string]string{"id": "1"})
	d := primeira(t, h)
	espera(t, "a tentativa registrar a recusa", func() bool { return entrega(t, h, d.ID).Attempt == 1 })
	got := entrega(t, h, d.ID)
	if got.Status != 0 || !strings.Contains(got.Err, "webhook:") {
		t.Fatalf("a entrega não foi barrada pelo endereço: %+v", got)
	}
}

// Assinar já recusa, para o erro aparecer no formulário e não numa entrega
// que nunca sai.
func TestAssinarRecusaEnderecoPrivado(t *testing.T) {
	h := New(Options{Env: trilha.Prod, Logger: quieto(), Events: []string{"x"}})
	if _, err := h.Subscribe(nil, Subscription{URL: "https://169.254.169.254/hook"}); err == nil {
		t.Fatal("cadastrou um endereço de metadados")
	}
	if subs, _ := h.Subscriptions(context.Background(), ""); len(subs) != 0 {
		t.Fatal("gravou mesmo assim")
	}
}

func quieto() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// Em dev o parceiro está na sua máquina: um receptor em localhost:4000 é como
// qualquer pessoa experimenta isto na primeira vez, e uma checagem que recusa
// isso é uma checagem que alguém desliga inteira.
func TestEmDevOLocalPassa(t *testing.T) {
	h := New(Options{Env: trilha.Dev, Logger: quieto()})
	for _, u := range []string{"http://127.0.0.1:4000/hook", "http://localhost:4000/hook", "http://10.0.0.5/hook"} {
		if err := h.check(u); err != nil {
			t.Fatalf("dev recusou %s: %v", u, err)
		}
	}
}

// Mas os metadados da nuvem não passam nem em dev: não é o ambiente de
// desenvolvimento de ninguém, e uma configuração de dev que sobe para produção
// é exatamente como esse caso morde.
func TestNemEmDevOsMetadadosPassam(t *testing.T) {
	h := New(Options{Env: trilha.Dev, Logger: quieto()})
	for _, u := range []string{"http://169.254.169.254/latest/meta-data/", "http://0.0.0.0/hook"} {
		if err := h.check(u); err == nil {
			t.Fatalf("dev aceitou %s", u)
		}
	}
}
