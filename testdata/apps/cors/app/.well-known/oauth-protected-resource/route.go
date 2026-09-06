// The discovery document of RFC 9728: fetched from another origin by a client
// that has no token yet, while every other route of this app is same-origin.
package oauthresource

import "github.com/emersonjoe/trilha"

var CORS = trilha.CORS{Origins: []string{"*"}, Methods: []string{"GET"}}

func GET(c *trilha.Ctx) error { return c.JSON(200, map[string]string{"resource": "x"}) }
