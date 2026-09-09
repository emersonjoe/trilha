# Spec 083 — módulo trilha/webhook

- **Issue**: [#112](https://github.com/emersonjoe/trilha/issues/112) — a issue é a fonte do escopo.
- **Branch**: `083-webhook`
- **Versão**: 0.65.0

## Por quê

Avisar outra aplicação é o que todo app interno acaba precisando, e o que o iniciante escreve é
um `http.Post` dentro do manipulador. Isso tem quatro problemas, e nenhum deles aparece no dia
em que o código é escrito.

A requisição do usuário fica esperando o servidor do outro lado — que é de outra empresa, de
outra rede, e às vezes de outra década. Não há assinatura, então o receptor não tem como saber
que a chamada veio de você e não de quem descobriu a URL. Não há retentativa: o parceiro
reiniciou às três da manhã e aquele evento simplesmente não existiu. E não há registro, então
quando alguém pergunta "vocês mandaram?" a resposta é procurar no log.

Tem um quinto, mais feio: a URL é do parceiro, mas quem digita é alguém de dentro. Um webhook
apontado para `http://169.254.169.254/` é o seu servidor buscando as credenciais da própria
máquina e entregando para quem cadastrou o endereço.

## O que muda

Módulo opcional `trilha/webhook`: assinatura, retentativa com espera crescente, registro de
cada entrega e uma tela.

```go
// app/setup.go
Hooks = webhook.New(webhook.Options{
	Store:  webhook.Memory(),
	Events: []string{"documento.criado", "documento.processado", "fluxo.concluido"},
	Env:    a.Env(),
})

// onde o evento acontece — uma linha, e a resposta não espera a rede
Hooks.Emit(c, "documento.processado", doc)
```

O que o `Emit` faz: acha as assinaturas do tenant para aquele evento, grava uma entrega por
assinatura e volta. A entrega sai num worker, com:

```
POST https://parceiro/hook
Content-Type: application/json
X-Webhook-Id: dlv_…                (idempotência do lado de lá)
X-Webhook-Event: documento.processado
X-Webhook-Timestamp: 1757343845
X-Webhook-Signature: sha256=…      (HMAC de "timestamp.corpo")
```

2xx em 10 s é `Delivered`. Qualquer outra coisa espera e tenta de novo — 1 min, 5, 30, 2 h,
12 h — e na sexta desiste em `Failed`, guardando o último status e o primeiro quilobyte da
resposta. **O corpo da resposta é guardado porque é ele que diz o que houve**: "422
campo_x obrigatório" resolve em um minuto o que um "falhou" não resolve em uma tarde.

| Símbolo | O que é |
|---|---|
| `New(Options) *Hooks`, `(*Hooks).Setup(a)` | o motor; `Setup` liga os workers e o relógio das retentativas |
| `Emit(c, evento, payload) error` | grava as entregas e volta |
| `Subscribe(c, Subscription) (trilha.Secret, error)` | cadastra e devolve o segredo — a única vez que ele aparece |
| `Revoke`, `Retry`, `Ping` | o que a tela faz |
| `Handle(c) error` | um POST só, despachado pelo campo `action` |
| `Verify(r, segredo) ([]byte, error)` | o outro lado: quem **recebe** um webhook em Go |
| `Store`, `Memory()` | onde as assinaturas e as entregas ficam |
| `ui.WebhooksPanel(c, dados, opts)` | cadastrar, ver entregas, reenviar, testar |

**A URL é validada, e é a parte que ninguém escreve sozinho.** `https://` sempre; `http://`
apenas em `Env: Dev`; e endereço que resolve para fora da internet pública é recusado no
cadastro **e outra vez na hora de entregar** — porque um DNS que responde uma coisa no cadastro
e outra na entrega é o ataque, não o acidente.

Uma correção ao que estava escrito aqui antes de o módulo existir: **em dev, loopback e rede
privada passam**. Um receptor em `localhost:4000` é como qualquer pessoa experimenta isto na
primeira vez, e uma checagem que recusa isso é uma checagem que alguém desliga inteira — o que
custaria mais do que a regra protege. O link-local (`169.254.0.0/16`, onde moram as credenciais
da nuvem), o não especificado, o multicast e o reservado continuam recusados em todo ambiente:
não são o ambiente de desenvolvimento de ninguém, e uma configuração de dev que sobe para
produção é exatamente como esse caso morde.

O segredo é um `trilha.Secret` (spec 075): cifrado na coluna, mascarado no log, e mostrado uma
vez só, com o `ui.SecretOnce`.

## Fora de escopo

- **Construir sobre o `task` (#111), como a issue propõe.** Uma entrega já tem máquina de
  estados própria e uma hora para acontecer; o `task` não sabe agendar e deduplica por chave,
  que não é o que uma entrega quer. Empilhar os dois daria dois registros do mesmo trabalho e
  duas telas discordando. O motor daqui é o mesmo formato — pool pequeno, `panic` virando erro,
  `Shutdown` que espera — com um relógio a mais, que é a parte que o `task` não tem.
- **`webhook.SQL(db)`.** Mesma escolha do `task` e de todo store deste repositório: escolher
  dialeto de placeholder e ser dono de um DDL é o que o framework não faz. A receita traz o
  arquivo inteiro.
- **A seção `webhooks` do `trilha openapi`.** É o único item do aceite que fica de fora desta
  versão: o gerador hoje descreve rotas que existem no `app/`, e webhooks são rotas do
  *parceiro* — a seção pede um vocabulário novo no gerador, não um campo a mais. Merece spec
  própria, e a issue fica aberta para ela.
- **Assinatura por chave assimétrica, e `X-Webhook-Signature` com mais de um segredo ativo.**
  Rotação de segredo é real e é a próxima coisa a fazer; hoje `Subscribe` de novo é o caminho.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `crypto/hmac`, `crypto/sha256`, `net`, `net/http`, `sync`, `time` |
| VI — teste primeiro | assinatura, backoff, desistência, SSRF no cadastro e na entrega, reenvio ligado ao original |
| VII — segurança por padrão | HTTPS fora de dev, IP privado recusado duas vezes, segredo cifrado e mostrado uma vez, `Verify` com prazo e comparação em tempo constante |
| Convenção nova | rota no `examples/blog` + teste de integração |

## Tarefas

- [x] T001 Teste que falha: a assinatura confere do outro lado, com `Verify`
- [x] T002 `Subscription`, `Delivery`, `Store`, `Memory`, assinatura, `Verify`
- [x] T003 Testes: 500 → espera e tenta de novo; cinco falhas → `Failed` com corpo; IP privado recusado no cadastro e na entrega
- [x] T004 `Emit`, o motor, `Setup`, `Shutdown`, `Retry`, `Ping`, `Handle`
- [x] T005 `ui.WebhooksPanel`
- [x] T006 Uso no `examples/blog` + teste de integração
- [x] T007 Receita en + pt (com o store SQL), referência en + pt
- [x] T008 `CHANGELOG.md`, `version`, `ROADMAP.md`, `make test`, `scripts/release.sh 0.65.0`

## Aceitação

- **SC-001** O que o `Emit` manda é aceito pelo `Verify` do outro lado, e recusado quando o
  corpo, o horário ou o segredo mudam.
- **SC-002** Um receptor que responde 500 é tentado de novo na hora certa; cinco falhas param
  em `Failed` com o último status e o começo do corpo.
- **SC-003** URL para o link-local dos metadados da nuvem é recusada no cadastro e também na
  entrega, em qualquer ambiente; loopback e rede privada são recusados fora de dev.
- **SC-004** `http://` passa em dev e é recusado fora dele.
- **SC-005** O reenvio é uma entrega nova, ligada à original, e a original continua lá.
- **SC-006** O `Emit` não espera a rede: o manipulador volta antes da entrega.
- **SC-007** `TestNoExternalDeps` continua verde.
