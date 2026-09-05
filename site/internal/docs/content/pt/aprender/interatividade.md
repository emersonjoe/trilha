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

## O que isso não é

Não é SPA. Não há roteador no cliente, estado compartilhado, hidratação de componente nem
*diff* de DOM — a troca é `outerHTML`, e a fonte da verdade continua sendo o servidor. Uma
tela que precise de estado local rico (um editor, um canvas) merece JavaScript próprio; o
fragmento resolve o caso comum, que é a maioria das telas.

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
