# Spec 154 — UI operacional para produtos e agentes

- **Issues**: [#248](https://github.com/emersonjoe/trilha/issues/248) e
  [#249](https://github.com/emersonjoe/trilha/issues/249) — as issues são a fonte do escopo.
- **Branch**: `codex/154-ui-feedback`
- **Versão**: 0.133.0

## Por quê

O kit `ui` já entrega componentes, ícones, diálogos, campos e avisos, mas duas partes do contrato
continuam implícitas. Ferramentas e agentes precisam ler código ou documentação livre para descobrir
o catálogo, enquanto produtos precisam reinventar cores operacionais e o ciclo de erro de cada
formulário assíncrono.

O segundo problema apareceu na homologação real do Trilha Cloud: depois de um `await`, o
`Event.currentTarget` já não aponta para o formulário. O erro de uma operação executada dentro de
um diálogo então cai no toast global, visualmente atrás do modal. Cada aplicação pode corrigir isso
sozinha, mas a repetição deixa acessibilidade, foco e estado pendente inconsistentes.

## O que muda

- `trilha ui components [--json]`, `trilha ui icons [--json]` e
  `trilha inspect api ui.Component` tornam o catálogo do kit consultável por pessoas e agentes.
- `ui.theme.css` passa a expor tokens semânticos para sucesso, alerta, informação, destrutivo e
  sombras, em temas claro e escuro.
- `ui.FormError()` renderiza o ponto estável de feedback de um formulário.
- O runtime expõe `ui.formError(form, message, options)`, `ui.clearFormErrors(form)` e
  `ui.formPending(form)`. O erro fica dentro do formulário e do diálogo aberto, usa `role=alert`,
  recebe foco e pode marcar um campo com `aria-invalid`. `formPending` marca `aria-busy`, desabilita
  submits e devolve uma função que restaura o estado anterior.
- O site documenta o contrato em inglês e português com um exemplo copiável de submissão
  assíncrona e uma demonstração do feedback dentro de `ui.Dialog`.
- O Trilha Cloud passa a carregar o runtime publicado pelo framework e usa essas funções nos seus
  formulários, mantendo regras e mensagens específicas no produto.

```go
form := h.Form(
    h.ID("profile-form"),
    ui.FormError(),
    ui.Field("email", "E-mail", ui.Input(h.ID("email"), h.Name("email"))),
    ui.Submit(h.Text("Salvar")),
)
```

```js
form.addEventListener("submit", async (event) => {
  event.preventDefault();
  const form = event.currentTarget;
  ui.clearFormErrors(form);
  const settled = ui.formPending(form);
  try {
    await save(new FormData(form));
  } catch (error) {
    ui.formError(form, error.message, { field: form.elements.email });
  } finally {
    settled();
  }
});
```

## Fora de escopo

- Transformar toda submissão em requisição automática; o produto continua dono do endpoint,
  payload e tratamento de sucesso.
- Mover regras de produto, polling de execução, evidências ou publicação do Trilha Cloud para o
  framework.
- Substituir imediatamente todo o design system próprio do Trilha Cloud pelo kit `ui`.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | o componente é `h.Node` e o runtime usa apenas APIs do navegador |
| III — coerente com Go | o símbolo Go é pequeno; comportamento progressivo fica no asset do kit |
| VI — teste primeiro | cada interface pública nasce de teste no `ui` e é validada novamente no Trilha Cloud |
| VII — segurança por padrão | mensagens entram por `textContent`; nenhuma resposta remota vira HTML |
| Idioma | referência e exemplo são publicados em inglês e português |

## Segurança e privacidade

- Mensagens vindas de APIs são tratadas como texto, nunca como HTML.
- O helper não registra payloads, credenciais ou valores de campos.
- O estado pendente preserva controles originalmente desabilitados.
- Links de ação são criados apenas com `href` e texto fornecidos explicitamente pelo chamador.

## Tarefas

- [x] T001 Abrir as issues de catálogo/tokens e feedback assíncrono.
- [x] T002 Registrar a spec 154 sobre a `origin/main` atual.
- [x] T003 Publicar `ui.FormError` e o runtime de erro/estado pendente com testes.
- [x] T004 Atualizar a referência e os exemplos bilíngues do site.
- [x] T005 Migrar e homologar o Trilha Cloud usando o runtime do framework.
- [x] T006 Atualizar versão, changelog e roadmap; executar a suíte e o fluxo de release.

## Aceitação

- [x] Agentes descobrem componentes, ícones e símbolos sem interpretar o código-fonte.
- [x] Produtos usam tokens semânticos iguais nos temas claro e escuro.
- [x] Um erro assíncrono dentro de diálogo permanece visível dentro dele.
- [x] O campo associado recebe foco e `aria-invalid` sem inserir HTML vindo da API.
- [x] O botão de envio e `aria-busy` são restaurados mesmo quando a operação falha.
- [x] O site mostra o uso completo em inglês e português.
- [x] A homologação UI-to-UI do Trilha Cloud permanece verde usando o runtime do framework.

## Evidência

- `make test` e `make security` passaram no framework em 2026-09-15; `govulncheck` não encontrou
  vulnerabilidades.
- A homologação UI-to-UI do Trilha Cloud passou com 15 formulários, 27 ações e 10 links internos
  validados, incluindo conflito 409 visível dentro do diálogo, detalhes técnicos, evidências,
  publicação com conflito remoto, Kanban e responsividade.
