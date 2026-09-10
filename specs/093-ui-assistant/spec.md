# Spec 093 — ui.Assistant: o assistente no canto, sem virar SPA

- **Issue**: [#142](https://github.com/emersonjoe/trilha/issues/142) — a issue é a fonte do escopo.
- **Branch**: `093-ui-assistant`
- **Versão**: 0.74.0

## Por quê

O `ui.Chat` resolve a conversa **dentro** de uma página. O formato que todo app administrativo
pede é o outro: um botão fixo no canto que abre um painel sobre a tela, presente na área
autenticada inteira, com a dica mudando conforme a rota e o contexto da página indo junto com a
pergunta.

No Verba isso são ~150 linhas de TSX. Reescrever launcher, dialog, foco, Escape, `aria-expanded`,
estado de envio, histórico e streaming em cada app custa muito e produz uma acessibilidade
diferente em cada um — que é exatamente o tipo de coisa que um kit existe para não repetir.

## O que muda

**Três peças pequenas, nenhuma delas um segundo protocolo de chat.**

**1. `ui.ChatOpts.Context`** — campos opacos que o servidor define e que viajam com a mensagem:

```go
ui.ChatOpts{Action: "/api/chat", Context: map[string]string{"folha": id, "rota": c.URL().Path}}
```

Viram `<input type=hidden>` dentro do formulário do chat, em ordem de nome. Com script, o
`ui.chat.js` os manda no corpo JSON, em `context`; sem script, eles são campos de formulário como
qualquer outro. Um caminho só, e o valor nunca é HTML — é atributo escapado por construção.

**2. `ai.ServeOpts.Context`** — onde esses campos chegam:

```go
ai.ServeOpts{Context: func(c *trilha.Ctx, campos map[string]string) []ai.Message {
	return []ai.Message{{Role: "user", Content: "A folha aberta é a " + campos["folha"] + "."}}
}}
```

O que a função devolve entra na frente do histórico. O framework não guarda conversa nem inventa
onde a página termina e o modelo começa: quem decide o que o contexto vira é o app.

**3. `ui.Assistant`** — a composição:

```go
ui.Assistant(c, ui.AssistantOpts{
	Action: "/api/assistente",
	Page:   "/painel/assistente",           // a conversa como página, para quem está sem JS
	Label:  "Assistente",
	Hint:   "Perguntando sobre a folha " + id,
	Chat:   ui.ChatOpts{Context: map[string]string{"folha": id}},
})
```

Sai um `<a>` fixo no canto e um `<dialog>`. **Sem JavaScript o link é um link**: leva à página de
conversa, que é a mesma rota respondendo inteira. **Com JavaScript** ele abre o diálogo sobre a
tela — `showModal()`, que é foco preso e Escape de graça, do navegador — e o `aria-expanded` do
botão acompanha. Dois assistentes na mesma página têm ids independentes, porque tudo é derivado
do `ID`.

Monta-se **uma vez no layout**, e não classifica página nenhuma como ilha: é HTML do servidor com
o script do kit que já estava lá.

## Fora de escopo

- **Guardar a conversa.** O histórico é o que a página trouxe, como no `ui.Chat`. Sessão de
  conversa é do app, e é o que a issue pede que o framework não faça.
- **Um segundo protocolo.** O `ui.Assistant` renderiza um `ui.Chat`; o streaming, os erros e o
  Markdown são os que já existem.
- **Arrastar, redimensionar, lembrar aberto entre páginas.** São preferências, não acessibilidade,
  e cada uma pede um lugar para guardar estado que o framework não tem.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | HTML, CSS e o `ui.js` que já existe |
| VII — segurança por padrão | o contexto é atributo escapado, e o CSRF é o do formulário do chat |
| Convenção nova | rota no `examples/blog` (área autenticada) e teste de integração |
| Acessível sem script | o launcher é um link para uma página que funciona |

## Tarefas

- [x] T001 Teste que falha: launcher com `aria-expanded`, ids independentes, contexto no form
- [x] T002 `ChatOpts.Context` e os campos escondidos, com o `ui.chat.js` mandando no corpo
- [x] T003 `ai.ServeOpts.Context`, lendo do JSON e do formulário
- [x] T004 `ui.Assistant` e o CSS do launcher e do painel
- [x] T005 `aria-expanded` no `ui.js`, e o link que vira botão quando há script
- [x] T006 `examples/blog`: assistente na área autenticada, com contexto da rota, e a página sem JS
- [x] T007 Documentação (en + pt) e catálogo do kit
- [x] T008 `CHANGELOG.md`, `version`, `ROADMAP.md`, `make test`, `scripts/release.sh 0.74.0`

## Aceitação

- **SC-001** O launcher tem `aria-expanded`, `aria-controls` e abre o `<dialog>` com foco e Escape.
- **SC-002** Sem JavaScript, o link abre uma página de conversa que funciona.
- **SC-003** Os campos de contexto chegam à Action pelos dois caminhos — JSON e formulário — e
  nenhum deles aparece como HTML.
- **SC-004** Dois assistentes na mesma página não compartilham id nenhum.
- **SC-005** O exemplo autenticado mostra o contexto derivado da rota, e o teste prova que ele
  chegou.
