# Plano — spec 062

## Onde cada coisa entra

| Arquivo | O que muda |
|---|---|
| `ui/markdown.go` (novo) | `Markdown`, `MarkdownOpts`, o parser de blocos e o de inline |
| `ui/markdown_test.go`, `ui/testdata/markdown.golden` (novos) | corpus, casos de escape e de link |
| `ui/fuzz_test.go` | alvo novo: saída sem tag, sem esquema perigoso, sem pânico |
| `ai/serve.go` (novo) | `Serve`, `ServeOpts` e os nomes dos eventos |
| `ai/serve_test.go` (novo) | ordem dos eventos, erro do modelo, resposta sem SSE |
| `ui/chat.go` (novo) | `Chat`, `ChatOpts`, `ChatMessage`, `ChatScript` |
| `ui/assets/ui.chat.js` (novo) | envio, leitura do SSE, bolha, passo, render final |
| `ui/assets/ui.css` | `.ui-chat`, `.ui-bubble`, `.ui-step` |
| `ui/ui.go` | `//go:embed` e `Files` com o `ui.chat.js` |
| `internal/scaffold/ui_test.go` | o `trilha ui` passa a escrever cinco scripts |
| `examples/assistente` | `public/chat.js` sai, `page.go` usa `ui.Chat`, `route.go` vira uma linha |
| docs | referência `ui` e `ai` nas duas línguas |

## Ordem

`ui.Markdown` primeiro, sozinho e testado, porque é o que carrega o risco; `ai.Serve` em
seguida, que não depende dele; `ui.Chat` e o script depois, que dependem dos dois; o exemplo
por último, que é onde se vê se a assinatura serve.

## O que não entra

- CommonMark completo: referência de link, HTML embutido, lista de definição, nota de rodapé.
  O alvo é o que um modelo escreve, não um compilador de especificação.
- Realce de sintaxe: o bloco de código sai com `class="language-x"` e quem quiser cor põe o
  seu. Realce é tabela de palavra por linguagem, e isso não cabe no runtime.
- Sessão de chat, persistência de conversa e "retomar de onde parou" no servidor: é do app.
