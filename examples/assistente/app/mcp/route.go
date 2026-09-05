// Package mcp exposes the example's tools to MCP hosts (Streamable HTTP).
package mcp

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/assistente/internal/ferramentas"
)

// POST handles MCP JSON-RPC messages.
func POST(c *trilha.Ctx) error { return ferramentas.MCP.ServeHTTP(c) }
