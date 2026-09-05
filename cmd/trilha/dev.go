package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/emersonjoe/trilha/internal/dev"
)

func cmdDev(args []string) error {
	fs := flag.NewFlagSet("dev", flag.ContinueOnError)
	addr := fs.String("addr", ":3000", t("flag addr"))
	if err := fs.Parse(args); err != nil {
		return err
	}
	p, err := findProject()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	s := &dev.Server{
		Root: p.Root,
		Addr: *addr,
		Out:  os.Stdout,
		Generate: func() error {
			_, err := generate(p)
			return err
		},
	}
	return s.Run(ctx)
}
