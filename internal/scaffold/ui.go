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
	UIKeptOwn   = "kept (yours)"
)

// ErrUIModified is returned when ui.css/ui.js were edited locally and force is false.
var ErrUIModified = errors.New("ui kit files were modified locally; use --force to overwrite")

// stampFmt is a one-line header carrying the hash of the body that follows,
// written in the comment syntax of the file it heads. It is what lets a later
// write tell an untouched older copy (hash matches its own body) from one the
// project edited (it does not).
type stampFmt struct{ open, close string }

var (
	cssStamp = stampFmt{"/* trilha ui ", " */"}
	mdStamp  = stampFmt{"<!-- trilha agents ", " -->"}
)

func (f stampFmt) apply(body []byte) []byte {
	return append([]byte(f.open+digest(body)+f.close+"\n"), body...)
}

func (f stampFmt) untouched(content []byte) bool {
	if !bytes.HasPrefix(content, []byte(f.open)) {
		return false
	}
	nl := bytes.IndexByte(content, '\n')
	if nl < 0 {
		return false
	}
	head := string(content[len(f.open):nl])
	if len(head) < 16+len(f.close) {
		return false
	}
	return head[:16] == digest(content[nl+1:])
}

func digest(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:8])
}

// stamp and untouched keep the ui kit's own calls short.
func stamp(body []byte) []byte      { return cssStamp.apply(body) }
func untouched(content []byte) bool { return cssStamp.untouched(content) }

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
