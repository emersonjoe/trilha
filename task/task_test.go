package task

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

func novo(t *testing.T, o Options) *Tasks {
	t.Helper()
	if o.Logger == nil {
		o.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	tk := New(o)
	if err := tk.Setup(nil); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { tk.Shutdown(context.Background()) })
	return tk
}

// espera segura o teste até a condição valer, para não depender de sleep — que
// é lento quando passa e mentiroso quando falha.
func espera(t *testing.T, porque string, cond func() bool) {
	t.Helper()
	limite := time.Now().Add(3 * time.Second)
	for time.Now().Before(limite) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("esperei demais por: %s", porque)
}

func estado(t *testing.T, tk *Tasks, id string) Task {
	t.Helper()
	got, err := tk.Get(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// A tarefa roda fora da requisição e o estado sobra para a tela ler: começou,
// os passos, terminou.
func TestRodaEGuardaOAndamento(t *testing.T) {
	solta := make(chan struct{})
	tk := novo(t, Options{})
	tk.Handle("processar", func(ctx context.Context, p *Progress) error {
		p.Step("OCR", 1, 2)
		<-solta
		p.Step("Indexar", 2, 2)
		return nil
	})

	id, err := tk.Run(nil, "processar", "doc-1")
	if err != nil {
		t.Fatal(err)
	}
	espera(t, "o primeiro passo", func() bool { return estado(t, tk, id).Step == "OCR" })
	meio := estado(t, tk, id)
	if meio.State != Running || meio.N != 1 || meio.Of != 2 || meio.Percent() != 50 {
		t.Fatalf("no meio: %+v (%d%%)", meio, meio.Percent())
	}
	close(solta)

	espera(t, "o fim", func() bool { return !estado(t, tk, id).State.Live() })
	fim := estado(t, tk, id)
	if fim.State != Done || fim.Err != "" || fim.Ended.IsZero() {
		t.Fatalf("no fim: %+v", fim)
	}
	if fim.Percent() != 100 {
		t.Fatalf("percentual = %d", fim.Percent())
	}
}

// O duplo clique no botão não processa o documento duas vezes: com uma viva,
// o segundo Run devolve o id da primeira.
func TestMesmaChaveNaoRodaDuasVezes(t *testing.T) {
	solta := make(chan struct{})
	var chamadas int
	var mu sync.Mutex
	tk := novo(t, Options{Workers: 4})
	tk.Handle("processar", func(ctx context.Context, p *Progress) error {
		mu.Lock()
		chamadas++
		mu.Unlock()
		<-solta
		return nil
	})

	primeiro, err := tk.Run(nil, "processar", "doc-1")
	if err != nil {
		t.Fatal(err)
	}
	segundo, err := tk.Run(nil, "processar", "doc-1")
	if err != nil {
		t.Fatal(err)
	}
	if primeiro != segundo {
		t.Fatalf("dois ids para a mesma chave: %s e %s", primeiro, segundo)
	}
	// Outra chave é outra tarefa: o dedupe é por chave, não por nome.
	outro, err := tk.Run(nil, "processar", "doc-2")
	if err != nil {
		t.Fatal(err)
	}
	if outro == primeiro {
		t.Fatal("chaves diferentes viraram a mesma tarefa")
	}
	close(solta)
	espera(t, "as duas terminarem", func() bool {
		return !estado(t, tk, primeiro).State.Live() && !estado(t, tk, outro).State.Live()
	})

	// E depois de terminar, a mesma chave roda de novo — senão um documento
	// só poderia ser processado uma vez na vida.
	terceiro, err := tk.Run(nil, "processar", "doc-1")
	if err != nil {
		t.Fatal(err)
	}
	if terceiro == primeiro {
		t.Fatal("a chave ficou presa depois de terminar")
	}
	mu.Lock()
	defer mu.Unlock()
	if chamadas < 2 {
		t.Fatalf("a função rodou %d vezes", chamadas)
	}
}

// Panic vira erro gravado. Um panic numa goroutine derruba o processo inteiro,
// e perder o servidor web porque um documento tinha uma página ruim não é uma
// troca que alguém faria de propósito.
func TestPanicViraErroENaoDerrubaOProcesso(t *testing.T) {
	tk := novo(t, Options{})
	tk.Handle("explode", func(ctx context.Context, p *Progress) error {
		panic("uma página ruim")
	})
	tk.Handle("depois", func(ctx context.Context, p *Progress) error { return nil })

	id, _ := tk.Run(nil, "explode", "doc-1")
	espera(t, "a falha", func() bool { return !estado(t, tk, id).State.Live() })
	got := estado(t, tk, id)
	if got.State != Failed || !strings.Contains(got.Err, "uma página ruim") {
		t.Fatalf("%+v", got)
	}

	// E o motor continua de pé: é isso que o teste está mesmo provando.
	outro, err := tk.Run(nil, "depois", "x")
	if err != nil {
		t.Fatal(err)
	}
	espera(t, "a próxima rodar", func() bool { return estado(t, tk, outro).State == Done })
}

// O erro da função vira a mensagem que a tela mostra.
func TestErroViraMensagem(t *testing.T) {
	tk := novo(t, Options{})
	tk.Handle("falha", func(ctx context.Context, p *Progress) error {
		return errors.New("o OCR não leu a página 3")
	})
	id, _ := tk.Run(nil, "falha", "doc-1")
	espera(t, "a falha", func() bool { return !estado(t, tk, id).State.Live() })
	if got := estado(t, tk, id); got.State != Failed || got.Err != "o OCR não leu a página 3" {
		t.Fatalf("%+v", got)
	}
}

// Nome sem Handle é erro no Run, e não uma tarefa parada para sempre
// esperando uma função que não existe.
func TestNomeDesconhecidoEErroNaHora(t *testing.T) {
	tk := novo(t, Options{})
	if _, err := tk.Run(nil, "ninguem-registrou", "x"); !errors.Is(err, ErrUnknownTask) {
		t.Fatalf("erro = %v", err)
	}
}

// O Shutdown espera quem está rodando: um deploy no meio de um processamento
// não pode cortar o processamento no meio.
func TestShutdownEsperaTerminar(t *testing.T) {
	terminou := make(chan struct{})
	tk := New(Options{Logger: quieto(), Shutdown: 5 * time.Second})
	tk.Handle("longa", func(ctx context.Context, p *Progress) error {
		time.Sleep(150 * time.Millisecond)
		close(terminou)
		return nil
	})
	if err := tk.Setup(nil); err != nil {
		t.Fatal(err)
	}
	id, _ := tk.Run(nil, "longa", "x")
	espera(t, "começar", func() bool { return estado(t, tk, id).State == Running })

	if err := tk.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-terminou:
	default:
		t.Fatal("o shutdown voltou antes de a tarefa terminar")
	}
	if got := estado(t, tk, id); got.State != Done {
		t.Fatalf("%+v", got)
	}
}

// Mas não espera para sempre: passado o prazo, o context da tarefa é
// cancelado — senão uma tarefa travada segura o deploy.
func TestShutdownCancelaDepoisDoPrazo(t *testing.T) {
	viuOCancelamento := make(chan struct{})
	tk := New(Options{Logger: quieto(), Shutdown: 50 * time.Millisecond})
	tk.Handle("travada", func(ctx context.Context, p *Progress) error {
		<-ctx.Done()
		close(viuOCancelamento)
		return ctx.Err()
	})
	if err := tk.Setup(nil); err != nil {
		t.Fatal(err)
	}
	id, _ := tk.Run(nil, "travada", "x")
	espera(t, "começar", func() bool { return estado(t, tk, id).State == Running })

	inicio := time.Now()
	tk.Shutdown(context.Background())
	select {
	case <-viuOCancelamento:
	default:
		t.Fatal("a tarefa não foi cancelada")
	}
	if passou := time.Since(inicio); passou > 2*time.Second {
		t.Fatalf("o shutdown levou %s", passou)
	}
	// Cancelada não é falha: é interrompida, que é o que a tela precisa
	// dizer para alguém apertar "tentar de novo" sem medo.
	if got := estado(t, tk, id); got.State != Interrupted {
		t.Fatalf("%+v", got)
	}
}

// O Cancel chega na função. Não é um kill: quem ignora o context roda até o
// fim, e quem respeita para onde olhou.
func TestCancelChegaNaFuncao(t *testing.T) {
	tk := novo(t, Options{})
	tk.Handle("longa", func(ctx context.Context, p *Progress) error {
		<-ctx.Done()
		return ctx.Err()
	})
	id, _ := tk.Run(nil, "longa", "x")
	espera(t, "começar", func() bool { return estado(t, tk, id).State == Running })
	if err := tk.Cancel(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	espera(t, "acabar", func() bool { return !estado(t, tk, id).State.Live() })
	if got := estado(t, tk, id); got.State != Interrupted {
		t.Fatalf("%+v", got)
	}
}

// O bug clássico: tarefa "running" para sempre, de um processo que morreu. Na
// subida seguinte ela vira interrupted, e a tela para de esperar.
func TestSubidaMarcaOQueOProcessoAnteriorDeixou(t *testing.T) {
	store := Memory()
	fantasma := Task{ID: "abc", Name: "processar", Key: "doc-1", State: Running,
		Queued: time.Now().Add(-time.Hour), Started: time.Now().Add(-time.Hour)}
	if err := store.Save(context.Background(), fantasma); err != nil {
		t.Fatal(err)
	}

	tk := novo(t, Options{Store: store})
	got := estado(t, tk, "abc")
	if got.State != Interrupted || got.Ended.IsZero() {
		t.Fatalf("%+v", got)
	}
	if !strings.Contains(got.Err, "process") {
		t.Fatalf("a mensagem não explica: %q", got.Err)
	}
	// E a chave ficou livre: era isso que o "running" eterno impedia.
	tk.Handle("processar", func(ctx context.Context, p *Progress) error { return nil })
	novoID, err := tk.Run(nil, "processar", "doc-1")
	if err != nil || novoID == "abc" {
		t.Fatalf("id = %q, err = %v", novoID, err)
	}
}

// Tentar de novo é o motivo de o Handle ser separado do Run: depois de um
// reinício a closure não existe mais, e a função registrada por nome existe.
func TestRetryRodaDeNovoDepoisDeFalhar(t *testing.T) {
	var vezes int
	var mu sync.Mutex
	tk := novo(t, Options{})
	tk.Handle("instavel", func(ctx context.Context, p *Progress) error {
		mu.Lock()
		vezes++
		primeira := vezes == 1
		mu.Unlock()
		if primeira {
			return errors.New("a primeira sempre falha")
		}
		return nil
	})

	id, _ := tk.Run(nil, "instavel", "doc-1")
	espera(t, "falhar", func() bool { return estado(t, tk, id).State == Failed })

	outro, err := tk.Retry(nil, id)
	if err != nil {
		t.Fatal(err)
	}
	if outro == id {
		t.Fatal("o retry devia ser uma execução nova, com id próprio")
	}
	espera(t, "a segunda", func() bool { return estado(t, tk, outro).State == Done })
	// A que falhou continua lá: a tela perderia o motivo de alguém ter
	// apertado o botão.
	if got := estado(t, tk, id); got.State != Failed {
		t.Fatalf("a falha sumiu: %+v", got)
	}
}

// Retry de algo que ainda está indo é esperar, não uma segunda execução.
func TestRetryDoQueEstaRodandoDevolveAMesma(t *testing.T) {
	solta := make(chan struct{})
	tk := novo(t, Options{})
	tk.Handle("longa", func(ctx context.Context, p *Progress) error { <-solta; return nil })
	id, _ := tk.Run(nil, "longa", "x")
	espera(t, "começar", func() bool { return estado(t, tk, id).State == Running })
	if outro, err := tk.Retry(nil, id); err != nil || outro != id {
		t.Fatalf("id = %q, err = %v", outro, err)
	}
	close(solta)
}

// A fila cheia é uma resposta, e não uma fatia que cresce: a tela consegue
// dizer "daqui a pouco", e um vazamento de memória não consegue dizer nada.
func TestFilaCheiaEErro(t *testing.T) {
	solta := make(chan struct{})
	defer close(solta)
	tk := novo(t, Options{Queue: 1})
	tk.Handle("longa", func(ctx context.Context, p *Progress) error { <-solta; return nil })

	tk.Run(nil, "longa", "a") // esta pega o worker
	espera(t, "ocupar o worker", func() bool { return ativa(t, tk, "longa", "a") == Running })
	tk.Run(nil, "longa", "b") // esta ocupa a fila
	if _, err := tk.Run(nil, "longa", "c"); err == nil || !strings.Contains(err.Error(), "queue is full") {
		t.Fatalf("erro = %v", err)
	}
}

// List é a tela de administração: mais nova primeiro, filtrada pelo que a
// pessoa escolheu.
func TestListFiltraEOrdena(t *testing.T) {
	tk := novo(t, Options{})
	tk.Handle("a", func(ctx context.Context, p *Progress) error { return nil })
	tk.Handle("b", func(ctx context.Context, p *Progress) error { return errors.New("não") })
	primeiro, _ := tk.Run(nil, "a", "1")
	ultimo, _ := tk.Run(nil, "b", "2")
	espera(t, "as duas", func() bool {
		return !estado(t, tk, primeiro).State.Live() && !estado(t, tk, ultimo).State.Live()
	})

	todas, err := tk.List(context.Background(), ListParams{})
	if err != nil || len(todas) != 2 {
		t.Fatalf("%d tarefas, err = %v", len(todas), err)
	}
	if todas[0].ID != ultimo {
		t.Fatal("a mais nova não veio primeiro")
	}
	falhas, _ := tk.List(context.Background(), ListParams{State: Failed})
	if len(falhas) != 1 || falhas[0].ID != ultimo {
		t.Fatalf("filtro por estado: %+v", falhas)
	}
	if porNome, _ := tk.List(context.Background(), ListParams{Name: "a"}); len(porNome) != 1 {
		t.Fatalf("filtro por nome: %+v", porNome)
	}
}

// Percent tem de sobreviver a "0 de 0", que é a divisão que alguém faz
// eventualmente.
func TestPercentNaoDivideporZero(t *testing.T) {
	if got := (Task{}).Percent(); got != -1 {
		t.Fatalf("sem passos = %d", got)
	}
	if got := (Task{State: Done}).Percent(); got != 100 {
		t.Fatalf("pronta = %d", got)
	}
	if got := (Task{N: 9, Of: 4}).Percent(); got != 100 {
		t.Fatalf("passou do fim = %d", got)
	}
}

// ativa é o estado da tarefa viva desta chave, ou "" quando não há nenhuma.
func ativa(t *testing.T, tk *Tasks, name, key string) State {
	t.Helper()
	id, err := tk.store.Active(context.Background(), name, key)
	if err != nil || id == "" {
		return ""
	}
	return estado(t, tk, id).State
}

func quieto() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }
