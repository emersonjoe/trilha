// Package mcp serves the /api of this blog as MCP tools, so an agent that
// speaks the protocol and nothing else can list and create posts.
package mcp

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/ai/mcp"
)

// POST is the Streamable HTTP endpoint. The server is built once in
// app/setup.go from the routes of this app and the OpenAPI document next to
// this file; here it only answers.
func POST(c *trilha.Ctx) error {
	return trilha.Use[*mcp.Server](c).ServeHTTP(c)
}
