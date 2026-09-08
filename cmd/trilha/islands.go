package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/emersonjoe/trilha/internal/islands"
)

// islandTypes reads the c.Island calls of the project and returns the
// declaration file they imply, or nil when the app mounts no island: a project
// without islands has nothing to declare and gets no file.
func islandTypes(p *project) (*islands.Result, []byte, error) {
	res, err := islands.Scan(filepath.Join(p.Root, "app"))
	if err != nil {
		return nil, nil, err
	}
	if len(res.Islands) == 0 {
		return res, nil, nil
	}
	return res, []byte(res.TypeScript()), nil
}

// writeIslandTypes keeps public/islands.d.ts next to the modules it describes.
// The file belongs to the generator, so an app that removed its last island
// does not keep a declaration of islands that no longer exist.
func writeIslandTypes(p *project) (*islands.Result, bool, error) {
	res, src, err := islandTypes(p)
	if err != nil {
		return nil, false, err
	}
	out := filepath.Join(p.Root, islands.FileName)
	old, readErr := os.ReadFile(out)
	if src == nil {
		if readErr == nil && ours(old) {
			return res, true, os.Remove(out)
		}
		return res, false, nil
	}
	if readErr == nil && string(old) == string(src) {
		return res, false, nil
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return nil, false, err
	}
	return res, true, os.WriteFile(out, src, 0o644)
}

// checkIslandTypes is the same comparison without writing, for the CI line.
func checkIslandTypes(p *project) error {
	_, src, err := islandTypes(p)
	if err != nil {
		return err
	}
	out := filepath.Join(p.Root, islands.FileName)
	old, readErr := os.ReadFile(out)
	switch {
	case src == nil && (readErr != nil || !ours(old)):
		return nil
	case src != nil && readErr == nil && string(old) == string(src):
		return nil
	}
	fmt.Fprintln(os.Stderr, t("islands stale hint"))
	return fmt.Errorf(t("islands stale"), islands.FileName)
}

// ours says the file on disk is one the generator wrote, so removing it is not
// throwing away something the app hand-wrote.
func ours(src []byte) bool {
	return len(src) >= len(islands.Header) && string(src[:len(islands.Header)]) == islands.Header
}

// islandNotes says what the reader could not type, once, on stderr: an island
// still mounts without a type, and a warning is not a failure.
func islandNotes(res *islands.Result) {
	if res == nil {
		return
	}
	for _, n := range res.Notes {
		fmt.Fprintf(os.Stderr, "! %s\n", n)
	}
}
