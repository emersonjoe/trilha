// Package ai talks to chat-completion APIs that follow the OpenAI protocol
// (OpenAI, Azure, Ollama, LM Studio, Groq, OpenRouter, vLLM...) and builds
// agents on top of them: tools, handoffs, parallel and chained agents. It
// depends only on the standard library.
package ai

import "encoding/json"

// Message is one chat message.
type Message struct {
	Role       string     `json:"role"` // system | user | assistant | tool
	Content    string     `json:"content,omitempty"`
	Name       string     `json:"name,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// ToolCall is a function call requested by the model.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"` // "function"
	Function FunctionCall `json:"function"`
}

// FunctionCall names the function and carries its JSON arguments.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolDef is a tool as declared to the model.
type ToolDef struct {
	Type     string      `json:"type"` // "function"
	Function FunctionDef `json:"function"`
}

// FunctionDef describes a function and its JSON Schema parameters.
type FunctionDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
	Strict      bool            `json:"strict,omitempty"`
}

// ResponseFormat asks for JSON output, optionally constrained by a schema.
type ResponseFormat struct {
	Type       string          `json:"type"` // "text" | "json_object" | "json_schema"
	JSONSchema json.RawMessage `json:"json_schema,omitempty"`
}

// Request is a chat completion request.
type Request struct {
	Model          string          `json:"model,omitempty"`
	Messages       []Message       `json:"messages"`
	Tools          []ToolDef       `json:"tools,omitempty"`
	ToolChoice     any             `json:"tool_choice,omitempty"` // "auto" | "none" | "required" | {"type":"function",...}
	Temperature    *float64        `json:"temperature,omitempty"`
	MaxTokens      int             `json:"max_tokens,omitempty"`
	ResponseFormat *ResponseFormat `json:"response_format,omitempty"`
	Stream         bool            `json:"stream,omitempty"`
	// Extra fields are merged into the JSON body (provider-specific options).
	Extra map[string]any `json:"-"`
}

// MarshalJSON merges Extra into the body.
func (r Request) MarshalJSON() ([]byte, error) {
	type plain Request
	b, err := json.Marshal(plain(r))
	if err != nil || len(r.Extra) == 0 {
		return b, err
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	for k, v := range r.Extra {
		m[k] = v
	}
	return json.Marshal(m)
}

// Usage counts tokens.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Add accumulates usage.
func (u *Usage) Add(o Usage) {
	u.PromptTokens += o.PromptTokens
	u.CompletionTokens += o.CompletionTokens
	u.TotalTokens += o.TotalTokens
}

// Choice is one completion.
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// Response is a chat completion response.
type Response struct {
	ID      string   `json:"id"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Text returns the first choice's content.
func (r *Response) Text() string {
	if r == nil || len(r.Choices) == 0 {
		return ""
	}
	return r.Choices[0].Message.Content
}

// ToolCalls returns the first choice's tool calls.
func (r *Response) ToolCalls() []ToolCall {
	if r == nil || len(r.Choices) == 0 {
		return nil
	}
	return r.Choices[0].Message.ToolCalls
}

// Delta is one streamed fragment.
type Delta struct {
	Content   string
	ToolCalls []ToolCall // completed calls, delivered once each when finished
	Finish    string     // finish_reason when the stream ends
	Usage     *Usage
}

// Helpers to build messages.
func System(s string) Message    { return Message{Role: "system", Content: s} }
func User(s string) Message      { return Message{Role: "user", Content: s} }
func Assistant(s string) Message { return Message{Role: "assistant", Content: s} }

// ToolResult builds the tool message answering a call.
func ToolResult(callID, content string) Message {
	return Message{Role: "tool", ToolCallID: callID, Content: content}
}
