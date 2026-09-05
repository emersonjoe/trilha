# Feature Specification: Fragmentos — atualização parcial e envio sem recarga

**Feature Branch**: `018-fragmentos` | **Created**: 2026-09-05 | **Status**: Implementada (v0.10.0)
**Input**: issues [#20](https://github.com/emersonjoe/trilha/issues/20) e
[#21](https://github.com/emersonjoe/trilha/issues/21) (ROADMAP, Fase 1). A #21 depende da
#20 e usa o mesmo mecanismo, então as duas viram uma spec só: separá-las produziria duas
metades de uma coisa.

## O problema

Hoje uma rota devolve a página inteira. Para trocar uma tabela filtrada, uma lista ou um
contador, quem usa o Trilha escreve `fetch` e manipulação de DOM à mão — e a CSP padrão,
sem `unsafe-inline`, torna isso mais chato do que deveria. O resultado prático é que o
degrau entre "formulário que recarrega a página" e "SPA" está vazio, e a saída fácil
(adotar React) contradiz a tese do projeto.

O mesmo vale para formulários: `c.Bind` + `FieldErrors` + `c.Render` já dão o ciclo
completo, mas com recarga. Falta enviar, mostrar que está enviando e receber de volta **o
mesmo HTML** com os erros de campo, sem sair da página.

## A ideia

Uma requisição pode pedir **um pedaço** da página, e a mesma rota que serve a página serve
o pedaço. O servidor não ganha um segundo modo de renderizar: ganha uma pergunta.

```go
func Page(c *trilha.Ctx) (h.Node, error) {
	lista := listaDe(c)              // já filtrada por c.Query("q")
	if c.Fragment() == "lista" {
		return lista, nil            // sem layouts, só o pedaço
	}
	return h.Div(cabecalho(), busca(), lista), nil
}
```

No HTML, um atributo — nenhum manipulador inline, nada que a CSP recuse:

```go
h.Form(h.Method("get"), ui.Swap("lista"), ...)   // data-trilha-target="lista"
h.Div(h.ID("lista"), ...)                        // o alvo
```

Com JavaScript, o clique ou o envio busca o fragmento e troca o elemento. Sem JavaScript, o
link navega e o formulário envia como sempre — a mesma rota devolve a página inteira,
porque ninguém mandou o cabeçalho. É aprimoramento progressivo de verdade: a versão sem
script não é um degradado, é o caminho normal.

## Cenários

1. **Filtro de lista** — a pessoa digita e envia a busca; só a lista pisca. Com o script
   desligado, a página recarrega filtrada, e a URL é a mesma nos dois casos.
2. **Formulário com erro** — o envio volta 422 com `FieldErrors` e o mesmo HTML do
   formulário, agora com as mensagens; o foco vai para o primeiro campo com erro e a
   rolagem não se mexe.
3. **Formulário salvo** — o handler decide: navegação normal segue o PRG de sempre; a
   requisição de fragmento recebe a tela nova (formulário limpo e lista atualizada) sem sair
   do lugar.
4. **Redirecionamento** — um handler que redireciona durante uma requisição de fragmento faz
   o navegador navegar de verdade, em vez de enfiar a página de destino dentro de uma `<div>`.
5. **Servidor fora do ar** — falha de rede ou 5xx caem para a navegação normal: o botão
   nunca fica morto.
6. **Voltar** — o botão "voltar" do navegador desfaz uma troca feita por link.

## Requisitos funcionais

- **FR-001** `Ctx.Fragment() string` devolve o alvo pedido (cabeçalho `Trilha-Fragment`), ou
  `""` numa navegação comum.
- **FR-002** Numa requisição de fragmento, `renderPage` e `Ctx.Render` **não** aplicam
  layouts, e a resposta não recebe `<!doctype>`, `<html>` nem o script de recarga do `dev`.
- **FR-003** Toda resposta HTML leva `Vary: Trilha-Fragment`, para que nenhum cache sirva um
  pedaço no lugar da página (ou o contrário).
- **FR-004** Um redirecionamento durante uma requisição de fragmento vira **204** com
  `Trilha-Location`; o cliente navega para lá.
- **FR-005** `data-trilha-target="id"` em `<a>` e `<form>` (helper `ui.Swap`) busca o
  fragmento e substitui o elemento de mesmo `id`, sem manipulador inline.
- **FR-006** Enquanto a requisição corre, o alvo fica com `aria-busy="true"` e o botão que
  enviou fica desabilitado; ao terminar, o foco e a posição do cursor dentro do alvo são
  restaurados.
- **FR-007** Falha de rede, 5xx ou alvo ausente na resposta caem para a navegação normal
  (`location.assign` ou `form.submit()`).
- **FR-008** O CSRF continua obrigatório: o envio leva o mesmo corpo do formulário, com
  `_csrf`. Requisição de outra origem não consegue mandar o cabeçalho `Trilha-Fragment`
  sem preflight de CORS, e o Trilha não responde a preflight.
- **FR-009** Navegação por link atualiza o histórico (`pushState`) e o botão "voltar"
  refaz a troca; um formulário `GET` usa `replaceState`, para que recarregar mantenha o filtro.
- **FR-010** Nenhuma dependência nova: o cliente cabe no `ui.js` (que hoje tem 127 linhas),
  e o custo por requisição sem fragmento não muda.
- **FR-011** `examples/cadastro` demonstra os dois usos (busca na lista e envio sem recarga)
  e continua funcionando com o JavaScript desligado, com teste dos dois caminhos.

## Fora de escopo

Ilhas com estado próprio (issue #22), navegação de página inteira no cliente (#23), upload
com progresso e WebSocket (#24), diferenciação de DOM (*morphing*) e transmissão de várias
regiões numa resposta só. A troca é do elemento inteiro, por `outerHTML`: é o que dá para
explicar em uma frase, e o resto é otimização que ninguém pediu ainda.

## Critérios de aceitação

- Com o script desligado, o exemplo funciona igual, pelo mesmo endereço.
- O HTML do site e dos exemplos continua sem manipulador inline (`TestNoInlineEventHandlers`).
- Nenhuma alocação nova no caminho de quem não usa fragmento.
