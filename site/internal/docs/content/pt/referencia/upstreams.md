---
title: Upstreams
description: Encaminhar um prefixo de URL para uma API que já existe, com a credencial da sessão.
---

Um app que fala com uma API sua é uma coisa; um app que *é* a frente de uma API que já
existe é outra. O segundo precisa de `/api/*` na mesma origem — sem CORS, sem o endereço da
API no navegador — e precisa que a chamada saia com a credencial da sessão. Um app Next.js
escreve a primeira metade no `next.config.ts` e a segunda à mão. `Config.Upstreams` é as
duas:

```go
// app/setup.go
func Config(cfg *trilha.Config) {
	cfg.Upstreams = map[string]trilha.Upstream{
		"/api/": {
			Target: os.Getenv("API_URL"), // http://api:8801
			Headers: func(c *trilha.Ctx, hdr http.Header) {
				if tok := sessao.Token(c); tok != "" {
					hdr.Set("Authorization", "Bearer "+tok)
				}
			},
			Timeout: 60 * time.Second,
		},
	}
}
```

| Campo | Papel |
|---|---|
| `Target` | URL base da API; um caminho nele prefixa toda requisição encaminhada |
| `Headers` | roda com os cabeçalhos de saída, antes de a requisição partir |
| `Timeout` | limita a troca inteira (padrão 30s; `trilha.NoTimeout` desliga) |
| `CSRF` | `trilha.Off` deixa de exigir o token nas escritas |
| `Transport` | substitui o `http.DefaultTransport` (mTLS até a API, um teste) |

## O que o framework decide

- **Uma rota do app ganha.** O upstream é a última rota do prefixo, então um `route.go` em
  `/api/documentos` responde antes dele — e dá para migrar a API para Go endpoint a
  endpoint sem o front saber.
- **O prefixo mais longo ganha**, então `/api/v2/` pode apontar para outro lugar que
  `/api/`.
- **O corpo passa em stream nos dois sentidos.** `MaxBodyBytes` e o write deadline foram
  escritos para handlers, não para um cano: um upload de 200 MB e um PDF voltando
  atravessam inteiros.
- **A resposta passa intacta.** `Content-Type`, `Content-Disposition`, `Cache-Control` e
  `ETag` do upstream vencem; os cabeçalhos de segurança do Trilha ficam de pé onde o
  upstream não disse nada.
- **CSRF ligado por padrão** em `POST`/`PUT`/`PATCH`/`DELETE`, como em qualquer escrita do
  app — o `ui.js` já manda o cabeçalho. `CSRF: trilha.Off` é para API autenticada por chave.
- **Upstream fora do ar é 502 e lento é 504**, os dois em `problem+json` diga o `Accept` o
  que disser: o cliente de `/api/` é um script, nunca um navegador na barra de endereço.

## O que não atravessa

O `Authorization` que o navegador mandou é **descartado** antes de o `Headers` rodar. A
credencial é a da sessão, não a do cliente: quem manda um `Bearer` próprio não fala com a
API por tabela. Cabeçalhos hop-by-hop (`Connection`, `Upgrade`, `Proxy-*`) são removidos,
como em qualquer proxy. `X-Request-ID` e `traceparent` atravessam, para o log do app e o da
API serem sobre a mesma requisição; `X-Forwarded-For`, `-Proto` e `-Host` dizem quem pediu.

O `Cookie` **é** repassado — uma API com sessão própria ainda precisa dele. Se não é o seu
caso, apague no `Headers`:

```go
Headers: func(c *trilha.Ctx, hdr http.Header) {
	hdr.Del("Cookie")
	hdr.Set("Authorization", "Bearer "+token(c))
},
```

## Nunca um proxy aberto

O alvo mora na configuração; a requisição só escolhe o caminho abaixo do prefixo. Não há
cabeçalho, query ou caminho que o mova. Um alvo que não é URL vira uma reclamação no log na
subida, não um 502 uma hora depois — e o app continua respondendo todo o resto.

O `trilha audit` aponta um `Target` escrito como `http://` para um host que não é esta
máquina (a credencial da sessão atravessando a rede em claro) e um upstream sem `Headers`
num app que exige login (credencial provavelmente esquecida).

## Onde isso encaixa

O proxy responde fora da cadeia de middleware, então ninguém pôs o usuário no contexto da
requisição: leia a sessão do cookie, com `Session(c)` e não com `User(c)`.

```go
func Token(c *trilha.Ctx) string {
	u, err := Flow.Session(c)
	if err != nil {
		return ""
	}
	return u.Extra["api_token"]
}
```

O `examples/local-login` é a forma inteira: um login contra a tabela de usuários do próprio
app e `/api/` encaminhado com o que esse login guardou. Veja também
[Auth](/pt/referencia/auth) para a sessão sem OIDC e a receita
[Um app na frente de uma API existente](/pt/receitas/api-existente).
