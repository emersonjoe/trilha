# Spec 095 — a receita `login`: sessão local, sem OIDC e sem senha de brinquedo

- **Issue**: [#116](https://github.com/emersonjoe/trilha/issues/116) — a issue é a fonte do escopo.
- **Branch**: `095-receita-login`
- **Versão**: 0.75.0

## Por quê

A issue lista `login` entre as receitas da primeira entrega e ele ficou de fora da 0.69.0. É o que
falta para as outras: a #117 espera uma tela de usuários, e uma tela de usuários sem tabela de
gente e sem sessão é uma tela sobre nada.

Hoje quem quer login local tem duas saídas, e as duas custam. O `--template app` traz um, mas
serve para quem está começando um projeto — quem já tem um não vai recomeçar por causa disso. O
`examples/local-login` é a outra: um exemplo para clonar, com os caminhos e o módulo para trocar
à mão.

## O que muda

```
$ trilha add login
  + internal/usuarios/usuarios.go       a tabela de gente, em memória
  + internal/usuarios/usuarios_test.go
  + internal/sessao/sessao.go           auth.Sessions, sem provedor
  + app/entrar/page.go                  a tela e o POST que confere a senha
  + app/sair/route.go
  ~ app/setup.go                        trilha.Provide(a, usuarios.New())
```

O que a receita entrega, e por que cada peça:

- **`internal/usuarios`** é a tabela do app: e-mail, nome, papel e `pbkdf2_sha256$…`. A resposta a
  um e-mail que não existe e a uma senha errada é a mesma, e leva o mesmo tempo — dizer qual das
  duas errou conta a quem está adivinhando que a conta existe.
- **`internal/sessao`** é o `auth.Sessions` sem provedor: ninguém de fora diz quem a pessoa é.
- **`app/entrar` e `app/sair`** são a tela e o fim dela.

**Ninguém nasce com senha.** `usuarios.New()` lê `ADMIN_EMAIL` e `ADMIN_PASSWORD` do ambiente: com
os dois, semeia o primeiro administrador; sem eles, a tabela sobe vazia, o log diz isso uma vez e
a tela de login mostra a linha que falta (só em dev). Uma receita que escrevesse
`admin@exemplo.com` / `admin` num projeto de verdade seria uma porta que alguém esquece aberta,
e o exemplo do repositório existe justamente para ser o lugar onde credencial de brinquedo mora.

Guardar rota é o passo seguinte, e a receita imprime qual: um `middleware.go` com
`sessao.Flow.Require()` na pasta que a pessoa quer fechar. A receita não adivinha qual é.

## Fora de escopo

- **A tela de usuários** — convidar, trocar papel, desativar, resetar senha. É a receita `users`,
  que depende desta e é a que a #117 espera. Ela chega com o `Requires`, que hoje não existe
  porque nenhuma receita precisava de outra.
- **Store em SQL e migrações.** O padrão do repositório: interface e memória aqui, SQL na receita
  do cookbook. O framework não é dono do esquema de ninguém.
- **OIDC.** Já existe, é `auth.Provider`, e não é uma receita: é uma configuração.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `auth` e `ui`, que já são do repositório |
| VI — teste primeiro | a receita escreve um teste, e o e2e da CI aplica ela num projeto novo |
| VII — segurança por padrão | sem credencial padrão; resposta e tempo iguais para e-mail e senha errados |

## Tarefas

- [x] T001 Teste que falha: `add login` escreve os arquivos e liga o `Provide`
- [x] T002 A receita: tabela, sessão, tela de entrar, sair, teste
- [x] T003 O e2e da CI aplicando `login` junto com as outras três
- [x] T004 Documentação (en + pt) e a lista de receitas
- [x] T005 `CHANGELOG.md`, `version`, `ROADMAP.md`, `make test`, `scripts/release.sh 0.75.0`

## Aceitação

- **SC-001** `trilha add login` num projeto novo, sem uma edição, passa `trilha check`.
- **SC-002** Sem `ADMIN_EMAIL`/`ADMIN_PASSWORD` a tabela sobe vazia e ninguém entra.
- **SC-003** Com os dois, o administrador entra e a sessão vale nas rotas guardadas.
- **SC-004** Senha errada e e-mail inexistente dão a mesma resposta.
- **SC-005** Rodar de novo não escreve nada e não duplica a linha do `setup.go`.
