// Package cookbooksrc embeds the Go sources of the cookbook recipes, so the
// site's MCP server can answer search_code with path:line windows into the
// code a recipe teaches. The copies are commit-tested against
// examples/cookbook: a source that drifts fails the site build, and the
// failure says to copy the files again.
package cookbooksrc

import "embed"

//go:embed sources
var files embed.FS

// Names lists the source files, sorted, as "sources/x.go".
func Names() ([]string, error) {
	entries, err := files.ReadDir("sources")
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			out = append(out, "sources/"+e.Name())
		}
	}
	return out, nil
}

// Read returns one file's content.
func Read(name string) ([]byte, error) {
	return files.ReadFile(name)
}
