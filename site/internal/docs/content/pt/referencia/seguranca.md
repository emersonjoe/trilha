---
title: Segurança
description: Configuração completa de cabeçalhos, proxies, limite de taxa, cookies assinados e eventos.
---

## Config.Security

| Campo | Padrão | Cabeçalho |
|---|---|---|
| `CSP` | política com nonce (abaixo) | `Content-Security-Policy` |
| `CSPExtra map[string][]string` | — | acrescenta origens a diretivas da política padrão |
| `HSTS` | `max-age=31536000; includeSubDomains` (só em HTTPS) | `Strict-Transport-Security` |
| `PermissionsPolicy` | `camera=(), microphone=(), geolocation=(), payment=(), usb=()` | `Permissions-Policy` |
| `COOP` | `same-origin` | `Cross-Origin-Opener-Policy` |
| `FrameOptions` | `DENY` | `X-Frame-Options` |
| `Referrer` | `strict-origin-when-cross-origin` | `Referrer-Policy` |

`trilha.Off` em qualquer campo remove o cabeçalho. `X-Content-Type-Options: nosniff` é
sempre enviado. Política padrão:

```text
default-src 'self'; script-src 'self' 'nonce-…'; style-src 'self' 'unsafe-inline';
img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none';
base-uri 'self'; form-action 'self'
```

`c.Nonce()` devolve o nonce da requisição; `trilha.NonceAttr(c)` o coloca em um `h.Script`.
`trilha.NonceFrom(r)` dá o mesmo valor para quem só tem o `*http.Request` —
`html/template`, `templ`, um handler seu —, então a casca de um app em migração não
precisa de um middleware próprio para alcançá-lo.
Ajuste em `Setup` por `a.Security()`.

### Quando a resposta é do hospedeiro

Um app montado dentro de um servidor que já responde pelas próprias respostas tem dois
cabeçalhos a mais, não um: o hospedeiro escreveu a política, e o app escreve de novo.

| Campo | Efeito |
|---|---|
| `Delegated bool` | não escreve cabeçalho nenhum — nem os seis que têm `Off`, nem o `nosniff` que não tem |
| `Nonce func(*http.Request) string` | o nonce vem do hospedeiro, uma chamada por requisição que pedir |

```go
a.Security().Delegated = true
a.Security().Nonce = func(r *http.Request) string { return host.NonceOf(r) }
```

`Delegated` é uma decisão, não um padrão: o valor zero escreve os cabeçalhos, então um
`Security{...}` escrito à mão nunca os desliga por omissão. O boot registra a delegação uma
vez no log, porque resposta sem cabeçalho precisa aparecer em algum lugar.

Sem `Nonce`, `c.Nonce()` inventa um valor por requisição — o que está certo para um app que
publica a própria CSP e errado para um que não publica: a política do hospedeiro nunca ouviu
falar daquele nonce, e o navegador recusa o script. Com `Nonce` devolvendo string vazia,
`trilha.NonceAttr(c)` não renderiza atributo nenhum em vez de `nonce=""`.

## Proxies confiáveis

`Config.TrustedProxies []string` (CIDR ou IP) ou `TRILHA_TRUSTED_PROXIES=a,b`. Efeitos quando
o peer é confiável: `c.ClientIP()` lê `X-Forwarded-For` (o IP mais à direita que não seja
proxy), `X-Forwarded-Proto: https` liga HSTS e marca cookies como `Secure`.

## Hosts permitidos

`Config.AllowedHosts []string` ou `TRILHA_ALLOWED_HOSTS=a,b`. A requisição cujo `Host` não
está na lista é respondida com 400 antes do roteador, das sondas e do CORS, e emite um evento
`host`. Lista vazia = sem conferência.

| Padrão | Libera | Não libera |
|---|---|---|
| `exemplo.com` | `exemplo.com`, `exemplo.com:8443`, `EXEMPLO.com.` | `sub.exemplo.com` |
| `*.exemplo.com` | `app.exemplo.com` | `exemplo.com`, `a.b.exemplo.com` |

Em `Dev`, `localhost`, `127.0.0.1` e `::1` passam sempre. O que se compara é o host que o app
recebe — atrás de um proxy que reescreve o `Host`, liste o que o proxy manda.

## Limite de taxa

`Config.RateLimit{RPS float64, Burst int}` aplica um *token bucket* por `ClientIP` antes dos
middlewares. `trilha.Limit(rps, burst) MiddlewareFunc` cria um limitador independente para
uma subárvore. Resposta: 429 com `Retry-After` (segundos) e evento `rate`.
`trilha.ErrRateLimited` pode ser devolvido por um handler para o mesmo efeito.

## Cookies assinados

| Símbolo | Descrição |
|---|---|
| `c.SetSigned(nome, valor, ttl) error` | grava cookie `valor|expira|hmac` com `HttpOnly`, `SameSite=Lax`, `Secure` em HTTPS; `ErrNoSecret` sem chave |
| `c.Signed(nome) (string, bool)` | lê e verifica assinatura e prazo |
| `c.ClearCookie(nome)` | expira um cookie |
| `trilha.NewSigner(chaves...)`, `Sign`, `Verify` | o assinador (HMAC-SHA256) para uso direto |
| `Config.Secret`, `Config.PreviousSecret` | `TRILHA_SECRET`, `TRILHA_SECRET_PREVIOUS` (base64 ou texto, ≥ 32 bytes) |

Sem segredo: em `dev` uma chave efêmera é gerada (o `trilha dev` mantém uma por sessão); em
`prod` o app avisa no log e `SetSigned` devolve `ErrNoSecret`.

## Timeouts

`Config.Timeouts{ReadHeader 10s, Read 30s, Write 60s, Idle 120s, MaxHeaderBytes 64 KiB}`.
Para respostas longas (SSE, download), chame `c.NoWriteDeadline()` antes de escrever.

## Eventos de segurança

```go
type SecurityEvent struct {
	Kind      string // csrf | auth | body | host | rate | panic
	Status    int
	Method    string
	Path      string
	IP        string
	RequestID string
}
```

Registrados com `slog.Warn("security", ...)` e entregues a `Config.OnSecurityEvent`, uma vez
por requisição.

## `trilha audit`

Verifica: `TRILHA_SECRET`, `TRILHA_TRUSTED_PROXIES`, `trilha_gen.go` atualizado, versão do
Go, `.gitignore`, `go vet` e `govulncheck` (`--no-vuln` para pular). Código de saída 1 com
item crítico.

`TRILHA_SECRET` ausente só é crítico quando o código assina alguma coisa — `SetSigned`,
`Signed`, um `Signer` próprio, `Config.Secret` ou o pacote `auth`. Num app cuja sessão não é
a do Trilha, vira aviso: um segredo que não assina nada entra no `.env`, no deploy e na
rotação, e no dia em que alguém o girar não acontece nada — que é a pior coisa que um
segredo pode ensinar. Definido e curto demais segue crítico nos dois casos: quem definiu
quis usar.

Ele também avisa da escrita que nenhum `Kind` alcança. Um `route.go` é API, e API não confere
o token de CSRF, então uma rota de `POST` num app que também serve páginas quase sempre quer
`var Kind = trilha.KindPage` num `kind.go` acima dela — uma linha para o ramo inteiro, veja
[Convenções de arquivo](/pt/referencia/convencoes#o-kind-segue-a-subarvore). Ligar o
`Config.CSRFForAPI` responde a mesma pergunta pelo outro lado e também cala o aviso.

## Segredos em repouso

O framework tinha segredo, assinador e cookies assinados. O que não tinha era *cifre isto para
guardar* — e o que se escreve no lugar é um token em claro, depois um AES copiado da internet
com IV fixo, depois a chave inteira voltando num `GET` e aparecendo no DevTools.

```go
sealed, err := trilha.Seal([]byte(chave)) // AES-256-GCM, nonce aleatório
plain, err := trilha.Open(sealed)         // segredo atual, depois o PreviousSecret
```

| Símbolo | Papel |
|---|---|
| `Seal([]byte) ([]byte, error)` | cifra para guardar; `ErrNoSecret` sem segredo, nunca um valor em claro |
| `Open([]byte) ([]byte, error)` | decifra; `ErrSealed` para o que não abre, sem dizer por quê |
| `Secret` | uma string que não vaza por descuido |
| `s.Reveal()` | o valor, lido de propósito |
| `s.Sealed()` / `SecretFrom(b)` | a forma cifrada e a volta |
| `s.Value()` / `s.Scan()` | `driver.Valuer` e `sql.Scanner`: a coluna guarda o selo |
| `ui.SecretField(c, nome, rótulo, atual)` | o campo de senha para um formulário escrito à mão |

**O `Value()` é o método do driver e o `Reveal()` é o leitor.** A issue que pediu este tipo tinha
o contrário; não dá: um `Secret` é uma string por baixo, e o `database/sql` converte sozinho
qualquer valor de kind string — então, a menos que `Value()` seja o `driver.Valuer`, passar um
`Secret` para uma query grava o texto em claro, em silêncio.

**Máscara voltando é "não mudou".** Um formulário não tem como mandar "não mexi nisto", então
uma tela que preenche o campo ou devolve o segredo para o navegador ou o apaga no primeiro save
que ninguém redigitou. O `Bind` trata vazio **ou máscara** como não mudou, o `trilha.Settings`
desenha um `Secret` como campo de senha sempre vazio, e a linha de ajuda diz isso.

**Rotação.** O `Open` tenta o segredo atual e depois o `Config.PreviousSecret`; o `Seal` sempre
usa o atual. Então: ponha a chave antiga em `TRILHA_PREVIOUS_SECRET`, publique o novo
`TRILHA_SECRET`, deixe tudo ser re-cifrado, e só então tire a antiga. O `trilha audit` avisa
quando há `trilha.Secret` no projeto e nenhum segredo anterior — rodar a chave nessa hora é o
momento em que todo token guardado para de abrir.

:::warning
A chave vem do segredo da app. Isto é cifra **contra um dump do banco e contra um backup**, não
contra o operador nem contra quem tem o ambiente — esses têm a chave por definição. Cifrar contra
a sua própria infraestrutura precisa de um gerenciador de chaves, e o framework não finge ser um.
:::
