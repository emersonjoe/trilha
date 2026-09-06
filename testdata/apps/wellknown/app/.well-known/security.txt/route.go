// /.well-known/ is the one dot-prefixed folder the scanner descends into; the
// package name cannot be ".well-known/security.txt", so it is declared
// differently and imported by alias.
package security

import "github.com/emersonjoe/trilha"

func GET(c *trilha.Ctx) error { return c.Text(200, "Contact: mailto:security@example.com\n") }
