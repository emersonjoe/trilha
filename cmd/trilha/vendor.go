package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// vendorDir is where a downloaded module lands: public/, so it is served like
// any other asset, under a directory that says where it came from.
const vendorDir = "public/vendor"

// vendorLock is the record of what was downloaded. It is committed, and it is
// what --check compares the files against.
const vendorLock = "vendor.lock"

// defaultVendorBase is a CDN that serves npm packages as ES modules. It is a
// default, not a dependency: --from or TRILHA_VENDOR_BASE points anywhere.
const defaultVendorBase = "https://esm.sh"

// vendorMax is the ceiling for one module. An island helper that does not fit
// in eight megabytes is not a helper.
const vendorMax = 8 << 20

// pinned is one line of vendor.lock.
type pinned struct {
	Name    string
	Version string
	Sum     string
	URL     string
	File    string
}

func cmdVendor(args []string) error {
	fs := flag.NewFlagSet("vendor", flag.ContinueOnError)
	check := fs.Bool("check", false, t("flag vendor check"))
	from := fs.String("from", "", t("flag vendor from"))
	// Flags on either side of the package, the way migrate and ui describe do.
	var spec string
	rest := make([]string, 0, len(args))
	for _, a := range args {
		if spec == "" && !strings.HasPrefix(a, "-") {
			spec = a
			continue
		}
		rest = append(rest, a)
	}
	if err := fs.Parse(rest); err != nil {
		return err
	}
	p, err := findProject()
	if err != nil {
		return err
	}
	locked, err := readLock(p)
	if err != nil {
		return err
	}
	switch {
	case *check:
		return checkVendor(p, locked)
	case spec == "":
		return listVendor(locked)
	}
	base := *from
	if base == "" {
		base = os.Getenv("TRILHA_VENDOR_BASE")
	}
	if base == "" {
		base = defaultVendorBase
	}
	return addVendor(p, locked, spec, base)
}

// addVendor downloads one module, writes it under public/vendor and records
// what it was. It resolves nothing: what the module imports is the module's
// business, and an island helper that needs a resolver is the wrong helper.
func addVendor(p *project, locked []pinned, spec, base string) error {
	name, version, ok := strings.Cut(spec, "@")
	if strings.HasPrefix(spec, "@") { // a scoped package: @preact/signals@1.2.3
		name, version, ok = cutScoped(spec)
	}
	if !ok || name == "" || version == "" {
		return errors.New(t("vendor usage"))
	}
	u, err := url.Parse(strings.TrimSuffix(base, "/") + "/" + name + "@" + version)
	if err != nil {
		return err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf(t("vendor scheme"), u.Scheme)
	}
	body, err := fetchVendor(u.String())
	if err != nil {
		return err
	}
	sum := sha256.Sum256(body)
	rel := filepath.Join(vendorDir, vendorFile(name))
	out := filepath.Join(p.Root, rel)
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(out, body, 0o644); err != nil {
		return err
	}
	locked = append(without(locked, name), pinned{
		Name: name, Version: version, Sum: hex.EncodeToString(sum[:]),
		URL: u.String(), File: filepath.ToSlash(rel),
	})
	if err := writeLock(p, locked); err != nil {
		return err
	}
	fmt.Printf(t("vendor done"), name, version, filepath.ToSlash(rel), len(body))
	fmt.Println(t("vendor hint"))
	return nil
}

// cutScoped splits @scope/name@version, which has two at-signs.
func cutScoped(spec string) (name, version string, ok bool) {
	i := strings.LastIndex(spec, "@")
	if i <= 0 {
		return "", "", false
	}
	return spec[:i], spec[i+1:], true
}

func fetchVendor(u string) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	res, err := client.Get(u)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(t("vendor http"), res.StatusCode, u)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, vendorMax+1))
	if err != nil {
		return nil, err
	}
	if len(body) > vendorMax {
		return nil, fmt.Errorf(t("vendor too big"), vendorMax)
	}
	return body, nil
}

// checkVendor re-hashes what is on disk. A module that changed under a version
// that did not is the thing the lock exists to catch.
func checkVendor(p *project, locked []pinned) error {
	var bad []string
	seen := map[string]bool{}
	for _, l := range locked {
		seen[l.File] = true
		body, err := os.ReadFile(filepath.Join(p.Root, filepath.FromSlash(l.File)))
		if err != nil {
			bad = append(bad, fmt.Sprintf(t("vendor missing"), l.File, l.Name))
			continue
		}
		sum := sha256.Sum256(body)
		if hex.EncodeToString(sum[:]) != l.Sum {
			bad = append(bad, fmt.Sprintf(t("vendor changed"), l.File, l.Name, l.Version))
		}
	}
	files, _ := filepath.Glob(filepath.Join(p.Root, vendorDir, "*.js"))
	sort.Strings(files)
	for _, f := range files {
		rel := filepath.ToSlash(filepath.Join(vendorDir, filepath.Base(f)))
		if !seen[rel] {
			bad = append(bad, fmt.Sprintf(t("vendor unpinned"), rel))
		}
	}
	if len(bad) == 0 {
		fmt.Printf(t("vendor ok"), len(locked))
		return nil
	}
	for _, b := range bad {
		fmt.Fprintln(os.Stderr, "✗", b)
	}
	return errors.New(t("vendor failed"))
}

func listVendor(locked []pinned) error {
	if len(locked) == 0 {
		fmt.Println(t("vendor empty"))
		return nil
	}
	fmt.Printf("%-28s %-12s %s\n", t("MODULE"), t("VERSION"), t("FILE"))
	for _, l := range locked {
		fmt.Printf("%-28s %-12s %s\n", l.Name, l.Version, l.File)
	}
	return nil
}

// vendorFile is the name the module gets in public/vendor: one file, one flat
// directory, so the import in an island is a path anyone can read.
func vendorFile(name string) string {
	name = strings.TrimPrefix(name, "@")
	name = strings.ReplaceAll(name, "/", "-")
	return name + ".js"
}

func without(locked []pinned, name string) []pinned {
	out := locked[:0:0]
	for _, l := range locked {
		if l.Name != name {
			out = append(out, l)
		}
	}
	return out
}

// readLock reads vendor.lock. A project that never vendored anything has no
// file, and that is not an error.
func readLock(p *project) ([]pinned, error) {
	data, err := os.ReadFile(filepath.Join(p.Root, vendorLock))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []pinned
	for i, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Fields(line)
		if len(f) != 5 {
			return nil, fmt.Errorf(t("vendor lock line"), vendorLock, i+1)
		}
		out = append(out, pinned{Name: f[0], Version: f[1], Sum: f[2], File: f[3], URL: f[4]})
	}
	return out, nil
}

func writeLock(p *project, locked []pinned) error {
	sort.Slice(locked, func(i, j int) bool { return locked[i].Name < locked[j].Name })
	var sb strings.Builder
	sb.WriteString("# trilha vendor. name version sha256 file url\n")
	sb.WriteString("# Written by trilha vendor; checked by trilha vendor --check.\n")
	for _, l := range locked {
		fmt.Fprintf(&sb, "%s %s %s %s %s\n", l.Name, l.Version, l.Sum, l.File, l.URL)
	}
	return os.WriteFile(filepath.Join(p.Root, vendorLock), []byte(sb.String()), 0o644)
}
