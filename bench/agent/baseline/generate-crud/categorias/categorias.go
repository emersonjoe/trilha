// Package categorias holds the register's type. It is memory here, which is
// the honest starting point: the rows last as long as the process.
package categorias

// Categoria is one row. The comment documents the rule for whoever renders
// the form and whoever handles the POST; in this app the check is written by
// hand, there is no tag language.
type Categoria struct {
	ID   string
	Nome string // required, at most 80 characters
}
