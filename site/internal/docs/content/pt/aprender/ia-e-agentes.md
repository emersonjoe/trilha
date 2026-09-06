---
title: IA e agentes
description: Chame um modelo, dê ferramentas a um agente, transfira a conversa entre agentes, use e exponha MCP, e transmita a resposta em streaming.
---

O pacote `ai` fala o protocolo de *chat completions* da OpenAI, que hoje é a língua franca dos
provedores: OpenAI, Groq, Mistral, OpenRouter, Ollama, LM Studio e vLLM aceitam as mesmas
requisições. Você configura a URL e o modelo por variáveis de ambiente e o código não muda.
Como todo o Trilha, `ai` e `ai/mcp` não trazem dependências fora da biblioteca padrão.

```bash
export OPENAI_API_KEY=sk-...                 # ou qualquer token do seu provedor
export OPENAI_BASE_URL=http://localhost:11434/v1   # Ollama local, por exemplo
export TRILHA_AI_MODEL=qwen2.5:7b
```

## Uma chamada

```go
cli := ai.NewFromEnv()
resp, err := cli.Chat(ctx, ai.Request{Messages: []ai.Message{
    ai.System("Responda em uma frase."),
    ai.User("O que é um layout no Trilha?"),
}})
fmt.Println(resp.Text())
```

`Stream` entrega a resposta aos pedaços; `Delta.Content` traz o texto e `Delta.ToolCalls`
os argumentos de ferramentas conforme chegam.

## Ferramentas

Uma ferramenta é um nome, uma descrição, um JSON Schema para os argumentos e uma função Go.
`ai.Typed` decodifica os argumentos em uma struct para você:

```go
clima := ai.NewTool("clima", "Temperatura atual em uma cidade.",
    ai.Schema(`{"type":"object","properties":{"cidade":{"type":"string"}},"required":["cidade"]}`),
    ai.Typed(func(ctx context.Context, in struct{ Cidade string }) (string, error) {
        return buscarTemperatura(ctx, in.Cidade)
    }))
```

Erros e pânicos dentro da ferramenta viram texto para o modelo ("error: ..."), nunca derrubam
o servidor. O modelo lê o erro e decide o que fazer, que é o comportamento que você quer em
um agente.

## Agentes

Um agente é instruções + ferramentas. `ai.Run` executa o laço modelo → ferramentas → modelo
até a resposta final (ou `MaxTurns`, padrão 10). Chamadas de ferramentas na mesma rodada
rodam em paralelo.

```go
assistente := &ai.Agent{
    Name:         "Assistente",
    Instructions: "Responda em português, de forma curta.",
    Tools:        []*ai.Tool{clima},
}
res, err := ai.Run(ctx, cli, assistente, "Está frio em Curitiba?")
fmt.Println(res.Output)        // texto final
fmt.Println(res.Steps)         // cada ferramenta chamada, com argumentos e saída
```

`res.Messages` é a conversa inteira; passe-a como histórico na próxima chamada para manter
o contexto: `ai.Run(ctx, cli, assistente, "E amanhã?", res.Messages...)`.

## Multiagentes

Três formas de compor agentes, da mais simples à mais controlada:

- **Handoff**: `Handoffs: []*ai.Agent{tradutor}` cria a ferramenta `transfer_to_tradutor`.
  Quando o modelo a chama, o tradutor assume a conversa: as instruções trocam, o histórico
  fica. É o padrão "triagem → especialista".
- **Agente como ferramenta**: `pesquisador.AsTool(cli, "Pesquisa um tema")` faz o agente
  principal chamar o outro como uma função e continuar ele mesmo a conversa.
- **Orquestração em Go**: `ai.Parallel` roda vários agentes de uma vez e `ai.Chain` passa a
  saída de um como entrada do próximo. Você fica com o controle no código, sem depender de o
  modelo "lembrar" de delegar.

## Streaming até o navegador

`c.Stream()` transforma a resposta em Server-Sent Events e `ai.RunStream` entrega os eventos
do agente (texto, chamada de ferramenta, resultado, handoff, fim):

```go
func POST(c *trilha.Ctx) error {
    var in struct{ Message string; History []ai.Message }
    if err := c.BindJSON(&in); err != nil { return err }
    s := c.Stream()
    _, err := ai.RunStream(c.Context(), cli, assistente, in.Message, func(ev ai.Event) {
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

No cliente, um `fetch` com `POST` e leitura do corpo com `ReadableStream` basta (o
`EventSource` do navegador só faz `GET`). O exemplo `examples/assistente` traz o `chat.js`
completo em 60 linhas.

## MCP: usar e expor ferramentas

O *Model Context Protocol* padroniza como hosts (Claude, Cursor, VS Code...) descobrem e
chamam ferramentas. O Trilha implementa os dois lados.

**Cliente**: as ferramentas de qualquer servidor MCP viram `*ai.Tool` para os seus agentes.

```go
fs, err := mcp.Dial(ctx, mcp.Stdio("npx", "-y", "@modelcontextprotocol/server-filesystem", "."))
tools, err := fs.Tools(ctx)
agente.Tools = append(agente.Tools, tools...)
```

`mcp.HTTP(url, headers)` conecta a servidores remotos (Streamable HTTP).

**Servidor**: as ferramentas do seu app ficam disponíveis para hosts externos com uma rota:

```go
// app/mcp/route.go
var servidor = mcp.NewServer("meu-app", "1.0", clima, buscarPedido)

func POST(c *trilha.Ctx) error { return servidor.ServeHTTP(c) }
```

Proteja a rota como qualquer API (middleware com token, limite de taxa). O servidor emite
`Mcp-Session-Id` no `initialize` e rejeita mensagens sem sessão. Para hosts que preferem
stdio, `servidor.ServeStdio(ctx, os.Stdin, os.Stdout)` em um `main` separado.

## Seu projeto explicado para um agente

Os capítulos acima falam do agente que a sua aplicação executa. Esta seção fala do agente que
edita a sua aplicação — Claude Code, Cursor, Copilot — e do arquivo que ele lê primeiro.

```bash
trilha agents             # num projeto que já existe
trilha new loja --agents  # já na criação
```

O `--agents` é flag do `new`; num projeto que já existe o comando é o `trilha agents`, e a
atualização vinda de uma versão anterior tem cinco linhas na
[receita de migração](/pt/receitas/migracao#ligar-os-arquivos-de-agente-num-projeto-que-ja-existe).

Ele grava dois arquivos na raiz. O `AGENTS.md` é do framework: as três convenções, os comandos
e o que cada um verifica, e o que não fazer (editar `trilha_gen.go`, acrescentar dependência,
pôr segredo no código). O `CLAUDE.md` é seu: três linhas apontando para o `AGENTS.md` e espaço
para o que este repositório precisar.

Nenhum dos dois existe sem que você peça. Suporte a agentes é escolha do time, não convenção do
framework, então `trilha new` sozinho deixa seu projeto exatamente como deixava antes.

O `AGENTS.md` é atualizado como o kit ui: ele leva o hash do próprio corpo, então uma cópia
intocada de uma versão anterior é regravada em silêncio e uma que você editou pede `--force`.
Acrescente suas regras nele e elas sobrevivem à próxima atualização — o comando recusa em vez
de sobrescrever.

Dois comandos existem para esse leitor em particular. O `trilha ctx` imprime o mapa do projeto
— cada rota com seu arquivo e seus métodos, cada operação de API com o que recebe e o que
devolve, os tipos envolvidos, o que o `app/setup.go` provê — numa leitura só, em vez de uma
dúzia de arquivos abertos, com `--json` quando quem lê é uma ferramenta. O `trilha check` é o
portão único antes de dizer que terminou: `gen`, `gofmt`, `vet`, `test`, `audit` e `openapi`
num comando, parando na primeira falha, com `--fix` para os dois problemas que ninguém devia
precisar ouvir duas vezes. Todo problema que ele reporta vem com o arquivo, a linha e a frase
que resolve, então descobrir isso não custa uma ida e volta a mais. Os dois estão na
[CLI](/pt/referencia/cli#trilha-check).

:::note
Esta documentação também é publicada em texto puro, muito mais barato para um agente ler do que
o HTML em volta: o [/pt/llms.txt](/pt/llms.txt) é o índice, uma linha por página, e o
[/pt/llms-full.txt](/pt/llms-full.txt) é tudo concatenado, blocos de código inclusive.
:::

## Desafio

Dê ao agente do exemplo uma ferramenta `buscar_post` que consulte a API do blog
(`/api/posts/{id}`) e peça: "resuma o post ola-trilha".

:::solucao
```go
buscarPost := ai.NewTool("buscar_post", "Busca um post do blog pelo slug.",
    ai.Schema(`{"type":"object","properties":{"slug":{"type":"string"}},"required":["slug"]}`),
    ai.Typed(func(ctx context.Context, in struct{ Slug string }) (string, error) {
        p, ok := posts.BySlug(in.Slug)
        if !ok {
            return "", fmt.Errorf("post não encontrado: %s", in.Slug)
        }
        return p.Title + "\n\n" + p.Body, nil
    }))
assistente.Tools = append(assistente.Tools, buscarPost)
```
Por ser uma chamada em processo, não há HTTP nem chave: a ferramenta lê o repositório
direto. Quando a fonte é externa, use o `ctx` para respeitar o cancelamento do cliente.
:::
