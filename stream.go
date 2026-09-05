package trilha

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// Stream is a Server-Sent Events writer returned by Ctx.Stream.
type Stream struct {
	c  *Ctx
	fl http.Flusher
}

// Stream switches the response to text/event-stream, disables the write
// deadline and returns a writer for events. Use it from a GET route:
//
//	func GET(c *trilha.Ctx) error {
//		s := c.Stream()
//		for chunk := range chunks {
//			if err := s.Send("delta", chunk); err != nil { return err }
//		}
//		return s.Send("done", "")
//	}
//
// Compression is skipped for streams, and each event is flushed right away.
func (c *Ctx) Stream() *Stream {
	h := c.w.Header()
	h.Set("Content-Type", "text/event-stream; charset=utf-8")
	h.Set("Cache-Control", "no-cache, no-transform")
	h.Set("X-Accel-Buffering", "no")
	if h.Get("Connection") == "" && c.r.ProtoMajor == 1 {
		h.Set("Connection", "keep-alive")
	}
	_ = c.NoWriteDeadline()
	c.w.WriteHeader(http.StatusOK)
	s := &Stream{c: c}
	if fl, ok := http.ResponseWriter(c.w).(http.Flusher); ok {
		s.fl = fl
	}
	s.Flush()
	return s
}

// Send writes one event. An empty name sends an unnamed message (received by
// EventSource.onmessage). Multi-line data is split into several data: lines.
func (s *Stream) Send(event, data string) error {
	var b strings.Builder
	if event != "" {
		b.WriteString("event: ")
		b.WriteString(event)
		b.WriteByte('\n')
	}
	for _, line := range strings.Split(data, "\n") {
		b.WriteString("data: ")
		b.WriteString(line)
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	if _, err := s.c.w.Write([]byte(b.String())); err != nil {
		return err
	}
	s.Flush()
	return nil
}

// JSON marshals v and sends it as the event data.
func (s *Stream) JSON(event string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("stream: %w", err)
	}
	return s.Send(event, string(b))
}

// Comment sends a comment line (keeps proxies from timing out the stream).
func (s *Stream) Comment(text string) error {
	_, err := fmt.Fprintf(s.c.w, ": %s\n\n", text)
	s.Flush()
	return err
}

// Flush pushes buffered bytes to the client.
func (s *Stream) Flush() {
	if s.fl != nil {
		s.fl.Flush()
	}
}

// Done reports when the client went away.
func (s *Stream) Done() <-chan struct{} { return s.c.r.Context().Done() }
