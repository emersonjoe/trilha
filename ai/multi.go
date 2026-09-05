package ai

import (
	"context"
	"encoding/json"
	"sync"
)

// AsTool exposes an agent as a tool of another agent: the caller passes
// {"input": "..."} and receives the sub-agent's final text.
func (a *Agent) AsTool(cli *Client, description string) *Tool {
	if description == "" {
		description = firstLine(a.Instructions)
	}
	return NewTool(handoffName(a)[len("transfer_to_"):], description,
		Schema(`{"type":"object","properties":{"input":{"type":"string","description":"What to ask this agent"}},"required":["input"]}`),
		func(ctx context.Context, args json.RawMessage) (string, error) {
			var in struct {
				Input string `json:"input"`
			}
			if err := json.Unmarshal(args, &in); err != nil {
				return "", err
			}
			res, err := Run(ctx, cli, a, in.Input)
			if err != nil {
				return "", err
			}
			return res.Output, nil
		})
}

// Parallel runs the agents concurrently on the same input and returns the
// results in order. The first error cancels the others.
func Parallel(ctx context.Context, cli *Client, input string, agents ...*Agent) ([]*Result, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	results := make([]*Result, len(agents))
	errs := make([]error, len(agents))
	var wg sync.WaitGroup
	for i, a := range agents {
		wg.Add(1)
		go func(i int, a *Agent) {
			defer wg.Done()
			results[i], errs[i] = Run(ctx, cli, a, input)
			if errs[i] != nil {
				cancel()
			}
		}(i, a)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return results, err
		}
	}
	return results, nil
}

// Chain runs the agents in sequence, feeding each output as the next input.
// It returns every intermediate result; the last one holds the final output.
func Chain(ctx context.Context, cli *Client, input string, agents ...*Agent) ([]*Result, error) {
	var out []*Result
	cur := input
	for _, a := range agents {
		res, err := Run(ctx, cli, a, cur)
		if err != nil {
			return out, err
		}
		out = append(out, res)
		cur = res.Output
	}
	return out, nil
}
