package mcp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/ai"
)

// Server exposes ai.Tools over MCP.
type Server struct {
	name, version string
	tools         []*ai.Tool
	byName        map[string]*ai.Tool
	mu            sync.Mutex
	sessions      map[string]time.Time
	// SessionTTL expires idle HTTP sessions (default 1h).
	SessionTTL time.Duration
}

// NewServer creates a server publishing the given tools.
func NewServer(name, version string, tools ...*ai.Tool) *Server {
	s := &Server{name: name, version: version, byName: map[string]*ai.Tool{}, sessions: map[string]time.Time{}, SessionTTL: time.Hour}
	for _, t := range tools {
		s.tools = append(s.tools, t)
		s.byName[t.Name] = t
	}
	return s
}

// handle processes one JSON-RPC message; nil means no reply (notification).
func (s *Server) handle(ctx context.Context, msg []byte) []byte {
	var req request
	if err := json.Unmarshal(msg, &req); err != nil {
		return mustJSON(response{JSONRPC: "2.0", Error: &rpcError{Code: codeParse, Message: "parse error"}})
	}
	if req.JSONRPC != "2.0" || req.Method == "" {
		return mustJSON(response{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: codeInvalidRequest, Message: "invalid request"}})
	}
	if len(req.ID) == 0 {
		return nil
	}
	reply := func(result any) []byte {
		b, err := json.Marshal(result)
		if err != nil {
			return mustJSON(response{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: codeInternal, Message: err.Error()}})
		}
		return mustJSON(response{JSONRPC: "2.0", ID: req.ID, Result: b})
	}
	fail := func(code int, msg string) []byte {
		return mustJSON(response{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: code, Message: msg}})
	}
	switch req.Method {
	case "initialize":
		return reply(initializeResult{ProtocolVersion: ProtocolVersion, Capabilities: map[string]any{"tools": map[string]any{"listChanged": false}}, ServerInfo: info{s.name, s.version}})
	case "ping":
		return reply(map[string]any{})
	case "tools/list":
		infos := make([]ToolInfo, 0, len(s.tools))
		for _, t := range s.tools {
			schema := t.Parameters
			if len(schema) == 0 {
				schema = json.RawMessage(`{"type":"object","properties":{}}`)
			}
			infos = append(infos, ToolInfo{Name: t.Name, Description: t.Description, InputSchema: schema})
		}
		return reply(map[string]any{"tools": infos})
	case "tools/call":
		var p struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return fail(codeInvalidParams, "invalid params")
		}
		t, ok := s.byName[p.Name]
		if !ok {
			return fail(codeInvalidParams, "unknown tool: "+p.Name)
		}
		if len(p.Arguments) == 0 {
			p.Arguments = json.RawMessage("{}")
		}
		out, err := safeCall(ctx, t, p.Arguments)
		if err != nil {
			return reply(CallResult{Content: []Content{{Type: "text", Text: err.Error()}}, IsError: true})
		}
		return reply(CallResult{Content: []Content{{Type: "text", Text: out}}})
	default:
		return fail(codeMethodNotFound, "method not found: "+req.Method)
	}
}

func safeCall(ctx context.Context, t *ai.Tool, args json.RawMessage) (out string, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("tool %s panicked: %v", t.Name, r)
		}
	}()
	if t.Func == nil {
		return "", errors.New("tool has no implementation")
	}
	return t.Func(ctx, args)
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

// ServeStdio runs the server over a reader/writer pair until EOF or ctx end.
// Typical use: s.ServeStdio(ctx, os.Stdin, os.Stdout).
func (s *Server) ServeStdio(ctx context.Context, r io.Reader, w io.Writer) error {
	t := Pipe(r, w, nil)
	for {
		msg, err := t.Recv(ctx)
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		}
		if out := s.handle(ctx, msg); out != nil {
			if err := t.Send(ctx, out); err != nil {
				return err
			}
		}
	}
}

// ServeHTTP handles one Streamable HTTP request from a Trilha route:
//
//	func POST(c *trilha.Ctx) error { return server.ServeHTTP(c) }
func (s *Server) ServeHTTP(c *trilha.Ctx) error {
	return s.serve(c.Context(), c.Writer(), c.Request())
}

// Handler adapts the server to net/http for use outside Trilha.
func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = s.serve(r.Context(), w, r)
	})
}

func (s *Server) serve(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return nil
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
	if err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return nil
	}
	var probe request
	_ = json.Unmarshal(body, &probe)
	sid := r.Header.Get("Mcp-Session-Id")
	switch {
	case probe.Method == "initialize":
		w.Header().Set("Mcp-Session-Id", s.newSession())
	case sid == "" || !s.validSession(sid):
		http.Error(w, "missing or expired Mcp-Session-Id; send initialize first", http.StatusNotFound)
		return nil
	}
	out := s.handle(ctx, body)
	if out == nil {
		w.WriteHeader(http.StatusAccepted)
		return nil
	}
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(out)
	return err
}

func (s *Server) newSession() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	id := hex.EncodeToString(b[:])
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for k, t := range s.sessions {
		if now.Sub(t) > s.SessionTTL {
			delete(s.sessions, k)
		}
	}
	s.sessions[id] = now
	return id
}

func (s *Server) validSession(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.sessions[id]
	if !ok || time.Since(t) > s.SessionTTL {
		delete(s.sessions, id)
		return false
	}
	s.sessions[id] = time.Now()
	return true
}
