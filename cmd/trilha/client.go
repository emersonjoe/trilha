package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/emersonjoe/trilha/internal/client"
)

// cmdClient generates the Go client of an API that already exists. The document
// is data: it names types and paths, never a file to open or a command to run,
// and the only thing fetched from the network is the document itself.
func cmdClient(args []string) error {
	fs := flag.NewFlagSet("client", flag.ContinueOnError)
	out := fs.String("out", "internal/api", t("flag client out"))
	pkg := fs.String("package", "", t("flag client package"))
	check := fs.Bool("check", false, t("flag client check"))
	// The document is pulled out before parsing so the flags may come on either
	// side of it.
	var src string
	rest := make([]string, 0, len(args))
	for _, a := range args {
		if src == "" && !strings.HasPrefix(a, "-") {
			src = a
			continue
		}
		rest = append(rest, a)
	}
	if err := fs.Parse(rest); err != nil {
		return err
	}
	if src == "" {
		return errors.New(t("client needs a doc"))
	}
	if *pkg == "" {
		*pkg = filepath.Base(*out)
	}
	data, err := readDoc(src)
	if err != nil {
		return err
	}
	res, err := client.Generate(data, client.Options{Package: *pkg, Source: src})
	if err != nil {
		return err
	}
	path := filepath.Join(*out, "client.go")
	if *check {
		cur, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if string(cur) == string(res.Source) {
			fmt.Println("✓", fmt.Sprintf(t("client fresh"), path))
			return nil
		}
		return fmt.Errorf(t("client stale"), path)
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, res.Source, 0o644); err != nil {
		return err
	}
	for _, n := range res.Notes {
		fmt.Printf("  %s: %s\n", n.Where, n.What)
	}
	fmt.Printf(t("client done")+"\n", path, *pkg)
	return nil
}

// readDoc reads the document from a file or from a URL. Only http and https:
// a scheme this does not know is a mistake worth saying out loud, not a thing
// to try.
func readDoc(src string) ([]byte, error) {
	if !strings.HasPrefix(src, "http://") && !strings.HasPrefix(src, "https://") {
		if i := strings.Index(src, "://"); i > 0 {
			return nil, fmt.Errorf(t("client bad scheme"), src[:i])
		}
		return os.ReadFile(src)
	}
	c := &http.Client{Timeout: 30 * time.Second}
	resp, err := c.Get(src)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(t("client fetch failed"), src, resp.StatusCode)
	}
	// A document larger than this is not a document anyone reviews.
	return io.ReadAll(io.LimitReader(resp.Body, 32<<20))
}
