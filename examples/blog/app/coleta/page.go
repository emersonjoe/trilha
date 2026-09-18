// Package coleta is the screen filled where the network comes and goes: the
// page is kept by the service worker (`var Offline = true` in offline.go) and
// the form waits in the browser's outbox until the network comes back.
//
// The interesting half is on the server. The resend carries the key the form
// was born with, trilha.Idempotent recognises it, and the second arrival
// answers what the first one answered instead of collecting the same note
// twice.
package coleta

import (
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// janela é quanto tempo uma chave é lembrada: o outbox de um aparelho que
// passou um dia sem rede ainda encontra a chave dele do lado de cá.
const janela = 48 * time.Hour

type anotacao struct {
	chave   string
	texto   string
	carimbo time.Time
	quando  time.Time
}

var (
	mu    sync.Mutex
	notas = map[string]anotacao{}
)

// Page desenha o formulário, o que está esperando na fila e o que já chegou.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Coleta")
	linhas := []h.Node{}
	for _, n := range lista() {
		linhas = append(linhas, h.Li(h.Text(n.texto)))
	}
	return h.Div(h.Class("ui-stack"),
		ui.PageHeader("Coleta"),
		ui.Muted(h.Text("Esta tela abre sem rede. O que você enviar espera na fila e sai quando a rede voltar.")),
		ui.Outbox(c),
		formulario(c),
		h.Ul(linhas...),
		ui.OfflineScript(c),
	), nil
}

// formulario é o que espera no outbox quando não há rede: trilha.OfflineForm
// dá a ele a chave da submissão e o lugar do carimbo do cliente.
func formulario(c *trilha.Ctx) h.Node {
	return h.Form(h.Method("post"), h.Action("/coleta"),
		trilha.CSRFInput(c),
		trilha.OfflineForm(c),
		ui.Field("texto", "Anotação", ui.Input(h.ID("texto"), h.Name("texto"), h.Required())),
		ui.Submit(h.Text("Enviar")),
	)
}

// POST guarda a anotação. O reenvio chega com a mesma chave: é a mesma
// coleta, e a resposta é a mesma.
func POST(c *trilha.Ctx) error {
	texto := c.Form("texto")
	if texto == "" {
		return trilha.Errorf(http.StatusBadRequest, "escreva alguma coisa")
	}
	repetida, err := trilha.Idempotent(c, janela)
	if err != nil {
		return err
	}
	if !repetida {
		guardar(c, texto)
	}
	return c.Redirect("/coleta")
}

// guardar grava a anotação sob a chave da submissão. A mesma chave sobrescreve
// o que estava lá — último-escreve-ganha, com o carimbo do cliente registrado
// junto, que é o que permite explicar depois qual escrita ganhou.
func guardar(c *trilha.Ctx, texto string) {
	chave := trilha.IdempotencyKey(c)
	carimbo, _ := trilha.QueuedAt(c)
	mu.Lock()
	defer mu.Unlock()
	notas[chave] = anotacao{chave: chave, texto: texto, carimbo: carimbo, quando: time.Now()}
}

func lista() []anotacao {
	mu.Lock()
	defer mu.Unlock()
	out := make([]anotacao, 0, len(notas))
	for _, n := range notas {
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].quando.After(out[j].quando) })
	return out
}
