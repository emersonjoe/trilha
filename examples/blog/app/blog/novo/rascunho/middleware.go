package rascunho

import "github.com/emersonjoe/trilha"

// MiddlewarePOST guards the draft with the same token the form carries. The
// route is an API — it answers JSON, and its errors are problem+json, which is
// what the island reads — but its client is the page, with the page's cookies,
// so the exemption APIs get does not apply here.
func MiddlewarePOST(c *trilha.Ctx, next trilha.Next) error {
	return trilha.RequireCSRF(c, next)
}
