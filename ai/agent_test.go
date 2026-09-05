package ai

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func climaTool(calls *atomic.Int32) *Tool {
	return NewTool("clima", "Temperatura de uma cidade", Schema(`{"type":"object","properties":{"cidade":{"type":"string"}},"required":["cidade"]}`),
		Typed(func(ctx context.Context, in struct{ Cidade string }) (string, error) {
			calls.Add(1)
			time.Sleep(20 * time.Millisecond)
			if in.Cidade == "Atlântida" {
				return "", errors.New("cidade desconhecida")
			}
			return "27°C em " + in.Cidade, nil
		}))
}

func TestRunWithToolsInParallel(t *testing.T) {
	f := newFake(t,
		fakeReply{calls: []ToolCall{call("c1", "clima", `{"cidade":"Recife"}`), call("c2", "clima", `{"cidade":"Campinas"}`), call("c3", "nao_existe", `{}`), call("c4", "clima", `{"cidade":"Atlântida"}`)}},
		fakeReply{text: "Recife 27°C, Campinas 27°C."},
	)
	var n atomic.Int32
	agent := &Agent{Name: "assistente", Instructions: "Responda em pt-BR.", Tools: []*Tool{climaTool(&n)}}
	start := time.Now()
	res, err := Run(context.Background(), f.client(), agent, "Clima?")
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 55*time.Millisecond {
		t.Fatal("tools should run in parallel")
	}
	if res.Output != "Recife 27°C, Campinas 27°C." || res.Turns != 2 || n.Load() != 3 || res.Usage.TotalTokens != 30 {
		t.Fatalf("%+v n=%d", res, n.Load())
	}
	if len(res.Steps) != 4 || res.Steps[0].Output != "27°C em Recife" || !strings.Contains(res.Steps[2].Output, "unknown tool") || res.Steps[3].Err == nil {
		t.Fatalf("%+v", res.Steps)
	}
	// Second request carried the tool results in order.
	req := f.lastReq()
	if req.Messages[0].Role != "system" || req.Messages[1].Content != "Clima?" || req.Messages[2].Role != "assistant" || len(req.Messages) != 7 {
		t.Fatalf("%+v", req.Messages)
	}
	if req.Messages[3].Role != "tool" || req.Messages[3].ToolCallID != "c1" || req.Messages[6].Content != "error: cidade desconhecida" {
		t.Fatalf("%+v", req.Messages[3:])
	}
	if len(req.Tools) != 1 || req.Tools[0].Function.Name != "clima" {
		t.Fatal(req.Tools)
	}
	if !json.Valid(req.Tools[0].Function.Parameters) {
		t.Fatal("schema")
	}
}

func TestRunMaxTurns(t *testing.T) {
	f := newFake(t, fakeReply{calls: []ToolCall{call("1", "clima", `{}`)}}, fakeReply{calls: []ToolCall{call("2", "clima", `{}`)}}, fakeReply{calls: []ToolCall{call("3", "clima", `{}`)}})
	var n atomic.Int32
	agent := &Agent{Name: "a", Tools: []*Tool{climaTool(&n)}, MaxTurns: 2}
	_, err := Run(context.Background(), f.client(), agent, "x")
	if !errors.Is(err, ErrMaxTurns) || f.reqCount() != 2 {
		t.Fatal(err, f.reqCount())
	}
}

func TestRunStreamEvents(t *testing.T) {
	f := newFake(t, fakeReply{text: "Verificando", calls: []ToolCall{call("c1", "clima", `{"cidade":"Recife"}`)}}, fakeReply{text: "Faz 27°C."})
	var n atomic.Int32
	agent := &Agent{Name: "a", Tools: []*Tool{climaTool(&n)}}
	var types []string
	var text strings.Builder
	res, err := RunStream(context.Background(), f.client(), agent, "Clima?", func(e Event) {
		types = append(types, e.Type)
		if e.Type == "text" {
			text.WriteString(e.Text)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Output != "Faz 27°C." || text.String() != "VerificandoFaz 27°C." {
		t.Fatalf("%q %q", res.Output, text.String())
	}
	got := strings.Join(types, " ")
	if !strings.HasPrefix(got, "text ") || !strings.Contains(got, "tool_call tool_result text") || !strings.HasSuffix(got, " done") {
		t.Fatal(got)
	}
}

func TestHandoff(t *testing.T) {
	f := newFake(t,
		fakeReply{calls: []ToolCall{call("h1", "transfer_to_financeiro", `{"reason":"boleto"}`)}},
		fakeReply{text: "Segunda via enviada."},
	)
	fin := &Agent{Name: "Financeiro", Instructions: "Você cuida de cobranças."}
	triagem := &Agent{Name: "Triagem", Instructions: "Encaminhe.", Handoffs: []*Agent{fin}}
	var handoffs []string
	res, err := RunStream(context.Background(), f.client(), triagem, "Quero a 2ª via", func(e Event) {
		if e.Type == "handoff" {
			handoffs = append(handoffs, e.Agent)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Agent != fin || res.Output != "Segunda via enviada." || len(handoffs) != 1 || handoffs[0] != "Financeiro" {
		t.Fatalf("%+v %v", res, handoffs)
	}
	first := f.reqs[0]
	if len(first.Tools) != 1 || first.Tools[0].Function.Name != "transfer_to_financeiro" || !strings.Contains(first.Tools[0].Function.Description, "Financeiro") {
		t.Fatalf("%+v", first.Tools)
	}
	second := f.reqs[1]
	if second.Messages[0].Content != "Você cuida de cobranças." || second.Messages[1].Content != "Quero a 2ª via" || second.Tools != nil {
		t.Fatalf("%+v", second.Messages)
	}
	if res.Steps[0].HandoffTo != "Financeiro" {
		t.Fatal(res.Steps)
	}
}

func TestAgentAsTool(t *testing.T) {
	f := newFake(t,
		fakeReply{calls: []ToolCall{call("t1", "tradutor", `{"input":"hello"}`)}}, // orchestrator asks the sub-agent
		fakeReply{text: "olá"},               // sub-agent answers
		fakeReply{text: "A tradução é: olá"}, // orchestrator finishes
	)
	cli := f.client()
	tradutor := &Agent{Name: "Tradutor", Instructions: "Traduza para pt-BR."}
	chefe := &Agent{Name: "Chefe", Tools: []*Tool{tradutor.AsTool(cli, "")}}
	res, err := Run(context.Background(), cli, chefe, "Traduza hello")
	if err != nil || res.Output != "A tradução é: olá" || res.Steps[0].Output != "olá" {
		t.Fatalf("%v %+v", err, res)
	}
	if f.reqs[1].Messages[0].Content != "Traduza para pt-BR." || f.reqs[1].Messages[1].Content != "hello" {
		t.Fatalf("%+v", f.reqs[1].Messages)
	}
}

func TestParallelAndChain(t *testing.T) {
	f := newFake(t, fakeReply{text: "um"}, fakeReply{text: "dois"}, fakeReply{text: "três"})
	cli := f.client()
	a, b, c := &Agent{Name: "A"}, &Agent{Name: "B"}, &Agent{Name: "C"}
	rs, err := Parallel(context.Background(), cli, "x", a, b, c)
	if err != nil || len(rs) != 3 || rs[0].Agent != a || rs[2].Agent != c {
		t.Fatalf("%v %+v", err, rs)
	}
	f2 := newFake(t, fakeReply{text: "resumo"}, fakeReply{text: "RESUMO"})
	rs, err = Chain(context.Background(), f2.client(), "texto longo", a, b)
	if err != nil || rs[1].Output != "RESUMO" || f2.reqs[1].Messages[0].Content != "resumo" {
		t.Fatalf("%v %+v", err, rs)
	}
	f3 := newFake(t, fakeReply{status: 500}, fakeReply{text: "ok"})
	if _, err := Parallel(context.Background(), f3.client(), "x", a, b); err == nil {
		t.Fatal("first error must propagate")
	}
}

func TestHistoryIsReused(t *testing.T) {
	f := newFake(t, fakeReply{text: "Sou o Trilha."}, fakeReply{text: "Já disse: Trilha."})
	cli := f.client()
	agent := &Agent{Name: "a", Instructions: "Seja breve."}
	r1, _ := Run(context.Background(), cli, agent, "Quem é você?")
	r2, err := Run(context.Background(), cli, agent, "Repita.", r1.Messages...)
	if err != nil || r2.Output != "Já disse: Trilha." {
		t.Fatal(err, r2)
	}
	m := f.reqs[1].Messages
	if len(m) != 4 || m[0].Role != "system" || m[1].Content != "Quem é você?" || m[2].Content != "Sou o Trilha." || m[3].Content != "Repita." {
		t.Fatalf("%+v", m)
	}
}
