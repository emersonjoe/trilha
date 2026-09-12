---
title: O app administrável numa tarde
description: O trilha new --template app escreve uma aplicação administrável inteira — login, usuários, permissões, perfil, auditoria, chaves de API, configurações, organizações. Esta é a tarde que vai do comando à produção.
---

Toda aplicação interna é a mesma aplicação antes de ser a sua: alguém entra, alguém é
convidado, um papel decide o que cada um pode abrir, uma tela de conta, uma trilha de quem fez
o quê, uma chave para o script que chama a API, uma tela de configurações para um valor não
precisar de deploy e — mais cedo do que qualquer um planeja — uma segunda organização.

São três semanas de trabalho que ninguém lembra de ter decidido fazer. O `trilha new
--template app` escreve isso, e o que ele escreve não é um template especial: são as mesmas
[receitas do `trilha add`](/pt/referencia/cli#trilha-add) que você rodaria uma a uma, aplicadas
na criação. Esta página é a tarde seguinte — o comando, o primeiro login, o primeiro papel, a
sua entidade, a segunda organização e a produção.

## 1. O comando

```bash
trilha new empresa --template app
cd empresa
```

```text
  + app/admin/middleware.go
  + app/error.go
  + app/items/id_/page.go
  + app/items/new/page.go
  + app/items/page.go
  + app/layout.go
  + app/middleware.go
  + app/not_found.go
  + app/page.go
  + app/setup.go
  + app_test.go
  + internal/store/store.go
  + public/style.css
  + .gitignore
  + go.mod
  + public/ui.theme.css
  + public/ui.css
  + public/ui.js
  + public/ui.nav.js
  + public/ui.upload.js
  + public/ui.live.js
  + public/ui.chat.js
  + public/ui.island.js
  + public/ui.tree.js
  + internal/usuarios/usuarios.go
  + internal/usuarios/usuarios_test.go
  + internal/sessao/sessao.go
  + app/entrar/page.go
  + app/sair/route.go
  + internal/sessao/sessaotest/sessaotest.go
  + login_test.go
  + internal/auditoria/store.go
  + app/admin/auditoria/page.go
  + app/admin/chaves/page.go
  + internal/config/config.go
  + app/admin/config/page.go
  + internal/usuarios/convites.go
  + internal/usuarios/convites_test.go
  + internal/usuarios/papeis.go
  + app/admin/usuarios/page.go
  + app/admin/usuarios/middleware.go
  + app/convite/token_/page.go
  + usuarios_test.go
  + internal/acesso/acesso.go
  + internal/acesso/acesso_test.go
  + app/admin/permissoes/page.go
  + app/admin/permissoes/middleware.go
  + permissoes_test.go
  + internal/usuarios/perfil.go
  + internal/usuarios/perfil_test.go
  + app/perfil/page.go
  + app/perfil/middleware.go
  + app/perfil/email/token_/route.go
  + perfil_test.go
  + internal/organizacoes/organizacoes.go
  + internal/organizacoes/organizacoes_test.go
  + app/organizacoes/page.go
  + app/organizacoes/middleware.go
  + organizacoes_test.go

✓ projeto criado em empresa

  cd empresa
  trilha dev
```

A lista tem duas metades, e distinguir uma da outra vale o minuto que leva:

```text
empresa/
├── app/
│   ├── middleware.go        ← a guarda de sessão sobre a árvore toda: é ela que mantém gente fora
│   ├── layout.go            ← ui.Shell: menu lateral, barra de cima, menu do usuário — o menu é seu
│   ├── page.go              ← o painel, desenhado sobre o store de demonstração
│   ├── items/               ← a listagem de demonstração. Apague quando a sua entidade chegar
│   ├── entrar/, sair/       ← receita login
│   ├── perfil/, convite/    ← receitas profile e users
│   ├── organizacoes/        ← receita tenant
│   └── admin/               ← uma pasta por tela de administração, atrás de admin/middleware.go
├── internal/
│   ├── store/store.go       ← o store em memória da demonstração (não o de SQL; veja o passo 5)
│   ├── usuarios/            ← a tabela de usuários, convites, papéis, perfil
│   ├── sessao/              ← o auth.Sessions e o ajudante de teste
│   ├── acesso/              ← a matriz de permissões, como dados
│   ├── auditoria/, config/  ← a trilha de auditoria e a seção de configurações
│   └── organizacoes/        ← organizações e quem pertence a qual
├── *_test.go                ← um teste de integração por receita, na raiz, sobre o app real
└── trilha_gen.go            ← gerado; comite, nunca edite
```

Tudo de `internal/usuarios/usuarios.go` para baixo, na saída acima, veio de uma receita — e o
`app/setup.go` diz isso: as que tinham algo a ligar deixaram a linha marcada onde ligaram:

```text
// trilha:add tenant
// trilha:link users-permissions
// trilha:add settings
// trilha:add api-keys
// trilha:add audit
// trilha:add login
```

Esses marcadores são como o próximo `trilha add` acha o lugar dele, então deixe-os onde estão;
o código em volta é seu.

## 2. A primeira execução

Três variáveis de ambiente e mais nada:

```bash
export TRILHA_SECRET=$(trilha secret)      # assina o cookie de sessão
export ADMIN_EMAIL=ana@empresa.com         # o primeiro usuário
export ADMIN_PASSWORD=uma-senha-que-ninguem-adivinha
trilha dev
```

O `TRILHA_SECRET` não é opcional aqui: a sessão é um cookie assinado, e sem chave o login
recusa manter o estado em vez de fingir que mantém. O `trilha secret` imprime uma.

O primeiro administrador é semeado do ambiente na subida, uma vez, com o papel `admin` — uma
aplicação cujo primeiro usuário é uma senha num arquivo de código é uma aplicação com uma porta
que alguém esquece. Sem as duas variáveis o app sobe do mesmo jeito, a tela de entrar responde,
e ele avisa, na tela e no log:

```text
WARN usuarios: nobody can sign in yet fix="set ADMIN_EMAIL and ADMIN_PASSWORD and restart"
```

Abra `http://localhost:3000`, seja mandado para `/entrar`, entre, e todas as telas da lista
abaixo respondem 200. A primeira coisa a fazer de dentro é `/organizacoes`: crie a organização,
o que a põe na sua sessão e liga tudo do passo 6.

:::warning
**Todo store de um projeto novo é de memória.** Usuários, linhas de auditoria, chaves de API,
configurações, organizações — tudo se perde no restart, e o log avisa uma vez por seção. É
deliberado: o esqueleto roda sem banco na primeira tarde, e cada módulo aceita um store de
verdade quando você tiver um (`trilha add store` e depois o campo `Store`/`SettingsStore` que
cada pacote documenta). Não ponha na frente de um cliente antes disso.
:::

### O e-mail que ninguém garantiu

O `auth.Options.RequireVerifiedEmail` é de um dia que ainda não chegou neste projeto. Ele lê a
claim `email_verified` do provedor, então só tem o que ler quando há um provedor; o login que
este template traz é a sua própria tabela, e o endereço nela é aquele para onde o convite foi.
O equivalente local já está na tela de perfil: trocar o e-mail manda um link para o endereço
**novo**, e só a rota que esse link abre muda a linha.

No dia em que o login passa a ser de um provedor, a opção é a chave — e ela recusa **antes** do
`OnLogin`, para que uma regra sua nunca rode sobre um endereço que ninguém garantiu:

```go
func AdminSSO() *auth.Auth {
	p := auth.OIDC(
		os.Getenv("OIDC_ISSUER"),
		os.Getenv("OIDC_CLIENT_ID"),
		os.Getenv("OIDC_CLIENT_SECRET"),
		"https://empresa.example.com/auth/callback",
	)
	return auth.New(p, auth.Options{
		Store:                auth.NewMemoryStore(),
		RequireVerifiedEmail: true,
		LoginPath:            "/entrar",
		AfterLogin:           "/",
	})
}
```

As telas não mudam: elas perguntam ao `sessao.Atual(c)` quem está ali, e essa resposta tem a
mesma forma dos dois jeitos. [Autenticação](/pt/aprender/autenticacao) conta a história inteira.

## 3. Permissões, como dados

O `internal/acesso/acesso.go` é a matriz: módulos são o que a aplicação protege, níveis são
ordenados e cada um implica os de baixo, e um papel é uma linha. Acrescentar o seu primeiro
módulo é uma linha num arquivo:

```go
var AdminPolicy = auth.Policy{
	Modules: []string{"relatorios", "usuarios", "pedidos"},
	Levels:  auth.Levels{"ver", "editar", "administrar"},
	Roles: map[string]auth.Grants{
		"admin":     auth.All("administrar"),
		"comercial": {"pedidos": "editar", "relatorios": "ver"},
		"leitor":    {"relatorios": "ver"},
	},
}
```

`/admin/permissoes` é esse valor numa tela: o `ui.PolicyGrid` desenha um papel por linha, um
módulo por coluna e um nível em cada célula; abaixo da grade estão os papéis em si — criar,
remover, ver quem tem cada um — e abaixo deles, o que a pessoa que está olhando pode fazer. A
tela que edita a matriz é guardada pela matriz (`usuarios`/`administrar`), e não por um papel
escrito no middleware dela: uma exceção que mora fora da matriz é uma exceção que ninguém vê na
grade.

Uma pasta sua pede um nível do mesmo jeito:

```go
var adminOrders = adminSessions.RequirePolicy(AdminPolicy, "pedidos", "editar")
```

```go
func AdminOrdersMiddleware(c *trilha.Ctx, next trilha.Next) error { return adminOrders(c, next) }
```

Um nível é sobre um módulo, então as regras que um nível não expressa — o dono de um registro,
as linhas de uma organização — continuam sendo uma função, de propósito:

```go
var AdminOwnRows = adminSessions.RequireFunc(func(u *auth.User, c *trilha.Ctx) bool {
	return u.Tenant != "" && u.Tenant == c.Param("org")
})
```

Todo o resto da aplicação pergunta à matriz e nunca ao nome do papel, e é por isso que um papel
novo é uma linha numa tela e não um deploy. O [`auth`](/pt/referencia/auth) tem os detalhes.

## 4. As telas que já vieram

| Tela | URL | Receita | O que é |
|---|---|---|---|
| Entrar / sair | `/entrar`, `POST /sair` | `login` | a sua tabela de usuários, PBKDF2, sessão com 30 min de ociosidade |
| Minha conta | `/perfil` | `profile` | nome, senha, e-mail confirmado no endereço novo, e a lista de sessões abertas com "encerrar as outras" (`LogoutOthers`) |
| Usuários | `/admin/usuarios`, `/convite/{token}` | `users` | convidar (sem senha: quem a define é a pessoa, no link), papel, desativar, redefinir |
| Permissões | `/admin/permissoes` | `permissions` | a grade acima, e os papéis abaixo dela |
| Auditoria | `/admin/auditoria` | `audit` | quem fez o quê, em quê e de onde — `c.Audit(...)` é uma linha e a tela lê |
| Chaves de API | `/admin/chaves` | `api-keys` | emitir (aparece uma vez), revogar, escopos — e o uso por chave, rota e dia nos últimos 30 |
| Configurações | `/admin/config` | `settings` | um struct com tags é a tela; os padrões são com o que o app roda antes de alguém salvar |
| Organizações | `/organizacoes` | `tenant` | criar, trocar, ativar, membros e configurações por organização |

Trocar a senha encerra todas as outras sessões, inclusive a que trocou: uma senha é trocada
porque alguém pode ter a antiga, e esse alguém pode estar logado agora.

Quatro receitas existem e **não** estão neste template. Elas estão a um comando de distância, e
cada uma põe as telas onde você apontar:

```bash
trilha add approvals    # a fila que espera uma pessoa: abrir, decidir, prazo
trilha add search       # uma caixa sobre vários tipos de coisa, agrupados por tipo
trilha add connections  # serviços externos: nome, URL, segredo selado e um botão Testar
trilha add share-link   # um link assinado com prazo, para quem não tem conta
```

Uma receita que precisa de outra recusa e diz qual (`Needs`), então ordená-las errado custa uma
mensagem e não um projeto quebrado.

### O menu não se escreve sozinho

O `app/layout.go` é seu, então nenhuma receita o edita — o que significa que as telas acima
existem e ainda não estão no menu lateral. Cole:

```go
func AdminNav(admin bool) []ui.NavGroup {
	return []ui.NavGroup{
		{Label: "Work", Items: []ui.NavItem{
			{Href: "/", Label: "Dashboard", Icon: "house"},
			{Href: "/admin/pedidos", Label: "Orders", Icon: "search"},
			{Href: "/perfil", Label: "My account", Icon: "user"},
			{Href: "/organizacoes", Label: "Organisations", Icon: "building"},
		}},
		{Label: "Admin", Hide: !admin, Items: []ui.NavItem{
			{Href: "/admin/usuarios", Label: "Users", Icon: "users"},
			{Href: "/admin/permissoes", Label: "Permissions", Icon: "lock"},
			{Href: "/admin/auditoria", Label: "Audit", Icon: "list"},
			{Href: "/admin/chaves", Label: "API keys", Icon: "key"},
			{Href: "/admin/config", Label: "Settings", Icon: "settings"},
		}},
	}
}
```

`Hide` é cosmético. Quem mantém alguém fora de `/admin` é o `app/admin/middleware.go`; esconder
o link só poupa a pessoa do 403.

## 5. A sua entidade

Escreva o struct primeiro — as tags são a tela — e depois gere as três páginas em volta dele:

```bash
trilha generate crud pedidos.Pedido --at app/admin/pedidos
```

```text
  + internal/pedidos/pedido_store.go
  + app/admin/pedidos/page.go
  + app/admin/pedidos/new/page.go
  + app/admin/pedidos/id_/page.go
  + pedido_crud_test.go
  + app/setup.go
  app/admin/middleware.go fecha esta pasta: o teste gerado abre uma sessão antes, com empresa/internal/sessao/sessaotest

✓ /admin/pedidos já responde; trilha_gen.go está atualizado

✓ /admin/pedidos/new já responde; trilha_gen.go está atualizado

✓ /admin/pedidos/{id} já responde; trilha_gen.go está atualizado
```

Duas coisas nessa saída são o template pagando o que prometeu. O destino é dentro de
`app/admin/`, então o teste gerado abre uma sessão com o ajudante que a receita de login
escreveu, em vez de afirmar um 401 que ninguém queria. E o `app/setup.go` ganhou a linha que
provê o store — uma interface, com uma implementação em memória, para que as telas não mudem no
dia em que o SQL chegar. Ponha a entidade no menu (acima) e a tarde termina com uma aplicação
funcionando:

```bash
trilha check
```

```text
✓ gen
✓ gofmt
✓ vet
✓ test
✓ audit
– openapi (skipped)
ok
```

### E com banco

O `--store postgres` (ou `sqlite`) escreve a implementação SQL dessa mesma interface e mais uma
migração, e precisa da receita [`store`](/pt/referencia/store), que é dona do `internal/store`:

```bash
trilha add store
trilha generate crud pedidos.Pedido --store postgres --at app/admin/pedidos
```

Duas coisas a saber antes de rodar o primeiro desses **neste** template:

- `internal/store/store.go` já está ocupado pelo store em memória da listagem de demonstração.
  Aposente a demonstração antes de a receita chegar — `rm -rf app/items internal/store/store.go
  app_test.go`, tire as linhas do `store.New()` do `app/setup.go` e aponte o painel para a sua
  entidade — ou os dois pacotes colidem e quem avisa é o `go vet`.
- Dali em diante o app precisa de `DATABASE_URL` para subir, testes inclusive, porque o
  `store.Setup` abre o pool e aplica as migrações antes da primeira requisição. Escolha um
  driver e faça o import em branco dele em `internal/store/driver.go`;
  [Banco de dados](/pt/receitas/banco-de-dados) é o resto.

## 6. A segunda organização

Multi-tenant por uma coluna é a forma comum, e esquecer essa coluna em uma consulta de quarenta
é o bug comum dela: o relatório que mostra as linhas de outro, descoberto por um cliente.

O framework carrega a organização na sessão e recusa sessão sem uma. A consulta é sua:

```go
func AdminOrdersOf(c *trilha.Ctx, db *sql.DB) ([]AdminOrder, error) {
	rows, err := db.QueryContext(c.Context(), `
		SELECT id, cliente, total
		  FROM pedidos
		 WHERE org_id = $1
		 ORDER BY criado_em DESC
		 LIMIT 50`, auth.Tenant(c))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AdminOrder
	for rows.Next() {
		var o AdminOrder
		if err := rows.Scan(&o.ID, &o.Cliente, &o.Total); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}
```

O `auth.Tenant(c)` lê a sessão — nunca a URL, nunca um campo escondido, porque valor que o
navegador manda é valor que o navegador escolhe. Abaixo da guarda de sessão, nas pastas que têm
linhas, vai a outra metade:

```go
var AdminTenantGuard = adminSessions.RequireTenant()
```

Menos em `/organizacoes`: guardar a tela onde a organização é escolhida com a regra de que já
tem que haver uma escolhida é um laço sem saída.

E o `trilha audit` conta. Ele não tem parser de SQL e diz isso — o que ele aponta é um lugar
para olhar, não um veredito — mas "esta tabela é filtrada por tenant em sete consultas e nesta
não" é uma frase sobre a qual alguém consegue agir:

```text
✓ consultas que talvez estejam sem o filtro de tenant
```

## 7. Produção

O [checklist de produção](/pt/receitas/checklist-de-producao) é a forma longa. Num projeto que
saiu deste template, o primeiro dia é assim:

```bash
trilha audit --no-vuln
```

```text
✗ TRILHA_SECRET não definido neste ambiente
    cookies assinados (sessão) não funcionam em produção; gere com: openssl rand -base64 32
! TRILHA_TRUSTED_PROXIES não definido
    atrás de um proxy (nginx, load balancer) defina os CIDRs para HSTS, IP do cliente e rate limit corretos
! AllowedHosts não definido
    liste os hosts que o app atende (Config.AllowedHosts ou TRILHA_ALLOWED_HOSTS); sem isso um cabeçalho Host forjado envenena caches e links de redefinição
✓ métricas não expostas
! 2 campo(s) string vêm de fora sem limite de tamanho
    acrescente max= na tag validate: sem ele quem recusa é a coluna, em produção, com a mensagem do driver (Amount, Suporte)
✓ verificações de prontidão registradas
! a política declara relatorios e nenhuma rota exige
    ponha auth.RequirePolicy(Policy, módulo, nível) no middleware.go daquela área, ou tire o módulo da política
✓ segredo do cliente OIDC fora do código
! 1 rota(s) de escrita em route.go sem CSRF
    um route.go é API, e API não confere o token: um formulário de outro site consegue postar em /sair. Diga que o ramo é de páginas com `var Kind = trilha.KindPage` num kind.go acima delas, ou ligue Config.CSRFForAPI se a API for mesmo o cliente
✓ trilha_gen.go atualizado
✓ Go 1.25.14
✓ .gitignore cobre .trilha/ e bin/
✓ consultas que talvez estejam sem o filtro de tenant
✓ go vet limpo
erro: 1 item(ns) crítico(s)
```

Quatro deles são seus para responder antes do primeiro deploy:

1. `TRILHA_SECRET` no ambiente — um de verdade, o mesmo em todas as réplicas, rotacionado como
   qualquer outro segredo. Sem ele a sessão não funciona.
2. `Config.AllowedHosts` (ou `TRILHA_ALLOWED_HOSTS`) com os nomes de host que o app atende.
3. O módulo `relatorios` que a matriz declara e nenhuma rota exige: ou guarde aquela área com
   ele, ou tire o módulo — permissão que não concede nada é pior do que nenhuma, porque alguém
   vai concedê-la.
4. O `POST /sair` sem conferência de CSRF: é um `route.go`, e um `route.go` é API. A correção é
   um `kind.go` dizendo que aquele ramo é de páginas, e a mensagem o nomeia.

Depois disso o `trilha check` é o portão — gen, gofmt, vet, test, audit e openapi num comando só
— e o [Docker](/pt/receitas/docker) é a imagem: o `trilha build` faz um binário estático único
com o `public/` embutido, e o contêiner é esse arquivo mais o ambiente.

## Daqui para onde

- [`trilha add`](/pt/referencia/cli#trilha-add) — as dezessete receitas, das quais este template
  já começa com oito.
- [Autenticação](/pt/aprender/autenticacao) e [`auth`](/pt/referencia/auth) — sessão, política,
  tenant e chaves de API, a fundo.
- [Telas prontas](/pt/aprender/telas-prontas) — o que o `ui` desenha, e as demos vivas.
- [Banco de dados](/pt/receitas/banco-de-dados), [Tarefas](/pt/receitas/tarefas),
  [E-mail](/pt/receitas/email) — as três coisas que este app pede em seguida.
