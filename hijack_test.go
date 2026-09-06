package trilha

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Issue #24: WebSocket stays out of the core, so what has to work is the way
// out — a handler taking the connection over. The test speaks the protocol by
// hand: if a hand-written client gets through the handshake and back a frame,
// any library will.
func TestHijackServesWebSocketHandshake(t *testing.T) {
	app := New(Config{Logger: quiet()})
	app.Register(Route{Pattern: "/ws", Methods: map[string]HandlerFunc{"GET": func(c *Ctx) error {
		conn, rw, err := c.Hijack()
		if err != nil {
			return err
		}
		defer conn.Close()
		acc := wsAccept(c.Request().Header.Get("Sec-WebSocket-Key"))
		rw.WriteString("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: " + acc + "\r\n\r\n")
		if err := rw.Flush(); err != nil {
			return err
		}
		msg, err := wsRead(rw.Reader)
		if err != nil {
			return err
		}
		rw.Write(wsTextFrame("echo:" + msg))
		return rw.Flush()
	}}})
	srv := httptest.NewServer(app.Handler())
	defer srv.Close()

	conn, err := net.Dial("tcp", strings.TrimPrefix(srv.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	key := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef"))
	fmtReq := "GET /ws HTTP/1.1\r\nHost: x\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Version: 13\r\nSec-WebSocket-Key: " + key + "\r\n\r\n"
	if _, err := io.WriteString(conn, fmtReq); err != nil {
		t.Fatal(err)
	}
	br := bufio.NewReader(conn)
	res, err := http.ReadResponse(br, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("status %d: the response must be hijackable", res.StatusCode)
	}
	if got := res.Header.Get("Sec-WebSocket-Accept"); got != wsAccept(key) {
		t.Fatalf("accept %q", got)
	}
	if _, err := conn.Write(wsMaskedTextFrame("olá")); err != nil {
		t.Fatal(err)
	}
	got, err := wsRead(br)
	if err != nil || got != "echo:olá" {
		t.Fatal(got, err)
	}
}

// The connection a handler takes over outlives the request timeouts: an idle
// socket is the normal state of a WebSocket, not a stuck request.
func TestHijackClearsDeadlines(t *testing.T) {
	app := New(Config{Logger: quiet()})
	done := make(chan error, 1)
	app.Register(Route{Pattern: "/raw", Methods: map[string]HandlerFunc{"GET": func(c *Ctx) error {
		conn, rw, err := c.Hijack()
		if err != nil {
			return err
		}
		defer conn.Close()
		rw.WriteString("HTTP/1.1 101 Switching Protocols\r\n\r\n")
		rw.Flush()
		// A read with no deadline blocks; with the server's ReadTimeout still
		// on the connection it would fail immediately.
		conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
		_, err = rw.Reader.ReadByte()
		done <- err
		return nil
	}}})
	srv := httptest.NewServer(app.Handler())
	defer srv.Close()
	conn, err := net.Dial("tcp", strings.TrimPrefix(srv.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	io.WriteString(conn, "GET /raw HTTP/1.1\r\nHost: x\r\n\r\n")
	select {
	case err := <-done:
		ne, ok := err.(net.Error)
		if !ok || !ne.Timeout() {
			t.Fatalf("read ended with %v; the handler's own deadline must be the only one", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("handler never returned")
	}
}

// ---- a hand-written pinch of RFC 6455, test only -------------------------

func wsAccept(key string) string {
	sum := sha1.Sum([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	return base64.StdEncoding.EncodeToString(sum[:])
}

func wsTextFrame(s string) []byte {
	b := []byte{0x81, byte(len(s))}
	return append(b, s...)
}

func wsMaskedTextFrame(s string) []byte {
	var mask [4]byte
	rand.Read(mask[:])
	out := []byte{0x81, byte(0x80 | len(s))}
	out = append(out, mask[:]...)
	for i := 0; i < len(s); i++ {
		out = append(out, s[i]^mask[i%4])
	}
	return out
}

func wsRead(r *bufio.Reader) (string, error) {
	head := make([]byte, 2)
	if _, err := io.ReadFull(r, head); err != nil {
		return "", err
	}
	n := int(head[1] & 0x7f)
	var mask []byte
	if head[1]&0x80 != 0 {
		mask = make([]byte, 4)
		if _, err := io.ReadFull(r, mask); err != nil {
			return "", err
		}
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}
	for i := range buf {
		if mask != nil {
			buf[i] ^= mask[i%4]
		}
	}
	return string(bytes.TrimSpace(buf)), nil
}
