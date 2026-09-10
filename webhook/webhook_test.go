package webhook

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// relogio é um tempo que o teste controla: sem isso, provar um backoff de doze
// horas custaria doze horas.
type relogio struct {
	mu  sync.Mutex
	now time.Time
}

func (r *relogio) agora() time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.now
}

func (r *relogio) anda(d time.Duration) {
	r.mu.Lock()
	r.now = r.now.Add(d)
	r.mu.Unlock()
}

// parceiro é o servidor do outro lado: guarda o que chegou e responde o que o
// teste mandar responder.
type parceiro struct {
	mu       sync.Mutex
	chamadas []recebida
	status   int
	corpo    string
	srv      *httptest.Server
}

type recebida struct {
	id, evento, ts, assinatura string
	corpo                      []byte
}

func novoParceiro(t *testing.T) *parceiro {
	t.Helper()
	p := &parceiro{status: 200, corpo: "ok"}
	p.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		p.mu.Lock()
		p.chamadas = append(p.chamadas, recebida{
			id: r.Header.Get(HeaderID), evento: r.Header.Get(HeaderEvent),
			ts: r.Header.Get(HeaderTimestamp), assinatura: r.Header.Get(HeaderSignature),
			corpo: body,
		})
		status, corpo := p.status, p.corpo
		p.mu.Unlock()
		w.WriteHeader(status)
		io.WriteString(w, corpo)
	}))
	t.Cleanup(p.srv.Close)
	return p
}

func (p *parceiro) responde(status int, corpo string) {
	p.mu.Lock()
	p.status, p.corpo = status, corpo
	p.mu.Unlock()
}

func (p *parceiro) recebidas() []recebida {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]recebida{}, p.chamadas...)
}

func motor(t *testing.T, r *relogio, o Options) *Hooks {
	t.Helper()
	if o.Logger == nil {
		o.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if len(o.Events) == 0 {
		o.Events = []string{"documento.processado", "fluxo.concluido"}
	}
	// httptest fica em 127.0.0.1, que é justamente o que a checagem recusa: o
	// teste liga o campo que existe para isso, e há um teste só para a
	// checagem.
	o.AllowPrivateURL = true
	h := New(o)
	h.now = r.agora
	if err := h.Setup(nil); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { h.Shutdown(context.Background()) })
	return h
}

// espera é um teto, não uma soneca: ele volta assim que a condição vale. O
// teto é generoso porque a CI roda esta suíte com o detector de corrida, que é
// várias vezes mais lento — e um teto apertado ali vira uma falha que não
// diz nada sobre o código.
func espera(t *testing.T, porque string, cond func() bool) {
	t.Helper()
	limite := time.Now().Add(10 * time.Second)
	for time.Now().Before(limite) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("esperei demais por: %s", porque)
}

func entrega(t *testing.T, h *Hooks, id string) Delivery {
	t.Helper()
	d, err := h.store.Delivery(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func primeira(t *testing.T, h *Hooks) Delivery {
	t.Helper()
	var d Delivery
	espera(t, "a entrega ser gravada", func() bool {
		lista, _ := h.Deliveries(context.Background(), ListParams{})
		if len(lista) == 0 {
			return false
		}
		d = lista[len(lista)-1]
		return true
	})
	return d
}

// O que sai daqui tem de ser aceito pelo Verify do outro lado. É o teste que
// prova que os dois lados falam a mesma língua, e é por isso que ele usa o
// Verify de verdade em vez de recomputar o HMAC à mão.
func TestOQueSaiEhAceitoPeloVerify(t *testing.T) {
	r := &relogio{now: time.Now()}
	p := novoParceiro(t)
	h := motor(t, r, Options{})
	segredo, err := h.Subscribe(nil, Subscription{URL: p.srv.URL, Events: []string{"documento.processado"}})
	if err != nil {
		t.Fatal(err)
	}

	if err := h.Emit(nil, "documento.processado", map[string]any{"id": "doc-1", "paginas": 3}); err != nil {
		t.Fatal(err)
	}
	espera(t, "o parceiro receber", func() bool { return len(p.recebidas()) == 1 })
	got := p.recebidas()[0]

	if got.evento != "documento.processado" || !strings.HasPrefix(got.id, "dlv_") {
		t.Fatalf("cabeçalhos = %+v", got)
	}
	// O Verify do outro lado, com a requisição montada como ela chegou.
	req := httptest.NewRequest("POST", "/hook", strings.NewReader(string(got.corpo)))
	req.Header.Set(HeaderTimestamp, got.ts)
	req.Header.Set(HeaderSignature, got.assinatura)
	corpo, err := Verify(req, segredo.Reveal())
	if err != nil {
		t.Fatalf("o outro lado recusou o que nós assinamos: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(corpo, &payload); err != nil || payload["id"] != "doc-1" {
		t.Fatalf("corpo = %s (%v)", corpo, err)
	}
	espera(t, "a entrega virar entregue", func() bool {
		return primeira(t, h).State == Delivered
	})
	if d := primeira(t, h); d.Status != 200 || d.Attempt != 1 {
		t.Fatalf("%+v", d)
	}
}

// E tem de ser recusado quando qualquer coisa muda: o corpo, o horário, o
// segredo. Um Verify que aceita um desses não protege nada.
func TestVerifyRecusaOQueMudou(t *testing.T) {
	segredo := "whsec_teste"
	corpo := []byte(`{"id":"doc-1"}`)
	ts := time.Now().Unix()
	assina := func(s string, quando int64, b []byte) *http.Request {
		req := httptest.NewRequest("POST", "/hook", strings.NewReader(string(b)))
		req.Header.Set(HeaderTimestamp, strconv.FormatInt(quando, 10))
		req.Header.Set(HeaderSignature, Sign(s, strconv.FormatInt(quando, 10), b))
		return req
	}
	if _, err := Verify(assina(segredo, ts, corpo), segredo); err != nil {
		t.Fatalf("recusou o que estava certo: %v", err)
	}

	casos := map[string]*http.Request{
		// Os cabeçalhos do original, com outro corpo: é o caso que o HMAC
		// existe para pegar.
		"corpo trocado depois de assinar": func() *http.Request {
			req := httptest.NewRequest("POST", "/hook", strings.NewReader(`{"id":"doc-2"}`))
			req.Header = assina(segredo, ts, corpo).Header
			return req
		}(),
		"segredo errado": assina("whsec_outro", ts, corpo),
		"velho demais":   assina(segredo, ts-int64(Tolerance.Seconds())-60, corpo),
		"do futuro":      assina(segredo, ts+int64(Tolerance.Seconds())+60, corpo),
		"sem assinatura": httptest.NewRequest("POST", "/hook", strings.NewReader(string(corpo))),
	}
	for nome, req := range casos {
		if _, err := Verify(req, segredo); err == nil {
			t.Fatalf("%s passou", nome)
		}
	}
	// Segredo vazio não é "sem verificação": é recusa.
	if _, err := Verify(assina(segredo, ts, corpo), ""); err == nil {
		t.Fatal("segredo vazio passou")
	}
}

// Um parceiro que responde 500 é tentado de novo, na hora que o backoff diz —
// e não antes, que é o que separa uma retentativa de um laço.
func TestErroDoParceiroEsperaETentaDeNovo(t *testing.T) {
	r := &relogio{now: time.Now()}
	p := novoParceiro(t)
	p.responde(500, "banco fora do ar")
	h := motor(t, r, Options{Tick: 5 * time.Millisecond,
		Backoff: []time.Duration{time.Minute, 5 * time.Minute}})
	if _, err := h.Subscribe(nil, Subscription{URL: p.srv.URL}); err != nil {
		t.Fatal(err)
	}
	h.Emit(nil, "fluxo.concluido", map[string]string{"id": "f-1"})

	espera(t, "a primeira tentativa", func() bool { return len(p.recebidas()) == 1 })
	d := primeira(t, h)
	espera(t, "a primeira falha ser gravada", func() bool { return entrega(t, h, d.ID).Attempt == 1 })
	got := entrega(t, h, d.ID)
	if got.State != Pending || got.Status != 500 || !strings.Contains(got.Response, "banco fora do ar") {
		t.Fatalf("%+v", got)
	}
	if !got.NextTry.Equal(r.agora().Add(time.Minute)) {
		t.Fatalf("a próxima tentativa está marcada para %s, e o backoff diz %s",
			got.NextTry, r.agora().Add(time.Minute))
	}

	// Antes da hora, ninguém tenta.
	r.anda(30 * time.Second)
	time.Sleep(30 * time.Millisecond)
	if n := len(p.recebidas()); n != 1 {
		t.Fatalf("tentou %d vezes antes da hora", n)
	}
	// Passada a hora, tenta.
	r.anda(31 * time.Second)
	// Esperar a tentativa ser *gravada*, e não só chegar do outro lado: entre
	// as duas coisas o motor ainda não marcou a próxima hora, e adiantar o
	// relógio aí marca-a a partir do futuro — a terceira tentativa nunca
	// vence, e o teste falha por uma corrida que só ele tem.
	espera(t, "a segunda tentativa ser gravada", func() bool { return entrega(t, h, d.ID).Attempt == 2 })

	// E quando o parceiro volta, entrega.
	p.responde(200, "ok")
	r.anda(6 * time.Minute)
	espera(t, "a entrega", func() bool { return entrega(t, h, d.ID).State == Delivered })
	if got := entrega(t, h, d.ID); got.Attempt != 3 {
		t.Fatalf("tentativas = %d", got.Attempt)
	}
}

// Acabada a paciência, desiste — e guarda o que o parceiro disse, que é o que
// resolve o problema em um minuto em vez de uma tarde.
func TestDepoisDeTodasAsTentativasDesiste(t *testing.T) {
	r := &relogio{now: time.Now()}
	p := novoParceiro(t)
	p.responde(422, `{"erro":"campo destinatario obrigatorio"}`)
	backoff := []time.Duration{time.Minute, 2 * time.Minute}
	h := motor(t, r, Options{Tick: 5 * time.Millisecond, Backoff: backoff})
	h.Subscribe(nil, Subscription{URL: p.srv.URL})
	h.Emit(nil, "fluxo.concluido", map[string]string{"id": "f-1"})

	d := primeira(t, h)
	for i := range backoff {
		// Gravada, e não só recebida: adiantar o relógio antes de o motor
		// marcar a próxima hora é marcá-la a partir do futuro.
		espera(t, "a tentativa ser gravada", func() bool { return entrega(t, h, d.ID).Attempt == i+1 })
		r.anda(backoff[i] + time.Second)
	}
	espera(t, "desistir", func() bool { return entrega(t, h, d.ID).State == Failed })
	got := entrega(t, h, d.ID)
	if got.Attempt != len(backoff)+1 {
		t.Fatalf("tentativas = %d, queria %d", got.Attempt, len(backoff)+1)
	}
	if got.Status != 422 || !strings.Contains(got.Response, "campo destinatario obrigatorio") {
		t.Fatalf("o que o parceiro disse não ficou guardado: %+v", got)
	}
	if got.Ended.IsZero() {
		t.Fatal("desistiu sem hora")
	}
	// E não tenta mais.
	r.anda(24 * time.Hour)
	time.Sleep(30 * time.Millisecond)
	if n := len(p.recebidas()); n != len(backoff)+1 {
		t.Fatalf("continuou tentando depois de desistir: %d", n)
	}
}

// Reenviar é uma entrega nova ligada à original, com os mesmos bytes — e a
// original continua lá, porque é o motivo de alguém ter apertado o botão.
func TestReenviarCriaOutraLigadaAOriginal(t *testing.T) {
	r := &relogio{now: time.Now()}
	p := novoParceiro(t)
	p.responde(500, "não")
	h := motor(t, r, Options{Tick: 5 * time.Millisecond,
		Backoff: []time.Duration{time.Minute}})
	h.Subscribe(nil, Subscription{URL: p.srv.URL})
	h.Emit(nil, "fluxo.concluido", map[string]string{"id": "f-1"})

	d := primeira(t, h)
	espera(t, "a primeira falha", func() bool { return entrega(t, h, d.ID).Attempt == 1 })
	r.anda(2 * time.Minute)
	espera(t, "desistir", func() bool { return entrega(t, h, d.ID).State == Failed })

	p.responde(200, "ok")
	novoID, err := h.Retry(nil, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if novoID == d.ID {
		t.Fatal("o reenvio devia ser uma entrega nova")
	}
	espera(t, "o reenvio", func() bool { return entrega(t, h, novoID).State == Delivered })
	novo := entrega(t, h, novoID)
	if novo.RetryOf != d.ID {
		t.Fatalf("o reenvio não aponta para a original: %+v", novo)
	}
	if string(novo.Payload) != string(entrega(t, h, d.ID).Payload) {
		t.Fatal("o reenvio mandou outros bytes; a assinatura seria de outro evento")
	}
	if entrega(t, h, d.ID).State != Failed {
		t.Fatal("a falha original sumiu")
	}
}

// O Emit não espera a rede. É o motivo de o módulo existir, e o parceiro que
// demora é quem prova.
func TestEmitNaoEsperaARede(t *testing.T) {
	r := &relogio{now: time.Now()}
	solta := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		<-solta
		w.WriteHeader(200)
	}))
	defer srv.Close()
	defer close(solta)

	h := motor(t, r, Options{})
	h.Subscribe(nil, Subscription{URL: srv.URL})
	inicio := time.Now()
	if err := h.Emit(nil, "fluxo.concluido", map[string]string{"id": "f-1"}); err != nil {
		t.Fatal(err)
	}
	if passou := time.Since(inicio); passou > time.Second {
		t.Fatalf("o Emit esperou %s pelo parceiro", passou)
	}
}

// Evento fora da lista é erro na hora. A lista é fechada para que um erro de
// digitação no Emit não vire um evento que ninguém assina — que é idêntico,
// visto de fora, a um parceiro que não está ouvindo.
func TestEventoForaDaListaEErro(t *testing.T) {
	h := motor(t, &relogio{now: time.Now()}, Options{})
	if err := h.Emit(nil, "documento.procesado", nil); err == nil {
		t.Fatal("aceitou um evento com erro de digitação")
	}
	if _, err := h.Subscribe(nil, Subscription{URL: "https://exemplo.com/h", Events: []string{"nao.existe"}}); err == nil {
		t.Fatal("aceitou assinar um evento que não existe")
	}
}

// Assinatura revogada para de receber, e as entregas dela continuam na tela.
func TestRevogadaNaoRecebeMais(t *testing.T) {
	r := &relogio{now: time.Now()}
	p := novoParceiro(t)
	h := motor(t, r, Options{})
	segredo, _ := h.Subscribe(nil, Subscription{URL: p.srv.URL})
	_ = segredo
	subs, _ := h.Subscriptions(context.Background(), "")
	if len(subs) != 1 {
		t.Fatalf("assinaturas = %d", len(subs))
	}
	h.Emit(nil, "fluxo.concluido", map[string]string{"id": "1"})
	espera(t, "a primeira", func() bool { return len(p.recebidas()) == 1 })

	if err := h.Revoke(nil, subs[0].ID); err != nil {
		t.Fatal(err)
	}
	h.Emit(nil, "fluxo.concluido", map[string]string{"id": "2"})
	time.Sleep(30 * time.Millisecond)
	if n := len(p.recebidas()); n != 1 {
		t.Fatalf("a revogada recebeu: %d", n)
	}
	if lista, _ := h.Deliveries(context.Background(), ListParams{}); len(lista) != 1 {
		t.Fatalf("as entregas da revogada sumiram: %d", len(lista))
	}
	// E a assinatura continua na listagem, marcada. A tela precisa dela: as
	// entregas já feitas apontam para ali, e "qual endereço era esse?" é a
	// pergunta que alguém faz depois. Quem para a entrega é o Wants.
	depois, _ := h.Subscriptions(context.Background(), "")
	if len(depois) != 1 || !depois[0].Revoked {
		t.Fatalf("a revogada sumiu da listagem: %+v", depois)
	}
	if depois[0].Wants("fluxo.concluido") {
		t.Fatal("uma assinatura revogada continua querendo eventos")
	}
}

// A assinatura só ouve o que pediu.
func TestAssinaturaSoOuveOQuePediu(t *testing.T) {
	r := &relogio{now: time.Now()}
	p := novoParceiro(t)
	h := motor(t, r, Options{})
	h.Subscribe(nil, Subscription{URL: p.srv.URL, Events: []string{"fluxo.concluido"}})

	h.Emit(nil, "documento.processado", nil)
	time.Sleep(30 * time.Millisecond)
	if n := len(p.recebidas()); n != 0 {
		t.Fatalf("recebeu um evento que não assinou: %d", n)
	}
	h.Emit(nil, "fluxo.concluido", nil)
	espera(t, "o evento assinado", func() bool { return len(p.recebidas()) == 1 })
}

// O segredo aparece uma vez. O que fica guardado é um trilha.Secret, que se
// mascara sozinho quando alguém o imprime — inclusive num log.
func TestSegredoApareceUmaVezEDepoisSeMascara(t *testing.T) {
	r := &relogio{now: time.Now()}
	p := novoParceiro(t)
	h := motor(t, r, Options{})
	segredo, err := h.Subscribe(nil, Subscription{URL: p.srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(segredo.Reveal(), "whsec_") || len(segredo.Reveal()) < 40 {
		t.Fatalf("segredo = %q", segredo.Reveal())
	}
	subs, _ := h.Subscriptions(context.Background(), "")
	guardado := subs[0].Secret
	if got := guardado.String(); strings.Contains(got, segredo.Reveal()) {
		t.Fatalf("o segredo saiu inteiro numa impressão: %q", got)
	}
}

// Um evento chega uma vez ao parceiro, ainda que a mesma entrega entre na fila
// duas vezes — o Emit põe ela lá, e o relógio põe de novo quando a linha
// vence. Dois workers segurando o mesmo id ao mesmo tempo é o parceiro
// recebendo o mesmo evento duas vezes, que é a coisa que um webhook não pode
// fazer sem avisar.
func TestAMesmaEntregaNaoSaiDuasVezes(t *testing.T) {
	r := &relogio{now: time.Now()}
	var chamadas int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		atomic.AddInt32(&chamadas, 1)
		// A entrega demora: é durante ela que o relógio bate de novo e acha a
		// mesma linha, ainda pendente e ainda vencida.
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	// Relógio rápido e vários workers: a combinação que faz a mesma linha ser
	// pescada de novo enquanto a primeira tentativa ainda está de pé.
	h := motor(t, r, Options{Tick: time.Millisecond, Workers: 4, HTTP: srv.Client()})
	h.Subscribe(nil, Subscription{URL: srv.URL})
	h.Emit(nil, "fluxo.concluido", map[string]string{"id": "1"})

	espera(t, "a entrega terminar", func() bool { return primeira(t, h).State == Delivered })
	// E mais um tanto de tiques depois do fim, para nada aparecer atrasado.
	time.Sleep(30 * time.Millisecond)
	if n := atomic.LoadInt32(&chamadas); n != 1 {
		t.Fatalf("o parceiro recebeu o mesmo evento %d vezes", n)
	}
}
