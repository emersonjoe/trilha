package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/blog/internal/documentos"
	"github.com/emersonjoe/trilha/webhook"
)

// oParceiro é o outro lado: um servidor que recebe e verifica a assinatura com
// o webhook.Verify, que é o que um app em Go escreveria de verdade.
type oParceiro struct {
	mu       sync.Mutex
	eventos  []string
	corpos   []string
	segredo  string
	recusou  int
	srv      *httptest.Server
	responde int
}

func recebedor(t *testing.T) *oParceiro {
	t.Helper()
	p := &oParceiro{responde: 200}
	p.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.mu.Lock()
		segredo, status := p.segredo, p.responde
		p.mu.Unlock()

		corpo, err := webhook.Verify(r, segredo)
		if err != nil {
			p.mu.Lock()
			p.recusou++
			p.mu.Unlock()
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		p.mu.Lock()
		p.eventos = append(p.eventos, r.Header.Get(webhook.HeaderEvent))
		p.corpos = append(p.corpos, string(corpo))
		p.mu.Unlock()
		w.WriteHeader(status)
		io.WriteString(w, "ok")
	}))
	t.Cleanup(p.srv.Close)
	return p
}

func (p *oParceiro) usaOSegredo(s string) {
	p.mu.Lock()
	p.segredo = s
	p.mu.Unlock()
}

func (p *oParceiro) recusadas() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.recusou
}

func (p *oParceiro) recebidos() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string{}, p.eventos...)
}

// clienteComWebhooks é o app com o motor de webhooks domado: dev, para o
// httptest em http://127.0.0.1 passar pela checagem de endereço, e o relógio
// das retentativas rápido.
func clienteComWebhooks(t *testing.T) (*client, *webhook.Hooks) {
	t.Helper()
	t.Setenv("BLOG_TASK_STEP", "1ms")
	documentos.Reset()
	c := newClient(t, "dev")
	hooks := trilha.Use[*webhook.Hooks](c.app)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		hooks.Shutdown(ctx)
	})
	return c, hooks
}

func esperaAte(t *testing.T, porque string, cond func() bool) {
	t.Helper()
	limite := time.Now().Add(5 * time.Second)
	for time.Now().Before(limite) {
		if cond() {
			return
		}
		time.Sleep(3 * time.Millisecond)
	}
	t.Fatalf("esperei demais por: %s", porque)
}

// #112 — o ciclo inteiro: alguém cadastra o endereço pela tela, o
// processamento termina, e o parceiro recebe o evento assinado.
func TestCadastrarPelaTelaEReceberOEvento(t *testing.T) {
	p := recebedor(t)
	c, _ := clienteComWebhooks(t)

	rec := c.PostForm("/webhooks", url.Values{
		"action": {"subscribe"}, "url": {p.srv.URL},
		"label": {"Parceiro"}, "events": {"documento.processado"},
	})
	rec.WantStatus(http.StatusSeeOther)

	// O segredo aparece uma vez, na tela seguinte, e some depois.
	tela := c.Get("/webhooks").WantStatus(200)
	corpo := tela.Body.String()
	segredo := entreAspas(t, corpo, "whsec_")
	if !strings.Contains(corpo, "ui-secret-once") {
		t.Fatalf("o segredo não foi mostrado:\n%s", primeiros(corpo, 600))
	}
	if depois := c.Get("/webhooks").Body.String(); strings.Contains(depois, "whsec_") {
		t.Fatal("o segredo apareceu de novo; ele existe para um render só")
	}
	p.usaOSegredo(segredo)

	// Agora o trabalho de verdade: processar um documento dispara o evento.
	id, _ := primeiroDoc(t)
	c.PostForm("/tarefas", url.Values{"documento": {id}}).WantStatus(http.StatusSeeOther)

	esperaAte(t, "o parceiro receber o evento", func() bool { return len(p.recebidos()) == 1 })
	if got := p.recebidos()[0]; got != "documento.processado" {
		t.Fatalf("evento = %q", got)
	}
	p.mu.Lock()
	json := p.corpos[0]
	p.mu.Unlock()
	if !strings.Contains(json, `"id":"`+id+`"`) {
		t.Fatalf("o corpo não traz o documento: %s", json)
	}
	if n := p.recusadas(); n != 0 {
		t.Fatalf("o parceiro recusou %d entregas: a assinatura não confere", n)
	}

	// E a tela mostra a entrega, com o status do parceiro.
	esperaAte(t, "a entrega aparecer na tela", func() bool {
		return strings.Contains(c.Get("/webhooks").Body.String(), "Entregue")
	})
}

// Segredo errado do lado de lá: o parceiro recusa, e a tela mostra o 401 —
// que é exatamente o que alguém precisa ver para descobrir que colou errado.
func TestSegredoErradoApareceNaTela(t *testing.T) {
	p := recebedor(t)
	p.usaOSegredo("whsec_outro_completamente")
	c, _ := clienteComWebhooks(t)

	c.PostForm("/webhooks", url.Values{"action": {"subscribe"}, "url": {p.srv.URL}})
	c.Get("/webhooks") // consome o segredo

	id, _ := primeiroDoc(t)
	c.PostForm("/tarefas", url.Values{"documento": {id}})

	esperaAte(t, "o parceiro recusar", func() bool { return p.recusadas() > 0 })
	esperaAte(t, "a recusa aparecer na tela", func() bool {
		return strings.Contains(c.Get("/webhooks").Body.String(), ">401<")
	})
	if len(p.recebidos()) != 0 {
		t.Fatal("o parceiro aceitou uma assinatura que não é dele")
	}
}

// O botão de testar manda uma entrega de verdade, com a mesma assinatura — um
// teste que toma atalho é um teste que passa para uma configuração que não vai
// funcionar.
func TestBotaoDeTestarMandaUmaEntregaDeVerdade(t *testing.T) {
	p := recebedor(t)
	c, hooks := clienteComWebhooks(t)

	c.PostForm("/webhooks", url.Values{"action": {"subscribe"}, "url": {p.srv.URL}})
	segredo := entreAspas(t, c.Get("/webhooks").Body.String(), "whsec_")
	p.usaOSegredo(segredo)

	subs, _ := hooks.Subscriptions(context.Background(), "")
	if len(subs) != 1 {
		t.Fatalf("assinaturas = %d", len(subs))
	}
	c.PostForm("/webhooks", url.Values{"action": {"ping"}, "id": {subs[0].ID}}).
		WantStatus(http.StatusSeeOther)

	esperaAte(t, "o ping chegar", func() bool { return len(p.recebidos()) == 1 })
	if got := p.recebidos()[0]; got != "webhook.ping" {
		t.Fatalf("evento = %q", got)
	}
	if p.recusadas() != 0 {
		t.Fatal("o ping foi recusado: ele não está assinado como os outros")
	}
}

// Endereço que volta para dentro é recusado no formulário, com o motivo — e
// não vira uma assinatura que nunca entrega.
func TestEnderecoPrivadoEhRecusadoNoFormulario(t *testing.T) {
	c, hooks := clienteComWebhooks(t)
	// Em dev o loopback do httptest passa; o que não passa em lugar nenhum é
	// o endereço dos metadados da nuvem.
	c.PostForm("/webhooks", url.Values{
		"action": {"subscribe"}, "url": {"http://169.254.169.254/latest/meta-data/"},
	}).WantStatus(http.StatusSeeOther)

	subs, _ := hooks.Subscriptions(context.Background(), "")
	if len(subs) != 0 {
		t.Fatalf("cadastrou o endereço de metadados: %+v", subs)
	}
	if corpo := c.Get("/webhooks").Body.String(); !strings.Contains(corpo, "link-local") {
		t.Fatalf("a tela não explicou a recusa:\n%s", primeiros(corpo, 800))
	}
}

// Revogar para de receber, e as entregas antigas continuam na tela.
func TestRevogarParaDeReceber(t *testing.T) {
	p := recebedor(t)
	c, hooks := clienteComWebhooks(t)
	c.PostForm("/webhooks", url.Values{"action": {"subscribe"}, "url": {p.srv.URL}})
	p.usaOSegredo(entreAspas(t, c.Get("/webhooks").Body.String(), "whsec_"))
	subs, _ := hooks.Subscriptions(context.Background(), "")

	id, _ := primeiroDoc(t)
	c.PostForm("/tarefas", url.Values{"documento": {id}})
	esperaAte(t, "a primeira entrega", func() bool { return len(p.recebidos()) == 1 })

	c.PostForm("/webhooks", url.Values{"action": {"revoke"}, "id": {subs[0].ID}}).
		WantStatus(http.StatusSeeOther)
	outro := outroDoc(t, id)
	c.PostForm("/tarefas", url.Values{"documento": {outro}})
	time.Sleep(80 * time.Millisecond)
	if n := len(p.recebidos()); n != 1 {
		t.Fatalf("a revogada recebeu: %d", n)
	}
	corpo := c.Get("/webhooks").Body.String()
	if !strings.Contains(corpo, "Revogado") || !strings.Contains(corpo, "documento.processado") {
		t.Fatal("a tela perdeu a assinatura revogada ou as entregas dela")
	}
}

// entreAspas acha um valor que começa com o prefixo dentro de um atributo
// value="…" do HTML.
func entreAspas(t *testing.T, corpo, prefixo string) string {
	t.Helper()
	i := strings.Index(corpo, `value="`+prefixo)
	if i < 0 {
		t.Fatalf("não achei %q na página", prefixo)
	}
	resto := corpo[i+len(`value="`):]
	fim := strings.IndexByte(resto, '"')
	if fim < 0 {
		t.Fatal("atributo sem fim")
	}
	return resto[:fim]
}

func primeiros(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// outroDoc é qualquer documento que não seja o dado.
func outroDoc(t *testing.T, exceto string) string {
	t.Helper()
	lista, _ := documentos.Buscar(documentos.Consulta{Limite: 50})
	for _, d := range lista {
		if d.ID != exceto {
			return d.ID
		}
	}
	t.Fatal("o exemplo só tem um documento")
	return ""
}
