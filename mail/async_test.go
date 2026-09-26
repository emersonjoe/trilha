package mail

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/textproto"
	"sync"
	"testing"
	"time"

	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/task"
)

// roteiro answers each delivery with the next error of the list (nil after
// it runs out) and counts the attempts.
type roteiro struct {
	mu     sync.Mutex
	erros  []error
	vezes  int
	espera chan struct{}
}

func (r *roteiro) Deliver(ctx context.Context, from string, to []string, raw []byte) error {
	if r.espera != nil {
		<-r.espera
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.vezes++
	if len(r.erros) == 0 {
		return nil
	}
	err := r.erros[0]
	r.erros = r.erros[1:]
	return err
}

func (r *roteiro) tentativas() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.vezes
}

func runner(t *testing.T) *task.Tasks {
	t.Helper()
	tk := task.New(task.Options{})
	if err := tk.Setup(nil); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tk.Shutdown(context.Background()) })
	return tk
}

func final(t *testing.T, tk *task.Tasks) task.Task {
	t.Helper()
	fim := time.Now().Add(5 * time.Second)
	for time.Now().Before(fim) {
		ts, err := tk.List(context.Background(), task.ListParams{Name: TaskName})
		if err != nil {
			t.Fatal(err)
		}
		if len(ts) == 1 && !ts[0].State.Live() {
			return ts[0]
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("a entrega não terminou")
	return task.Task{}
}

var msg = Message{To: []string{"ana@org.br"}, Subject: "Oi", Body: h.Text("oi")}

// #286: o Send volta na hora; o servidor lento fica com o runner.
func TestSendAssincronoNaoEspera(t *testing.T) {
	tk := runner(t)
	r := &roteiro{espera: make(chan struct{})}
	m := New(Options{From: "a@b.com", Transport: r, Tasks: tk, Logger: silencio()})
	inicio := time.Now()
	if err := m.Send(context.Background(), msg); err != nil {
		t.Fatal(err)
	}
	if time.Since(inicio) > time.Second {
		t.Fatal("o Send esperou o servidor")
	}
	close(r.espera)
	if got := final(t, tk); got.State != task.Done || r.tentativas() != 1 {
		t.Fatalf("%+v, %d tentativas", got, r.tentativas())
	}
	// O que dá para saber sem o servidor continua sendo erro no Send.
	if err := m.Send(context.Background(), Message{Subject: "sem ninguém"}); err == nil {
		t.Fatal("sem destinatário tem que ser erro no Send")
	}
}

func TestSendAssincronoTentaDeNovoSoOTemporario(t *testing.T) {
	espera := []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond}

	tk := runner(t)
	r := &roteiro{erros: []error{&textproto.Error{Code: 451, Msg: "greylisted"}, context.DeadlineExceeded}}
	m := New(Options{From: "a@b.com", Transport: r, Tasks: tk, Backoff: espera, Logger: silencio()})
	if err := m.Send(nil, msg); err != nil {
		t.Fatal(err)
	}
	if got := final(t, tk); got.State != task.Done || r.tentativas() != 3 {
		t.Fatalf("4xx e timeout são temporários: %+v, %d tentativas", got, r.tentativas())
	}

	tk = runner(t)
	r = &roteiro{erros: []error{&textproto.Error{Code: 550, Msg: "no such user"}}}
	m = New(Options{From: "a@b.com", Transport: r, Tasks: tk, Backoff: espera, Logger: silencio()})
	if err := m.Send(nil, msg); err != nil {
		t.Fatal(err)
	}
	if got := final(t, tk); got.State != task.Failed || r.tentativas() != 1 {
		t.Fatalf("5xx é permanente: %+v, %d tentativas", got, r.tentativas())
	}

	tk = runner(t)
	sempre := &textproto.Error{Code: 421, Msg: "busy"}
	r = &roteiro{erros: []error{sempre, sempre, sempre, sempre, sempre}}
	m = New(Options{From: "a@b.com", Transport: r, Tasks: tk, Backoff: espera, Logger: silencio()})
	if err := m.Send(nil, msg); err != nil {
		t.Fatal(err)
	}
	if got := final(t, tk); got.State != task.Failed || r.tentativas() != 4 {
		t.Fatalf("as esperas acabam: %+v, %d tentativas", got, r.tentativas())
	}
}

func TestTemporario(t *testing.T) {
	for err, want := range map[error]bool{
		&textproto.Error{Code: 450}: true,
		&textproto.Error{Code: 554}: false,
		context.DeadlineExceeded:    true,
		errors.New("x509: unknown"): false,
	} {
		if temporary(err) != want {
			t.Errorf("temporary(%v) = %v", err, !want)
		}
	}
}

func silencio() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }
