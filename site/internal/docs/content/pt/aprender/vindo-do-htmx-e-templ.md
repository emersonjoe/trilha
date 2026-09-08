---
title: Vindo do htmx e do templ
description: Como se chama, no Trilha, cada peça de uma pilha Go + templ + htmx — e o que não tem equivalente.
---

Se você já escreve Go, renderiza HTML no servidor e troca pedaços da página, já tomou as duas
decisões sobre as quais este framework foi construído. Esta página não é um tutorial: é uma
tabela de tradução, mais a lista honesta do que falta.

## A tabela

| htmx + templ | Trilha |
|---|---|
| arquivos `.templ`, compilados por `templ generate` | `h.Div(h.Class("card"), h.Text(titulo))` — funções Go comuns, com escape por padrão, sem etapa de build e sem segunda linguagem |
| `templ.Component` como parâmetro | `h.Node` como parâmetro — mesma composição, mesmo aninhamento |
| `@templ.Raw(...)` | `h.Raw(...)`, gritante de propósito, com o mesmo aviso |
| `hx-get="/x" hx-target="#lista"` | `ui.Swap("lista")` num `<a>` ou `<form>` comum; a rota responde a mesma URL e o `c.Fragment()` diz ao handler qual pedaço foi pedido |
| `hx-post` num formulário | o `method="post"` do próprio formulário mais o `ui.Swap` — o método fica onde o HTML o põe |
| `hx-indicator="#spinner"` | `ui.Indicator("lista")`, com limiar para a resposta rápida não piscar |
| `hx-disabled-elt` | automático: um segundo gatilho para um alvo que já está no ar é ignorado |
| `hx-boost` | `ui.Navigate("id")` mais `ui.NavigateScript(c)` — por subárvore, não pelo documento inteiro |
| `hx-push-url="false"` | `ui.NoPush()` |
| `hx-confirm` | `ui.Confirm(título, descrição)` |
| `hx-trigger="every 2s"` | ainda não — veja abaixo |
| `hx-swap-oob` | ainda não — veja abaixo |
| Alpine `x-data` para estado local | `c.Island("/coisa.js", props, recuo)` — veja [O teto](/pt/aprender/o-teto) |
| `templ.WithNonce` e CSP ligados à mão | por padrão; `c.Nonce()` e `trilha.NonceAttr(c)` |
| middleware de CSRF que você escolheu e ligou | por padrão nas rotas de formulário; `trilha.CSRFInput(c)` |
| roteamento em `net/http` escrito por você, ou chi | pastas em `app/`, conferidas pelo compilador através de um arquivo gerado |

## O que não tem equivalente, e por quê

**Troca fora de banda (`hx-swap-oob`).** Uma resposta mudar um segundo elemento sem relação — o
contador do carrinho no cabeçalho quando você adiciona um item — não tem resposta hoje. O
contorno é o fragmento trocado conter os dois, ou trocar o pedaço que os contém. É uma falta
de verdade, e está registrada em vez de disfarçada.

**Gatilho por tempo (`hx-trigger="every 2s"`).** Não há poller declarativo. Para dado ao vivo a
resposta do framework é *server-sent events*, que é o que a recarga do `dev` já usa; para o
caso geral você escreve o `setInterval` numa ilha.

**Gatilho por evento qualquer.** O htmx dispara um pedido a partir de qualquer evento do DOM
com qualquer modificador (`keyup changed delay:500ms`). O Trilha dispara no que o HTML já
dispara: clique de link e envio de formulário. O resto é ilha.

**Recarga de template sem recompilar.** O `templ` recarrega um template sem recompilar Go. O
Trilha reconstrói o binário — cerca de um segundo num app de exemplo — porque as páginas
**são** Go. Você troca um pouco de latência no desenvolvimento pelo compilador conferindo cada
rota.

## O que você ganha que a pilha não tinha

- **Rotas a partir de pastas, conferidas pelo compilador.** `app/blog/slug_/page.go` vira rota
  registrada em Go gerado. Página cuja função tem assinatura errada é erro de compilação, não
  um 404 que você descobre em homologação.
- **`trilha check`** — um comando que roda geração, `gofmt`, `vet`, testes, auditoria de
  segurança e a conferência do OpenAPI, nessa ordem, parando na primeira falha.
- **Validação e erro por campo no mesmo handler que renderiza HTML.** `c.Bind(&in)` devolve
  `trilha.FieldErrors`; `ui.Errors(errs, "email")` põe a mensagem ao lado do campo, e o mesmo
  handler responde 422 com o formulário de volta.
- **Um kit `ui` sem dependência** — cerca de quarenta componentes tipados sobre CSS prefixado,
  copiados para o seu projeto para você editar.
- **Um binário no fim**, com `public/` embutido, que roda sem a CLI.

## O que continua igual

O instinto. Renderizar no servidor, mandar HTML, deixar o navegador fazer o que navegador faz,
e recorrer ao cliente só onde o cliente é de fato quem sabe. O Trilha não pede que você mude
isso — pede que você pare de ligar os fios à mão.

Se você está migrando uma aplicação de verdade, a [receita de migração](/pt/receitas/migracao)
trata de projeto Next.js; esta mudança é menor, porque a decisão difícil já está tomada.

## Desafio

Você está portando uma tela em que o htmx fazia `hx-get="/busca" hx-target="#resultados"
hx-trigger="keyup changed delay:300ms"`. O Trilha não tem gatilho por evento. Porte assim
mesmo, mantendo a busca funcionando para quem não tem JavaScript.

:::solution
O formulário é o gatilho, e o *debounce* é a única coisa que sobra para escrever:

```go
h.Form(h.Method("get"), h.Action("/busca"), ui.Swap("resultados"),
    ui.Input(h.Name("q"), h.Value(q)),
    ui.Submit(h.Text("Buscar")),
    ui.Spinner(ui.Indicator("resultados")),
)
h.Div(h.ID("resultados"), linhas())
```

```js
let t;
document.addEventListener("input", (e) => {
  const campo = e.target.closest('form[data-trilha-target] input[name=q]');
  if (!campo) return;
  clearTimeout(t);
  t = setTimeout(() => campo.form.requestSubmit(), 300);
});
```

Três coisas vieram de graça. A URL continua carregando `?q=`, porque é um formulário `GET`,
então o resultado é linkável e o Voltar funciona. O botão de enviar continua lá, então a tela
funciona com o script bloqueado. E o `ui.Indicator` só mostra o spinner se a resposta demorar
mais que o limiar, o que numa busca local é nunca — justamente o piscar que quem usa htmx
tenta evitar ajustando o `delay:`.
:::
