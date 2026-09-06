// RFC 8414 puts the authorization server metadata at a fixed /.well-known/
// path; the package name cannot be the folder name, so it is declared here.
package wellknown

import "github.com/emersonjoe/trilha"

// Metadata is the authorization server metadata of RFC 8414.
type Metadata struct {
	Issuer   string `json:"issuer" validate:"required,uri"`
	AuthzURL string `json:"authorization_endpoint" validate:"required,uri"`
	TokenURL string `json:"token_endpoint" validate:"required,uri"`
}

// GET answers the metadata a client reads before starting the flow.
func GET(c *trilha.Ctx) error {
	return c.JSON(200, Metadata{Issuer: "https://example.com"})
}
