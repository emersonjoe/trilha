// Package mcp implements the Model Context Protocol (JSON-RPC 2.0 over stdio
// or Streamable HTTP) as a client, to bring external tools into an ai.Agent,
// and as a server, to expose your app's tools to MCP hosts.
package mcp

import "encoding/json"

// ProtocolVersion is the MCP revision implemented.
const ProtocolVersion = "2025-03-26"

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (e *rpcError) Error() string { return e.Message }

const (
	codeParse          = -32700
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
	codeInternal       = -32603
)

// ToolInfo is a tool as listed by a server.
type ToolInfo struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// Content is one item of a tool result.
type Content struct {
	Type     string `json:"type"` // text | image | resource
	Text     string `json:"text,omitempty"`
	Data     string `json:"data,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
}

// CallResult is the result of tools/call.
type CallResult struct {
	Content []Content `json:"content"`
	IsError bool      `json:"isError,omitempty"`
}

// Text joins the text items with newlines.
func (r CallResult) Text() string {
	var s string
	for _, c := range r.Content {
		if c.Type == "text" {
			if s != "" {
				s += "\n"
			}
			s += c.Text
		}
	}
	return s
}

type initializeParams struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    map[string]any `json:"capabilities"`
	ClientInfo      info           `json:"clientInfo"`
}

type initializeResult struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    map[string]any `json:"capabilities"`
	ServerInfo      info           `json:"serverInfo"`
}

type info struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}
