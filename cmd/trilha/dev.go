package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/emersonjoe/trilha/internal/dev"
)

func cmdDev(args []string) error {
	fs := flag.NewFlagSet("dev", flag.ContinueOnError)
	addr := fs.String("addr", defaultDevAddr, t("flag addr"))
	if err := fs.Parse(args); err != nil {
		return err
	}
	given := false
	fs.Visit(func(f *flag.Flag) { given = given || f.Name == "addr" })
	container := !given && inContainer()
	listen := devAddr(*addr, given, container)
	p, err := findProject()
	if err != nil {
		return err
	}
	if pkg := embeddedPackage(p); pkg != "" {
		return fmt.Errorf(t("embedded no binary"), pkg)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	fmt.Print(devNotice(listen, container))
	s := &dev.Server{
		Root:   p.Root,
		Module: p.Module,
		Addr:   listen,
		Out:    os.Stdout,
		Generate: func() error {
			_, err := generate(p)
			return err
		},
	}
	return s.Run(ctx)
}

// defaultDevAddr keeps the dev server on this machine. Its port serves what
// production hides — error pages with source, the route inspector,
// /_trilha/report — and a café's network has no business reading them (#318).
const defaultDevAddr = "127.0.0.1:3000"

// inContainer says the CLI runs inside a container, where a loopback address
// is out of reach of the published port. A var so tests can stub it.
var inContainer = func() bool {
	for _, f := range []string{"/.dockerenv", "/run/.containerenv"} {
		if _, err := os.Stat(f); err == nil {
			return true
		}
	}
	return os.Getenv("KUBERNETES_SERVICE_HOST") != ""
}

// devAddr is the address the dev server listens on: -addr when it was typed,
// every interface inside a container, loopback otherwise.
func devAddr(flagAddr string, given, container bool) string {
	if !given && container {
		return ":3000"
	}
	return flagAddr
}

// exposedAddr says addr listens beyond loopback. A name other than
// localhost, or something that is not host:port, counts as exposed: the
// warning is cheaper than the guess.
func exposedAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return true
	}
	if host == "localhost" {
		return false
	}
	ip := net.ParseIP(host)
	return ip == nil || !ip.IsLoopback()
}

// devNotice is the warning printed before an exposed dev server starts, or ""
// for a loopback one.
func devNotice(addr string, container bool) string {
	if !exposedAddr(addr) {
		return ""
	}
	n := fmt.Sprintf(t("dev exposed"), addr, defaultDevAddr)
	if container {
		n += t("dev container")
	}
	return n
}
