package ai

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestChat(t *testing.T) {
	f := newFake(t, fakeReply{text: "Olá!"})
	cli := f.client()
	cli.Headers = map[string]string{"X-Extra": "1"}
	temp := 0.2
	resp, err := cli.Chat(context.Background(), Request{Messages: []Message{System("pt"), User("oi")}, Temperature: &temp,
		ResponseFormat: &ResponseFormat{Type: "json_object"}, Extra: map[string]any{"top_k": 3}})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text() != "Olá!" || resp.Usage.TotalTokens != 15 {
		t.Fatalf("%+v", resp)
	}
	req := f.lastReq()
	if req.Model != "fake-1" || len(req.Messages) != 2 || req.Messages[0].Role != "system" || *req.Temperature != 0.2 || req.ResponseFormat.Type != "json_object" {
		t.Fatalf("%+v", req)
	}
	h := f.headers[0]
	if h.Get("Authorization") != "Bearer sk-test" || h.Get("X-Extra") != "1" || h.Get("Content-Type") != "application/json" {
		t.Fatal(h)
	}
}

func TestExtraIsMerged(t *testing.T) {
	b, _ := json.Marshal(Request{Messages: []Message{User("x")}, Extra: map[string]any{"top_k": 3}})
	if !strings.Contains(string(b), `"top_k":3`) || !strings.Contains(string(b), `"messages"`) {
		t.Fatal(string(b))
	}
}

func TestErrors(t *testing.T) {
	f := newFake(t, fakeReply{status: 429})
	_, err := f.client().Chat(context.Background(), Request{Messages: []Message{User("x")}})
	var e *Error
	if !errors.As(err, &e) || e.Status != 429 || e.Code != "boom" || e.Message != "scripted failure" {
		t.Fatalf("%v", err)
	}
	if strings.Contains(err.Error(), "sk-test") {
		t.Fatal("key leaked")
	}
}

func TestStreamTextAndToolCalls(t *testing.T) {
	f := newFake(t, fakeReply{text: "Vou verificar o clima.", calls: []ToolCall{
		call("c1", "clima", `{"cidade":"Recife"}`), call("c2", "clima", `{"cidade":"Campinas"}`)}})
	var text strings.Builder
	var calls []ToolCall
	var finish string
	var usage Usage
	err := f.client().Stream(context.Background(), Request{Messages: []Message{User("x")}}, func(d Delta) error {
		text.WriteString(d.Content)
		calls = append(calls, d.ToolCalls...)
		if d.Finish != "" {
			finish = d.Finish
		}
		if d.Usage != nil {
			usage = *d.Usage
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if text.String() != "Vou verificar o clima." || finish != "tool_calls" || usage.TotalTokens != 15 {
		t.Fatalf("text=%q finish=%q usage=%+v", text.String(), finish, usage)
	}
	if len(calls) != 2 || calls[0].ID != "c1" || calls[0].Function.Name != "clima" || calls[0].Function.Arguments != `{"cidade":"Recife"}` || calls[1].Function.Arguments != `{"cidade":"Campinas"}` {
		t.Fatalf("%+v", calls)
	}
	req := f.lastReq()
	if !req.Stream {
		t.Fatal("stream flag")
	}
}

func TestStreamStopsOnCallbackError(t *testing.T) {
	f := newFake(t, fakeReply{text: "abcdefghij"})
	n := 0
	err := f.client().Stream(context.Background(), Request{Messages: []Message{User("x")}}, func(d Delta) error {
		n++
		return errors.New("chega")
	})
	if err == nil || err.Error() != "chega" || n != 1 {
		t.Fatal(err, n)
	}
}

func TestStreamCancel(t *testing.T) {
	f := newFake(t, fakeReply{text: "abcdefghij"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := f.client().Stream(ctx, Request{Messages: []Message{User("x")}}, func(Delta) error { return nil })
	if err == nil {
		t.Fatal("expected cancellation error")
	}
}

func TestNewFromEnv(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "k")
	t.Setenv("OPENAI_BASE_URL", "http://localhost:11434/v1/")
	t.Setenv("TRILHA_AI_MODEL", "llama3")
	c := NewFromEnv()
	if c.APIKey != "k" || c.BaseURL != "http://localhost:11434/v1" || c.Model != "llama3" {
		t.Fatalf("%+v", c)
	}
	t.Setenv("OPENAI_BASE_URL", "")
	t.Setenv("TRILHA_AI_MODEL", "")
	if c := NewFromEnv(); c.BaseURL != DefaultBaseURL || c.Model != "gpt-4o-mini" {
		t.Fatalf("%+v", c)
	}
}
