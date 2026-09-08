# Spec 063 — A matriz de permissões

- **Issue**: #100 — a issue é a fonte do escopo; aponte para ela, não a reescreva aqui.
- **Branch**: `063-auth-policy`
- **Versão**: 0.45.0

## Por quê

O `RequireFunc` da #62 é o gancho certo e a folha em branco errada: quem recebe
`func(*User, *Ctx) bool` escreve `u.HasRole("admin") || (u.HasRole("analista") && módulo ==
"docs")` na terceira tela e erra na quarta. Uma lista de papéis responde "esta pessoa é
admin"; o que a aplicação pergunta é "esta pessoa pode editar documentos".

A medição da issue é o argumento: a permissão do app real vive em quatro lugares — o serviço
em Python, 26 dos 28 routers, o hook do front e a tela que edita a matriz — e os quatro
repetem a mesma decisão.

## O que muda

A matriz vira dado, declarado uma vez, e responde nos quatro lugares: o middleware que guarda,
o botão que se esconde, o 403 que explica e a tela que edita. O contrato completo está na
referência de `auth`, nas duas línguas.

O que vale registrar aqui é o que **não** é óbvio:

**A ordem de `Levels` é o significado.** `administrar ⊇ editar ⊇ ver` é o que deixa a célula
guardar um valor em vez de três booleanos. Por isso é um slice, não um conjunto.

**Ausência é negação.** Módulo que o papel não nomeia é módulo que ele não alcança, e `Default`
nasce vazio. Papel apagado tem que perder acesso, não herdar o de outro.

**O 403 diz o que faltou** — `needs editar on docs`. É a frase que a pessoa repete para quem
administra; "forbidden" transforma dois minutos em uma thread de suporte.

**A grade não importa `auth`.** `ui.PolicyGrid` recebe uma interface de quatro métodos que a
`auth.Policy` satisfaz. O kit não pode arrastar autenticação para dentro de todo app que
desenha um botão.

**O retrato é de propósito.** `PolicyFrom` devolve um valor, não uma referência viva: uma
política que mudasse debaixo de uma requisição deixaria a mesma requisição responder duas
vezes — liberada no middleware, negada no botão.

**Célula forjada é descartada em silêncio.** Responder 400 para um módulo que não existe só
diria a quem forjou qual nome tentar em seguida.

## Fora de escopo

- **ABAC por registro** (dono do documento, linha do inquilino) continua `RequireFunc` à mão. A
  política é por módulo; uma que alcançasse o dado precisaria do dado, e aí seria consulta.
- **`trilha ctx` mostrando módulo e nível por rota.** A issue pede, e é trabalho de scanner:
  ler `var x = RequirePolicy(...)` no `middleware.go` e ligar à rota. Fica para issue própria,
  registrada ao fechar esta.
- **Quórum** (o CPAD do app medido) — é regra de processo, não de acesso.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | Nada novo no `go.mod`. |
| IV — superfície pequena e estável | Só adiciona; a grade entra por interface para não acoplar `ui` a `auth`. |
| VI — teste primeiro | Tabela de cobertura de níveis, as negações que têm que ser não, o guarda por um app de verdade, o ida-e-volta da grade e o exemplo ponta a ponta. |
| VII — segurança por padrão | Anônimo nunca chega ao predicado; ausência é negação; a tela que edita a matriz é guardada no próprio exemplo. |

## Tarefas

- [x] T001 `auth/policy.go`: `Policy`, `Grants`, `Levels`, `All`, `Can`, `Level`, `RequirePolicy`.
- [x] T002 `auth/policystore.go`: `PolicyStore`, `PolicyFrom`, `BindPolicy`.
- [x] T003 `ui/policygrid.go`: a grade, por interface.
- [x] T004 Testes: níveis, negações, guarda, ida-e-volta.
- [x] T005 `trilha audit`: módulo declarado que nenhuma rota exige.
- [x] T006 `examples/local-login` com a política, a tela e o teste.
- [x] T007 Referência de `auth` nas duas línguas.
- [ ] T008 `CHANGELOG.md`, `version`, `make test` verde e release.

## Aceitação

- **SC-001** Nível maior cobre menor; papel desconhecido nega; papel apagado perde acesso.
- **SC-002** Anônimo vai ao login; logado sem permissão recebe 403 dizendo o que faltou.
- **SC-003** A grade renderiza a matriz e o que ela posta volta a ser a matriz, descartando
  módulo e nível que não existem.
- **SC-004** `trilha audit` avisa sobre módulo que nenhuma rota exige.
