package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
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
		add("ok", t("hosts ok"), t("hosts ok hint"))
	}

	// Observability (NIST SP 800-53 AU-9: audit information is protected;
	// OWASP API Security 2023 API8: no unprotected monitoring endpoint).
	metricsOn := os.Getenv("TRILHA_METRICS") != "" || metricsConfigured(src)
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
	// Mail. The dev mode writes .eml files into a directory and says so, which
	// is exactly right on a laptop and silent data loss in production: nobody
	// finds out the invitations were never sent until somebody asks why they
	// never arrived.
	if strings.Contains(src, "mail.New(") || strings.Contains(src, "mail.FromEnv(") {
		if os.Getenv("TRILHA_MAIL_URL") == "" {
			add("warn", t("mail unset"), t("mail unset hint"))
		} else {
			add("ok", t("mail ok"), "")
		}
	}
	// A string that comes from outside with no size limit is the column the
	// database refuses in production, with the driver's message instead of the
	// field's.
	if fields := unboundedStrings(src); len(fields) > 0 {
		sort.Strings(fields)
		add("warn", fmt.Sprintf(t("string unbounded"), len(fields)),
			fmt.Sprintf(t("string unbounded hint"), strings.Join(first(fields, 5), ", ")))
	}
	if !strings.Contains(src, ".Check(") {
		add("warn", t("no checks"), t("no checks hint"))
	} else {
		add("ok", t("checks ok"), "")
	}

	// A module in the policy that no route requires (spec 063). The matrix says
	// the area exists and is protected; if nothing asks for it, the protection
	// is a sentence in a file and the area is open. It is a warning and not a
	// critical because the guard may be one commit away.
	if mods := policyModules(src); len(mods) > 0 {
		guarded := map[string]bool{}
		for _, m := range policyRequired(src) {
			guarded[m] = true
		}
		var loose []string
		for _, m := range mods {
			if !guarded[m] {
				loose = append(loose, m)
			}
		}
		sort.Strings(loose)
		if len(loose) > 0 {
			add("warn", fmt.Sprintf(t("policy loose"), strings.Join(loose, ", ")), t("policy loose hint"))
		} else {
			add("ok", t("policy ok"), "")
		}
	}

	// A layout string in a page (spec 066). time.Format("02/01/2006") is the
	// line every application writes four times and gets subtly different each
	// time — and it ignores the app's zone, so a date shown to somebody in
	// another country is simply wrong.
	if n := len(timeFormatRe.FindAllString(src, -1)); n > 0 {
		add("warn", fmt.Sprintf(t("time format"), n), t("time format hint"))
	}

	// c.Audit on a route nobody guards (spec 067). The trail would record
	// "anonymous did it", which is the one answer an audit trail exists to
	// never have to give.
	if strings.Contains(src, ".Audit(") && !strings.Contains(src, ".Require") {
		add("warn", t("audit anon"), t("audit anon hint"))
	}

	// The island runtime (spec 060) is a file of the kit, linked by Ctx.Island.
	// A project that uses an island without it renders the fallback and nothing
	// else, silently — which is the failure this check exists to name.
	if strings.Contains(src, ".Island(") {
		if _, err := os.Stat(filepath.Join(p.Root, "public", "ui.island.js")); err != nil {
			add("critical", t("island runtime missing"), t("island runtime missing hint"))
		} else {
			add("ok", t("island runtime ok"), "")
		}
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
	} else {
		// CSRF on the writes that live in a route.go (spec 055): a route.go is
		// an API by default, and an API does not check the token. In an app
		// that also serves pages, a POST route with no Kind above it and no
		// CSRFForAPI accepts a form posted from another site — and it does so
		// in silence, which is why it is worth saying out loud here.
		if open := openWrites(p, res); len(open) > 0 && !strings.Contains(src, "CSRFForAPI") {
			add("warn", fmt.Sprintf(t("csrf open writes"), len(open)),
				fmt.Sprintf(t("csrf open writes hint"), strings.Join(open, ", ")))
		}
		// Events with nothing above them: everyone who connects receives
		// everything the route sends.
		if open := openStreams(p, res); len(open) > 0 {
			add("warn", fmt.Sprintf(t("stream open"), len(open)),
				fmt.Sprintf(t("stream open hint"), strings.Join(open, ", ")))
		}
		// A trail that names nobody is an expensive log file.
		if anon := anonymousAudit(p, res); len(anon) > 0 {
			add("warn", fmt.Sprintf(t("audit anonymous"), len(anon)),
				fmt.Sprintf(t("audit anonymous hint"), strings.Join(anon, ", ")))
		}
		if out, err := gen.Generate(res); err == nil {
			cur, _ := os.ReadFile(filepath.Join(p.Root, gen.FileName))
			if string(cur) != string(out) {
				add("warn", t("gen stale"), t("gen stale hint"))
			} else {
				add("ok", t("gen fresh"), "")
			}
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

	// Upstreams (spec 056). A proxy carries the session's credential to
	// another service: in the clear it is a token on the wire, and with no
	// Headers at all it is probably a credential somebody forgot.
	if strings.Contains(src, "Upstreams") {
		if plain := plainTargets(src); len(plain) > 0 {
			add("critical", t("upstream plaintext"), fmt.Sprintf(t("upstream plaintext hint"), strings.Join(plain, ", ")))
		}
		if upstreamWithoutCredential(src) {
			add("warn", t("upstream no credential"), t("upstream no credential hint"))
		}
		// No Timeout is thirty seconds by default, and thirty seconds per
		// pending request is what takes the whole app down when the API on the
		// other side gets slow — the failure arrives as "our app is down",
		// which sends everybody looking in the wrong place.
		if !strings.Contains(src, "Timeout:") {
			add("warn", t("upstream no timeout"), t("upstream no timeout hint"))
		}
	}

	if liveWithoutAuth(src) {
		add("warn", t("live no auth"), t("live no auth hint"))
	}

	// A hand-written <iframe> (spec 070). The trap is not where it looks: the
	// default hardening sends X-Frame-Options: DENY and frame-ancestors 'none'
	// on every answer, so the framed document refuses, the frame is blank and
	// the console blames a policy the developer did not write.
	if handWrittenIframe(src) {
		add("warn", t("iframe by hand"), t("iframe by hand hint"))
	}

	// A sealed value is only readable with the key that sealed it (spec 075).
	// Rotating without keeping the previous key is not a warning about a
	// theoretical risk: it is the moment every stored token stops opening.
	if strings.Contains(src, "trilha.Secret") && os.Getenv("TRILHA_PREVIOUS_SECRET") == "" {
		add("warn", t("secret no previous"), t("secret no previous hint"))
	}

	// Multi-tenant by column (spec 078). Forgetting the column in one query
	// out of forty is invisible in review and very visible to counting.
	if strings.Contains(src, "auth.Tenant(") {
		if gaps := tenantGaps(p.Root); len(gaps) > 0 {
			add("warn", t("tenant gaps"), t("tenant gaps hint")+"\n"+strings.Join(gaps, "\n"))
		} else {
			add("ok", t("tenant gaps"), "")
		}
	}

	// Vendored JavaScript (spec 066). A file under public/vendor that
	// vendor.lock does not name is third-party code the repository accepted
	// without recording where it came from: nobody can tell a version bump
	// from a tampered file (NIST SP 800-161 supply chain; OWASP A08).
	for _, f := range unpinnedVendor(p) {
		add("warn", fmt.Sprintf(t("vendor unpinned"), f), t("vendor unpinned hint"))
	}

	// Login without a rate limit is a password guessing machine with the
	// app's own uptime (OWASP ASVS 2.2.1).
	if loginWithoutLimit(src, os.Getenv("TRILHA_RATE_LIMIT") != "") {
		add("warn", t("login no limit"), t("login no limit hint"))
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

// handWrittenIframe reports an <iframe> drawn by the app itself, with no
// ui.Preview anywhere. Preview exists because the framed answer has to allow
// being framed, and a page cannot fix that from outside.
func handWrittenIframe(src string) bool {
	return (strings.Contains(src, "h.Iframe(") || strings.Contains(src, "<iframe")) &&
		!strings.Contains(src, "ui.Preview(")
}

// unpinnedVendor lists the modules in public/vendor that vendor.lock does not
// account for. A project that vendored nothing has no directory and no finding.
func unpinnedVendor(p *project) []string {
	files, err := filepath.Glob(filepath.Join(p.Root, vendorDir, "*.js"))
	if err != nil || len(files) == 0 {
		return nil
	}
	locked, err := readLock(p)
	if err != nil {
		return nil
	}
	known := map[string]bool{}
	for _, l := range locked {
		known[l.File] = true
	}
	var out []string
	for _, f := range files {
		rel := filepath.ToSlash(filepath.Join(vendorDir, filepath.Base(f)))
		if !known[rel] {
			out = append(out, rel)
		}
	}
	sort.Strings(out)
	return out
}

// openWrites lists the route.go routes that take a body method with no Kind
// deciding them, in an app that also serves pages. A page.go route enforces
// CSRF; the same form action moved into a route.go does not, and the two look
// identical from the outside.
// guardsCSRF says the route puts trilha.RequireCSRF in front of this method,
// through its own middleware.go or one above it.
func guardsCSRF(p *project, r scan.Route, method string) bool {
	chain := append(append([]scan.Ref{}, r.Middlewares...), r.MiddlewaresByMethod[method]...)
	for _, ref := range chain {
		if strings.Contains(middlewareSource(p, ref), "RequireCSRF") {
			return true
		}
	}
	return false
}

// middlewareSource reads the file the middleware came from. The audit already
// reads the project's source; this narrows it to the one package that decides.
func middlewareSource(p *project, ref scan.Ref) string {
	dir := strings.TrimPrefix(ref.ImportPath, p.Module+"/")
	data, err := os.ReadFile(filepath.Join(p.Root, filepath.FromSlash(dir), "middleware.go"))
	if err != nil {
		return ""
	}
	return string(data)
}

func openWrites(p *project, res *scan.Result) []string {
	pages := false
	for _, r := range res.Routes {
		if r.Kind == "page" {
			pages = true
			break
		}
	}
	if !pages {
		return nil
	}
	var out []string
	for _, r := range res.Routes {
		if r.Kind != "api" || r.KindRef != nil {
			continue
		}
		for _, m := range r.Methods {
			if m != "POST" && m != "PUT" && m != "PATCH" && m != "DELETE" {
				continue
			}
			// A route that asks for the token with trilha.RequireCSRF is
			// answering the page, and it already said so.
			if guardsCSRF(p, r, m) {
				continue
			}
			out = append(out, r.Pattern)
			break
		}
	}
	return out
}

// plainTargets lists the upstream targets written as http:// to a host that is
// not this machine. localhost is the dev loop; anything else is the session's
// credential crossing a network in the clear.
var targetRe = regexp.MustCompile(`Target:\s*"(http://[^"]*)"`)

func plainTargets(src string) []string {
	var out []string
	for _, m := range targetRe.FindAllStringSubmatch(src, -1) {
		host := strings.SplitN(strings.TrimPrefix(m[1], "http://"), "/", 2)[0]
		host = strings.SplitN(host, ":", 2)[0]
		switch host {
		case "localhost", "127.0.0.1", "[::1]", "::1":
			continue
		}
		out = append(out, m[1])
	}
	return out
}

// upstreamWithoutCredential reports an upstream that injects nothing in an app
// that has a login. The proxy does not forward the browser's Authorization, so
// with no Headers the API is called anonymously — which is either a forgotten
// credential or a decision worth writing down.
func upstreamWithoutCredential(src string) bool {
	if !strings.Contains(src, "Upstreams") || strings.Contains(src, "Headers:") {
		return false
	}
	return strings.Contains(src, ".Require()") || strings.Contains(src, ".RequireRole(") || strings.Contains(src, ".RequireFunc(")
}

// liveWithoutAuth reports a stream nobody guards. ui.Live opens an EventSource
// that stays open for as long as the page does, and a route answering it
// without a session is a connection anyone can hold and read.
func liveWithoutAuth(src string) bool {
	if !strings.Contains(src, "ui.Live(") {
		return false
	}
	for _, s := range []string{".Require()", ".RequireRole(", ".RequireFunc("} {
		if strings.Contains(src, s) {
			return false
		}
	}
	return true
}

// loginWithoutLimit reports a login nobody limits (OWASP ASVS 2.2.1).
func loginWithoutLimit(src string, envLimit bool) bool {
	return strings.Contains(src, ".Login(") && !strings.Contains(src, "RateLimit") && !envLimit
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

// metricsConfigured reports whether the source opens the metrics endpoint.
// Only Config.Observability.Metrics does that — cache.Options has a field of
// the same name that just picks the registry the counters go to, and matching
// it made the reference app report an endpoint it never served. So the search
// is anchored on the Observability field: the assignment
// (Observability.Metrics = ...) and the literal (Observability{... Metrics: ...},
// with the type written or elided).
func metricsConfigured(src string) bool {
	const field = "Observability"
	for i := 0; ; {
		j := strings.Index(src[i:], field)
		if j < 0 {
			return false
		}
		i += j + len(field)
		rest := src[i:]
		if strings.HasPrefix(rest, ".Metrics") {
			return true
		}
		if strings.HasPrefix(rest, ":") { // Config{Observability: {...}}
			rest = strings.TrimLeft(rest[1:], " \t\r\n")
		}
		if strings.HasPrefix(rest, "{") && strings.Contains(literalBody(rest), "Metrics:") {
			return true
		}
	}
}

// literalBody returns what sits between the brace that opens s and its match,
// so the search stops at the end of the literal instead of running into the
// rest of the file.
func literalBody(s string) string {
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			if depth--; depth == 0 {
				return s[1:i]
			}
		}
	}
	return s
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

// policyModules reads the module names out of an auth.Policy declaration. It is
// a regular expression and not a parse because the audit reads the project the
// way a reviewer does: the declaration is one literal, and a project that hides
// it behind a function has moved past what this check can promise.
func policyModules(src string) []string {
	m := policyModulesRe.FindStringSubmatch(src)
	if m == nil {
		return nil
	}
	var out []string
	for _, q := range quotedRe.FindAllStringSubmatch(m[1], -1) {
		out = append(out, q[1])
	}
	return out
}

// policyRequired is the modules some route actually asks for.
func policyRequired(src string) []string {
	var out []string
	for _, m := range policyRequireRe.FindAllStringSubmatch(src, -1) {
		out = append(out, m[1])
	}
	return out
}

var (
	policyModulesRe = regexp.MustCompile(`(?s)Modules:\s*\[\]string\{([^}]*)\}`)
	policyRequireRe = regexp.MustCompile(`RequirePolicy\([^,]+,\s*"([^"]+)"`)
	quotedRe        = regexp.MustCompile(`"([^"]*)"`)
)

// timeFormatRe finds a call that formats a moment with a layout of its own. The
// reference layout is unmistakable — 2006, 01, 02, 15:04 — so this does not
// have to guess.
var timeFormatRe = regexp.MustCompile(`\.Format\(\s*"[^"]*(2006|15:04|Jan)`)

// routeSource reads the files of one route's folder — the page.go, the
// route.go, whatever else is there. It is how a check asks what a route
// actually does without reading the whole project again.
func routeSource(p *project, r scan.Route) string {
	dir := filepath.Join(p.Root, filepath.FromSlash(r.Dir))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var b strings.Builder
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		if data, err := os.ReadFile(filepath.Join(dir, e.Name())); err == nil {
			b.Write(data)
		}
	}
	return b.String()
}

// openStreams lists the routes that open an event stream with nothing above
// them. Everyone who connects receives everything the route sends, which is
// fine for a clock and is a leak for anything with a name in it.
//
// The rule is "no middleware above", and not "no authentication above": the
// audit sees that there is a chain, not what the chain does, and saying
// otherwise would be guessing.
func openStreams(p *project, res *scan.Result) []string {
	var out []string
	for _, r := range res.Routes {
		if len(r.Middlewares) > 0 || len(r.MiddlewaresByMethod) > 0 {
			continue
		}
		if strings.Contains(routeSource(p, r), "c.Stream()") {
			out = append(out, r.Pattern)
		}
	}
	return out
}

// anonymousAudit lists the routes that write to the trail with nothing above
// them. The record exists and does not say who: a trail that names nobody is
// an expensive log file.
func anonymousAudit(p *project, res *scan.Result) []string {
	var out []string
	for _, r := range res.Routes {
		if len(r.Middlewares) > 0 || len(r.MiddlewaresByMethod) > 0 {
			continue
		}
		src := routeSource(p, r)
		// A route that names the actor itself has answered this: an invitation
		// is accepted by somebody with no session and a name — the link says
		// whose it is — and c.SetActor is how the trail learns it.
		if strings.Contains(src, "c.Audit(") && !strings.Contains(src, "c.SetActor(") {
			out = append(out, r.Pattern)
		}
	}
	return out
}

// fieldTag matches an exported string field with a struct tag: the shape a
// form or a JSON body arrives in.
var fieldTag = regexp.MustCompile("(?m)^\\s*([A-Z]\\w*)\\s+string\\s+`([^`]*)`")

// unboundedStrings lists the string fields that come from outside with no
// size limit. The request body has a ceiling, so this is not a way to run the
// process out of memory — it is the column the database refuses in production,
// with the driver's message instead of the field's.
//
// A field with oneof= or len= is already bounded, and a field with no form: or
// json: tag does not come from outside.
func unboundedStrings(src string) []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range fieldTag.FindAllStringSubmatch(src, -1) {
		tag := m[2]
		if !strings.Contains(tag, "form:") && !strings.Contains(tag, "json:") {
			continue
		}
		i := strings.Index(tag, `validate:"`)
		if i < 0 {
			continue // no validation at all is a different conversation
		}
		rest := tag[i+len(`validate:"`):]
		if j := strings.IndexByte(rest, '"'); j >= 0 {
			rest = rest[:j]
		}
		if strings.Contains(rest, "max=") || strings.Contains(rest, "oneof=") || strings.Contains(rest, "len=") {
			continue
		}
		// The same field name in two structs is one thing to fix in the
		// reader's head, and two lines of noise in the message.
		if seen[m[1]] {
			continue
		}
		seen[m[1]] = true
		out = append(out, m[1])
	}
	return out
}

// first is the head of a list, for a message that names examples instead of
// everything: a warning nobody can read is a warning nobody acts on.
func first(all []string, n int) []string {
	if len(all) <= n {
		return all
	}
	return append(append([]string{}, all[:n]...), "…")
}
