// Package dev implements `trilha dev`: a polling file watcher, a builder and
// a supervisor that runs the app behind a reverse proxy with live reload.
package dev

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Snapshot maps a relative path to its modification time and size.
type Snapshot map[string]string

// ignoredDirs are never watched.
var ignoredDirs = map[string]bool{".git": true, "bin": true, ".trilha": true, "node_modules": true, "testdata": true}

// Watchable reports whether a path (relative, slash-separated) matters for
// a rebuild: Go sources, go.mod/go.sum and anything under public/.
func Watchable(rel string) bool {
	if rel == "trilha_gen.go" {
		return false
	}
	if strings.HasPrefix(rel, "public/") {
		return true
	}
	if rel == "go.mod" || rel == "go.sum" {
		return true
	}
	return strings.HasSuffix(rel, ".go")
}

// Take walks root and records every watchable file.
func Take(root string) Snapshot {
	snap := Snapshot{}
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			name := d.Name()
			if p != root && (ignoredDirs[name] || strings.HasPrefix(name, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !Watchable(rel) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		snap[rel] = info.ModTime().UTC().Format(time.RFC3339Nano) + ":" + itoa(info.Size())
		return nil
	})
	return snap
}

// Diff returns the paths that changed between a and b.
func Diff(a, b Snapshot) []string {
	var out []string
	for k, v := range b {
		if a[k] != v {
			out = append(out, k)
		}
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			out = append(out, k)
		}
	}
	return out
}

// Change is one debounced batch of file changes.
type Change struct {
	Paths []string
	// StaticOnly is true when only files under public/ changed and public/
	// neither appeared nor disappeared: the app can reload without a rebuild.
	StaticOnly bool
}

// HasPublic reports whether the snapshot contains files under public/.
func (s Snapshot) HasPublic() bool {
	for k := range s {
		if strings.HasPrefix(k, "public/") {
			return true
		}
	}
	return false
}

// Classify decides whether a batch needs a rebuild. hadPublic/hasPublic are
// the public/ presence before and after the batch.
func Classify(paths []string, hadPublic, hasPublic bool) Change {
	c := Change{Paths: paths, StaticOnly: hadPublic && hasPublic}
	for _, p := range paths {
		if !strings.HasPrefix(p, "public/") {
			c.StaticOnly = false
			break
		}
	}
	return c
}

// Watch polls root every interval and sends batches of changes on the
// returned channel, debounced so bursts of saves become one event.
func Watch(root string, interval, debounce time.Duration, stop <-chan struct{}) <-chan Change {
	ch := make(chan Change, 1)
	go func() {
		defer close(ch)
		last := Take(root)
		var pending []string
		var since time.Time
		hadPublic := last.HasPublic()
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				cur := Take(root)
				if d := Diff(last, cur); len(d) > 0 {
					if len(pending) == 0 {
						hadPublic = last.HasPublic()
					}
					pending = append(pending, d...)
					since = time.Now()
					last = cur
				}
				if len(pending) > 0 && time.Since(since) >= debounce {
					ch <- Classify(pending, hadPublic, last.HasPublic())
					pending = nil
				}
			}
		}
	}()
	return ch
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

var _ = os.Getenv
