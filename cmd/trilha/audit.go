package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/emersonjoe/trilha/internal/gen"
	"github.com/emersonjoe/trilha/internal/scan"
)

type check struct {
	level string // ok | warn | critical
	title string
	hint  string
}

func cmdAudit(args []string) error {
	fs := flag.NewFlagSet("audit", flag.ContinueOnError)
	noVuln := fs.Bool("no-vuln", false, t("flag no-vuln"))
	if err := fs.Parse(args); err != nil {
		return err
	}
	p, err := findProject()
	if err != nil {
		return err
	}
	checks := runAudit(p, !*noVuln)
	critical := 0
	for _, c := range checks {
		mark := map[string]string{"ok": "✓", "warn": "!", "critical": "✗"}[c.level]
		fmt.Printf("%s %s\n", mark, c.title)
		if c.hint != "" {
			fmt.Printf("    %s\n", c.hint)
		}
		if c.level == "critical" {
			critical++
		}
	}
	if critical > 0 {
		return fmt.Errorf(t("critical items"), critical)
	}
	fmt.Println(t("no critical"))
	return nil
}

func runAudit(p *project, vuln bool) []check {
	var out []check
	add := func(level, title, hint string) { out = append(out, check{level, title, hint}) }

	src := projectSource(p.Root)

	// Secret. A missing secret only breaks what the secret signs: an app that
	// never calls SetSigned would be told to invent a key that protects
	// nothing, and a key nobody uses is a key nobody notices being rotated.
	if s := os.Getenv("TRILHA_SECRET"); s == "" {
		if signsCookies(src) {
			add("critical", t("secret unset"), t("secret unset hint"))
		} else {
			add("warn", t("secret unused"), t("secret unused hint"))
		}
	} else if len(s) < 32 {
		add("critical", t("secret short"), t("secret short hint"))
	} else {
		add("ok", t("secret ok"), "")
	}
	if os.Getenv("TRILHA_TRUSTED_PROXIES") == "" {
		add("warn", t("proxies unset"), t("proxies unset hint"))
	} else {
		add("ok", t("proxies ok"), "")
	}

	// Host validation (spec 034): without a list, any Host the client sends
	// is the one the app builds its own links with.
	if os.Getenv("TRILHA_ALLOWED_HOSTS") == "" && !strings.Contains(src, "AllowedHosts") {
		add("warn", t("hosts unset"), t("hosts unset hint"))
	} else {
		add("ok", t("hosts ok"), "")
	}

	// Observability (NIST SP 800-53 AU-9: audit information is protected;
	// OWASP API Security 2023 API8: no unprotected monitoring endpoint).
	metricsOn := os.Getenv("TRILHA_METRICS") != "" || strings.Contains(src, "Metrics:")
	tok := os.Getenv("TRILHA_OBS_TOKEN")
	trusted := os.Getenv("TRILHA_OBS_TRUSTED") != "" || strings.Contains(src, "Trusted:")
	switch {
	case tok != "" && len(tok) < 32:
		add("critical", t("obs token short"), t("obs token short hint"))
	case metricsOn && tok == "" && !trusted:
		add("critical", t("metrics exposed"), t("metrics exposed hint"))
	case metricsOn:
		add("ok", t("metrics protected"), "")
	default:
		add("ok", t("metrics off"), "")
	}
	if strings.Contains(src, "0.0.0.0/0") || strings.Contains(os.Getenv("TRILHA_OBS_TRUSTED"), "0.0.0.0/0") {
		add("warn", t("obs open"), t("obs open hint"))
	}
	if !strings.Contains(src, ".Check(") {
		add("warn", t("no checks"), t("no checks hint"))
	} else {
		add("ok", t("checks ok"), "")
	}

	// Asset cache (spec 017): a long cache on a fixed address is stale CSS
	// for a year.
	if strings.Contains(src, "immutable") && !strings.Contains(src, ".Asset(") {
		add("warn", t("asset immutable"), t("asset immutable hint"))
	}

	// OIDC login (spec 016): the client secret and the redirect address are
	// the two mistakes that show up in every OAuth review.
	if strings.Contains(src, "trilha/auth") {
		hard, cleartext := 0, 0
		for _, call := range authCalls(src) {
			if i := secretArg(call.name); i < len(call.args) && strings.HasPrefix(call.args[i], `"`) {
				hard++
			}
			if last := call.args[len(call.args)-1]; strings.HasPrefix(last, `"http://`) &&
				!strings.Contains(last, "localhost") && !strings.Contains(last, "127.0.0.1") {
				cleartext++
			}
		}
		switch {
		case hard > 0:
			add("critical", t("oidc secret hard"), t("oidc secret hard hint"))
		default:
			add("ok", t("oidc secret ok"), "")
		}
		if cleartext > 0 {
			add("critical", t("oidc cleartext"), t("oidc cleartext hint"))
		}
	}

	// Generated file up to date.
	res, err := scan.Scan(p.Root, p.Module)
	if err != nil {
		add("critical", t("app invalid"), err.Error())
	} else if src, err := gen.Generate(res); err == nil {
		cur, _ := os.ReadFile(filepath.Join(p.Root, gen.FileName))
		if string(cur) != string(src) {
			add("warn", t("gen stale"), t("gen stale hint"))
		} else {
			add("ok", t("gen fresh"), "")
		}
	}

	// CLI and library: a newer CLI writes generated code that the library in
	// go.mod may not have yet, and the error then shows up inside generated
	// code — the worst place to look for it.
	if lib, replaced := libVersion(p.Root); !replaced && lib != "" {
		if lib != "v"+version {
			add("warn", fmt.Sprintf(t("cli skew"), version, strings.TrimPrefix(lib, "v")), t("cli skew hint"))
		} else {
			add("ok", t("cli match"), "")
		}
	}

	// Go version.
	v := strings.TrimPrefix(runtime.Version(), "go")
	if strings.HasPrefix(v, "1.2") && v < "1.22" {
		add("critical", fmt.Sprintf(t("go unsupported"), v), t("go unsupported hint"))
	} else {
		add("ok", "Go "+v, "")
	}

	// .gitignore.
	if gi, err := os.ReadFile(filepath.Join(p.Root, ".gitignore")); err != nil || !strings.Contains(string(gi), ".trilha") {
		add("warn", t("gitignore missing"), t("gitignore hint"))
	} else {
		add("ok", t("gitignore ok"), "")
	}

	// go vet.
	if outb, err := runCmd(p.Root, "go", "vet", "./..."); err != nil {
		add("warn", t("vet problems"), strings.TrimSpace(string(outb)))
	} else {
		add("ok", t("vet clean"), "")
	}

	// govulncheck (optional, needs network).
	if vuln {
		if outb, err := runCmd(p.Root, "go", "run", "golang.org/x/vuln/cmd/govulncheck@latest", "./..."); err != nil {
			txt := strings.TrimSpace(string(outb))
			if strings.Contains(txt, "Vulnerability") || strings.Contains(txt, "vulnerabilit") {
				add("critical", t("vuln found"), lastLines(txt, 8))
			} else {
				add("warn", t("vuln failed"), t("vuln failed hint")+lastLines(txt, 2))
			}
		} else {
			add("ok", t("vuln clean"), "")
		}
	}
	return out
}

// projectSource concatenates the Go sources of the project, so the checks can
// look for configuration the environment does not reveal.
// signsCookies reports whether the app signs anything with TRILHA_SECRET: the
// Ctx helpers, a Signer of its own, a Secret set in Config, or the auth package,
// whose login flow keeps its state in a signed cookie.
func signsCookies(src string) bool {
	for _, s := range []string{".SetSigned(", ".Signed(", "NewSigner(", "Secret:", "trilha/auth"} {
		if strings.Contains(src, s) {
			return true
		}
	}
	return false
}

func projectSource(root string) string {
	var b strings.Builder
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".trilha", "node_modules", "vendor", "bin":
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			if data, err := os.ReadFile(path); err == nil {
				b.Write(data)
			}
		}
		return nil
	})
	return b.String()
}

func runCmd(dir, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	return cmd.CombinedOutput()
}

func lastLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n    ")
}

// authCall is one call to auth.OIDC/EntraID/Keycloak/Cognito found in the sources.
type authCall struct {
	name string
	args []string
}

// authCalls finds the provider constructors and splits their arguments at the
// top level, so that os.Getenv("X") stays one argument.
func authCalls(src string) []authCall {
	var out []authCall
	for _, name := range []string{"OIDC", "EntraID", "Keycloak", "Cognito", "Clerk"} {
		needle := "auth." + name + "("
		for i := 0; ; {
			j := strings.Index(src[i:], needle)
			if j < 0 {
				break
			}
			start := i + j + len(needle)
			args, end := splitArgs(src[start:])
			if len(args) > 0 {
				out = append(out, authCall{name: name, args: args})
			}
			i = start + end
		}
	}
	return out
}

// splitArgs reads until the closing parenthesis, splitting on commas that are
// not inside nested parentheses or a string.
func splitArgs(s string) ([]string, int) {
	var args []string
	depth, quoted, start := 0, false, 0
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case quoted && c == '\\':
			i++
		case c == '"':
			quoted = !quoted
		case quoted:
		case c == '(':
			depth++
		case c == ')' && depth > 0:
			depth--
		case c == ')':
			args = append(args, strings.TrimSpace(s[start:i]))
			return args, i + 1
		case c == ',' && depth == 0:
			args = append(args, strings.TrimSpace(s[start:i]))
			start = i + 1
		}
	}
	return nil, len(s)
}

// secretArg is the position of the client secret in each constructor.
func secretArg(name string) int {
	if name == "Keycloak" || name == "Cognito" {
		// Keycloak: baseURL, realm, clientID, clientSecret, redirectURL
		// Cognito: region, userPoolID, clientID, clientSecret, redirectURL
		return 3
	}
	// OIDC, EntraID and Clerk: issuer|tenant|frontendAPI, clientID,
	// clientSecret, redirectURL.
	return 2
}

// libVersion reads the version of the trilha library required by go.mod.
// replaced is true for a local replace directive, where comparing versions
// says nothing.
func libVersion(root string) (version string, replaced bool) {
	b, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", false
	}
	const mod = "github.com/emersonjoe/trilha"
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "replace ") && strings.Contains(line, mod) {
			return "", true
		}
		f := strings.Fields(strings.TrimPrefix(line, "require "))
		if len(f) >= 2 && f[0] == mod && strings.HasPrefix(f[1], "v") {
			version = f[1]
		}
	}
	return version, false
}
