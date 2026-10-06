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
**204 com o cabeçalho `Trilha-Location`**. O padrão redirecionar-depois-de-gravar
sobrevive.

O que o kit faz com ele depende da página. Numa página com região `ui.Navigate` — um app que
já navega no lugar — ele **segue o redirect no lugar**: busca o destino, troca a região, põe o
endereço dele na barra como entrada nova e mostra o `c.Flash` num toast, uma vez. Voltar leva
ao formulário por `GET`; nunca reenvia. Fora disso, carrega o destino de verdade, como antes.
`ui.Follow()` num gatilho (ou num formulário em volta de vários) liga o seguir sem região — o
mesmo alvo é pedido de novo no endereço novo — e `ui.NoFollow()` desliga. O `ui.UploadTo`
segue do mesmo jeito.

Quando o destino tem outra moldura — entrar, sair, trocar de organização ou de idioma, tudo
que muda o cabeçalho e o menu fora da região —, responda `c.RedirectReload("/…")`: o mesmo 303
sem JavaScript, e uma carga de verdade com ele. Um destino que não tem a região também carrega
inteiro, e o aviso vai junto.

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

Responder o fragmento no próprio `POST` economiza a segunda ida, e deixa a barra onde estava.
Diga o endereço do que você desenhou, e o kit o põe lá:

```go
doc := documentos.Criar(in)
c.PushURL("/documentos/" + doc.ID)        // uma entrada nova no histórico
return c.Render(200, painel(c, doc))
```

`c.ReplaceURL(caminho)` põe o endereço na entrada que o gatilho criou em vez de uma nova — um
link para `?aba=marcos&x=1` cujo endereço canônico é `?aba=marcos` —, e `c.ReplaceURL("")`
deixa a barra como está. O contrato é o do PRG: o endereço é um `GET` que desenha esta tela,
então recarregar, Voltar e um link compartilhado a mostram de novo. Os dois aceitam um caminho
deste site, como o `Redirect`, e não escrevem nada numa página inteira. Um formulário `GET`
com `ui.Swap` substitui a própria entrada por padrão (dez letras digitadas não são dez páginas
para voltar); `ui.PushHistory()` dá uma entrada por busca, e Voltar desfaz o filtro.

## Enquanto o servidor responde

Uma troca que leva 40 ms deveria deixar a página exatamente como estava; uma que leva dois
segundos deveria dizer isso. O `ui.Indicator` marca o elemento que aparece enquanto um alvo
espera, e ele só aparece depois que o pedido passou de um limiar — 120 ms por padrão:

```go
h.Form(h.Method("get"), ui.Swap("lista"),
    ui.Input(h.Name("q")),
    ui.Submit(h.Text("Buscar")),
    ui.Spinner(ui.Indicator("lista")),   // escondido até a espera valer menção
)
h.Div(h.ID("lista"), linhas())
```

O limiar é o ponto. Mostrar um spinner em toda resposta é o piscar que as pessoas escrevem CSS
para esconder; mostrá-lo só nas lentas é informação. `ui.PendingAfter(300)` no gatilho muda o
limiar, e vários indicadores podem observar o mesmo alvo — um spinner ao lado do botão, uma
barra no cabeçalho.

Enquanto um alvo espera, três elementos ficam com `data-trilha-pending`: o alvo, o gatilho e
todo indicador daquele alvo. O alvo também ganha `aria-busy`, então um leitor de tela é
avisado sem nenhuma estilização sua. `trilha:pending` e `trilha:settled` disparam no
`document` para o que o CSS não resolve.

Uma segunda escrita — `POST`, `PUT`, `PATCH`, `DELETE` — num alvo que já tem uma no ar é
ignorada, então o salvar não sai duas vezes. Uma segunda *leitura* é a intenção mais nova e
vence: o `GET` no ar é abortado, então clicar em "2" e depois em "3" num paginador lento mostra
a página 3, com a 3 na barra de endereço e uma entrada nova no histórico. As marcas de espera
ficam até o último pedido assentar. Não há nada para ligar em nenhum dos casos.

Onde o navegador tem `startViewTransition`, a substituição faz *crossfade* em vez de pular;
onde não tem, ou onde o sistema pede menos movimento, nada muda. O `ui.NoTransition()`
desliga num gatilho.

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

Logo antes de o elemento antigo sair, dispara `trilha:before-swap` com `e.detail.target`
(ainda na página), `e.detail.id` e, numa navegação, `e.detail.url` — o lugar de parar um timer
ou um observer que o seu script começou. Monte no `trilha:swap`, filtrando por
`detail.target`; desmonte no `trilha:before-swap`; o que tem estado próprio é ilha.

Os scripts que vêm dentro do elemento novo rodam, como rodariam na página inteira: cada
`<script src>` da mesma origem uma vez por URL por documento — o runtime de uma ilha, o
`ui.LiveScript`, um arquivo do seu app. Um `<script>` inline que veio numa resposta nunca
roda: executar HTML de resposta é o caminho do XSS, e a CSP recusaria de qualquer jeito.
Ponha o código num arquivo em `public/`.

`window.ui.swap(id, html, status)`, `window.ui.hydrate(el)` e `window.ui.activate(el)` (os
scripts acima) estão expostos para quem precisar fazer a troca à mão.

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
  montadas, cada uma uma vez. Quem monta é o `public/ui.island.js`, um arquivo do kit ligado
  por `<script src>` — não há script inline de ilha, então `script-src 'self'` basta para a
  CSP, e o runtime é cacheado como qualquer outro asset. Projeto que usa ilha sem esse
  arquivo leva um crítico do `trilha check`, porque a falha é silenciosa de outro jeito: o
  conteúdo de origem aparece e nada acontece.
- **Uma ilha que chega dentro de um fragmento ou de uma navegação no cliente também monta.**
  O runtime ouve o `trilha:swap`; quando ele ainda não está na página, o kit roda a tag que
  veio com o conteúdo novo, porque um `<script>` escrito por `outerHTML` nunca roda.

### A porta de saída

A ilha é a fronteira onde outra biblioteca entra, e onde o custo dela para. Web Components
não precisam de nada daqui — `customElements.define` e a tag é a ilha. Para Alpine, htmx ou
o que for, ponha o arquivo em `public/` e importe do módulo da ilha; para React, uma build
ESM em `public/` e um `createRoot(el)` dentro da montagem. A página em volta não é obrigada
a virar componente, e o resto do projeto não fica sabendo da escolha.

A CSP padrão é `script-src 'self'`, então módulo vindo de CDN é recusado até você abrir a
mão — decisão, não acidente.

A ilha também é para onde se vai quando o modelo de trocas acaba — arrastar, edição
colaborativa, qualquer coisa cuja verdade mora no navegador enquanto a pessoa age.
[O teto](/pt/aprender/o-teto) é sobre reconhecer esse momento, e sobre as regras que impedem
uma ilha de virar uma SPA sem ninguém perceber.

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

Um formulário dentro da região também navega no lugar, sem mudança na rota dele. Um `GET` (um
filtro, uma busca) é um link com query: um endereço novo, e Voltar devolve o filtro anterior.
Um `POST` vai como o navegador mandaria — sem o cabeçalho de fragmento —, então a rota responde
o que responde sem JavaScript: o 303 é seguido no mesmo pedido, o destino entra na região, o
endereço dele vai para a barra e o `c.Flash` vira toast; a página do 422 entra com o foco no
primeiro campo inválido e o endereço intacto. Voltar nunca reenvia. Resposta que não é HTML,
que é download, ou destino sem a região, é do navegador: o formulário é enviado de verdade.
`ui.NoNavigate()` num formulário o deixa de fora; formulários com `ui.Swap` ou `ui.UploadTo`
são dos próprios scripts. Um link para um endereço que redireciona também é um `GET` só, e o
aviso do destino aparece.

O navegador mantém os costumes — Voltar e Avançar funcionam e restauram a rolagem da entrada
para onde voltam, `Cmd`-clique abre aba, `target` e `download` passam intactos. O kit
acrescenta `aria-busy` durante a espera, roda os scripts que a região nova traz e dispara
`trilha:swap`, então uma ilha, um `ui.Defer` ou um `ui.Poll` dentro da página nova funcionam.
Um segundo clique cancela a primeira requisição. Um redirecionamento é seguido no mesmo pedido
e o endereço do destino vai para a barra; 5xx, resposta que não é HTML ou página sem aquele id
desiste e navega de verdade.

A carga de uma página inteira é anunciada pelo leitor de tela; uma troca não é, então o kit
anuncia: o foco vai para o primeiro `h1` da região nova, e uma região viva visualmente oculta,
`#trilha-route-announcer`, lê o `<title>` novo (ou o título da seção, quando o `<title>` não
mudou) — e fica quieta quando o `h1` que recebeu o foco já disse o mesmo. Voltar e Avançar
anunciam do mesmo jeito. `ui.NavigateFocus("region")` ao lado do `ui.Navigate` mantém o foco
na região, e `"none"` deixa onde estava.

### Pedir antes do clique

Uma ida e volta de 200 ms ainda aparece quando é o clique que a começa. `ui.Prefetch()` ao lado
do `ui.Navigate` (ou num link) pede a próxima página assim que a pessoa mostra a intenção de
abri-la — o ponteiro parado no link por 80 ms, o foco do teclado chegando nele, um dedo tocando —,
e o clique encontra a resposta lá, ou ainda a caminho e compartilhada. Nunca por viewport, que é
onde mora o tráfego inútil; nunca num link `ui.NoPrefetch()`, num download, em outra origem ou
numa conexão em economia de dados ou 2G. Uma resposta espera 10 segundos pelo clique
(`ui.PrefetchTTL(ms)` muda), oito no máximo; redirecionamento ou qualquer coisa que não seja uma
página `200` não é guardada, e o clique pede por conta própria. O `trilha:swap` diz
`detail.prefetched`, para o app medir o ganho.

O pedido diz `Purpose: prefetch`, e o `c.IsPrefetch()` lê esse cabeçalho — e o `Sec-Purpose` do
próprio navegador. Uma rota cuja leitura tem efeito colateral que pode ser pulado (um contador de
visualizações) confere. O clique usa a resposta antecipada sem pedir de novo, então uma leitura
que precisa ser registrada toda vez — uma trilha de acesso que a lei exige — deixa os links dela
de fora com `ui.NoPrefetch()`.

### Um painel ao lado da página

"O documento aberto ao lado da lista, o próximo a um clique" é um `ui.Sheet`: um painel cujo
corpo carrega sob demanda e que fica aberto enquanto a região navega.

```go
// app/layout.go — fora da região ui.Navigate
ui.Sheet(c, "leitor", ui.SheetOpts{Title: "Documento"}), ui.SheetScript(c),

// a lista, dentro dela
h.A(h.Href("/documentos/"+d.ID), ui.SheetOpen("leitor"), h.Text(d.Nome))

// app/documentos/id_/page.go — uma rota, o pedaço e a página
if c.Fragment() == "leitor-body" {
	return ui.SheetBody("leitor", leitor(c, doc)), nil
}
return h.Div(ui.H1(h.Text(doc.Nome)), leitor(c, doc)), nil
```

O link abre o painel, marca o corpo como pendente e pede ao próprio endereço o fragmento
`leitor-body`; outro link troca só o corpo, e o endereço não muda — um painel é um momento
(`SheetOpts.Push` o torna uma entrada, e Voltar fecha). O painel fica fora da região, então um
`ui.Preview` dentro dele não recarrega enquanto a lista ao lado navega. Fechar esvazia o corpo,
que é o que libera um quadro pesado. O foco vai para o título do painel; Esc e o botão de fechar
devolvem o foco ao link, cujo `aria-expanded` acompanha. Sem `Modal`, ele não prende o foco — é
conteúdo complementar, um `<aside>`; `SheetOpts.Modal` o torna um `<dialog>`. Abaixo de 768px ele
cobre a tela e fecha a gaveta do shell. Sem JavaScript o link leva à página do documento; o
`SheetOpts.Open` desenha o painel aberto, para uma lista que lê `?painel=…`.

Um diálogo carrega o corpo do mesmo jeito: `ui.DialogTrigger("ver", ui.Swap("ver-body"),
h.Href("/documentos/7"))` é um link que abre o `ui.Dialog("ver", …)` e troca o `#ver-body`, sem
mexer no endereço.

A regra de bolso: **fragmento** quando um handler responde um pedaço, **navegação** quando a
resposta é uma página e a moldura em volta deve ficar.

## O arquivo, e a barra que diz onde ele está

Mandar arquivo é o único lugar onde "a tela pisca" não é o problema — o problema é não
acontecer nada por trinta segundos. O navegador sabe quanto já subiu; ele só não tem como
dizer isso num envio de formulário comum.

```go
// app/anexos/page.go
h.Form(h.Method("post"), h.Action("/anexos"), h.Enctype("multipart/form-data"),
	ui.UploadTo("lista"),
	trilha.CSRFInput(c),
	ui.Field("arquivo", "Arquivo", ui.Input(h.ID("arquivo"), h.Name("arquivo"), h.Type("file"), h.Required())),
	ui.UploadBar(),
	ui.Submit(h.Text("Enviar")),
)
```

`ui.UploadTo(id)` envia o formulário por XHR e troca o `#id` pela resposta; `ui.UploadBar()`
é o `<progress>` que o kit preenche com o evento de progresso do próprio navegador; e
`ui.UploadScript(c)` carrega o comportamento — arquivo próprio de novo, para que uma página
sem upload não o baixe. Com o JavaScript desligado nada disso existe, e o formulário é o que
sempre foi: envia, o servidor responde, a página recarrega.

No servidor não há API nova. A requisição leva o `Trilha-Fragment`, então o mesmo handler que
desenha a página responde o pedaço:

```go
func POST(c *trilha.Ctx) error {
	if err := c.FormErr(); err != nil {
		return err
	}
	f, hdr, err := c.Request().FormFile("arquivo")
	if err != nil {
		return err
	}
	defer f.Close()
	anexos.Add(hdr.Filename, hdr.Size)
	if c.Fragment() != "" {
		return c.Render(200, lista()) // o pedaço, com o mesmo id
	}
	return c.Redirect("/anexos") // sem JavaScript: gravar, redirecionar, buscar
}
```

### O limite é do app; a exceção é da rota

O corpo tem teto no `Config.MaxBodyBytes` (1 MiB por padrão) — é esse teto que impede uma
requisição de comer a memória do servidor, e ele deve continuar de pé em toda rota que recebe
formulário. A rota que recebe arquivo diz isso por conta própria, no `middleware.go` dela:

```go
// app/anexos/middleware.go
func Middleware(c *trilha.Ctx, next trilha.Next) error {
	if c.Request().Method == "POST" {
		c.AllowBody(8 << 20)
		c.NoReadDeadline() // conexão ruim não é erro
	}
	return next()
}
```

No middleware, e não no handler: o CSRF lê o formulário antes do handler rodar, então lá
dentro o corpo já teria sido lido no limite antigo. O resto do app continua no 1 MiB, e
passar dos 8 MiB continua sendo 413 com a mensagem de sempre.

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
