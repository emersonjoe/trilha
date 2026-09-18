// Package app is the application of the example: one page, one API route and
// the three lines that turn the traces on.
package app

import (
	"context"
	"os"
	"strconv"
	"time"

	"github.com/emersonjoe/trilha"
	trilhaotel "github.com/emersonjoe/trilha/otel"
)

// Setup runs after trilha.New. Installing the exporter here is the whole
// integration: from this line on every request that reaches a route becomes a
// span, and the traceparent the caller sent is the parent of that span.
func Setup(a *trilha.App) error {
	shutdown, err := trilhaotel.Install(a, trilhaotel.Options{
		// The two standard variables, so this app talks to a collector
		// started by docker compose without a flag.
		Endpoint: os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		Service:  env("OTEL_SERVICE_NAME", "trilha-otel-example"),
		Version:  os.Getenv("OTEL_SERVICE_VERSION"),
		Sample:   sample(),
		// A collector on localhost, over plain HTTP. In production the
		// collector is reached over TLS and this line goes away.
		Insecure: true,
	})
	if err != nil {
		return err
	}
	// Without this the last seconds of traces are the ones that never arrive:
	// the exporter batches, and a process that exits takes the batch with it.
	a.OnShutdown(func(*trilha.App) error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return shutdown(ctx)
	})
	return nil
}

// sample reads OTEL_TRACES_SAMPLER_ARG, the fraction of traces kept for the
// requests that arrive with no decision already made. Anything unreadable
// keeps every trace, which is the right default for an example.
func sample() float64 {
	v, err := strconv.ParseFloat(os.Getenv("OTEL_TRACES_SAMPLER_ARG"), 64)
	if err != nil {
		return 1
	}
	return v
}

func env(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}
