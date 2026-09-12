# Spec 136 — receita: um chat de IA dentro do app

- **Issue**: [#186](https://github.com/emersonjoe/trilha/issues/186) — a issue é a fonte do
  escopo; os oito passos da página estão lá e não são recopiados aqui.
- **Branch**: `136-cookbook-chat-ia`
- **Versão**: 0.115.0

## Por quê

O site ensina os pedaços de um chat de IA — `learn/ai-and-agents.md` mostra `ai.Serve` +
`ui.Chat` em quinze linhas, `reference/ai.md` e `reference/ui.md#chat` descrevem cada símbolo,
`examples/assistente` é um app inteiro — e não ensina a receita. Quem vai colocar um chat
dentro do próprio app para em perguntas que nenhuma dessas páginas responde: onde fica a chave
do provedor, quem guarda o histórico, o que a página conta ao modelo, como o teste roda sem
gastar chamada, o que o visitante vê quando o provedor devolve 429.

Hoje a saída é ler `examples/assistente` inteiro e deduzir. O exemplo é um app de verdade —
tem MCP, tela de configuração, ferramentas próprias — e a pessoa que quer só um chat precisa
separar o que é essencial do que é daquele domínio. A receita faz essa separação uma vez.

## O que muda

Só documentação e o código que a sustenta. Nenhum símbolo novo no runtime, nenhuma convenção
nova em `app/`, nenhuma mudança de API pública.

1. **`site/internal/docs/content/en/cookbook/ai-chat.md`** e o espelho
   `pt/receitas/chat-de-ia.md`: a receita do zero à tela, na ordem dos oito passos da issue —
   cliente, rota, tela, histórico, limites e falhas, teste sem chave, demo, `ui.Assistant` como
   variante. Entra na tabela de `cookbook/index.md` nas duas locales, na navegação
   (`site/internal/docs/docs.go`) e, por consequência, no `llms.txt`.
2. **`examples/cookbook/aichat.go`**: as declarações que a página cita, compiladas por
   `go vet ./...` como as das outras receitas. Só biblioteca padrão: o histórico é um mapa com
   mutex e a página diz onde ele vira tabela.
3. **`site/internal/chatdemo` + `site/public/chat-demo.js`** e as rotas
   `site/app/demos/ai-chat/page.go` e `site/app/pt/demos/ai-chat/page.go`: a demo viva,
   roteirizada no navegador como a do assistente, sem chave e sem rede.
4. **`learn/ai-and-agents.md` § "A chat, ready-made"** (e o espelho pt) aponta para a receita.

```go
// app/api/chat/route.go — o coração da receita, o resto é decisão do app
func POST(c *trilha.Ctx) error {
	if ok, after := AIChatLimit.Allow(c.Actor()); !ok {
		return trilha.TooManyRequests(after)
	}
	c.Audit("ai.asked", c.Form("message"))
	return ai.ServeOpts{
		HTML:    ui.ChatHTML,
		Context: AIChatContext,
		Page:    AIChatPage,
	}.Serve(c, AIChatClient(), AIChatAgent())
}
```

### Decisões que a receita registra

- **A chave fica no ambiente, nunca no código nem na tela em claro.** `ai.NewFromEnv()` lê
  `OPENAI_API_KEY`/`OPENAI_BASE_URL`; para a Anthropic é o mesmo `Client` com `BaseURL` e
  `Headers` de lá, e a receita mostra as duas. O que o administrador muda sem deploy — modelo,
  temperatura, instruções — é `trilha.Settings[T]`; a chave, quando salva ali, é
  `trilha.Secret`.
- **O histórico é do app.** Não existe `c.Session` no framework: quem identifica a conversa é
  o cookie assinado da receita [Sessions](../../site/internal/docs/content/en/cookbook/sessions.md)
  (`c.Signed`), e o histórico mora num store do app, com teto de mensagens. A receita mostra o
  mapa em memória e diz, em uma linha, o que muda quando vira tabela.
- **O teste não fala com o provedor.** Um `ai.Client` com `HTTPClient` sobre um
  `http.RoundTripper` roteirizado responde no formato do provedor: a rota e a tela passam no
  `make test` sem rede e sem chave.
- **A demo do site é roteirizada no navegador**, pelo mesmo molde de `assistantdemo`: o
  `ui.Chat` é o real, o `ui.chat.js` é o real, só a resposta é determinística. O site é
  estático depois de exportado; um servidor de verdade não existe lá.

## Fora de escopo

- **Símbolo novo no `ai` para histórico persistido** — a receita mostra que isso é decisão do
  app; um store no framework escolheria schema por quem usa.
- **Provedor Anthropic como cliente próprio** — `ai.Client` já chega lá por `BaseURL`; um
  segundo cliente é outra issue.
- **Mexer em `examples/assistente`** — ele continua sendo o app completo; a receita é o
  caminho curto, e as duas páginas se apontam.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `examples/cookbook` e a demo usam apenas a stdlib e o próprio módulo; `TestNoExternalDeps` segue verde. |
| VI — teste primeiro | O teste da página, da tabela do índice, da demo em 200 sem chave e do snippet compilável entra antes do conteúdo. |
| VII — segurança por padrão | A receita escreve a chave fora do código, o `ctx.*` como dado do visitante a ser conferido, `ui.Markdown` sem HTML cru e `c.Audit` de cada pergunta. |
| Idioma | Página em inglês e em pt-BR no mesmo commit; código e comentários em inglês. |

## Tarefas

- [ ] T001 Teste que falha: `/cookbook/ai-chat` e `/pt/receitas/chat-de-ia` respondem e estão
      na tabela do índice; `/demos/ai-chat` e `/pt/demos/ai-chat` respondem 200 sem chave de
      API e trazem o `ui.Chat` real.
- [ ] T002 `examples/cookbook/aichat.go` com as declarações da receita (compila em `go vet`)
      e o cliente roteirizado do teste.
- [ ] T003 `site/internal/chatdemo`, `site/public/chat-demo.js` e as duas rotas de demo.
- [ ] T004 A página em `en/cookbook/ai-chat.md` e `pt/receitas/chat-de-ia.md`, a linha nas duas
      tabelas de índice, a navegação em `docs.go`, o link em `learn/ai-and-agents.md` (en e pt).
- [ ] T005 `CHANGELOG.md`, `version` em `cmd/trilha/main.go`, linha do `ROADMAP.md`.
- [ ] T006 `make test` verde e `make release VERSION=0.115.0 ISSUES="186"`.

## Aceitação

- **SC-001** `/cookbook/ai-chat` e `/pt/receitas/chat-de-ia` respondem 200, aparecem na tabela
  do índice da seção e no `llms.txt` da locale.
- **SC-002** Todo bloco `go` das duas páginas é uma declaração de `examples/cookbook`
  (`TestCookbookSnippetsAreReal` continua verde).
- **SC-003** `/demos/ai-chat` e `/pt/demos/ai-chat` respondem 200 com `OPENAI_API_KEY` vazia e
  sem rede, e trazem `data-trilha-chat`, `ui.chat.js` e o script da demo.
- **SC-004** `learn/ai-and-agents.md` § "A chat, ready-made" (e o espelho pt) linka a receita.
- **SC-005** `make test` verde; `TestLocalesInSync` e `TestInternalLinksResolve` incluídos.
