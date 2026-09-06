---
title: Segurança
description: O que o Trilha protege por padrão, como ajustar, e o que continua sendo responsabilidade sua.
---

O Trilha segue duas referências: o **NIST Cybersecurity Framework 2.0** (as funções
Identificar, Proteger, Detectar, Responder, Recuperar e Governar) e o **OWASP ASVS 4.0**
nível 2. Um framework web só consegue *proteger* e *detectar*; o restante é trabalho de quem
opera o app, e este capítulo diz exatamente onde termina um e começa o outro.

## O que já vem ligado

| Controle | Padrão | NIST CSF 2.0 | OWASP ASVS |
|---|---|---|---|
| Escape de HTML (`h`) e escape contextual (`tmpl`) | sempre | PR.DS | V5.3 |
| `Content-Security-Policy` com nonce por requisição | ligado | PR.PS | V14.4 |
| `Strict-Transport-Security` | ligado em HTTPS | PR.DS | V9.1 |
| `X-Frame-Options`, `X-Content-Type-Options`, `Referrer-Policy`, `Permissions-Policy`, `Cross-Origin-Opener-Policy` | ligados | PR.PS | V14.4 |
| CSRF por *double-submit cookie* em formulários | ligado | PR.AA | V4.2 |
| Limite do corpo da requisição (1 MiB) | ligado | PR.IR | V13.1 |
| Timeouts de leitura, escrita e ociosidade; limite de cabeçalhos | ligados | PR.IR | V13.1 |
| Estáticos restritos a `public/` | sempre | PR.DS | V12.3 |
| Erros opacos em produção; sem stack, sem caminho | ligado | PR.DS | V7.4 |
| Logs estruturados sem corpo nem cookies, com `request_id` | sempre | DE.CM | V7.1 |
| Eventos de segurança (CSRF, 401/403, 413, 429, panic) no log | sempre | DE.AE | V7.2 |
| Cookies assinados (`SetSigned`/`Signed`) | com `TRILHA_SECRET` | PR.AA | V3.4 |
| Limite de taxa por cliente | opcional | PR.IR | V11.1 |
| Proxies confiáveis (`X-Forwarded-*`) | opcional | PR.AA | V14.1 |

## CSP e scripts inline

A política padrão só permite scripts do próprio site ou com o **nonce** da requisição. Um
`<script>` inline precisa dele:

```go
h.Script(trilha.NonceAttr(c), h.Raw(`document.body.dataset.pronto = "1"`))
```

O script de recarga do `trilha dev` já usa o nonce. Para liberar uma origem externa (fontes,
CDN de imagens) sem reescrever a política, acrescente em `app/setup.go`:

```go
func Setup(a *trilha.App) error {
	a.Security().CSPExtra = map[string][]string{
		"style-src": {"https://fonts.googleapis.com"},
		"font-src":  {"https://fonts.gstatic.com"},
	}
	return nil
}
```

Para uma política totalmente sua, defina `a.Security().CSP` (a string pode conter
`{nonce}`); para desligar um cabeçalho, atribua `trilha.Off`.

## Atrás de um proxy

Se o app roda atrás de nginx, Caddy ou um balanceador, o `RemoteAddr` é o proxy. Diga ao
Trilha em quem confiar para que `X-Forwarded-For` e `X-Forwarded-Proto` valham:

```bash
TRILHA_TRUSTED_PROXIES=10.0.0.0/8,127.0.0.1
```

Só então `c.ClientIP()` devolve o cliente real, o HSTS é enviado e o limite de taxa conta por
cliente e não por proxy. Sem essa variável, cabeçalhos `X-Forwarded-*` são ignorados, o que
é o comportamento seguro.

## O host que você atende

O cabeçalho `Host` é escolhido por quem chama. Seu app o usa para montar URL absoluta — o link
de redefinição de senha, o e-mail de convite, um redirecionamento — e qualquer cache à frente
o usa como chave. Uma requisição com `Host: atacante.example` basta para pôr num e-mail que o
seu app envia um link apontando para o domínio de outra pessoa.

Liste os hosts que você atende e o resto leva 400 antes de o roteador rodar:

```go
trilha.Config{AllowedHosts: []string{"exemplo.com", "*.exemplo.com"}}
```

```bash
TRILHA_ALLOWED_HOSTS=exemplo.com,*.exemplo.com
```

A porta e a caixa não contam, então `exemplo.com:8443` passa. `*.exemplo.com` libera um rótulo
a mais — `app.exemplo.com` sim, `a.b.exemplo.com` não. Em `Dev`, `localhost` e os endereços de
loopback passam sempre, então copiar a lista de produção para a configuração de
desenvolvimento não quebra o dev server. Lista vazia não confere nada, que é o que recebe o
app que nunca ouviu falar disso.

A recusa é um evento de segurança de tipo `host`: aparece no log, na métrica e no
`OnSecurityEvent` como todo bloqueio.

:::note
A lista fala do host que o **app** recebe. Se um proxy reescreve o `Host`, escreva o que o
proxy manda, não o que o navegador digitou.
:::

## Sessão com cookie assinado

Um cookie assinado não pode ser forjado nem alterado, e vence sozinho:

```go
// no POST do login
if err := c.SetSigned("sessao", usuario.ID, 8*time.Hour); err != nil {
	return err
}

// no middleware da área restrita
id, ok := c.Signed("sessao")
if !ok {
	return trilha.RedirectCode("/entrar", 302)
}
```

A chave vem de `TRILHA_SECRET` (32 bytes ou mais; `openssl rand -base64 32`). Em
desenvolvimento o `trilha dev` gera uma chave efêmera por sessão. Em produção sem a
variável, `SetSigned` devolve erro e registra um aviso dizendo qual cookie, em qual rota —
uma vez por cookie, e só para quem de fato assina algum: um aviso em toda subida de um app
que tem sessão própria é o que ensina a equipe a não ler aviso. Para trocar a chave sem
derrubar sessões, coloque a antiga em `TRILHA_SECRET_PREVIOUS` até que expirem.

:::atencao
O cookie assinado garante integridade, não sigilo: o valor é legível por quem tem o cookie.
Guarde nele um identificador, nunca dados sensíveis.
:::

## Limite de taxa

Global, em `app/setup.go`, ou por subárvore, em um `middleware.go`:

```go
// app/api/middleware.go
var limit = trilha.Limit(5, 20) // 5 req/s por cliente, rajada de 20

func Middleware(c *trilha.Ctx, next trilha.Next) error {
	return limit(c, next)
}
```

A resposta é 429 com `Retry-After`. O contador vive na memória do processo: com várias
réplicas, cada uma conta a sua parte.

## Detectar e responder

Cada bloqueio gera uma linha `security` no log, com `kind`, `ip`, `path` e `request_id`, e
chama `Config.OnSecurityEvent` se você definir um. É o gancho para contar tentativas,
alertar ou bloquear um IP no firewall.

Antes de publicar, rode:

```bash
trilha audit
```

Ele confere `TRILHA_SECRET`, proxies, `trilha_gen.go`, versão do Go, `go vet` e
`govulncheck`, e sai com erro se houver item crítico.

## Arquivo que chega de fora

Upload é a única requisição em que o app grava o que outra pessoa mandou, com o nome que
outra pessoa escolheu. O `c.File` só devolve o arquivo depois das três conferências que
importam:

```go
func POST(c *trilha.Ctx) error {
	c.AllowBody(8 << 20) // a requisição; o limite do arquivo, abaixo, é outro
	up, err := c.File("arquivo", trilha.FileRules{
		MaxSize: 4 << 20,
		Accept:  []string{"image/*", "application/pdf"},
	})
	if err != nil {
		if errs, ok := err.(trilha.FieldErrors); ok {
			return c.Render(http.StatusUnprocessableEntity, pagina(c, errs))
		}
		return err
	}
	defer up.Close()
	caminho, err := up.Save("uploads") // nunca sai de "uploads"
	...
}
```

- **Tamanho**: `MaxSize` é por arquivo, à parte do `Config.MaxBodyBytes`. A rota que aceita
  um arquivo de 4 MB ainda precisa deixar passar um corpo um pouco maior (`c.AllowBody`),
  porque o corpo leva também os outros campos.
- **Tipo**: `Accept` é comparado com o tipo detectado nos primeiros 512 bytes do conteúdo,
  nunca com a extensão e nunca com o `Content-Type` que o cliente anunciou — um PDF
  renomeado para `foto.png` é um PDF. `up.MIME` é o que ele é de verdade e `up.Ext` é a
  extensão correspondente. Atenção: a biblioteca padrão detecta o que conhece; formato que é
  zip por dentro (`.docx`, `.xlsx`) volta como `application/zip`, e CSV como `text/plain`.
  Onde a diferença importa, olhe o conteúdo você mesmo.
- **Nome**: `up.Name` não tem diretório, nem separador de nenhuma das duas plataformas, nem
  caractere de controle, tem no máximo 100 caracteres, e nunca é vazio nem `..`. O
  `up.Save(dir)` grava dentro de `dir` com modo 0600 e um nome livre (`nota.pdf`, depois
  `nota-1.pdf`), então o segundo envio não come o primeiro.

Regra que falha vira `FieldErrors` no nome do campo — a mesma resposta do `Bind`, então o
formulário mostra a mensagem onde a pessoa está olhando, em vez de o app responder 500.

Duas coisas continuam sendo suas: **onde** o arquivo vai parar (um diretório fora do código,
um bucket, um banco) e **quem** pode mandar. E arquivo que o app devolve sai por uma rota
sua, com o tipo que você decidiu — nunca entregando o diretório de upload para o
`http.FileServer`.

## Outra origem chamando seu app

O navegador só deixa um script ler a resposta de outra origem quando o servidor autoriza.
Autorize em um lugar só, `Config.CORS`, com a lista de origens escrita por extenso:

```go
func Config(cfg *trilha.Config) error {
	cfg.CORS = trilha.CORS{
		Origins: []string{"https://painel.exemplo.com"},
		Methods: []string{"GET", "POST", "DELETE"},
		MaxAge:  10 * time.Minute,
	}
	return nil
}
```

Com isso o preflight `OPTIONS` é respondido pelo framework, antes do roteador — então vale
em qualquer rota, inclusive nos arquivos estáticos — e toda resposta para uma origem
liberada leva `Access-Control-Allow-Origin` e `Vary: Origin`.

Três coisas que o middleware caseiro costuma errar, e que este não erra:

- **`"*"` com credencial é recusado na subida, não em runtime.** `Origins:
  []string{"*"}` serve uma API pública; no instante em que você põe `Credentials: true` do
  lado, o `New` entra em pânico. O "conserto" habitual dessa dupla — ecoar de volta qualquer
  `Origin` que chegue — entrega a sessão dos seus usuários a qualquer site que peça.
- **A lista de origens é exata.** Sem subdomínio curinga: `https://app.exemplo.com` é uma
  entrada, e `exemplo.com.atacante.net` nunca casa por acidente.
- **O `Vary: Origin` sempre sai**, para um cache na frente do app nunca servir a resposta da
  origem liberada para outra pessoa.

Preflight de origem que ninguém listou volta 403 — o navegador está perguntando, e resposta
clara é o que aparece na aba de rede. Já a requisição **simples** de origem não listada é
servida como sempre, só que sem os cabeçalhos de CORS: quem esconde a resposta do script é o
navegador, e o cliente que não é navegador nunca foi quem estava sendo protegido aqui.

### Quando só alguns caminhos são públicos

`Config.CORS` é o app inteiro. Um documento de descoberta em `/.well-known/`, buscado de
outra origem por um cliente que ainda não tem sessão, são três caminhos em noventa — e
abrir os outros oitenta e sete para consertar três é trocar uma lacuna por uma superfície.
Essas rotas levam a própria política, no `route.go` que as serve:

```go
var CORS = trilha.CORS{Origins: []string{"*"}, Methods: []string{"GET"}}
```

A rota responde ao próprio preflight, com as mesmas checagens e o mesmo 403; o resto do app
segue de mesma origem. A rota que declara política decide sozinha — a lista do app não a
estreita, e ela não alarga a lista do app para mais ninguém. Veja
[Convenções](/pt/referencia/convencoes).

## O que continua sendo seu

- **Autenticação e autorização**: quem é o usuário e o que pode fazer. O Trilha dá o
  cookie assinado e o middleware; a regra de negócio é sua.
- **Dados em repouso**: criptografia do banco, backups, retenção.
- **TLS**: termine no proxy ou use um certificado no próprio `http.Server` via
  `a.Handler()`.
- **Segredos**: só em variáveis de ambiente ou em um cofre; nunca no repositório.
- **Governar, Identificar, Recuperar**: inventário, classificação de dados, plano de
  resposta e restauração são processos da organização. O `SECURITY.md` do projeto descreve
  como relatar vulnerabilidades do framework, e o
  [SECURITY-MODEL.md](https://github.com/emersonjoe/trilha/blob/main/docs/pt-BR/SECURITY-MODEL.md)
  é o modelo de ameaças escrito: contra o quê cada controle defende e o que continua aberto.

## Desafio

Faça a área `/painel` do seu app exigir sessão assinada, com um limite de 10 tentativas por
minuto no formulário de login, e registre em `OnSecurityEvent` quantos bloqueios ocorreram.

:::solucao
```go
// app/entrar/middleware.go
var limit = trilha.Limit(10.0/60, 10)

func Middleware(c *trilha.Ctx, next trilha.Next) error { return limit(c, next) }

// app/setup.go
var bloqueios atomic.Int64

func Setup(a *trilha.App) error {
	a.Config().OnSecurityEvent = func(e trilha.SecurityEvent) {
		if e.Kind == "rate" { bloqueios.Add(1) }
	}
	return nil
}
```

`a.Config()` dá acesso à configuração dentro de `Setup`; o limitador do login usa
`trilha.Limit` com 10 fichas por minuto.
:::
