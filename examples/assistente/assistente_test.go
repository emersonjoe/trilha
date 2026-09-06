package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/ai"
	"github.com/emersonjoe/trilha/examples/assistente/internal/ferramentas"
)

// fakeModel is a scripted OpenAI-compatible server: on the first call it asks
// for the "calcular" tool, on the second it answers with the tool output.
func fakeModel(t *testing.T) *httptest.Server {
	calls := 0
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ai.Request
		_ = json.NewDecoder(r.Body).Decode(&req)
		calls++
		w.Header().Set("Content-Type", "text/event-stream")
		chunk := func(delta string) {
			fmt.Fprintf(w, "data: %s\n\n", delta)
		}
		if calls == 1 {
			chunk(`{"choices":[{"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"c1","type":"function","function":{"name":"calcular","arguments":"{\"expressao\":"}}]}}]}`)
			chunk(`{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"(2+3)*4\"}"}}]}}]}`)
			chunk(`{"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`)
		} else {
			last := req.Messages[len(req.Messages)-1]
			for _, piece := range []string{"O resultado ", "é " + last.Content + "."} {
				b, _ := json.Marshal(piece)
				chunk(`{"choices":[{"delta":{"content":` + string(b) + `}}]}`)
			}
			chunk(`{"choices":[{"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`)
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
}

func newClient(t *testing.T) *trilha.TestClient {
	t.Helper()
	t.Setenv("TRILHA_ENV", "dev")
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	fm := fakeModel(t)
	t.Cleanup(fm.Close)
	ferramentas.Client = &ai.Client{BaseURL: fm.URL, Model: "fake"}
	ferramentas.Reset()
	ferramentas.Now = func() time.Time { return time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC) }
	return trilha.NewTestClient(t, newApp())
}

func TestPageRenders(t *testing.T) {
	res := newClient(t).Get("/").WantStatus(200).WantContains(`id="mensagens"`, "calcular")
	if csp := res.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "script-src 'self'") {
		t.Fatal(csp)
	}
}

func TestChatStreams(t *testing.T) {
	c := newClient(t)
	res := c.PostJSON("/api/chat", map[string]string{"message": "quanto é (2+3)*4?"}).WantStatus(200)
	if !strings.HasPrefix(res.Header().Get("Content-Type"), "text/event-stream") {
		t.Fatal(res.Header())
	}
	res.WantContains(
		"event: tool_call\ndata: {\"agent\":\"Assistente\",\"arguments\":\"{\\\"expressao\\\":\\\"(2+3)*4\\\"}\"",
		"event: tool_result\ndata: {\"agent\":\"Assistente\",\"arguments\":\"{\\\"expressao\\\":\\\"(2+3)*4\\\"}\",\"output\":\"20\"",
		"event: text\ndata: O resultado \n\n",
		"event: text\ndata: é 20.\n\n",
		"event: done\ndata: {\"agent\":\"Assistente\",\"history\":[",
		`"total_tokens":15`,
	)
	// Validation.
	c.PostJSON("/api/chat", map[string]string{"message": "  "}).WantStatus(422)
}

func TestMCPEndpoint(t *testing.T) {
	c := newClient(t)
	post := func(body, session string) *trilha.TestResponse {
		opts := []trilha.TestOption{trilha.WithBody("application/json", body)}
		if session != "" {
			opts = append(opts, trilha.WithHeader("Mcp-Session-Id", session))
		}
		return c.Request("POST", "/mcp", opts...)
	}
	res := post(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`, "")
	res.WantStatus(200).WantContains(`"trilha-assistente"`)
	sid := res.Header().Get("Mcp-Session-Id")
	if sid == "" {
		t.Fatal("sem sessão o cliente não consegue seguir")
	}
	post(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"salvar_nota","arguments":{"titulo":"compras","texto":"leite"}}}`, sid).
		WantStatus(200).WantContains("nota salva: compras")
	post(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"listar_notas"}}`, sid).
		WantContains("compras: leite")
	post(`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"hora_atual","arguments":{"fuso":"UTC"}}}`, sid).
		WantContains("05/09/2026 12:00 (UTC)")
	// Sem sessão não há endpoint.
	post(`{"jsonrpc":"2.0","id":5,"method":"tools/list"}`, "").WantStatus(404)
}

func TestCalcular(t *testing.T) {
	for in, want := range map[string]string{"1+2*3": "7", "(1+2)*3": "9", "-4/2": "-2", "10/4": "2.5"} {
		out, err := ferramentas.Tools[1].Func(context.Background(), json.RawMessage(`{"expressao":"`+in+`"}`))
		if err != nil || out != want {
			t.Fatal(in, out, err)
		}
	}
	for _, bad := range []string{"1/0", "2+", "abc"} {
		if _, err := ferramentas.Tools[1].Func(context.Background(), json.RawMessage(`{"expressao":"`+bad+`"}`)); err == nil {
			t.Fatal("expected error for", bad)
		}
	}
}
