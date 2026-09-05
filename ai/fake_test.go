package ai

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// fakeServer answers like an OpenAI-compatible endpoint. Each call pops the
// next scripted reply; replies can be plain text or tool calls.
type fakeServer struct {
	t       *testing.T
	mu      sync.Mutex
	replies []fakeReply
	reqs    []Request
	headers []http.Header
	srv     *httptest.Server
}

type fakeReply struct {
	text  string
	calls []ToolCall
	// status != 0 returns an error body.
	status int
}

func newFake(t *testing.T, replies ...fakeReply) *fakeServer {
	f := &fakeServer{t: t, replies: replies}
	f.srv = httptest.NewServer(http.HandlerFunc(f.handle))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeServer) client() *Client {
	return &Client{BaseURL: f.srv.URL + "/v1", APIKey: "sk-test", Model: "fake-1"}
}

func (f *fakeServer) handle(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/v1/chat/completions" || r.Method != "POST" {
		http.Error(w, "not found", 404)
		return
	}
	var req Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	f.mu.Lock()
	f.reqs = append(f.reqs, req)
	f.headers = append(f.headers, r.Header.Clone())
	var rep fakeReply
	if len(f.replies) > 0 {
		rep, f.replies = f.replies[0], f.replies[1:]
	} else {
		rep = fakeReply{text: "sem script"}
	}
	f.mu.Unlock()
	if rep.status != 0 {
		w.WriteHeader(rep.status)
		fmt.Fprintf(w, `{"error":{"message":"scripted failure","type":"server_error","code":"boom"}}`)
		return
	}
	finish := "stop"
	if len(rep.calls) > 0 {
		finish = "tool_calls"
	}
	if !req.Stream {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Response{ID: "chatcmpl-1", Model: "fake-1",
			Choices: []Choice{{Message: Message{Role: "assistant", Content: rep.text, ToolCalls: rep.calls}, FinishReason: finish}},
			Usage:   Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15}})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	fl := w.(http.Flusher)
	send := func(v any) {
		b, _ := json.Marshal(v)
		fmt.Fprintf(w, "data: %s\n\n", b)
		fl.Flush()
	}
	// Text in 3-byte pieces to exercise reassembly.
	for i := 0; i < len(rep.text); i += 3 {
		end := min(i+3, len(rep.text))
		send(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": rep.text[i:end]}}}})
	}
	for idx, c := range rep.calls {
		// Name first, then arguments split in two chunks.
		args := c.Function.Arguments
		half := len(args) / 2
		send(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"tool_calls": []any{
			map[string]any{"index": idx, "id": c.ID, "type": "function", "function": map[string]any{"name": c.Function.Name, "arguments": args[:half]}}}}}}})
		send(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"tool_calls": []any{
			map[string]any{"index": idx, "function": map[string]any{"arguments": args[half:]}}}}}}})
	}
	send(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": finish}}})
	send(map[string]any{"choices": []any{}, "usage": Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15}})
	fmt.Fprint(w, "data: [DONE]\n\n")
	fl.Flush()
}

func (f *fakeServer) lastReq() Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.reqs[len(f.reqs)-1]
}

func (f *fakeServer) reqCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.reqs)
}

func call(id, name, args string) ToolCall {
	return ToolCall{ID: id, Type: "function", Function: FunctionCall{Name: name, Arguments: args}}
}

var _ = strings.TrimSpace
