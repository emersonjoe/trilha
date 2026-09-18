// Package traco answers /traco: the second hop of the trace.
package traco

import (
	"io"
	"net/http"
	"os"

	"github.com/emersonjoe/trilha"
	trilhaotel "github.com/emersonjoe/trilha/otel"
)

// client carries the trace out with every call. Transport injects the
// traceparent of whatever span is in the request context, which is why the
// call below has to be made with c.Context().
var client = &http.Client{Transport: trilhaotel.Transport(nil)}

// GET calls another service and gives back what it answered. In the collector
// the two services are one trace: this span is the parent of the one the other
// side opens.
func GET(c *trilha.Ctx) error {
	target := os.Getenv("EXAMPLE_TARGET_URL")
	if target == "" {
		target = "http://localhost:3002/"
	}
	req, err := http.NewRequestWithContext(c.Context(), http.MethodGet, target, nil)
	if err != nil {
		return err
	}
	res, err := client.Do(req)
	if err != nil {
		// A hop that did not happen is a 502, and the span of this request
		// carries the status like any other.
		return &trilha.HTTPError{Code: http.StatusBadGateway, Message: "upstream unreachable"}
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{
		"trace":    c.TraceID(),
		"upstream": res.StatusCode,
		"bytes":    len(body),
	})
}
