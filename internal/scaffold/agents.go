package scaffold

import (
	"bytes"
	"embed"
	"errors"
	"os"
	"path/filepath"
	"text/template"
)

//go:embed agents
var agentsFS embed.FS

// ErrAgentsModified is returned when AGENTS.md was edited locally and force is false.
var ErrAgentsModified = errors.New("AGENTS.md was modified locally; use --force to overwrite")

// WriteAgents writes AGENTS.md and CLAUDE.md at the root of dir.
//
// AGENTS.md belongs to the framework: it carries a stamp with the hash of its
// own body, so an untouched older copy is refreshed in silence and a locally
// edited one is only overwritten with force. CLAUDE.md belongs to the project
// from the first line it gains, so it is only ever created.
func WriteAgents(dir string, d Data, force bool) ([]UIResult, error) {
	if d.Lang == "" {
		d.Lang = "en"
	}
	if d.T = texts[d.Lang]; d.T == nil {
		return nil, errors.New("scaffold: unknown language " + d.Lang)
	}
	want, err := agentsFile("agents/AGENTS."+d.Lang+".md", d)
	if err != nil {
		return nil, err
	}
	want = mdStamp.apply(want)
	res, err := writeStamped(filepath.Join(dir, "AGENTS.md"), want, force)
	out := []UIResult{{"AGENTS.md", res}}
	if err != nil {
		return out, err
	}

	claude, err := agentsFile("agents/CLAUDE."+d.Lang+".md", d)
	if err != nil {
		return out, err
	}
	dst := filepath.Join(dir, "CLAUDE.md")
	if _, err := os.Stat(dst); err == nil {
		return append(out, UIResult{"CLAUDE.md", UIKeptOwn}), nil
	}
	if err := os.WriteFile(dst, claude, 0o644); err != nil {
		return out, err
	}
	return append(out, UIResult{"CLAUDE.md", UICreated}), nil
}

// writeStamped applies the ui kit's rule to one stamped file: create it, keep
// it, refresh it when it is an untouched older copy, or refuse it.
func writeStamped(dst string, want []byte, force bool) (string, error) {
	cur, err := os.ReadFile(dst)
	switch {
	case err != nil:
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return "", err
		}
		return UICreated, os.WriteFile(dst, want, 0o644)
	case bytes.Equal(cur, want):
		return UIKept, nil
	case force || mdStamp.untouched(cur):
		return UIUpdated, os.WriteFile(dst, want, 0o644)
	default:
		return UIModified, ErrAgentsModified
	}
}

func agentsFile(name string, d Data) ([]byte, error) {
	src, err := agentsFS.ReadFile(name)
	if err != nil {
		return nil, err
	}
	t, err := template.New(filepath.Base(name)).Parse(string(src))
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, d); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
