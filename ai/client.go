package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// DefaultBaseURL is the OpenAI endpoint; set OPENAI_BASE_URL for others.
const DefaultBaseURL = "https://api.openai.com/v1"

// Client calls a chat-completions API.
type Client struct {
	// BaseURL ends with /v1 (or the provider's equivalent).
	BaseURL string
	// APIKey goes in the Authorization header. Never logged.
	APIKey string
	// Model is the default model for requests without one.
	Model string
	// Headers are extra request headers (e.g. OpenRouter's HTTP-Referer).
	Headers map[string]string
	// HTTPClient defaults to one with a 2-minute timeout.
	HTTPClient *http.Client
}

// NewFromEnv reads OPENAI_API_KEY, OPENAI_BASE_URL and TRILHA_AI_MODEL
// (fallback OPENAI_MODEL, then "gpt-4o-mini").
func NewFromEnv() *Client {
	c := &Client{
		BaseURL: strings.TrimSuffix(os.Getenv("OPENAI_BASE_URL"), "/"),
		APIKey:  os.Getenv("OPENAI_API_KEY"),
		Model:   os.Getenv("TRILHA_AI_MODEL"),
	}
	if c.BaseURL == "" {
		c.BaseURL = DefaultBaseURL
	}
	if c.Model == "" {
		c.Model = os.Getenv("OPENAI_MODEL")
	}
	if c.Model == "" {
		c.Model = "gpt-4o-mini"
	}
	return c
}

// Error is a non-2xx answer from the provider.
type Error struct {
	Status  int
	Code    string
	Message string
}

func (e *Error) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("ai: %d %s: %s", e.Status, e.Code, e.Message)
	}
	return fmt.Sprintf("ai: %d: %s", e.Status, e.Message)
}

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: 2 * time.Minute}
}

func (c *Client) do(ctx context.Context, req Request) (*http.Response, error) {
	if req.Model == "" {
		req.Model = c.Model
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	base := c.BaseURL
	if base == "" {
		base = DefaultBaseURL
	}
	hr, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSuffix(base, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	hr.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		hr.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	if req.Stream {
		hr.Header.Set("Accept", "text/event-stream")
	}
	for k, v := range c.Headers {
		hr.Header.Set(k, v)
	}
	resp, err := c.httpClient().Do(hr)
	if err != nil {
		return nil, fmt.Errorf("ai: %w", err)
	}
	if resp.StatusCode/100 != 2 {
		defer resp.Body.Close()
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		e := &Error{Status: resp.StatusCode, Message: strings.TrimSpace(string(b))}
		var wrap struct {
			Error struct {
				Message string `json:"message"`
				Code    any    `json:"code"`
				Type    string `json:"type"`
			} `json:"error"`
		}
		if json.Unmarshal(b, &wrap) == nil && wrap.Error.Message != "" {
			e.Message = wrap.Error.Message
			e.Code = fmt.Sprint(wrap.Error.Code)
			if e.Code == "<nil>" || e.Code == "" {
				e.Code = wrap.Error.Type
			}
		}
		return nil, e
	}
	return resp, nil
}

// Chat sends a request and returns the full response.
func (c *Client) Chat(ctx context.Context, req Request) (*Response, error) {
	req.Stream = false
	resp, err := c.do(ctx, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out Response
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("ai: decoding response: %w", err)
	}
	return &out, nil
}

// Stream sends a streaming request and calls fn for each fragment. Tool
// calls are assembled across chunks and delivered complete. Returning an
// error from fn stops the stream.
func (c *Client) Stream(ctx context.Context, req Request, fn func(Delta) error) error {
	req.Stream = true
	if req.Extra == nil {
		req.Extra = map[string]any{}
	}
	if _, ok := req.Extra["stream_options"]; !ok {
		req.Extra["stream_options"] = map[string]any{"include_usage": true}
	}
	resp, err := c.do(ctx, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return parseStream(resp.Body, fn)
}

type chunk struct {
	Choices []struct {
		Delta struct {
			Content   string `json:"content"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *Usage `json:"usage"`
}

// parseStream reads SSE "data:" lines until [DONE].
func parseStream(r io.Reader, fn func(Delta) error) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64<<10), 4<<20)
	calls := map[int]*ToolCall{}
	var order []int
	flushCalls := func() []ToolCall {
		if len(order) == 0 {
			return nil
		}
		out := make([]ToolCall, 0, len(order))
		for _, i := range order {
			out = append(out, *calls[i])
		}
		calls, order = map[int]*ToolCall{}, nil
		return out
	}
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var ch chunk
		if err := json.Unmarshal([]byte(data), &ch); err != nil {
			return fmt.Errorf("ai: bad stream chunk: %w", err)
		}
		d := Delta{Usage: ch.Usage}
		if len(ch.Choices) > 0 {
			c0 := ch.Choices[0]
			d.Content = c0.Delta.Content
			for _, tc := range c0.Delta.ToolCalls {
				cur, ok := calls[tc.Index]
				if !ok {
					cur = &ToolCall{Type: "function"}
					calls[tc.Index] = cur
					order = append(order, tc.Index)
				}
				if tc.ID != "" {
					cur.ID = tc.ID
				}
				if tc.Type != "" {
					cur.Type = tc.Type
				}
				if tc.Function.Name != "" {
					cur.Function.Name += tc.Function.Name
				}
				cur.Function.Arguments += tc.Function.Arguments
			}
			if c0.FinishReason != "" {
				d.Finish = c0.FinishReason
				d.ToolCalls = flushCalls()
			}
		}
		if d.Content == "" && d.Finish == "" && d.Usage == nil && len(d.ToolCalls) == 0 {
			continue
		}
		if err := fn(d); err != nil {
			return err
		}
	}
	if err := sc.Err(); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("ai: reading stream: %w", err)
	}
	if rest := flushCalls(); len(rest) > 0 {
		return fn(Delta{ToolCalls: rest, Finish: "tool_calls"})
	}
	return nil
}
