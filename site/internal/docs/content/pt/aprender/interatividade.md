---
title: Interatividade
description: Trocar um pedaço da página e enviar formulário sem recarregar, com o mesmo handler que serve a página inteira.
---

Uma página do Trilha é HTML inteiro: o navegador navega, o servidor responde, a tela pisca.
Isso funciona bem, mas não em toda tela — filtrar uma lista ou salvar um formulário não
deveria custar uma recarga.

O caminho aqui é **fragmento**: o mesmo link e o mesmo formulário de sempre, com um atributo
a mais. Com JavaScript ligado, o kit `ui` pede a página, o servidor devolve só o pedaço e o
navegador troca aquele elemento. Sem JavaScript, o link navega e o formulário envia — o
servidor devolve a página inteira porque ninguém pediu fragmento. Nenhuma rota nova, nenhum
handler novo, nenhuma dependência.

## Uma pergunta a mais no handler

`c.Fragment()` devolve o id que o cliente quer trocar, ou `""` numa navegação normal:

```go
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Clientes")
	return tela(c, c.Query("q")), nil
}

// tela é a página inteira quando não há fragmento, e o pedaço quando há:
// o elemento trocado precisa carregar o mesmo id.
func tela(c *trilha.Ctx, q string) h.Node {
	return h.Div(h.ID("lista"),
		h.Form(h.Method("get"), h.Action("/clientes"), ui.Swap("lista"),
			ui.Input(h.Name("q"), h.Value(q)),
			ui.Submit(h.Text("Buscar")),
		),
		lista(clientes.Buscar(q)),
	)
}
```

Quando a requisição traz o cabeçalho `Trilha-Fragment`, o Trilha:

- **pula os layouts** da rota (nada de `<html>`, `<head>`, barra de navegação);
- escreve só os nós que você devolveu, sem o envelope do documento e sem o script do
  dev server;
- responde com `Vary: Trilha-Fragment`, para um cache não guardar o pedaço no lugar da
  página.

Tudo o mais continua igual: middleware roda, CSRF é verificado, o status é o que você
mandou. `c.Fragment()` é só uma pergunta.

## O link e o formulário

No HTML, `ui.Swap("id")` marca quem participa:

```go
ui.ButtonLink("/clientes?pagina=2", ui.Swap("lista"), h.Text("Próxima"))

h.Form(h.Method("post"), h.Action("/clientes"), ui.Swap("tela"),
	trilha.CSRFInput(c),
	// campos…
)
```

O `ui.js` intercepta o clique (só botão esquerdo, sem Ctrl/Cmd, mesma origem) e o envio,
faz um `fetch` com o cabeçalho, e troca o elemento pelo HTML que voltou. Enquanto espera,
o alvo ganha `aria-busy="true"` (o CSS do kit deixa o bloco opaco e o cursor de espera).
`ui.NoPush()` no link evita mexer no histórico.

## Depois do POST

Um `POST` que redireciona continua redirecionando — inclusive no fragmento. Como o `fetch`
seguiria o 303 sozinho e devolveria a página nova em pedaço, o Trilha responde
**204 com o cabeçalho `Trilha-Location`**, e o `ui.js` navega de verdade. O padrão
redirecionar-depois-de-gravar sobrevive.

Quando faz mais sentido ficar na mesma tela, responda com o pedaço atualizado:

```go
func POST(c *trilha.Ctx) error {
	in, errs := ler(c)
	if len(errs) > 0 {
		return c.Render(422, tela(c, in, errs, "")) // formulário com os erros
	}
	clientes.Criar(in)
	if c.Fragment() != "" {
		return c.Render(200, tela(c, clientes.Cliente{}, nil, "Cadastro salvo!"))
	}
	return c.Redirect("/clientes?ok=1")
}
```

Em **422** o `ui.js` põe o foco no primeiro campo com `aria-invalid="true"` — é o que o
navegador faria sozinho numa recarga. Fora disso, ele devolve o foco (e a posição do cursor)
ao campo que estava em uso, procurando pelo `id` ou pelo `name`.

## Quando o fragmento não dá certo

O kit **nunca deixa a tela travada**: se a resposta for 5xx, se a rede cair ou se o pedaço
vier sem o id esperado, ele desiste e faz a navegação de verdade — o link vira `location`,
o formulário vira `form.submit()`. O usuário vê a página recarregar; não vê um clique que
não fez nada.

## Depois da troca

Elementos novos entram já hidratados: `[data-ui-fade]` e `[data-ui-show-when]` voltam a
funcionar sozinhos. Se você tem comportamento próprio, ouça o evento:

```js
document.addEventListener("trilha:swap", (e) => {
  // e.detail.target = elemento novo, e.detail.status = status da resposta
});
```

`window.ui.swap(id, html, status)` e `window.ui.hydrate(el)` estão expostos para quem
precisar fazer a troca à mão.

## A ilha: o que o fragmento não faz

Fragmento vem sempre do servidor. Um editor com prévia ao vivo, um canvas, um mapa que
arrasta: o estado está no cliente e não há ida e volta a fazer. Isso é uma **ilha** — um
pedaço da página que traz o próprio módulo, com tudo em volta continuando HTML comum.

```go
c.Island("/editor.js", map[string]any{"ppm": 200},
	h.Class("editor"),
	ui.Textarea(h.Name("corpo")),               // o conteúdo de origem: ainda é um campo
	h.P(h.Data("info", ""), h.Hidden()),        // preenchido pelo módulo
)
```

```html
<div data-trilha-island="/editor.js?v=9c1f" data-trilha-props="{&quot;ppm&quot;:200}" class="editor">…</div>
```

O módulo é um ES module comum em `public/`, e a exportação padrão dele é a montagem:

```js
export default function (el, props) {
  const area = el.querySelector("textarea");
  area.addEventListener("input", () => { /* … */ });
}
```

Quatro coisas saem desse formato:

- **Os filhos são o conteúdo de origem, e quem os renderiza é o servidor.** Script bloqueado,
  ainda a caminho ou 404: a página é o que sempre foi. A ilha acrescenta, não sustenta.
- **As props são dado.** Vão escapadas num atributo e voltam pelo `JSON.parse` — um valor
  vindo do banco não vira marcação. Vale o que o `encoding/json` serializa; o que não
  serializa avisa no log e deixa o conteúdo de origem em paz.
- **Sem bundler e sem hidratação global.** O módulo é um arquivo em `public/`, endereçado
  pelo `Asset` (então a URL leva o hash do conteúdo), e só as ilhas presentes na página são
  montadas, cada uma uma vez. O carregador é um único script inline com o nonce da
  requisição, e é por isso que a CSP padrão o aceita sem `unsafe-inline`.
- **Uma ilha que chega dentro de um fragmento também monta**: o carregador ouve o
  `trilha:swap`. O que ele precisa é já estar na página — ou seja, a página renderizou ao
  menos uma ilha própria.

### A porta de saída

A ilha é a fronteira onde outra biblioteca entra, e onde o custo dela para. Web Components
não precisam de nada daqui — `customElements.define` e a tag é a ilha. Para Alpine, htmx ou
o que for, ponha o arquivo em `public/` e importe do módulo da ilha; para React, uma build
ESM em `public/` e um `createRoot(el)` dentro da montagem. A página em volta não é obrigada
a virar componente, e o resto do projeto não fica sabendo da escolha.

A CSP padrão é `script-src 'self'`, então módulo vindo de CDN é recusado até você abrir a
mão — decisão, não acidente.

## A página inteira, sem a recarga

O fragmento troca um pedaço da página que um handler escolheu. A navegação é a outra
metade: a próxima página é *outra* página, e o que não deveria piscar é tudo em volta — o
cabeçalho, a barra lateral, a rolagem de uma lista longa.

```go
// app/painel-/layout.go
return h.Section(h.Class("app"), ui.Navigate("conteudo"), ui.NavigateScript(c),
    ui.Sidebar(ui.Nav(
        ui.NavLink("/painel", "Painel", cur == "/painel"),
        ui.NavLink("/relatorio", "Relatório", cur == "/relatorio"),
    )),
    h.Div(h.Class("app-content"), children),
), nil
```

`ui.Navigate(id)` marca uma região: um clique em link da mesma origem dentro dela busca a
próxima página e troca o `#id` pelo mesmo elemento dela. `ui.NavigateScript(c)` carrega o
comportamento — arquivo separado do `ui.js`, para que um app que não navegue assim não o
baixe. No servidor não muda nada: `/relatorio` é a mesma rota, respondendo o mesmo
documento. Recarregar, abrir em outra aba ou chegar com o JavaScript desligado dá a mesma
página.

Desligada por padrão, e desligada por link:

```go
ui.ButtonLink("/relatorio.pdf", ui.NoNavigate(), h.Text("Baixar"))
```

O navegador mantém os costumes — Voltar e Avançar funcionam e restauram a rolagem da entrada
para onde voltam, `Cmd`-clique abre aba, `target` e `download` passam intactos. O kit
acrescenta `aria-busy` durante a espera, leva o foco para o que entrou e dispara
`trilha:swap`, então uma ilha dentro da página nova monta. Um segundo clique cancela a
primeira requisição; 5xx, redirecionamento ou página sem aquele id desiste e navega de
verdade.

A regra de bolso: **fragmento** quando um handler responde um pedaço, **navegação** quando a
resposta é uma página e a moldura em volta deve ficar.

## O que isso não é

Não é SPA. Não há roteador no cliente, estado compartilhado, hidratação de componente nem
*diff* de DOM — a troca é `outerHTML`, e a fonte da verdade continua sendo o servidor. Uma
tela que precise de estado local rico (um editor, um canvas) merece JavaScript próprio, e a
ilha acima é onde esse JavaScript mora; o fragmento resolve o caso comum, que é a maioria
das telas.

Vale lembrar o limite de segurança: o cabeçalho `Trilha-Fragment` é personalizado, então um
site de terceiros não consegue mandá-lo sem *preflight* — e o Trilha não responde
*preflight*. Um fragmento só sai para a sua própria origem.

O exemplo `examples/cadastro` usa os dois: busca que filtra a lista e cadastro que salva sem
recarregar, ambos funcionando com o JavaScript desligado.

## Desafio

Faça a lista trocar sozinha enquanto o usuário digita, sem esperar o botão — e sem disparar
uma requisição por tecla.

:::solucao
```js
let t;
document.addEventListener("input", (e) => {
  const campo = e.target.closest("form[data-trilha-target] input[name=q]");
  if (!campo) return;
  clearTimeout(t);
  t = setTimeout(() => campo.form.requestSubmit(), 250);
});
```
`requestSubmit()` dispara o mesmo evento `submit` que o kit já escuta, então o
`data-trilha-target` continua valendo — e o formulário segue funcionando no clique do botão
para quem não tem JavaScript.
:::
