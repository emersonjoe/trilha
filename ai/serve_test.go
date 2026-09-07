package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha"
)

// serveApp mounts Serve at /api/chat with the options given.
func serveApp(t *testing.T, cli *Client, agent *Agent, o ServeOpts) *trilha.TestClient {
	t.Helper()
	a := trilha.New(trilha.Config{})
	a.Register(trilha.Route{
		Pattern: "/api/chat",
		Methods: map[string]trilha.HandlerFunc{
			"POST": func(c *trilha.Ctx) error { return o.Serve(c, cli, agent) },
		},
	})
	return trilha.NewTestClient(t, a)
}

// events splits an SSE body into (name, data) pairs, in order.
func events(t *testing.T, body string) [][2]string {
	t.Helper()
	var out [][2]string
	for _, block := range strings.Split(strings.TrimSpace(body), "\n\n") {
		name, data := "", ""
		for _, line := range strings.Split(block, "\n") {
			switch {
			case strings.HasPrefix(line, "event: "):
				name = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				if data != "" {
					data += "\n"
				}
				data += strings.TrimPrefix(line, "data: ")
			}
		}
		if name != "" || data != "" {
			out = append(out, [2]string{name, data})
		}
	}
	return out
}

func names(evs [][2]string) []string {
	out := make([]string, len(evs))
	for i, e := range evs {
		out[i] = e[0]
	}
	return out
}

// TestServeStreamsEvents is the contract the browser reads: the tool call and
// its result show up before the text, and the last event carries the history.
func TestServeStreamsEvents(t *testing.T) {
	tool := NewTool("hora", "Que horas são", Schema(`{"type":"object"}`),
		Typed(func(ctx context.Context, in struct{}) (string, error) { return "14:00", nil }))
	f := newFake(t,
		fakeReply{calls: []ToolCall{call("c1", "hora", "{}")}},
		fakeReply{text: "São 14:00."},
	)
	agent := &Agent{Name: "relogio", Instructions: "Responda curto.", Tools: []*Tool{tool}}
	cl := serveApp(t, f.client(), agent, ServeOpts{})

	res := cl.Request("POST", "/api/chat",
		trilha.WithHeader("Accept", "text/event-stream"),
		trilha.WithJSON(map[string]any{"message": "que horas são?"}),
	).WantStatus(http.StatusOK)

	evs := events(t, res.Body.String())
	got := strings.Join(names(evs), ",")
	for _, want := range []string{"tool_call", "tool_result", "text", "done"} {
		if !strings.Contains(got, want) {
			t.Fatalf("event %q missing in %q\n%s", want, got, res.Body.String())
		}
	}
	if strings.Index(got, "tool_call") > strings.Index(got, "text") {
		t.Fatalf("the tool came after the text: %q", got)
	}
	if got[len(got)-len("done"):] != "done" {
		t.Fatalf("done is not the last event: %q", got)
	}

	var last struct {
		Agent   string    `json:"agent"`
		Output  string    `json:"output"`
		History []Message `json:"history"`
	}
	if err := json.Unmarshal([]byte(evs[len(evs)-1][1]), &last); err != nil {
		t.Fatal(err)
	}
	if last.Agent != "relogio" || last.Output != "São 14:00." {
		t.Fatalf("done carries %+v", last)
	}
	if len(last.History) < 3 {
		t.Fatalf("history came back short: %+v", last.History)
	}
}

// TestServeWithoutStreamAnswersAtOnce is the request that arrives when the
// script is not there.
func TestServeWithoutStreamAnswersAtOnce(t *testing.T) {
	f := newFake(t, fakeReply{text: "Pronto."})
	cl := serveApp(t, f.client(), &Agent{Name: "a", Instructions: "x"}, ServeOpts{})

	res := cl.Request("POST", "/api/chat", trilha.WithJSON(map[string]any{"message": "oi"})).
		WantStatus(http.StatusOK)
	if strings.Contains(res.Body.String(), "event: ") {
		t.Fatalf("answered with a stream nobody asked for: %q", res.Body.String())
	}
	var body struct {
		Output  string    `json:"output"`
		History []Message `json:"history"`
	}
	res.JSON(&body)
	if body.Output != "Pronto." || len(body.History) == 0 {
		t.Fatalf("body is %+v", body)
	}
}

// TestServeReadsAForm is the no-JavaScript submit: a form field, and a page
// hook that renders the answer instead of returning JSON.
func TestServeReadsAForm(t *testing.T) {
	f := newFake(t, fakeReply{text: "Resposta inteira."})
	seen := ""
	o := ServeOpts{Page: func(c *trilha.Ctx, r *Result) error {
		seen = r.Output
		return c.Text(http.StatusOK, "<p>"+r.Output+"</p>")
	}}
	cl := serveApp(t, f.client(), &Agent{Name: "a", Instructions: "x"}, o)

	res := cl.Request("POST", "/api/chat", trilha.WithForm(map[string][]string{"message": {"oi"}})).
		WantStatus(http.StatusOK)
	if seen != "Resposta inteira." || !strings.Contains(res.Body.String(), "<p>Resposta inteira.</p>") {
		t.Fatalf("page hook got %q, body %q", seen, res.Body.String())
	}
}

// TestServeRefusesAnEmptyMessage keeps a blank submit from costing a call.
func TestServeRefusesAnEmptyMessage(t *testing.T) {
	f := newFake(t, fakeReply{text: "não deveria ser chamado"})
	cl := serveApp(t, f.client(), &Agent{Name: "a", Instructions: "x"}, ServeOpts{})
	cl.Request("POST", "/api/chat", trilha.WithJSON(map[string]any{"message": "   "})).
		WantStatus(http.StatusUnprocessableEntity)
	if f.reqCount() != 0 {
		t.Fatalf("the model was called %d times", f.reqCount())
	}
}

// TestServeCapsHistory: the history comes from the browser, so the ceiling is
// the server's, not the client's.
func TestServeCapsHistory(t *testing.T) {
	f := newFake(t, fakeReply{text: "ok"})
	cl := serveApp(t, f.client(), &Agent{Name: "a", Instructions: "x"}, ServeOpts{MaxHistory: 2})

	history := []Message{User("um"), Assistant("dois"), User("três"), Assistant("quatro"), User("cinco"), Assistant("seis")}
	cl.Request("POST", "/api/chat", trilha.WithJSON(map[string]any{"message": "sete", "history": history})).
		WantStatus(http.StatusOK)

	msgs := f.lastReq().Messages
	// system + the last two of the history + the new message.
	if len(msgs) != 4 {
		t.Fatalf("sent %d messages: %+v", len(msgs), msgs)
	}
	if msgs[1].Content != "cinco" || msgs[2].Content != "seis" {
		t.Fatalf("kept the wrong end of the history: %+v", msgs)
	}
}

// TestServeReadsAMessagesList accepts the other shape: one list where the last
// user turn is the message.
func TestServeReadsAMessagesList(t *testing.T) {
	f := newFake(t, fakeReply{text: "ok"})
	cl := serveApp(t, f.client(), &Agent{Name: "a", Instructions: "x"}, ServeOpts{})

	msgs := []Message{User("primeira"), Assistant("resposta"), User("segunda")}
	cl.Request("POST", "/api/chat", trilha.WithJSON(map[string]any{"messages": msgs})).
		WantStatus(http.StatusOK)

	sent := f.lastReq().Messages
	if sent[len(sent)-1].Content != "segunda" {
		t.Fatalf("last message is %q", sent[len(sent)-1].Content)
	}
	if len(sent) != 4 {
		t.Fatalf("sent %d messages: %+v", len(sent), sent)
	}
}

// TestServeTurnsAModelFailureIntoAnEvent: the status line left with the first
// byte of the stream, so the failure has to travel as an event and the
// connection has to close on its own.
func TestServeTurnsAModelFailureIntoAnEvent(t *testing.T) {
	f := newFake(t, fakeReply{status: http.StatusInternalServerError})
	cl := serveApp(t, f.client(), &Agent{Name: "a", Instructions: "x"}, ServeOpts{})

	res := cl.Request("POST", "/api/chat",
		trilha.WithHeader("Accept", "text/event-stream"),
		trilha.WithJSON(map[string]any{"message": "oi"}),
	).WantStatus(http.StatusOK)

	evs := events(t, res.Body.String())
	if len(evs) == 0 || evs[len(evs)-1][0] != "error" {
		t.Fatalf("no error event: %q", res.Body.String())
	}
	var e struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(evs[len(evs)-1][1]), &e); err != nil {
		t.Fatal(err)
	}
	if e.Message == "" {
		t.Fatalf("the error event has no message: %q", res.Body.String())
	}
}

// TestServeWithoutStreamReportsAFailureAsAProblem: nothing was written yet, so
// the status code is still available and the app answers like any other route.
func TestServeWithoutStreamReportsAFailureAsAProblem(t *testing.T) {
	f := newFake(t, fakeReply{status: http.StatusInternalServerError})
	cl := serveApp(t, f.client(), &Agent{Name: "a", Instructions: "x"}, ServeOpts{})
	res := cl.Request("POST", "/api/chat", trilha.WithJSON(map[string]any{"message": "oi"}))
	if res.Code < 400 {
		t.Fatalf("answered %d: %q", res.Code, res.Body.String())
	}
}
