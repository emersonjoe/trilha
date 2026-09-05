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

func newHandler(t *testing.T) http.Handler {
	t.Helper()
	t.Setenv("TRILHA_ENV", "dev")
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	fm := fakeModel(t)
	t.Cleanup(fm.Close)
	ferramentas.Client = &ai.Client{BaseURL: fm.URL, Model: "fake"}
	ferramentas.Reset()
	ferramentas.Now = func() time.Time { return time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC) }
	return newApp().Handler()
}

func TestPageRenders(t *testing.T) {
	h := newHandler(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	body := rec.Body.String()
	if rec.Code != 200 || !strings.Contains(body, `id="mensagens"`) || !strings.Contains(body, "calcular") {
		t.Fatal(rec.Code, body)
	}
	if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "script-src 'self'") {
		t.Fatal(csp)
	}
}

func TestChatStreams(t *testing.T) {
	h := newHandler(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/chat", strings.NewReader(`{"message":"quanto é (2+3)*4?"}`))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != 200 || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/event-stream") {
		t.Fatal(rec.Code, rec.Header())
	}
	for _, want := range []string{
		"event: tool_call\ndata: {\"agent\":\"Assistente\",\"arguments\":\"{\\\"expressao\\\":\\\"(2+3)*4\\\"}\"",
		"event: tool_result\ndata: {\"agent\":\"Assistente\",\"arguments\":\"{\\\"expressao\\\":\\\"(2+3)*4\\\"}\",\"output\":\"20\"",
		"event: text\ndata: O resultado \n\n",
		"event: text\ndata: é 20.\n\n",
		"event: done\ndata: {\"agent\":\"Assistente\",\"history\":[",
		`"total_tokens":15`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in:\n%s", want, body)
		}
	}
	// Validation.
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/api/chat", strings.NewReader(`{"message":"  "}`))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rec, req)
	if rec.Code != 422 {
		t.Fatal(rec.Code)
	}
}

func TestMCPEndpoint(t *testing.T) {
	h := newHandler(t)
	post := func(body, session string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/mcp", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if session != "" {
			req.Header.Set("Mcp-Session-Id", session)
		}
		h.ServeHTTP(rec, req)
		return rec
	}
	rec := post(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`, "")
	sid := rec.Header().Get("Mcp-Session-Id")
	if rec.Code != 200 || sid == "" || !strings.Contains(rec.Body.String(), `"trilha-assistente"`) {
		t.Fatal(rec.Code, rec.Body.String())
	}
	rec = post(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"salvar_nota","arguments":{"titulo":"compras","texto":"leite"}}}`, sid)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "nota salva: compras") {
		t.Fatal(rec.Body.String())
	}
	rec = post(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"listar_notas"}}`, sid)
	if !strings.Contains(rec.Body.String(), "compras: leite") {
		t.Fatal(rec.Body.String())
	}
	rec = post(`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"hora_atual","arguments":{"fuso":"UTC"}}}`, sid)
	if !strings.Contains(rec.Body.String(), "05/09/2026 12:00 (UTC)") {
		t.Fatal(rec.Body.String())
	}
	if rec := post(`{"jsonrpc":"2.0","id":5,"method":"tools/list"}`, ""); rec.Code != 404 {
		t.Fatal("session required:", rec.Code)
	}
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
