package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Transport moves JSON-RPC messages between client and server.
type Transport interface {
	// Send delivers one message.
	Send(ctx context.Context, msg []byte) error
	// Recv blocks for the next incoming message.
	Recv(ctx context.Context) ([]byte, error)
	Close() error
}

// Dialer opens a Transport; see Stdio and HTTP.
type Dialer func(ctx context.Context) (Transport, error)

// ---- stdio ----------------------------------------------------------------

type stdioTransport struct {
	w     io.Writer
	r     *bufio.Reader
	close func() error
	mu    sync.Mutex
}

// Pipe builds a newline-delimited JSON transport over any reader/writer pair
// (servers use os.Stdin/os.Stdout; tests use io.Pipe).
func Pipe(r io.Reader, w io.Writer, closer func() error) Transport {
	return &stdioTransport{w: w, r: bufio.NewReaderSize(r, 1<<20), close: closer}
}

// Stdio launches a command and speaks MCP over its stdin/stdout.
func Stdio(name string, args ...string) Dialer {
	return func(ctx context.Context) (Transport, error) {
		cmd := exec.CommandContext(ctx, name, args...)
		stdin, err := cmd.StdinPipe()
		if err != nil {
			return nil, err
		}
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return nil, err
		}
		if err := cmd.Start(); err != nil {
			return nil, fmt.Errorf("mcp: starting %s: %w", name, err)
		}
		return Pipe(stdout, stdin, func() error {
			_ = stdin.Close()
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return nil
		}), nil
	}
}

func (t *stdioTransport) Send(ctx context.Context, msg []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if bytes.ContainsRune(msg, '\n') {
		var buf bytes.Buffer
		if err := json.Compact(&buf, msg); err != nil {
			return err
		}
		msg = buf.Bytes()
	}
	_, err := t.w.Write(append(msg, '\n'))
	return err
}

func (t *stdioTransport) Recv(ctx context.Context) ([]byte, error) {
	type res struct {
		line []byte
		err  error
	}
	ch := make(chan res, 1)
	go func() {
		for {
			line, err := t.r.ReadBytes('\n')
			line = bytes.TrimSpace(line)
			if len(line) > 0 {
				ch <- res{line, nil}
				return
			}
			if err != nil {
				ch <- res{nil, err}
				return
			}
		}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-ch:
		return r.line, r.err
	}
}

func (t *stdioTransport) Close() error {
	if t.close != nil {
		return t.close()
	}
	return nil
}

// ---- Streamable HTTP ------------------------------------------------------

type httpTransport struct {
	url     string
	headers map[string]string
	client  *http.Client
	session string
	queue   chan []byte
	mu      sync.Mutex
}

// HTTP connects to a Streamable HTTP MCP endpoint (one POST per message;
// JSON or SSE responses are accepted). headers may carry Authorization.
func HTTP(url string, headers map[string]string) Dialer {
	return func(ctx context.Context) (Transport, error) {
		return &httpTransport{url: url, headers: headers, client: &http.Client{Timeout: 2 * time.Minute}, queue: make(chan []byte, 16)}, nil
	}
}

func (t *httpTransport) Send(ctx context.Context, msg []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.url, bytes.NewReader(msg))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}
	t.mu.Lock()
	if t.session != "" {
		req.Header.Set("Mcp-Session-Id", t.session)
	}
	t.mu.Unlock()
	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("mcp: %w", err)
	}
	defer resp.Body.Close()
	if sid := resp.Header.Get("Mcp-Session-Id"); sid != "" {
		t.mu.Lock()
		t.session = sid
		t.mu.Unlock()
	}
	if resp.StatusCode == http.StatusAccepted || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if resp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
		return fmt.Errorf("mcp: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	if strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		sc := bufio.NewScanner(resp.Body)
		sc.Buffer(make([]byte, 64<<10), 4<<20)
		for sc.Scan() {
			if line := sc.Text(); strings.HasPrefix(line, "data:") {
				if data := strings.TrimSpace(strings.TrimPrefix(line, "data:")); data != "" {
					t.queue <- []byte(data)
				}
			}
		}
		return sc.Err()
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(body)) > 0 {
		t.queue <- body
	}
	return nil
}

func (t *httpTransport) Recv(ctx context.Context) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case m := <-t.queue:
		return m, nil
	}
}

func (t *httpTransport) Close() error { return nil }
