package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/emersonjoe/trilha/ai"
	"github.com/emersonjoe/trilha/ai/mcp"
	"github.com/emersonjoe/trilha/internal/uidoc"
)

// The security posture of this server, because it is the part that is easy to
// get wrong and impossible to see from the outside:
//
//   - It is read-only unless `--write` is passed. The writing tool is not
//     refused at call time, it is never registered — a model cannot ask for a
//     tool it was not offered.
//   - No shell, ever. Every command runs as a program plus an argument slice,
//     and the program is this same binary, resolved once at startup.
//   - Every argument is checked against an allowlist before it reaches a
//     command line. What does not match is refused with the reason, not
//     quietly dropped.
//   - The working directory is the project this was started in, decided once.
//     No argument can move it.
//   - One command at a time, each with a deadline and a cap on how much output
//     it may produce, so a tool call cannot become a fork bomb or eat memory.
//   - Every call is written to stderr before it runs. stdout belongs to the
//     protocol; anything else printed there would corrupt the stream.
//   - Nothing here opens a network connection.
const mcpSecurityNote = "read-only unless --write; no shell; arguments allow-listed; one command at a time, with a deadline"

// mcpTimeout is per call. `check` runs the test suite, so it gets the long one.
const (
	mcpTimeout      = 60 * time.Second
	mcpCheckTimeout = 10 * time.Minute
	mcpMaxOutput    = 1 << 20 // 1 MiB, then truncated with a marker
)

var (
	// A component of the ui kit: a Go identifier, nothing else.
	reComponent = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]{0,40}$`)
	// A route address: what `trilha generate` accepts, minus anything that
	// could climb out of the project. The braces are there because a pattern
	// has parameters (/blog/{slug}); everything a shell would find
	// interesting is not on this list, and the list is what decides.
	rePath = regexp.MustCompile(`^/[A-Za-z0-9._~{}/-]{0,200}$`)
	// A Go type, possibly qualified by its package.
	reType = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,40}(\.[A-Za-z_][A-Za-z0-9_]{0,40})?$`)

	mcpMethods = map[string]bool{
		"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true, "OPTIONS": true,
	}
	mcpKinds = map[string]bool{"page": true, "route": true, "test": true, "component": true}
)

func cmdMCP(args []string) error {
	fs := flag.NewFlagSet("mcp", flag.ContinueOnError)
	write := fs.Bool("write", false, t("flag mcp write"))
	if err := fs.Parse(args); err != nil {
		return err
	}
	p, err := findProject()
	if err != nil {
		return err
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	r := &mcpRunner{root: p.Root, self: self}

	tools := []*ai.Tool{r.describeProject(), r.check(), r.routes(), r.uiDescribe()}
	if *write {
		tools = append(tools, r.generate())
	}

	names := make([]string, 0, len(tools))
	for _, t := range tools {
		names = append(names, t.Name)
	}
	// stderr, never stdout: stdout is the protocol.
	fmt.Fprintf(os.Stderr, "trilha mcp %s · %s · %s\n", version, p.Root, mcpSecurityNote)
	fmt.Fprintf(os.Stderr, "tools: %s\n", strings.Join(names, ", "))
	if !*write {
		fmt.Fprintf(os.Stderr, "%s\n", t("mcp read only"))
	}

	s := mcp.NewServer("trilha", version, tools...)
	return s.ServeStdio(context.Background(), os.Stdin, os.Stdout)
}

// mcpRunner runs this same CLI as a child process. Running the commands in
// process would be faster and wrong: they print to stdout, which here belongs
// to the protocol — and a separate process is also what makes the deadline and
// the output cap enforceable.
type mcpRunner struct {
	root string
	self string
	mu   sync.Mutex // one command at a time
}

func (r *mcpRunner) run(ctx context.Context, timeout time.Duration, args ...string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	fmt.Fprintf(os.Stderr, "→ trilha %s\n", strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, r.self, args...)
	cmd.Dir = r.root
	cmd.Stdin = nil
	out, err := cmd.CombinedOutput()
	if len(out) > mcpMaxOutput {
		out = append(out[:mcpMaxOutput], []byte("\n… truncated at 1 MiB")...)
	}
	if ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf(t("mcp timeout"), timeout)
	}
	if err != nil {
		// A command that fails is an answer, not a crash: `check` failing is
		// exactly what the caller wanted to know.
		return string(out), nil
	}
	return string(out), nil
}

func (r *mcpRunner) describeProject() *ai.Tool {
	return ai.NewTool("describe_project",
		"The map of this Trilha project as JSON: routes, API, types and setup. Same as `trilha ctx --json`. Read this before writing code that touches routes.",
		nil,
		func(ctx context.Context, _ json.RawMessage) (string, error) {
			return r.run(ctx, mcpTimeout, "ctx", "--json")
		})
}

func (r *mcpRunner) check() *ai.Tool {
	return ai.NewTool("check",
		"Run the project's gate as JSON: generate, gofmt, vet, tests, security audit and the OpenAPI check, stopping at the first failure. Same as `trilha check --json`. Slow — it runs the test suite.",
		nil,
		func(ctx context.Context, _ json.RawMessage) (string, error) {
			return r.run(ctx, mcpCheckTimeout, "check", "--json")
		})
}

func (r *mcpRunner) routes() *ai.Tool {
	return ai.NewTool("routes",
		"The route table in precedence order: methods, pattern and the file each one comes from. Same as `trilha routes`.",
		nil,
		func(ctx context.Context, _ json.RawMessage) (string, error) {
			return r.run(ctx, mcpTimeout, "routes")
		})
}

func (r *mcpRunner) uiDescribe() *ai.Tool {
	return ai.NewTool("ui_describe",
		"The ui kit catalogue: signature, what it is for and an example that compiles. With no name, one line per component. Same as `trilha ui describe`.",
		ai.Schema(`{"type":"object","properties":{"component":{"type":"string","description":"Component name, e.g. Field. Omit for the whole list."}}}`),
		func(ctx context.Context, raw json.RawMessage) (string, error) {
			var in struct {
				Component string `json:"component"`
			}
			if err := json.Unmarshal(nonEmpty(raw), &in); err != nil {
				return "", err
			}
			// This one answers from the catalogue the CLI carries, so it needs
			// no process and no project.
			if in.Component == "" {
				return string(uidoc.JSON()), nil
			}
			if !reComponent.MatchString(in.Component) {
				return "", fmt.Errorf(t("mcp bad arg"), "component", in.Component)
			}
			c, ok := uidoc.Lookup(in.Component)
			if !ok {
				if near := uidoc.Similar(in.Component); len(near) > 0 {
					return "", fmt.Errorf("%s: %s", in.Component, strings.Join(near, ", "))
				}
				return "", errors.New(in.Component + ": no such component")
			}
			b, err := json.MarshalIndent(c, "", "  ")
			return string(b), err
		})
}

// generate is the only tool that writes, and it is registered only when the
// person starting the server passed --write.
func (r *mcpRunner) generate() *ai.Tool {
	return ai.NewTool("generate",
		"Write the skeleton of a page, an API route, a test or a component, in the folder the convention asks for. Writes files. Same as `trilha generate`.",
		ai.Schema(`{"type":"object","required":["kind","target"],"properties":{
			"kind":{"type":"string","enum":["page","route","test","component"]},
			"target":{"type":"string","description":"URL path for page/route/test (/blog/{slug}); component name for component."},
			"methods":{"type":"array","items":{"type":"string","enum":["GET","POST","PUT","PATCH","DELETE","OPTIONS"]}},
			"bind":{"type":"string","description":"Type the methods with a body bind."},
			"form":{"type":"string","description":"Type whose form round trip to write."}}}`),
		func(ctx context.Context, raw json.RawMessage) (string, error) {
			var in struct {
				Kind    string   `json:"kind"`
				Target  string   `json:"target"`
				Methods []string `json:"methods"`
				Bind    string   `json:"bind"`
				Form    string   `json:"form"`
			}
			if err := json.Unmarshal(nonEmpty(raw), &in); err != nil {
				return "", err
			}
			args, err := generateArgs(in.Kind, in.Target, in.Methods, in.Bind, in.Form)
			if err != nil {
				return "", err
			}
			return r.run(ctx, mcpTimeout, args...)
		})
}

// generateArgs turns what the model asked for into an argument slice, or says
// why it will not. It is a function of its own so the refusals can be tested
// without a server, a project or a process.
func generateArgs(kind, target string, methods []string, bind, form string) ([]string, error) {
	if !mcpKinds[kind] {
		return nil, fmt.Errorf(t("mcp bad arg"), "kind", kind)
	}
	switch kind {
	case "component":
		if !reComponent.MatchString(target) {
			return nil, fmt.Errorf(t("mcp bad arg"), "target", target)
		}
	default:
		// A path that climbs, or that carries anything a shell would find
		// interesting, is refused here and never reaches a command line.
		if !rePath.MatchString(target) || strings.Contains(target, "..") {
			return nil, fmt.Errorf(t("mcp bad arg"), "target", target)
		}
	}
	args := []string{"generate", kind, target}
	if len(methods) > 0 {
		for _, m := range methods {
			if !mcpMethods[strings.ToUpper(m)] {
				return nil, fmt.Errorf(t("mcp bad arg"), "methods", m)
			}
		}
		args = append(args, "--methods", strings.ToUpper(strings.Join(methods, ",")))
	}
	for flagName, v := range map[string]string{"--bind": bind, "--form": form} {
		if v == "" {
			continue
		}
		if !reType.MatchString(v) {
			return nil, fmt.Errorf(t("mcp bad arg"), strings.TrimPrefix(flagName, "--"), v)
		}
		args = append(args, flagName, v)
	}
	return args, nil
}

// nonEmpty lets a tool with no arguments be called with nothing at all.
func nonEmpty(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage("{}")
	}
	return raw
}
