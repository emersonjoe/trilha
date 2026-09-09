package scaffold

import (
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strings"
)

// wireSetup puts the store where the pages find it.
//
// Without this the CRUD compiles and answers 500 on the first request, which
// is the worst possible outcome for a generator: it looks like it worked. So
// this is the one file the generator will edit rather than refuse — and it
// edits exactly one line, into a function whose shape the framework defines.
//
// It answers what it did, so the command can say so: a generator that changes
// a file you did not name has to tell you which one.
func (p crudPlan) wireSetup(root string) (string, error) {
	rel := "app/setup.go"
	abs := filepath.Join(root, filepath.FromSlash(rel))
	linha := fmt.Sprintf("\ttrilha.Provide[%s.%sStore](a, %s.New%sMemory())\n",
		p.Pkg, p.Type, p.Pkg, p.Type)

	src, err := os.ReadFile(abs)
	if os.IsNotExist(err) {
		novo, err := format.Source([]byte(p.newSetup(linha)))
		if err != nil {
			return "", fmt.Errorf("%s: %w", rel, err)
		}
		if err := writeFile(root, rel, novo, false); err != nil {
			return "", err
		}
		return rel, nil
	}
	if err != nil {
		return "", err
	}
	texto := string(src)
	if strings.Contains(texto, p.Type+"Store]") {
		return "", nil // já está lá; gerar de novo não duplica
	}
	i := strings.Index(texto, "func Setup(a *trilha.App) error {")
	if i < 0 {
		// The one shape this cannot edit. Saying the line is better than
		// guessing where it goes, because a Setup somebody wrote by hand is a
		// Setup with an order that matters.
		return "", fmt.Errorf("%s has no `func Setup(a *trilha.App) error`; add this line to yours:\n\n%s",
			rel, strings.TrimRight(linha, "\n"))
	}
	fim := strings.Index(texto[i:], "{") + i + 2 // depois de "{\n"
	novo := texto[:fim] + linha + texto[fim:]
	novo = addImport(novo, p.Import)
	formatado, err := format.Source([]byte(novo))
	if err != nil {
		return "", fmt.Errorf("%s: %w", rel, err)
	}
	if err := os.WriteFile(abs, formatado, 0o644); err != nil {
		return "", err
	}
	return rel, nil
}

// newSetup is the file for a project that has none. It is the smallest Setup
// that does something, in the shape the app template uses.
func (p crudPlan) newSetup(linha string) string {
	return fmt.Sprintf(`// Package app is where the routes live, and setup.go is what runs before the
// first request.
package app

import (
	"github.com/emersonjoe/trilha"

	%q
)

// Setup runs once, before the first request: it puts the dependencies where
// the pages find them with trilha.Use[T](c).
func Setup(a *trilha.App) error {
%s	return nil
}
`, p.Import, linha)
}

// addImport puts the package in the import block, next to the project's own
// imports when there is a group for them, and in a group of its own when there
// is not. gofmt sorts what is inside a group; it never moves a line between
// groups, which is why the placement matters here and nowhere else.
func addImport(src, path string) string {
	if strings.Contains(src, `"`+path+`"`) {
		return src
	}
	i := strings.Index(src, "\nimport (")
	if i < 0 {
		// A single-line import is not a block, and turning it into one is the
		// only way to add to it. Missing this quietly would be worse than
		// refusing: the file would look edited and stop compiling.
		return addToSingleImport(src, path)
	}
	fim := strings.Index(src[i:], "\n)")
	if fim < 0 {
		return src
	}
	fim += i
	return src[:fim] + "\n\n\t\"" + path + "\"" + src[fim:]
}

// addToSingleImport turns a one-line import into a block with both.
func addToSingleImport(src, path string) string {
	i := strings.Index(src, "\nimport \"")
	if i < 0 {
		return src
	}
	fim := strings.Index(src[i+1:], "\n")
	if fim < 0 {
		return src
	}
	fim += i + 1
	velho := strings.TrimPrefix(strings.TrimSpace(src[i+1:fim]), "import ")
	return src[:i] + "\nimport (\n\t" + velho + "\n\n\t\"" + path + "\"\n)" + src[fim:]
}
