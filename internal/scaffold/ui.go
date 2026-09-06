package scaffold

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/emersonjoe/trilha/ui"
)

// UIResult reports what WriteUI did with one file.
type UIResult struct {
	File   string
	Action string // one of the UI* constants
}

// Actions reported by WriteUI.
const (
	UICreated   = "created"
	UIUpdated   = "updated"
	UIKept      = "kept"
	UIKeptTheme = "kept (your theme)"
	UIModified  = "modified locally"
)

// ErrUIModified is returned when ui.css/ui.js were edited locally and force is false.
var ErrUIModified = errors.New("ui kit files were modified locally; use --force to overwrite")

const stampPrefix = "/* trilha ui "

// stamp prepends a header with the hash of the body, so a later WriteUI can
// tell an untouched older copy (hash matches) from a locally edited one.
func stamp(body []byte) []byte {
	return append([]byte(stampPrefix+digest(body)+" */\n"), body...)
}

func digest(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:8])
}

// untouched reports whether content is a stamped kit file whose body still
// matches its own stamp.
func untouched(content []byte) bool {
	if !bytes.HasPrefix(content, []byte(stampPrefix)) {
		return false
	}
	nl := bytes.IndexByte(content, '\n')
	if nl < 0 {
		return false
	}
	head := string(content[len(stampPrefix):nl])
	if len(head) < 16+3 {
		return false
	}
	return head[:16] == digest(content[nl+1:])
}

// WriteUI writes the kit into dir/public. ui.theme.css is only ever created
// (it belongs to the project); ui.css and ui.js are refreshed when untouched
// since the last write, and only with force when they were edited locally.
func WriteUI(dir string, force, cssOnly, jsOnly bool) ([]UIResult, error) {
	var out []UIResult
	var modified bool
	for _, name := range ui.Files {
		if isJS := strings.HasSuffix(name, ".js"); (cssOnly && isJS) || (jsOnly && !isJS) {
			continue
		}
		dst := filepath.Join(dir, "public", name)
		want := ui.Asset(name)
		if name != "ui.theme.css" {
			want = stamp(want)
		}
		cur, err := os.ReadFile(dst)
		switch {
		case err != nil: // missing
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return out, err
			}
			if err := os.WriteFile(dst, want, 0o644); err != nil {
				return out, err
			}
			out = append(out, UIResult{name, UICreated})
		case bytes.Equal(cur, want):
			out = append(out, UIResult{name, UIKept})
		case name == "ui.theme.css":
			out = append(out, UIResult{name, UIKeptTheme})
		case force || untouched(cur):
			if err := os.WriteFile(dst, want, 0o644); err != nil {
				return out, err
			}
			out = append(out, UIResult{name, UIUpdated})
		default:
			modified = true
			out = append(out, UIResult{name, UIModified})
		}
	}
	if modified {
		return out, ErrUIModified
	}
	return out, nil
}
