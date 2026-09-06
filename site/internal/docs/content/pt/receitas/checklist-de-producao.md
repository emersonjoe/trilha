---
title: Checklist de produção
description: O que conferir antes de publicar, em ordem: o que o trilha audit acha por você, o que ele não enxerga e as duas coisas a preparar para o dia em que der errado.
---

A lista abaixo é para ser lida de cima para baixo, uma vez, antes do primeiro deploy — e de
novo quando alguma coisa mudar de forma. Metade dela é um comando; a outra metade é uma decisão
que ninguém toma por você.

## Rode o comando primeiro

```bash
trilha audit
```

Ele se recusa a ser formalidade: sai com código diferente de zero em qualquer coisa crítica,
então o CI pode barrar nele. O que ele confere, na ordem dele:

| Checagem | Por que está na lista |
|---|---|
| `TRILHA_SECRET` definido e longo o bastante | sem segredo, cookies e CSRF são assinados com uma chave que muda a cada restart |
| proxies confiáveis declarados | sem eles o `ClientIP` é o que o visitante digitou, e o rate limit não protege ninguém |
| hosts permitidos declarados | uma requisição com o `Host` de outra pessoa recebe um link absoluto — e o seu cookie — apontando para lá |
| métricas não públicas, token longo o bastante | `/metrics` é o mapa do seu app: rotas, volumes, taxas de erro |
| pelo menos um `a.Check` | sem nenhum, o `ready` diz sim com o banco fora do ar |
| assets com `immutable` | o único cabeçalho de cache que é seguro e vale a pena, porque o `c.Asset` põe hash no nome |
| segredo OIDC fora do código, callback fora de texto claro | os dois jeitos de um login ser roubado |
| `trilha_gen.go` fresco, CLI e biblioteca na mesma versão | um arquivo gerado que discorda do `app/` serve as rotas da semana passada |
| Go suportado, `.gitignore` cobrindo `.env`, `go vet`, `govulncheck` | a higiene comum, que só faz falta quando falha |

Corrija tudo que for crítico. Um aviso é uma decisão: escreva por que, ou corrija.

## O que o comando não enxerga

### Configuração

```go
// Config is the production side of app/setup.go. Everything here has a
// default that works in dev and is wrong behind a proxy on the open
// internet — which is exactly the list worth reviewing before a deploy.
func Config(cfg *trilha.Config) error {
	// Who may say which Host: without this, a request with someone else's
	// Host is answered with your session cookie in it.
	cfg.AllowedHosts = strings.Split(os.Getenv("ALLOWED_HOSTS"), ",")
	// The proxy in front. Only these addresses may set X-Forwarded-For, so
	// ClientIP is the visitor and not whatever the visitor typed.
	cfg.TrustedProxies = []string{"10.0.0.0/8"}
	// A request that never finishes is a connection that never returns.
	cfg.Timeouts = trilha.Timeouts{
		ReadHeader: 5 * time.Second,
		Read:       30 * time.Second,
		Write:      30 * time.Second,
		Idle:       60 * time.Second,
		Shutdown:   20 * time.Second,
	}
	// The ceiling on a body nobody asked for; a route that receives files
	// raises its own with c.AllowBody.
	cfg.MaxBodyBytes = 1 << 20
	cfg.RateLimit = trilha.RateLimit{RPS: 20, Burst: 40}
	// Metrics are opt-in and never public. ConfigFromEnv already read
	// TRILHA_METRICS and TRILHA_OBS_TOKEN; what is left is who may scrape.
	cfg.Observability.Trusted = []string{"10.0.0.0/8"}
	// HSTS is a promise the browser remembers: turn it on when the
	// certificate is already working, not before.
	cfg.Security.HSTS = "max-age=31536000; includeSubDomains"
	return nil
}
```

Os timeouts são o item que se pula. Uma requisição que nunca termina é uma conexão que nunca
volta, e a falha parece "o site está lento" até parecer "o site está fora".

### Dados

- **Backup, e uma restauração que você de fato executou.** Um backup que ninguém restaurou é um
  arquivo, não um backup. Cronometre a restauração: esse número é a sua pior indisponibilidade.
- **Migrações aplicadas antes de a versão nova servir**, não pela instância que acabou de subir
  — com mais de uma instância, duas delas rodam a mesma migração ao mesmo tempo.
- **Um rollback que funciona.** Uma migração que remove uma coluna faz a versão anterior não
  subir. Acrescente a coluna, publique, pare de usá-la, remova na versão seguinte.

### Requisições

- **Limite de corpo** em `MaxBodyBytes`, elevado por rota com `c.AllowBody` só onde chega
  arquivo.
- **Rate limit** no que custa dinheiro: login, redefinição de senha, qualquer coisa que mande
  e-mail ou chame um modelo.
- **`AllowedHosts` e HSTS** juntos — o HSTS é uma promessa que o navegador guarda por um ano,
  então ligue depois que o certificado funciona, nunca antes.

### O que você vai olhar quando quebrar

- **Logs estruturados indo para algum lugar em que dá para pesquisar**, com o id da requisição
  dentro. O `c.Log()` já carrega ele.
- **A sonda `/_trilha/health/ready` ligada ao orquestrador**, e a `live` ligada ao restart — o
  contrário reinicia um contêiner para sempre porque o banco dele está fora.
- **Um alerta sobre algo que uma pessoa sente**: taxa de erro e latência p95, não CPU.
- **Nenhum dado pessoal nos logs.** Uma linha de log com um e-mail dentro é uma cópia da sua
  tabela de usuários num serviço de terceiro.

## As duas coisas a preparar para o dia ruim

1. **Como voltar atrás.** A imagem anterior, a tag anterior, e a certeza de que a versão
   anterior ainda conversa com o banco atual.
2. **Como girar o segredo.** `TRILHA_SECRET` recebe o valor novo, `TRILHA_SECRET_PREVIOUS` o
   antigo, pelo tempo que uma sessão dura. Sessões assinadas com a chave velha continuam
   valendo enquanto expiram; as novas usam a chave nova. Tirar o valor antigo encerra todas as
   sessões de uma vez, que é exatamente o que você quer se a chave vazou.

:::nota
Tudo aqui é a lista de um repositório. Se a sua tem um item que esta não tem, esse item vale
mais que todos estes — ele veio de uma indisponibilidade.
:::
