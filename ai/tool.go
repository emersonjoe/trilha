package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// ToolFunc executes a tool with the model's JSON arguments and returns text
// for the model. Errors are reported back to the model, not to the caller.
type ToolFunc func(ctx context.Context, args json.RawMessage) (string, error)

// Tool is a function the model may call.
type Tool struct {
	Name        string
	Description string
	// Parameters is a JSON Schema object; nil means no arguments.
	Parameters json.RawMessage
	Func       ToolFunc
	// handoff marks the synthetic transfer tools.
	handoff *Agent
}

// NewTool builds a tool. schema is a JSON Schema for the arguments (see Schema).
func NewTool(name, description string, schema json.RawMessage, fn ToolFunc) *Tool {
	if schema == nil {
		schema = json.RawMessage(`{"type":"object","properties":{}}`)
	}
	return &Tool{Name: name, Description: description, Parameters: schema, Func: fn}
}

// Schema is a convenience to write JSON Schema inline; it panics on invalid JSON
// so mistakes fail at startup.
func Schema(s string) json.RawMessage {
	if !json.Valid([]byte(s)) {
		panic("ai.Schema: invalid JSON: " + s)
	}
	return json.RawMessage(s)
}

// Typed wraps a tool function that takes a struct decoded from the arguments.
func Typed[T any](fn func(ctx context.Context, in T) (string, error)) ToolFunc {
	return func(ctx context.Context, args json.RawMessage) (string, error) {
		var in T
		if len(args) > 0 && string(args) != "null" {
			if err := json.Unmarshal(args, &in); err != nil {
				return "", fmt.Errorf("invalid arguments: %w", err)
			}
		}
		return fn(ctx, in)
	}
}

// Def returns the declaration sent to the model.
func (t *Tool) Def() ToolDef {
	return ToolDef{Type: "function", Function: FunctionDef{Name: t.Name, Description: t.Description, Parameters: t.Parameters}}
}

// call runs the tool, turning panics and errors into text for the model.
func (t *Tool) call(ctx context.Context, args string) (out string, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("tool %s panicked: %v", t.Name, r)
		}
	}()
	if t.Func == nil {
		return "", fmt.Errorf("tool %s has no implementation", t.Name)
	}
	raw := json.RawMessage(strings.TrimSpace(args))
	if len(raw) == 0 {
		raw = json.RawMessage("{}")
	}
	if !json.Valid(raw) {
		return "", fmt.Errorf("tool %s: arguments are not valid JSON: %s", t.Name, args)
	}
	return t.Func(ctx, raw)
}

func defs(tools []*Tool) []ToolDef {
	out := make([]ToolDef, 0, len(tools))
	for _, t := range tools {
		out = append(out, t.Def())
	}
	return out
}
