# Modelo de ameaças

> 🇧🇷 Português · [🇺🇸 English](../../SECURITY-MODEL.md)

O [`SECURITY.md`](SECURITY.md) diz como relatar uma vulnerabilidade e o que o framework
garante. Este documento diz **contra o quê** ele se defende, para que uma auditoria consiga
separar escolha deliberada de esquecimento — e para que as lacunas fiquem escritas em vez de
presumidas.

Ele descreve o framework, não o seu app. Tudo abaixo foi conferido contra o código deste
repositório; controle citado aqui existe e é alcançável pelo `Config` ou por uma chamada
documentada. O que não tem controle está marcado como **aberto**.

## Ativos

| Ativo | Por que vale a pena atacar |
|---|---|
| A sessão de quem visita | É a identidade: tomá-la é tomar a conta. |
| Os dados da aplicação | O que os handlers leem e escrevem, inclusive dados de outras pessoas. |
| O segredo de assinatura (`TRILHA_SECRET`) | Com ele o atacante fabrica qualquer cookie de sessão. |
| A disponibilidade do processo | Uma máquina, um binário: esgotá-lo derruba o site. |
| A cadeia de build | `trilha_gen.go`, o grafo de módulos, a CLI: código que roda com privilégio total. |
| Arquivos estáticos e uploads | Arquivo servido pela origem do app roda na origem do app. |

## Fronteiras de confiança

```text
   navegador ──(1)──► proxy / TLS ──(2)──► processo trilha ──(3)──► banco, APIs
   (hostil)                                   │      │
                                              │      └──(4)──► disco: public/, uploads
                                              │
   quem desenvolve ──(5)──► CLI trilha ───────┘  (código gerado, dependências)
```

1. **Navegador → app.** Tudo é escolhido por quem chama: método, alvo, cabeçalhos, cookies,
   corpo, `Host`, `Origin`. Nada que atravessa aqui é confiável.
2. **Proxy → app.** `X-Forwarded-For` e `X-Forwarded-Proto` só querem dizer alguma coisa se o
   par realmente for o seu proxy. O framework acredita neles **só** vindo de um CIDR de
   `Config.TrustedProxies`; de qualquer outro, vale o endereço do par.
3. **App → serviços de fora.** Fora do alcance do framework: ele não abre nem gerencia essas
   conexões. É do seu app — com uma exceção, o `Config.Upstreams`, onde quem faz a chamada é
   o framework. Ali o alvo é fixo na configuração e nenhuma parte da requisição o move; a
   credencial injetada pelo `Upstream.Headers` é a da sessão, nunca o `Authorization` que o
   navegador mandou; e a resposta volta com os cabeçalhos do próprio upstream. O que esse
   upstream devolve é dado de fora desta fronteira de confiança: é repassado ao cliente, não
   interpretado pelo app.
4. **App → disco.** `Config.Public` e `Config.Mounts` são um `fs.FS`, que não endereça nada
   acima da própria raiz; upload entra pelo `Upload.Save`, que recusa nome capaz de sair do
   diretório.
5. **Quem desenvolve → build.** A CLI gera código que o app compila. O arquivo gerado é
   commitado e o `trilha audit` avisa quando ele está velho, então geração silenciosa aparece
   na revisão.

## Agentes

- **Visitante anônimo** — quem chama, por padrão, toda rota sem middleware de autenticação.
- **Pessoa autenticada** — tem sessão; pode atacar o dado de outra pessoa pelas mesmas rotas.
- **Quem opera** — faz o deploy, guarda os segredos, lê os logs.
- **O proxy à frente** — confiável só onde o `TrustedProxies` disser.
- **Atacante na rede** — vê e forja requisições; presume-se que controle qualquer cabeçalho.

## Ameaças e controles (STRIDE)

Cada linha nomeia o controle e onde ele é configurado. A explicação de cada um mora na
[referência de segurança](https://emersonjoe.github.io/trilha/pt/referencia/seguranca); não é
recopiada aqui.

### Falsificação de identidade

| Ameaça | Controle |
|---|---|
| Cookie de sessão forjado | HMAC-SHA256 com chave de pelo menos 32 bytes, validade dentro do token, conferida em tempo constante (`SetSigned`/`Signed`). |
| Fixação de sessão | Identificador novo a cada login (`auth`). |
| Cookie roubado em HTTP | `HttpOnly`, `SameSite=Lax` e `Secure` sempre que a requisição é HTTPS, mais HSTS. |
| Trocar a chave sem deslogar todo mundo | `Config.PreviousSecret` continua verificando enquanto a nova assina. |
| Identidade forjada vinda do IdP | OIDC com `state`, `nonce`, PKCE `S256` e assinatura conferida contra o JWKS do provedor (`auth`). |
| IP de cliente forjado | `X-Forwarded-For` só é honrado vindo de `TrustedProxies`. |
| **`Host` forjado** (cache envenenado, link de redefinição apontando para o atacante) | `Config.AllowedHosts`: `Host` fora da lista leva 400 antes do roteador. **Desligado por padrão** — lista vazia mantém o comportamento de hoje. |

### Adulteração

| Ameaça | Controle |
|---|---|
| Requisição de outra origem | Token CSRF de duplo envio em todo formulário; `Config.CSRFForAPI` estende ao `route.go`. |
| Cookie editado pelo cliente | A assinatura cobre valor e validade; qualquer mudança falha na verificação. |
| HTML/JS injetado | O `h` escapa texto e atributo; o `tmpl` escapa por contexto; o `h.Raw` é a única saída explícita. |
| Script inline injetado na página | CSP com nonce por requisição; `base-uri`, `form-action` e `frame-ancestors` fechados. |
| Upload que não é o que diz ser | O tipo é detectado no conteúdo, nunca no nome nem no que o cliente anunciou (`FileRules.Accept`). |
| Travessia de caminho, na entrada e na saída | `fs.FS` para os estáticos; o `Upload.Save` recusa nome que saia do diretório e grava com modo 0600. |
| Download que o navegador roda em vez de guardar | O `Attachment` e o `Inline` põem o tipo a partir do conteúdo, mandam sempre `X-Content-Type-Options: nosniff`, e o `Inline` recusa HTML, SVG e XML de saída — documento com script servido da própria origem do app é XSS armazenado com passos a mais. |
| Nome de arquivo que escreve um cabeçalho por conta | O nome de um download passa pelo mesmo saneador do nome de um upload e sai como `filename*` percent-encoded mais um `filename` ASCII entre aspas: aspa, separador ou quebra de linha no nome não conseguem acrescentar parâmetro nem linha. |
| Segredo guardado em claro | O `trilha.Seal` cifra com AES-256-GCM sob uma chave derivada do segredo da app (HKDF-SHA256, chave diferente da que assina), com nonce aleatório e a versão do formato como dado adicional. O `trilha.Secret` é isso mais todas as saídas acidentais fechadas: JSON, `String` e `slog` respondem uma máscara, o `Reveal()` é o único jeito de ler, e o `driver.Valuer` faz o que chega à coluna ser o selo e nunca a string por baixo. **A chave é o segredo da app**: isto protege um dump do banco e um backup, não o operador nem quem tem o ambiente. |
| Documento enquadrado por uma página de outra origem | Toda resposta leva `X-Frame-Options: DENY` e `frame-ancestors 'none'`. O `Inline` — e só ele — afrouxa o par para a mesma origem naquela resposta, porque um documento que ninguém pode enquadrar não aparece no lugar, que é para o que o `Inline` existe. A folga nunca vale para uma página, para um download, nem para um app que escreveu o próprio `Security.CSP`. |

### Repúdio

| Ameaça | Controle |
|---|---|
| Requisição bloqueada não deixa rastro | Todo bloqueio (CSRF, 401/403, 413, 429, `host`, pânico) emite um `SecurityEvent`: `slog.Warn`, métrica `trilha_security_events_total` e `Config.OnSecurityEvent`. |
| Log que não dá para correlacionar | Um id por requisição, no log e no corpo do erro; o `traceparent` é honrado quando vem bem formado. |
| **Ação de negócio não é auditada** | **Aberto.** O framework registra o que bloqueia, não o que o seu app decide. A trilha de auditoria do domínio é sua. |

### Vazamento de informação

| Ameaça | Controle |
|---|---|
| Stack trace ou detalhe interno num erro | Em produção o `problem+json` leva status, título e id da requisição; o detalhe de um 5xx só aparece em `Dev`. |
| Métrica ou sonda lida por qualquer um | `Observability.Token` ou `Observability.Trusted`; o `trilha audit` reprova `/metrics` exposto sem nenhum dos dois. |
| Outra origem lendo as respostas | CORS desligado até ser configurado; `Cross-Origin-Opener-Policy: same-origin` e `Referrer-Policy: strict-origin-when-cross-origin` por padrão. |
| Listagem de diretório | O servidor de estáticos recusa diretório. |
| **Cookie assinado é legível** | **De propósito.** O `SetSigned` assina, não cifra: o valor é visível para quem tem o cookie. Ponha nele um identificador, não um segredo. |
| **O segredo no ambiente** | **Aberto.** O `TRILHA_SECRET` vive no ambiente do processo; o framework não se integra a gerenciador de segredos. O `trilha audit` confere que existe e que é longo o bastante — nada além disso. |

### Negação de serviço

| Ameaça | Controle |
|---|---|
| Cliente lento segurando conexão | `Timeouts` (cabeçalho 10s, leitura 30s, escrita 60s, ociosa 120s). |
| Corpo enorme | `Config.MaxBodyBytes` (1 MiB por padrão) responde 413; `MaxHeaderBytes` 64 KiB. |
| Enxurrada de requisições | Balde de fichas por IP (`Config.RateLimit`) e `trilha.Limit` para uma subárvore; 429 com `Retry-After`. |
| Handler que entra em pânico derruba o processo | Recuperado na borda, contado em `trilha_panics_total`, respondido como 500. |
| **Ataque volumétrico, ou o que custa mais ao app do que ao atacante** | **Aberto, e fora de escopo.** Um processo só não absorve isso: é da rede à frente (CDN, WAF, limites do provedor). |

### Elevação de privilégio

| Ameaça | Controle |
|---|---|
| Chegar a uma rota sem sessão | Middleware do `auth` sobre a subárvore; `HasRole` para papel. |
| Logout que não desloga | `Store` (por exemplo `MemoryStore`) revoga na hora; sem ele o cookie é sem estado e vale até vencer. |
| Dependência com vulnerabilidade conhecida | Zero dependências externas no runtime e na CLI, garantido por teste; `govulncheck` em todo CI. |
| Defeito no parsing de entrada de terceiro | `go test -race` e seis alvos de fuzzing no CI (casamento de rota, `Bind`, cookie assinado, `traceparent`, escape). |
| **Autorização dentro do domínio** | **Aberto, de propósito.** "Esta pessoa pode ver esta fatura?" depende do seu modelo de dados. O framework entrega a identidade e os papéis; a decisão é sua. |

## Sessão sem provedor

O `auth.Sessions` põe a credencial dentro do app em vez de num provedor de identidade, e
isso move três coisas neste modelo:

- **O hash da senha passa a ser um ativo deste app.** `auth.CheckPBKDF2` lê e
  `auth.HashPBKDF2` grava `pbkdf2_sha256$iteracoes$sal$hash`; a comparação é em tempo
  constante, e um hash que a função não consegue ler é um false, nunca um panic. O número de
  iterações viaja dentro de cada hash, então aumentar o padrão não tranca ninguém do lado de
  fora.
- **O login vira um oráculo de adivinhação se ninguém o limitar.** Com OIDC o provedor
  absorve isso; aqui é o `Config.RateLimit`, e o `trilha audit` diz quando um `Login` está
  sem ele. Responda a mesma mensagem para e-mail errado e senha errada, e demore o mesmo
  tanto.
- **O `User.Extra` viaja com a sessão** — no cookie assinado ou na `Store`. Ele é para um
  token que um upstream precisa, um tenant, um plano: nunca para uma senha, e nunca para
  mais do que um cookie deveria carregar.

A revogação é trabalho da `Store`, como no OIDC: sem uma, a sessão vive no cookie até
expirar e o `Logout` só faz aquele navegador parar de mandá-la.

## O que o framework não faz por você

- Decidir quem pode ver o quê (autorização de domínio).
- Guardar segredo, girá-lo ou mantê-lo fora dos seus logs.
- Proteger o que o seu handler faz com o dado depois do `Bind` — SQL, shell, API de terceiro.
- Absorver ataque volumétrico, nem substituir um WAF.
- Redirecionar para o host canônico (`www` → apex): declare os hosts que você atende e resolva
  o resto no proxy.
- Cifrar qualquer coisa em repouso.

## Como este documento se mantém honesto

Cada linha acima aponta para código deste repositório. Quando um controle muda, este arquivo
muda no mesmo commit — a mesma regra da documentação pública. Conferido contra a
[spec 004](../../specs/004-seguranca/) (cabeçalhos, CSRF, cookies assinados, rate limit), a
[spec 014](../../specs/014-observabilidade/) (eventos, sondas, métricas) e a
[spec 034](../../specs/034-modelo-de-ameacas/) (este documento e o `AllowedHosts`).
