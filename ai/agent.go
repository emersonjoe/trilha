package ai

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// Agent is a model configuration: instructions, tools and possible handoffs.
type Agent struct {
	Name         string
	Instructions string
	// Model overrides the client's default.
	Model string
	Tools []*Tool
	// Handoffs are agents this one may transfer the conversation to.
	Handoffs []*Agent
	// MaxTurns limits model calls per Run (default 10).
	MaxTurns int
	// Temperature, when set, is sent with every request.
	Temperature *float64
	// ResponseFormat constrains the final answer (e.g. JSON schema).
	ResponseFormat *ResponseFormat
}

// ErrMaxTurns is returned when the loop exceeds Agent.MaxTurns.
var ErrMaxTurns = errors.New("ai: max turns reached")

// Step records one tool execution or handoff.
type Step struct {
	Agent     string
	Tool      string
	CallID    string
	Arguments string
	Output    string
	Err       error
	HandoffTo string
}

// Result is what Run returns.
type Result struct {
	// Output is the final assistant text.
	Output string
	// Agent is the agent that produced the final answer (after handoffs).
	Agent *Agent
	// Messages is the whole conversation, reusable as history.
	Messages []Message
	Steps    []Step
	Usage    Usage
	Turns    int
}

// Event is emitted by RunStream.
type Event struct {
	// Type: text | tool_call | tool_result | handoff | done | error
	Type   string
	Text   string
	Step   *Step
	Agent  string
	Result *Result
	Err    error
}

// Run executes the agent loop: model → tools → model, until a final answer.
// history may carry earlier turns (Result.Messages of a previous Run).
func Run(ctx context.Context, cli *Client, agent *Agent, input string, history ...Message) (*Result, error) {
	return run(ctx, cli, agent, input, history, nil)
}

// RunStream is Run with text deltas and steps delivered through fn.
func RunStream(ctx context.Context, cli *Client, agent *Agent, input string, fn func(Event), history ...Message) (*Result, error) {
	if fn == nil {
		fn = func(Event) {}
	}
	return run(ctx, cli, agent, input, history, fn)
}

func handoffName(a *Agent) string {
	var b strings.Builder
	for _, r := range strings.ToLower(a.Name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return "transfer_to_" + strings.Trim(b.String(), "_")
}

// allTools returns the agent's tools plus one synthetic tool per handoff.
func (a *Agent) allTools() []*Tool {
	out := append([]*Tool{}, a.Tools...)
	for _, h := range a.Handoffs {
		out = append(out, &Tool{
			Name:        handoffName(h),
			Description: "Transfer the conversation to the agent " + h.Name + ". " + firstLine(h.Instructions),
			Parameters:  Schema(`{"type":"object","properties":{"reason":{"type":"string"}}}`),
			handoff:     h,
		})
	}
	return out
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}

func run(ctx context.Context, cli *Client, agent *Agent, input string, history []Message, emit func(Event)) (*Result, error) {
	if agent == nil {
		return nil, errors.New("ai: nil agent")
	}
	res := &Result{Agent: agent}
	msgs := make([]Message, 0, len(history)+2)
	if agent.Instructions != "" {
		msgs = append(msgs, System(agent.Instructions))
	}
	for _, m := range history {
		if m.Role != "system" {
			msgs = append(msgs, m)
		}
	}
	if input != "" {
		msgs = append(msgs, User(input))
	}
	maxTurns := agent.MaxTurns
	if maxTurns <= 0 {
		maxTurns = 10
	}
	for res.Turns < maxTurns {
		res.Turns++
		tools := agent.allTools()
		req := Request{Model: agent.Model, Messages: msgs, Tools: defs(tools), Temperature: agent.Temperature, ResponseFormat: agent.ResponseFormat}
		if len(tools) == 0 {
			req.Tools = nil
		}
		assistant, usage, err := complete(ctx, cli, req, emit)
		if err != nil {
			if emit != nil {
				emit(Event{Type: "error", Err: err, Agent: agent.Name})
			}
			return res, err
		}
		res.Usage.Add(usage)
		msgs = append(msgs, assistant)
		if len(assistant.ToolCalls) == 0 {
			res.Output = assistant.Content
			res.Agent = agent
			res.Messages = msgs
			if emit != nil {
				emit(Event{Type: "done", Agent: agent.Name, Result: res})
			}
			return res, nil
		}
		// Execute tool calls in parallel, keep the order for the transcript.
		outputs := make([]Step, len(assistant.ToolCalls))
		var wg sync.WaitGroup
		var next *Agent
		var nextMu sync.Mutex
		for i, tc := range assistant.ToolCalls {
			tool := findTool(tools, tc.Function.Name)
			step := Step{Agent: agent.Name, Tool: tc.Function.Name, CallID: tc.ID, Arguments: tc.Function.Arguments}
			if emit != nil {
				emit(Event{Type: "tool_call", Step: &step, Agent: agent.Name})
			}
			if tool == nil {
				step.Err = fmt.Errorf("unknown tool %q", tc.Function.Name)
				step.Output = "error: " + step.Err.Error()
				outputs[i] = step
				continue
			}
			if tool.handoff != nil {
				step.HandoffTo = tool.handoff.Name
				step.Output = "Transferred to " + tool.handoff.Name + "."
				outputs[i] = step
				nextMu.Lock()
				if next == nil {
					next = tool.handoff
				}
				nextMu.Unlock()
				continue
			}
			wg.Add(1)
			go func(i int, step Step, tool *Tool) {
				defer wg.Done()
				out, err := tool.call(ctx, step.Arguments)
				if err != nil {
					step.Err = err
					out = "error: " + err.Error()
				}
				step.Output = out
				outputs[i] = step
			}(i, step, tool)
		}
		wg.Wait()
		for _, step := range outputs {
			res.Steps = append(res.Steps, step)
			if emit != nil {
				s := step
				if step.HandoffTo != "" {
					emit(Event{Type: "handoff", Step: &s, Agent: step.HandoffTo})
				} else {
					emit(Event{Type: "tool_result", Step: &s, Agent: agent.Name})
				}
			}
			msgs = append(msgs, ToolResult(step.CallID, step.Output))
		}
		if next != nil {
			// The new agent takes over: swap the system prompt, keep the history.
			agent = next
			if len(msgs) > 0 && msgs[0].Role == "system" {
				msgs = msgs[1:]
			}
			if agent.Instructions != "" {
				msgs = append([]Message{System(agent.Instructions)}, msgs...)
			}
			if agent.MaxTurns > 0 && agent.MaxTurns+res.Turns > maxTurns {
				maxTurns = res.Turns + agent.MaxTurns
			}
		}
	}
	res.Messages = msgs
	if emit != nil {
		emit(Event{Type: "error", Err: ErrMaxTurns, Agent: agent.Name})
	}
	return res, ErrMaxTurns
}

// complete performs one model call, streaming when emit is set.
func complete(ctx context.Context, cli *Client, req Request, emit func(Event)) (Message, Usage, error) {
	if emit == nil {
		resp, err := cli.Chat(ctx, req)
		if err != nil {
			return Message{}, Usage{}, err
		}
		if len(resp.Choices) == 0 {
			return Message{}, resp.Usage, errors.New("ai: empty response")
		}
		m := resp.Choices[0].Message
		m.Role = "assistant"
		return m, resp.Usage, nil
	}
	var text strings.Builder
	var calls []ToolCall
	var usage Usage
	err := cli.Stream(ctx, req, func(d Delta) error {
		if d.Content != "" {
			text.WriteString(d.Content)
			emit(Event{Type: "text", Text: d.Content})
		}
		calls = append(calls, d.ToolCalls...)
		if d.Usage != nil {
			usage = *d.Usage
		}
		return nil
	})
	if err != nil {
		return Message{}, usage, err
	}
	return Message{Role: "assistant", Content: text.String(), ToolCalls: calls}, usage, nil
}

func findTool(tools []*Tool, name string) *Tool {
	for _, t := range tools {
		if t.Name == name {
			return t
		}
	}
	return nil
}
