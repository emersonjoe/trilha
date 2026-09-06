---
title: Fragmentos vivos
description: O ui.Poll atualiza um fragmento pelo relógio; ui.Live e ui.On atualizam quando o servidor avisa.
---

Um app que processa coisas em segundo plano — pipeline de documento, lote, execução de
fluxo — mostra estado que muda sem ninguém clicar. O Trilha já tinha as duas metades: o
`ui.Swap` troca um fragmento quando o visitante clica, e o `c.Stream()` manda Server-Sent
Events para quem escreve o JavaScript. Isto aqui é a cola, e é a mesma cola que cada tela
estava escrevendo à mão.

Tudo nesta página precisa do `ui.LiveScript(c)` na página — um `<script defer>` do
`ui.live.js`, que o `ui.Head` **não** carrega. Página sem nada para observar não baixa.

## Polling

```go
h.Div(h.ID("status"), ui.Poll("6s", "/docs/42/status"), status(doc))
```

O `ui.Poll(intervalo, src)` são dois atributos num elemento que tem `id`: o id é o
fragmento que ele pede. O intervalo é `"6s"`, `"500ms"` ou `"2m"`; o `src` é onde pedir,
ou o endereço da própria página quando vazio.

O cliente:

- **pausa enquanto a aba está escondida** e pede de novo assim que ela volta;
- **recua depois de um erro** — o dobro do intervalo a cada vez, até um minuto — e volta ao
  intervalo na primeira resposta boa;
- **respeita o `Retry-After`** num 429 ou num 5xx;
- **para** num 4xx que não seja 429: fragmento que responde 403 ou 404 não vai começar a
  funcionar no tique seguinte;
- **nunca troca um fragmento em que o visitante está digitando.**

O primeiro render já está na página, então quem está sem JavaScript vê o estado de quando
a página carregou — velho, nunca quebrado.

### O relógio é do servidor

| Chamada | Cabeçalho | Efeito |
|---|---|---|
| `c.PollStop()` | `Trilha-Poll: stop` | o polling acaba; a resposta ainda é o fragmento, então o último estado é o que fica |
| `c.PollEvery(d)` | `Trilha-Poll: 30s` | o intervalo muda daí em diante; nunca abaixo de um segundo |

```go
func Page(c *trilha.Ctx) (h.Node, error) {
	if c.Fragment() == "fila" {
		if fila.Acabou() {
			c.PollStop()
		}
		return blocoDaFila(), nil
	}
	...
}
```

O `c.Fragment()` continua sendo a única API do lado do servidor: a rota não sabe se quem
pediu foi um clique ou um tique.

## Eventos

Uma conexão por página, aberta pelo `ui.Live`:

```go
h.Body(ui.Live("/eventos"), ...)
```

e os fragmentos interessados dizem o que estão esperando:

```go
h.Div(h.ID("status"), ui.On("doc:42", "/docs/42/status"), status(doc))
```

A rota é um GET comum:

```go
func GET(c *trilha.Ctx) error {
	s := c.Stream()
	for ev := range bus.Subscribe(c.Context(), user) {
		if err := s.Notify(ev.Name); err != nil {
			return err
		}
	}
	return nil
}
```

O `Stream.Notify(nome)` manda o nome e nenhum dado. **O evento carrega o nome, nunca o
HTML**: o cliente pede de novo a rota do fragmento, então a autorização e o render
continuam onde já estavam, e a conexão nunca vira canal de dados.

Um fragmento com `ui.On` e `ui.Poll` juntos espera o evento enquanto a conexão está de pé
e volta para o relógio quando ela cai — o plano B custa um atributo.

Não há barramento no framework: a rota que responde `/eventos` é sua, e o que a acorda
também. O que o framework traz é a metade do cliente e o `Notify`.

## Segurança

O stream fica aberto enquanto a página estiver aberta, e um stream desprotegido é uma
conexão que qualquer um segura e lê. Ponha a rota atrás da mesma guarda das páginas que
ela serve — um `middleware.go` com `auth.Require()` sobre o ramo — que o `trilha audit`
avisa quando não há nenhuma.
