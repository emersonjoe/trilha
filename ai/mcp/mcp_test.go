package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha/ai"
)

func testServer() *Server {
	soma := ai.NewTool("soma", "Soma dois números",
		ai.Schema(`{"type":"object","properties":{"a":{"type":"number"},"b":{"type":"number"}},"required":["a","b"]}`),
		ai.Typed(func(ctx context.Context, in struct{ A, B float64 }) (string, error) {
			return strconv.FormatFloat(in.A+in.B, 'f', -1, 64), nil
		}))
	falha := ai.NewTool("falha", "Sempre falha", nil, func(context.Context, json.RawMessage) (string, error) {
		return "", errors.New("deu ruim")
	})
	panica := ai.NewTool("panica", "Entra em pânico", nil, func(context.Context, json.RawMessage) (string, error) { panic("x") })
	return NewServer("teste", "1.0", soma, falha, panica)
}

// newFakeModel scripts a model that first calls "soma" with args, then
// answers with the tool output followed by "!".
func newFakeModel(t *testing.T, args string) *ai.Client {
	calls := 0
	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ai.Request
		_ = json.NewDecoder(r.Body).Decode(&req)
		calls++
		w.Header().Set("Content-Type", "application/json")
		if calls == 1 {
			_ = json.NewEncoder(w).Encode(ai.Response{Choices: []ai.Choice{{Message: ai.Message{Role: "assistant", ToolCalls: []ai.ToolCall{{ID: "1", Type: "function", Function: ai.FunctionCall{Name: "soma", Arguments: args}}}}, FinishReason: "tool_calls"}}})
			return
		}
		last := req.Messages[len(req.Messages)-1]
		_ = json.NewEncoder(w).Encode(ai.Response{Choices: []ai.Choice{{Message: ai.Message{Role: "assistant", Content: last.Content + "!"}, FinishReason: "stop"}}})
	}))
	t.Cleanup(hs.Close)
	return &ai.Client{BaseURL: hs.URL, Model: "fake"}
}

func exercise(t *testing.T, c *Client) {
	t.Helper()
	ctx := context.Background()
	if c.Server.Name != "teste" || c.Server.ProtocolVersion != ProtocolVersion {
		t.Fatalf("%+v", c.Server)
	}
	tools, err := c.Tools(ctx)
	if err != nil || len(tools) != 3 || tools[0].Name != "soma" || !strings.Contains(string(tools[0].Parameters), `"required"`) {
		t.Fatal(err, tools)
	}
	out, err := tools[0].Func(ctx, json.RawMessage(`{"a":2,"b":3}`))
	if err != nil || out != "5" {
		t.Fatalf("%q %v", out, err)
	}
	if _, err := tools[1].Func(ctx, nil); err == nil || err.Error() != "deu ruim" {
		t.Fatal("isError must become an error:", err)
	}
	if _, err := tools[2].Func(ctx, nil); err == nil || !strings.Contains(err.Error(), "panicked") {
		t.Fatal(err)
	}
	if _, err := c.CallTool(ctx, "nao_existe", nil); err == nil || !strings.Contains(err.Error(), "unknown tool") {
		t.Fatal(err)
	}
	// An agent uses the MCP tool end to end against the fake model.
	agent := &ai.Agent{Name: "calc", Tools: tools}
	res, err := ai.Run(ctx, newFakeModel(t, `{"a":40,"b":2}`), agent, "40+2?")
	if err != nil || res.Steps[0].Output != "42" || res.Output != "42!" {
		t.Fatalf("%v %+v", err, res)
	}
}

func TestStdioTransport(t *testing.T) {
	srv := testServer()
	cr, sw := io.Pipe() // server writes → client reads
	sr, cw := io.Pipe() // client writes → server reads
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = srv.ServeStdio(ctx, sr, sw) }()
	c, err := Dial(ctx, func(context.Context) (Transport, error) {
		return Pipe(cr, cw, func() error { return cw.Close() }), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	exercise(t, c)
}

func TestHTTPTransport(t *testing.T) {
	srv := testServer()
	hs := httptest.NewServer(srv.Handler())
	defer hs.Close()
	ctx := context.Background()
	c, err := Dial(ctx, HTTP(hs.URL, map[string]string{"Authorization": "Bearer x"}))
	if err != nil {
		t.Fatal(err)
	}
	exercise(t, c)
	// Without a session the server refuses non-initialize calls.
	naked, _ := HTTP(hs.URL, nil)(ctx)
	body, _ := json.Marshal(request{JSONRPC: "2.0", ID: json.RawMessage("1"), Method: "tools/list"})
	if err := naked.Send(ctx, body); err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatal("expected 404 without session:", err)
	}
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != 405 || rec.Header().Get("Allow") != "POST" {
		t.Fatal(rec.Code)
	}
}

func TestServerHandleErrors(t *testing.T) {
	s := testServer()
	ctx := context.Background()
	if out := s.handle(ctx, []byte(`{bad`)); !strings.Contains(string(out), "-32700") {
		t.Fatal(string(out))
	}
	if out := s.handle(ctx, []byte(`{"jsonrpc":"2.0","id":1,"method":"nope"}`)); !strings.Contains(string(out), "-32601") {
		t.Fatal(string(out))
	}
	if out := s.handle(ctx, []byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)); out != nil {
		t.Fatal("notifications have no reply")
	}
	var r CallResult
	_ = json.Unmarshal([]byte(`{"content":[{"type":"text","text":"a"},{"type":"image","data":"x"},{"type":"text","text":"b"}]}`), &r)
	if r.Text() != "a\nb" {
		t.Fatal(r.Text())
	}
}
