// Package mcp serves the hosted, read-only documentation MCP endpoint.
package mcp

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/site/internal/docsmcp"
)

var server = docsmcp.New("0.134.0")

// POST speaks MCP Streamable HTTP over the documentation embedded in the site.
func POST(c *trilha.Ctx) error { return server.ServeHTTP(c) }
