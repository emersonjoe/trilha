package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/emersonjoe/trilha/ai"
)

// Client talks to one MCP server.
type Client struct {
	t       Transport
	seq     atomic.Int64
	mu      sync.Mutex
	pending map[string]chan response
	// Server is filled by Dial from the initialize result.
	Server struct{ Name, Version, ProtocolVersion string }
}

// Dial opens the transport and performs the initialize handshake.
func Dial(ctx context.Context, dial Dialer) (*Client, error) {
	t, err := dial(ctx)
	if err != nil {
		return nil, err
	}
	c := &Client{t: t, pending: map[string]chan response{}}
	go c.readLoop()
	var res initializeResult
	params := initializeParams{ProtocolVersion: ProtocolVersion, Capabilities: map[string]any{}, ClientInfo: info{"trilha", "0.1"}}
	if err := c.call(ctx, "initialize", params, &res); err != nil {
		_ = t.Close()
		return nil, fmt.Errorf("mcp: initialize: %w", err)
	}
	c.Server.Name, c.Server.Version, c.Server.ProtocolVersion = res.ServerInfo.Name, res.ServerInfo.Version, res.ProtocolVersion
	if err := c.notify(ctx, "notifications/initialized"); err != nil {
		_ = t.Close()
		return nil, err
	}
	return c, nil
}

func (c *Client) readLoop() {
	for {
		msg, err := c.t.Recv(context.Background())
		if err != nil {
			c.mu.Lock()
			for id, ch := range c.pending {
				ch <- response{Error: &rpcError{Code: codeInternal, Message: "transport closed: " + err.Error()}}
				delete(c.pending, id)
			}
			c.mu.Unlock()
			return
		}
		var resp response
		if json.Unmarshal(msg, &resp) != nil || len(resp.ID) == 0 {
			continue // server notification or garbage: ignored
		}
		c.mu.Lock()
		ch, ok := c.pending[string(resp.ID)]
		delete(c.pending, string(resp.ID))
		c.mu.Unlock()
		if ok {
			ch <- resp
		}
	}
}

func (c *Client) call(ctx context.Context, method string, params, out any) error {
	id := json.RawMessage(strconv.FormatInt(c.seq.Add(1), 10))
	var p json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return err
		}
		p = b
	}
	body, _ := json.Marshal(request{JSONRPC: "2.0", ID: id, Method: method, Params: p})
	ch := make(chan response, 1)
	c.mu.Lock()
	c.pending[string(id)] = ch
	c.mu.Unlock()
	if err := c.t.Send(ctx, body); err != nil {
		c.mu.Lock()
		delete(c.pending, string(id))
		c.mu.Unlock()
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case resp := <-ch:
		if resp.Error != nil {
			return fmt.Errorf("mcp: %s: %w", method, resp.Error)
		}
		if out != nil && len(resp.Result) > 0 {
			return json.Unmarshal(resp.Result, out)
		}
		return nil
	}
}

func (c *Client) notify(ctx context.Context, method string) error {
	body, _ := json.Marshal(request{JSONRPC: "2.0", Method: method})
	return c.t.Send(ctx, body)
}

// ListTools returns the server's tools, following pagination.
func (c *Client) ListTools(ctx context.Context) ([]ToolInfo, error) {
	var all []ToolInfo
	cursor := ""
	for {
		var res struct {
			Tools      []ToolInfo `json:"tools"`
			NextCursor string     `json:"nextCursor"`
		}
		params := map[string]any{}
		if cursor != "" {
			params["cursor"] = cursor
		}
		if err := c.call(ctx, "tools/list", params, &res); err != nil {
			return nil, err
		}
		all = append(all, res.Tools...)
		if res.NextCursor == "" {
			return all, nil
		}
		cursor = res.NextCursor
	}
}

// CallTool invokes a tool by name.
func (c *Client) CallTool(ctx context.Context, name string, args json.RawMessage) (CallResult, error) {
	var res CallResult
	if len(args) == 0 {
		args = json.RawMessage("{}")
	}
	err := c.call(ctx, "tools/call", map[string]any{"name": name, "arguments": args}, &res)
	return res, err
}

// Tools lists the server's tools as ai.Tool values ready for an Agent. A
// result flagged isError becomes a tool error (reported to the model).
func (c *Client) Tools(ctx context.Context) ([]*ai.Tool, error) {
	infos, err := c.ListTools(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*ai.Tool, 0, len(infos))
	for _, ti := range infos {
		name := ti.Name
		out = append(out, ai.NewTool(name, ti.Description, ti.InputSchema, func(ctx context.Context, args json.RawMessage) (string, error) {
			res, err := c.CallTool(ctx, name, args)
			if err != nil {
				return "", err
			}
			if res.IsError {
				return "", fmt.Errorf("%s", res.Text())
			}
			return res.Text(), nil
		}))
	}
	return out, nil
}

// Close shuts the transport (and kills a Stdio child).
func (c *Client) Close() error { return c.t.Close() }
