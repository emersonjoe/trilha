---
title: AI and agents
description: Call a model, give tools to an agent, hand the conversation over between agents, use and expose MCP, and stream the answer.
---

The `ai` package speaks OpenAI's *chat completions* protocol, which today is the lingua
franca of providers: OpenAI, Groq, Mistral, OpenRouter, Ollama, LM Studio and vLLM accept
the same requests. You configure URL and model through environment variables and the code
does not change. Like all of Trilha, `ai` and `ai/mcp` bring no dependencies outside the
standard library.

```bash
export OPENAI_API_KEY=sk-...                 # or any token from your provider
export OPENAI_BASE_URL=http://localhost:11434/v1   # local Ollama, for example
export TRILHA_AI_MODEL=qwen2.5:7b
```

## One call

```go
cli := ai.NewFromEnv()
resp, err := cli.Chat(ctx, ai.Request{Messages: []ai.Message{
    ai.System("Answer in one sentence."),
    ai.User("What is a layout in Trilha?"),
}})
fmt.Println(resp.Text())
```

`Stream` delivers the answer in chunks; `Delta.Content` carries the text and
`Delta.ToolCalls` the tool arguments as they arrive.

## Tools

A tool is a name, a description, a JSON Schema for the arguments and a Go function.
`ai.Typed` decodes the arguments into a struct for you:

```go
weather := ai.NewTool("weather", "Current temperature in a city.",
    ai.Schema(`{"type":"object","properties":{"city":{"type":"string"}},"required":["city"]}`),
    ai.Typed(func(ctx context.Context, in struct{ City string }) (string, error) {
        return fetchTemperature(ctx, in.City)
    }))
```

Errors and panics inside the tool become text for the model ("error: ..."), never bring the
server down. The model reads the error and decides what to do, which is the behavior you
want in an agent.

## Agents

An agent is instructions + tools. `ai.Run` executes the loop model → tools → model until the
final answer (or `MaxTurns`, default 10). Tool calls in the same round run in parallel.

```go
assistant := &ai.Agent{
    Name:         "Assistant",
    Instructions: "Answer briefly.",
    Tools:        []*ai.Tool{weather},
}
res, err := ai.Run(ctx, cli, assistant, "Is it cold in Curitiba?")
fmt.Println(res.Output)        // final text
fmt.Println(res.Steps)         // each tool called, with arguments and output
```

`res.Messages` is the whole conversation; pass it as history in the next call to keep the
context: `ai.Run(ctx, cli, assistant, "And tomorrow?", res.Messages...)`.

## Multi-agent

Three ways to compose agents, from the simplest to the most controlled:

- **Handoff**: `Handoffs: []*ai.Agent{translator}` creates the `transfer_to_translator`
  tool. When the model calls it, the translator takes over the conversation: the instructions
  change, the history stays. It is the "triage → specialist" pattern.
- **Agent as a tool**: `researcher.AsTool(cli, "Researches a topic")` makes the main agent
  call the other one as a function and keep the conversation itself.
- **Orchestration in Go**: `ai.Parallel` runs several agents at once and `ai.Chain` passes
  one's output as the next one's input. You keep control in code, without depending on the
  model "remembering" to delegate.

## Streaming to the browser

`c.Stream()` turns the response into Server-Sent Events and `ai.RunStream` delivers the
agent's events (text, tool call, result, handoff, end):

```go
func POST(c *trilha.Ctx) error {
    var in struct{ Message string; History []ai.Message }
    if err := c.BindJSON(&in); err != nil { return err }
    s := c.Stream()
    _, err := ai.RunStream(c.Context(), cli, assistant, in.Message, func(ev ai.Event) {
        switch ev.Type {
        case "text":
            _ = s.Send("text", ev.Text)
        case "done":
            _ = s.JSON("done", map[string]any{"history": ev.Result.Messages})
        }
    }, in.History...)
    return err
}
```

On the client, a `fetch` with `POST` and reading the body through `ReadableStream` is enough
(the browser's `EventSource` only does `GET`). That is the loop written by hand — worth reading
once, and then not writing again.

## A chat, ready-made

Every app with an assistant ends up writing that route and that script. `ai.Serve` and
`ui.Chat` are the two halves already written:

```go
// app/api/chat/route.go
func POST(c *trilha.Ctx) error {
    return ai.ServeOpts{HTML: ui.ChatHTML}.Serve(c, cli, assistant)
}

// the page
ui.Chat(c, ui.ChatOpts{Action: "/api/chat", History: msgs, Steps: true})
ui.ChatScript(c)
```

`ai.Serve` reads the message, runs the agent and emits the events with the names above;
`ui.Chat` renders the bubbles, the field and the `aria-live`, and `ui.chat.js` reads the
stream. Without JavaScript the form still submits and the same route answers the whole thing
at once — the screen never depends on the script running.

Model text is Markdown, and `ui.Markdown` is what puts it on the screen: a `<script>` in the
answer is text on the page, not a tag in the document. The history belongs to the app; the
framework keeps no chat session.

The whole thing assembled — the key, the settings an administrator changes, the page's
context, the history, the limit and the test that runs with no key — is the
[AI chat recipe](/cookbook/ai-chat), and it ends at [a running demo](/demos/ai-chat).

## MCP: use and expose tools

The *Model Context Protocol* standardizes how hosts (Claude, Cursor, VS Code...) discover
and call tools. Trilha implements both sides.

**Client**: the tools of any MCP server become `*ai.Tool` for your agents.

```go
fs, err := mcp.Dial(ctx, mcp.Stdio("npx", "-y", "@modelcontextprotocol/server-filesystem", "."))
tools, err := fs.Tools(ctx)
agent.Tools = append(agent.Tools, tools...)
```

`mcp.HTTP(url, headers)` connects to remote servers (Streamable HTTP).

**Server**: your app's tools become available to external hosts with one route:

```go
// app/mcp/route.go
var server = mcp.NewServer("my-app", "1.0", weather, findOrder)

func POST(c *trilha.Ctx) error { return server.ServeHTTP(c) }
```

Protect the route like any API (middleware with a token, rate limit). The server emits
`Mcp-Session-Id` on `initialize` and rejects messages without a session. For hosts that
prefer stdio, `server.ServeStdio(ctx, os.Stdin, os.Stdout)` in a separate `main`.

**The API you already have** does not need tools written by hand. `mcp.FromRoutes(a, opts)`
publishes every route under `/api/` as a tool — name, description and schema from the
OpenAPI document, the call going through the route's own chain, `tools/list` filtered by
what the caller's key may reach. The recipe is [Your API as agent tools](/cookbook/api-as-tools);
the details, in [mcp](/reference/mcp#your-api-as-tools).

## Your project, explained to an agent

The chapters above are about the agent your app runs. This section is about the agent that
edits your app — Claude Code, Cursor, Copilot — and the file it reads first.

```bash
trilha agents             # in a project that already exists
trilha new loja --agents  # at creation time
```

`--agents` is a flag of `new`; in a project that already exists the command is `trilha
agents`, and the upgrade from an older version is five lines in
[Migration](/cookbook/migration#turning-on-the-agent-files-in-a-project-that-already-exists).

It writes two files at the root. `AGENTS.md` is the framework's: the three conventions, the
commands and what each one checks, and what not to do (edit `trilha_gen.go`, add a dependency,
put a secret in the code). `CLAUDE.md` is yours: three lines pointing at `AGENTS.md`, and room
for whatever this repository needs.

Neither exists unless you ask. Support for agents is a choice of the team, not a convention of
the framework, so `trilha new` on its own leaves your project exactly as it did before.

`AGENTS.md` is refreshed the way the ui kit is: it carries the hash of its own body, so an
untouched copy from an older version is rewritten in silence and one you edited needs
`--force`. Add your rules to it and they survive the next upgrade — the command will refuse
rather than overwrite them.

Two commands exist for that reader in particular. `trilha ctx` prints the map of the project —
every route with its file and methods, each API operation with what it receives and returns,
the types involved, what `app/setup.go` provides — in one read instead of a dozen file
openings, with `--json` when the reader is a tool. `trilha check` is the single gate before
calling the work done: `gen`, `gofmt`, `vet`, `test`, `audit` and `openapi` in one command,
stopping at the first failure, with `--fix` for the two problems nobody should be told twice.
Every problem it reports carries the file, the line and the sentence that resolves it, so
finding that out costs no extra round trip. Both are in
[CLI](/reference/cli#trilha-check).

:::note
This documentation is also published as plain text, which is much cheaper for an agent to read
than the HTML around it: [/llms.txt](/llms.txt) is the index, one line per page, and
[/llms-full.txt](/llms-full.txt) is everything concatenated, code blocks included. The
Portuguese ones are at `/pt/llms.txt` and `/pt/llms-full.txt`.
:::

## Challenge

Give the example's agent a `find_post` tool that queries the blog API (`/api/posts/{id}`)
and ask: "summarize the post ola-trilha".

:::solution
```go
findPost := ai.NewTool("find_post", "Finds a blog post by slug.",
    ai.Schema(`{"type":"object","properties":{"slug":{"type":"string"}},"required":["slug"]}`),
    ai.Typed(func(ctx context.Context, in struct{ Slug string }) (string, error) {
        p, ok := posts.BySlug(in.Slug)
        if !ok {
            return "", fmt.Errorf("post not found: %s", in.Slug)
        }
        return p.Title + "\n\n" + p.Body, nil
    }))
assistant.Tools = append(assistant.Tools, findPost)
```
Being an in-process call, there is no HTTP and no key: the tool reads the repository
directly. When the source is external, use `ctx` to honor the client's cancellation.
:::
