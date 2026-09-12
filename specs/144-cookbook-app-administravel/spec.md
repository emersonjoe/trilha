# Spec 144 — o app administrável numa tarde

> Mudança de conteúdo do site (`site/internal/docs/content`) mais um arquivo de exemplo em
> `examples/cookbook`, sem convenção nova em `app/` e sem mudança de API pública: spec curta.
> Entra junto a correção de duas mensagens da CLI que a página precisaria colar quebradas
> (ver "O que muda"); é uma linha de tabela e um teste, não um plano à parte.

- **Issue**: #194 — a issue é a fonte do escopo; aponte para ela, não a reescreva aqui.
- **Branch**: `feat/cookbook-app-administravel`
- **Versão**: 0.123.0

## Por quê

`trilha new --template app` existe desde a 0.95 e é "o template do mês" desde a 0.100: um
comando e sai uma aplicação administrável inteira — login próprio, shell com menu, usuários,
permissões como matriz, perfil, auditoria, chaves de API, configurações e organizações. O site
diz que o template existe (`reference/cli.md#trilha-new`, uma linha) e não mostra o que ele
faz. Quem lê `learn/examples.md` vê seis apps em `examples/` e nenhum deles é esse.

O resultado é que a coisa mais pronta do framework é a menos visível: a pessoa que abriria o
projeto com um comando começa pelo `--template blog` e reconstrói à mão, em duas semanas, as
telas que o `trilha add` escreve em dois minutos. Falta o capítulo que acompanha a tarde
inteira — do comando à primeira tela, da primeira tela ao primeiro papel, do primeiro papel à
entidade do domínio, e daí à produção.

## O que muda

Uma página nova de cookbook, `cookbook/admin-app.md` (pt: `receitas/app-administravel.md`),
no formato dos outros capítulos: sete passos, na ordem de quem senta para trabalhar.

1. `trilha new empresa --template app` — a saída real do comando e a árvore comentada: o que é
   esqueleto (o `app/items` que se apaga), o que é receita, onde estão os marcadores
   `// trilha:add <receita>` do `app/setup.go`.
2. A primeira execução: `TRILHA_SECRET`, `ADMIN_EMAIL`/`ADMIN_PASSWORD`, a primeira
   organização, e o que o `auth.Sessions` do template garante (e o que o
   `Options.RequireVerifiedEmail` **não** garante aqui — ver a decisão abaixo).
3. Permissões: `auth.Policy` como dados em `internal/acesso`, `ui.PolicyGrid` na tela, e a
   rota nova guardada por `Flow.RequirePolicy` ou `Flow.RequireFunc`.
4. As telas que já vêm, uma linha cada, com a URL e o arquivo — e as quatro receitas que o
   template **não** inclui por padrão, com o comando que as acrescenta.
5. A entidade do domínio: `trilha add store`, `trilha generate crud pedidos.Pedido --store
   postgres --at app/admin/pedidos`, e onde ela aparece no menu do `Shell`.
6. Multi-tenant por coluna: `auth.Tenant(c)` na cláusula, `RequireTenant` no middleware e o
   que o `trilha audit` cobra quando uma consulta de quarenta esquece o filtro.
7. Produção: `trilha secret`, `AllowedHosts`, `trilha check`, e o Docker do cookbook.

A página entra na tabela de `cookbook/index.md` (en e pt), na tabela de `learn/examples.md`
(en e pt) e na navegação de `site/internal/docs/docs.go` — o que a põe no `llms.txt`, que é
gerado da navegação. Os blocos `go` saem de `examples/cookbook/adminapp.go`, arquivo novo que
compila com o resto do módulo, como todo bloco do cookbook.

### Três decisões que a página registra

**O template escreve oito receitas, não onze.** `recipeList` em `cmd/trilha/new.go` pede
`login, audit, api-keys, settings, users, permissions, profile, tenant`. `share-link`,
`approvals`, `search` e `connections` existem como receitas e não entram por padrão. A página
conta as oito como "o que sai do comando" e as quatro como "uma linha de distância"
(`trilha add approvals search connections share-link`), em vez de repetir a lista da issue —
que é a lista de receitas do kit, não a do template.

**`RequireVerifiedEmail` é do provedor, não do login local.** A opção é conferida no
`Callback` do OIDC (`auth/flow.go`); o `Flow.Login` do login local não passa por ela, porque
não há provedor para confiar. No app do template, quem garante o endereço é a troca de e-mail
em dois passos do `/perfil` — o link vai para o endereço novo e só a rota que ele abre muda a
linha. A página diz isso onde a issue pedia a opção, e mostra o `RequireVerifiedEmail: true`
como o que se liga no dia em que o login vira um provedor.

**Duas mensagens da CLI faltavam no `msgs`.** O passo 5 roda `trilha generate crud` dentro de
`app/admin/`, que é uma pasta guardada, e a saída real era:

```
  crud guard helper
%!(EXTRA string=app/admin/middleware.go, string=empresa/internal/sessao/sessaotest)
```

As chaves `crud guard helper` e `crud guard skip` nunca foram escritas em `cmd/trilha/i18n.go`,
então o `t()` devolvia a própria chave — uma string sem verbo — e o `Printf` despejava os
argumentos. Como a página cola saída real, a correção entra aqui: as duas entradas, nas duas
línguas, e um teste que varre `cmd/trilha/*.go` atrás de toda chave passada ao `t()` e exige
que exista no `msgs`, para que a próxima falte antes de sair impressa.

## Fora de escopo

- Demo viva do app administrável em `site/internal/demos`: o capítulo é sobre rodar o comando
  na própria máquina, e uma demo do shell inteiro seria a aplicação de novo, não uma peça.
- Mudar o que `--template app` escreve (incluir as outras quatro receitas, traduzir os
  caminhos): a página documenta o que o comando faz hoje; mudar isso é outra issue.
- Página de cookbook para as receitas `approvals`, `search`, `connections` e `share-link`
  isoladas — o capítulo aponta para a referência de cada uma.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | Conteúdo Markdown, um arquivo em `examples/cookbook` com a biblioteca padrão e duas entradas de mensagem na CLI; `TestNoExternalDeps` intocado. |
| VI — teste primeiro | `site/internal/docs/admin_app_test.go` e `cmd/trilha/i18n_test.go` falham antes do conteúdo e da correção; o e2e do `--template app` já prova que o app do capítulo compila e passa no `trilha check`. |
| VII — segurança por padrão | O passo 7 é o checklist de produção (segredo, `AllowedHosts`, `trilha check`) e o passo 6 é o filtro de tenant, que é a falha de segurança mais comum desta forma de app. |

## Tarefas

- [ ] T001 Teste que falha: `site/internal/docs/admin_app_test.go` — a página existe nas duas
      locales, está na tabela do índice do cookbook, está em `learn/examples.md` e nomeia as
      oito receitas do template e as quatro que ficam de fora.
- [ ] T002 Teste que falha: `cmd/trilha/i18n_test.go` — toda chave passada ao `t()` em
      `cmd/trilha/*.go` existe no `msgs`.
- [ ] T003 Correção: `crud guard helper` e `crud guard skip` no `msgs`, nas duas línguas.
- [ ] T004 `examples/cookbook/adminapp.go`: os blocos que a página cola.
- [ ] T005 A página em inglês e em português, com as saídas reais rodadas nesta versão;
      navegação em `docs.go`; linha no índice do cookbook e em `learn/examples.md`.
- [ ] T006 `CHANGELOG.md`, `version` em `cmd/trilha/main.go`, linha de versão do `ROADMAP.md`.
- [ ] T007 `make test` verde.

## Aceitação

- **SC-001**: `/cookbook/admin-app` e `/pt/receitas/app-administravel` respondem, aparecem na
  navegação das duas locales e no `llms.txt` de cada uma.
- **SC-002**: todo bloco `go` da página existe, caractere por caractere, em um `.go` do
  repositório (`TestCookbookSnippetsAreReal`).
- **SC-003**: as saídas de comando coladas são as que a CLI da versão 0.123.0 produz num
  projeto criado por `trilha new empresa --template app`.
- **SC-004**: `make test` verde (gofmt, vet, todos os pacotes, e2e da CLI).
