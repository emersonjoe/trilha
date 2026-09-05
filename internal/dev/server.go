package dev

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/emersonjoe/trilha"
)

// Generator regenerates trilha_gen.go; injected by the CLI.
type Generator func() error

// Server is the dev supervisor: public listener + proxy + child process.
type Server struct {
	Root     string
	Addr     string
	Generate Generator
	Out      io.Writer

	mu        sync.Mutex
	child     *exec.Cmd
	childPort int
	proxy     *httputil.ReverseProxy
	buildErr  string
	clients   map[chan string]struct{}
	binPath   string
	secret    string
}

// Run generates, builds, starts the child and watches for changes until ctx
// is cancelled.
func (s *Server) Run(ctx context.Context) error {
	if s.Out == nil {
		s.Out = os.Stdout
	}
	s.clients = map[chan string]struct{}{}
	s.binPath = filepath.Join(s.Root, ".trilha", "app")
	if err := os.MkdirAll(filepath.Dir(s.binPath), 0o755); err != nil {
		return err
	}
	_ = os.WriteFile(filepath.Join(s.Root, ".trilha", ".gitignore"), []byte("*\n"), 0o644)

	ln, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return fmt.Errorf("não consegui escutar em %s: %w", s.Addr, err)
	}
	srv := &http.Server{Handler: http.HandlerFunc(s.serveHTTP)}
	go func() { _ = srv.Serve(ln) }()
	fmt.Fprintf(s.Out, "→ http://localhost:%d\n", ln.Addr().(*net.TCPAddr).Port)

	s.rebuild()

	stop := make(chan struct{})
	changes := Watch(s.Root, 250*time.Millisecond, 100*time.Millisecond, stop)
	for {
		select {
		case <-ctx.Done():
			close(stop)
			s.stopChild()
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			return srv.Shutdown(shutdownCtx)
		case change := <-changes:
			if change.StaticOnly {
				fmt.Fprintf(s.Out, "↻ %s (estático)\n", summarize(change.Paths))
				s.broadcast("reload")
				continue
			}
			fmt.Fprintf(s.Out, "↻ %s\n", summarize(change.Paths))
			s.rebuild()
		}
	}
}

func summarize(paths []string) string {
	if len(paths) == 1 {
		return paths[0]
	}
	return fmt.Sprintf("%s (+%d)", paths[0], len(paths)-1)
}

// rebuild runs gen + go build, then swaps the child process.
func (s *Server) rebuild() {
	start := time.Now()
	if err := s.Generate(); err != nil {
		s.fail("trilha gen:\n" + err.Error())
		return
	}
	cmd := exec.Command("go", "build", "-o", s.binPath, ".")
	cmd.Dir = s.Root
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Run(); err != nil {
		s.fail(out.String())
		return
	}
	port, err := s.startChild()
	if err != nil {
		s.fail(err.Error())
		return
	}
	s.mu.Lock()
	s.buildErr = ""
	s.childPort = port
	target, _ := url.Parse("http://127.0.0.1:" + strconv.Itoa(port))
	s.proxy = httputil.NewSingleHostReverseProxy(target)
	s.proxy.FlushInterval = -1
	s.proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		http.Error(w, "app indisponível: "+err.Error(), http.StatusBadGateway)
	}
	s.mu.Unlock()
	fmt.Fprintf(s.Out, "✓ pronto (%s)\n", time.Since(start).Round(time.Millisecond))
	s.broadcast("reload")
}

func (s *Server) fail(msg string) {
	s.mu.Lock()
	s.buildErr = msg
	s.mu.Unlock()
	s.stopChild()
	fmt.Fprintf(s.Out, "✗ erro:\n%s\n", msg)
	s.broadcast("reload")
}

// startChild launches the built binary on a free port and waits for it.
func (s *Server) startChild() (int, error) {
	s.stopChild()
	port, err := freePort()
	if err != nil {
		return 0, err
	}
	cmd := exec.Command(s.binPath)
	cmd.Dir = s.Root
	cmd.Env = append(os.Environ(), "TRILHA_ENV=dev", "ADDR=127.0.0.1:"+strconv.Itoa(port))
	if os.Getenv("TRILHA_SECRET") == "" {
		// One ephemeral secret per dev session, so signed cookies survive rebuilds.
		cmd.Env = append(cmd.Env, "TRILHA_SECRET="+s.devSecret())
	}
	cmd.Stdout, cmd.Stderr = s.Out, s.Out
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	s.mu.Lock()
	s.child = cmd
	s.mu.Unlock()
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case err := <-exited:
			return 0, fmt.Errorf("o app terminou ao iniciar: %v", err)
		default:
		}
		conn, err := net.DialTimeout("tcp", "127.0.0.1:"+strconv.Itoa(port), 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return port, nil
		}
		time.Sleep(25 * time.Millisecond)
	}
	return 0, errors.New("o app não respondeu em 10 s")
}

func (s *Server) devSecret() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.secret == "" {
		b := make([]byte, 32)
		_, _ = rand.Read(b)
		s.secret = base64.StdEncoding.EncodeToString(b)
	}
	return s.secret
}

func (s *Server) stopChild() {
	s.mu.Lock()
	c := s.child
	s.child = nil
	s.proxy = nil
	s.mu.Unlock()
	if c == nil || c.Process == nil {
		return
	}
	_ = c.Process.Signal(os.Interrupt)
	done := make(chan struct{})
	go func() { _, _ = c.Process.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		_ = c.Process.Kill()
	}
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// ---- HTTP: proxy, error page and SSE ---------------------------------------

func (s *Server) serveHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/_trilha/events" {
		s.events(w, r)
		return
	}
	s.mu.Lock()
	proxy, buildErr := s.proxy, s.buildErr
	s.mu.Unlock()
	if buildErr != "" || proxy == nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusServiceUnavailable)
		msg := buildErr
		if msg == "" {
			msg = "compilando..."
		}
		_, _ = io.WriteString(w, trilha.CompileErrorPage(msg))
		return
	}
	proxy.ServeHTTP(w, r)
}

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "no streaming", 500)
		return
	}
	ch := make(chan string, 4)
	s.mu.Lock()
	s.clients[ch] = struct{}{}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.clients, ch)
		s.mu.Unlock()
	}()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(200)
	_, _ = io.WriteString(w, ": trilha dev\n\n")
	fl.Flush()
	ping := time.NewTicker(15 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case msg := <-ch:
			_, _ = fmt.Fprintf(w, "data: %s\n\n", msg)
			fl.Flush()
		case <-ping.C:
			_, _ = io.WriteString(w, ": ping\n\n")
			fl.Flush()
		}
	}
}

func (s *Server) broadcast(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for ch := range s.clients {
		select {
		case ch <- msg:
		default:
		}
	}
}
