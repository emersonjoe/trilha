# Spec 137 — receita: um agente que age sobre os dados do app

- **Issue**: [#187](https://github.com/emersonjoe/trilha/issues/187) — a issue é a fonte do
  escopo; os oito passos da página estão lá e não são recopiados aqui.
- **Branch**: `137-cookbook-agente-ia`
- **Versão**: 0.116.0

## Por quê

A [receita do chat](../136-cookbook-chat-ia/spec.md) fechou a pergunta do primeiro dia: como
colocar uma conversa dentro do app. A do segundo dia é outra — **o agente não conversa, ele
age**. Quem chega aqui já tem o chat na tela e para em quatro perguntas que nenhuma página
responde inteira: como o agente lê os meus dados sem enxergar tudo, o que ele pode fazer
sozinho e o que espera uma pessoa, como eu vejo depois o que ele fez, e como testo isso sem
gastar chamada.

Cada peça existe em algum lugar — `learn/ai-and-agents.md` mostra `ai.Agent` com uma
ferramenta e um handoff, `cookbook/api-as-tools.md` mostra a API virando ferramentas MCP,
`reference/ai.md` lista `Parallel`, `Chain`, `AsTool`, `reference/approval.md` descreve a
fila. Nenhuma monta as peças sobre um domínio: a pessoa junta sozinha, e a junção é onde
moram as decisões que custam caro depois — a ferramenta que lê sem o dono na consulta, a
escrita que o modelo executa sem ninguém no meio, o rastro que não existe no dia da pergunta.

## O que muda

Só documentação e o código que a sustenta. Nenhum símbolo novo no runtime, nenhuma convenção
nova em `app/`, nenhuma mudança de API pública.

1. **`site/internal/docs/content/en/cookbook/ai-agent.md`** e o espelho
   `pt/receitas/agente-de-ia.md`: a receita nos oito passos da issue — ferramentas de leitura,
   escrita com pessoa no meio, dois agentes, streaming, rastro, teste, MCP e demo. Entra na
   tabela de `cookbook/index.md` nas duas locales, na navegação (`site/internal/docs/docs.go`)
   e, por consequência, no `llms.txt`.
2. **`examples/cookbook/aiagent.go`**: as declarações que a página cita, compiladas por
   `go vet ./...` como as das outras receitas, e `aiagent_test.go` com o cliente roteirizado.
3. **`site/internal/agentdemo` + `site/public/agent-demo.js`** e as rotas
   `site/app/demos/ai-agent/page.go` e `site/app/pt/demos/ai-agent/page.go`: a demo viva,
   roteirizada no navegador, sem chave e sem rede, mostrando os passos e a fila.
4. **`learn/ai-and-agents.md` § "Agents" e § "Multi-agent"** (e o espelho pt) apontam para a
   receita; **`cookbook/api-as-tools.md`** aponta para o passo 1, que é o mesmo `Probe` do
   lado do agente.

```go
// A ferramenta de escrita não escreve: ela abre a fila e diz que abriu.
func AIAgentCancelTool(c *trilha.Ctx) *ai.Tool {
	return ai.NewTool("cancel_order", "Ask a person to cancel an order. …",
		ai.Schema(`{"type":"object","properties":{"order_id":{"type":"string"},"reason":{"type":"string"}},"required":["order_id","reason"]}`),
		ai.Typed(func(ctx context.Context, in struct{ OrderID, Reason string }) (string, error) {
			o, ok := AIAgentFindOrder(AIAgentWho(c), in.OrderID)
			if !ok {
				return "no such order for this customer", nil
			}
			id, err := AIAgentApprovals.Open(c, approval.Request{
				Kind: AIAgentCancelKind, Subject: "Cancel order " + o.ID, Target: "/orders/" + o.ID,
				Assign: approval.Role("support"), Due: time.Now().Add(AIAgentCancelDue),
				Data: map[string]string{"order_id": o.ID, "reason": in.Reason, "asked_by": "agent"},
			})
			…
			return "cancellation " + id + " is waiting for a person; nothing has changed yet", nil
		}))
}
```

### Decisões que a receita registra

- **O usuário logado entra na ferramenta, não no prompt.** Cada ferramenta é construída por
  requisição com o `*trilha.Ctx` dentro: a consulta leva o dono junto (`AIAgentWho(c)`), então
  o pedido de outra pessoa não é achado — a permissão é a consulta, não uma frase nas
  instruções. É a mesma decisão da receita do chat, aplicada agora à leitura de vários
  registros.
- **A ferramenta sobre a própria API não redeclara a API.** `App.Route` confere no arranque
  que a rota existe (uma ferramenta apontando para rota inexistente é erro de digitação que
  só aparece no dia da pergunta) e `App.Probe` responde, com os cabeçalhos de quem chamou, se
  aquela pessoa poderia chamar aquela rota — antes de chamar. Recusado é ferramenta que não
  aparece na lista, e não ferramenta que responde 403: um modelo não deve planejar em cima de
  uma operação que nunca vai poder executar. É o `cookbook/api-as-tools` do lado do agente.
- **Escrita é pedido, não ação.** A ferramenta de cancelar abre um `approval.Request` e
  devolve ao modelo a frase "está esperando uma pessoa; nada mudou ainda". Quem escreve no
  store é o `On(kind, fn)`, depois de uma pessoa decidir no `ui.Inbox`. O teste da receita é
  exatamente esse: o store não muda e a fila cresce.
- **Handoff é para quando o roteiro é do modelo.** Triagem → financeiro com `Handoffs` quando
  quem decide é a conversa; `Chain` e `Parallel` quando a ordem é do app — e a receita diz que
  a segunda é a resposta certa mais vezes do que parece, porque um passo que o app conhece não
  precisa que o modelo lembre de delegar.
- **O rastro é por ferramenta chamada, não por conversa.** `c.Audit` dentro do embrulho que
  toda ferramenta recebe (`AIAgentAudited`) grava ação, alvo e argumentos; o `Usage` do
  `Result` vira um registro no fim do turno. Sem isso, a pergunta "o que o agente fez ontem"
  não tem resposta, e ela sempre chega.
- **O teste é um roteiro de chamadas de ferramenta.** O `AIAgentFake` devolve, em ordem, as
  respostas do provedor: primeiro uma com `tool_calls`, depois o texto final. `MaxTurns` é o
  cinto — um modelo que insiste em chamar a mesma ferramenta para o loop com `ErrMaxTurns` em
  vez de parar a fatura.

## Fora de escopo

- **Símbolo novo no `ai` para "ferramenta que pede aprovação"** — o embrulho tem cinco linhas
  e a decisão de quem aprova é do app; um `ai.Approve` escolheria a fila por quem usa.
- **Persistir a fila** — `approval.Memory()` continua sendo o padrão da receita, com a linha
  que diz o que muda quando vira tabela.
- **Mexer em `examples/assistente`** — ele continua sendo o app completo com MCP e tela de
  configuração; a receita é o caminho curto e as duas páginas se apontam.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `examples/cookbook` e a demo usam apenas a stdlib e o próprio módulo; `TestNoExternalDeps` segue verde. |
| VI — teste primeiro | O teste da página nas duas locales, da demo em 200 sem rede e do "cancelar cai na fila e não no store" entra antes do conteúdo. |
| VII — segurança por padrão | O dono na consulta, o `Probe` antes da chamada, a escrita atrás de aprovação, o `c.Audit` por ferramenta e o `MaxTurns` como teto de gasto. |
| Idioma | Página em inglês e em pt-BR no mesmo commit; código, identificadores e comentários em inglês. |

## Tarefas

- [ ] T001 Teste que falha: `/cookbook/ai-agent` e `/pt/receitas/agente-de-ia` respondem e
      estão na tabela do índice e nos capítulos que apontam para eles; `/demos/ai-agent` e
      `/pt/demos/ai-agent` respondem 200 sem chave e sem rede.
- [ ] T002 Teste que falha em `examples/cookbook/aiagent_test.go`: com um cliente roteirizado,
      "cancelar o pedido 1043" chama a ferramenta, o pedido continua aberto no store e a fila
      tem um registro `pending`.
- [ ] T003 `examples/cookbook/aiagent.go` com as declarações da receita (compila em `go vet`).
- [ ] T004 `site/internal/agentdemo`, `site/public/agent-demo.js` e as duas rotas de demo.
- [ ] T005 A página em `en/cookbook/ai-agent.md` e `pt/receitas/agente-de-ia.md`, a linha nas
      duas tabelas de índice, a navegação em `docs.go`, os links em `learn/ai-and-agents.md` e
      em `cookbook/api-as-tools.md` (en e pt).
- [ ] T006 `CHANGELOG.md`, `version` em `cmd/trilha/main.go`, linha do `ROADMAP.md`.
- [ ] T007 `make test` verde e `make release VERSION=0.116.0 ISSUES="187"`.

## Aceitação

- **SC-001** `/cookbook/ai-agent` e `/pt/receitas/agente-de-ia` respondem 200, aparecem na
  tabela do índice da seção e no `llms.txt` da locale.
- **SC-002** Todo bloco `go` das duas páginas é uma declaração de `examples/cookbook`
  (`TestCookbookSnippetsAreReal` continua verde).
- **SC-003** `/demos/ai-agent` e `/pt/demos/ai-agent` respondem 200 com `OPENAI_API_KEY` vazia
  e sem rede, e trazem o `ui.Chat` com passos e a fila na tela.
- **SC-004** O teste do snippet prova que "cancelar" abre aprovação `pending` e não toca no
  store; um roteiro que só chama ferramenta bate em `ai.ErrMaxTurns`.
- **SC-005** `learn/ai-and-agents.md` §§ "Agents"/"Multi-agent" e `cookbook/api-as-tools.md`
  (e os espelhos pt) linkam a receita.
- **SC-006** `make test` verde; `TestLocalesInSync` e `TestInternalLinksResolve` incluídos.
