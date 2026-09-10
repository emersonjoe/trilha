# Spec 112 — o template `app` passa a usar a receita `login`

- **Issues**: [#144](https://github.com/emersonjoe/trilha/issues/144) e
  [#117](https://github.com/emersonjoe/trilha/issues/117) — as issues são a fonte do escopo.
- **Branch**: `112-template-usa-a-receita-login`
- **Versão**: 0.91.0

## Por quê

O template `app` traz um login próprio (`internal/session` + `app/login`) e a receita `login`
traz outro (`internal/sessao` + `entrar`/`sair`). Pedir os dois — `new --template app --with
login,users` — dá um projeto com duas portas e teste vermelho.

E há o problema maior, que a #117 já registrava: o login é a única tela do template que **não** é
receita, e por isso o `add users` (que depende de `login`) não reconhece o login que está lá.
Quem cria um projeto pelo template não consegue acrescentar as telas que dependem de sessão.

## O que muda

O template deixa de ter um login e passa a pedir a receita, como já faz com auditoria, chaves e
configurações:

```
trilha new minha-app --template app
  = esqueleto + --with login (em app/) + audit, api-keys, settings (em app/admin/)
```

- **`internal/session`, `app/login` e `app/logout` somem** do template. Entrar é `/entrar`, sair é
  `/sair`, e os dois vêm da receita.
- **A tabela de gente sai do `internal/store`.** O store do template fica com os itens, que é o
  que ele existe para demonstrar; usuários são da receita.
- **O `middleware.go` da raiz** deixa passar `/entrar` e `/convite` — o segundo porque quem abre um
  convite ainda não tem sessão, e é a receita `users` que o escreve.
- **A primeira senha vem do ambiente**, como a receita manda: `ADMIN_EMAIL` e `ADMIN_PASSWORD`. O
  template deixa de semear `admin@example.com / trilha-demo`, e a tela de entrar diz, em dev, quais
  duas variáveis faltam. Uma conta de demonstração num projeto que alguém vai levar para produção
  é uma porta que se esquece aberta — e era a última que o repositório ainda deixava.

Com isso, `trilha add users` (e `permissions`, `profile`, `tenant`) funciona num projeto criado
pelo template, que é o que a #117 pedia.

## Fora de escopo

- **Projetos gerados pelas versões 0.70–0.90.** Ficam como estão; a nota de release diz que o
  login virou receita.
- **Uma receita que detecta o login antigo e migra.** Migração de projeto alheio é outro assunto.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| Uma fonte | o login do template **é** a receita; não há segunda cópia para envelhecer |
| VI — teste primeiro | o e2e do template roda `check` e agora também `add users` em cima |
| VII — segurança por padrão | acabou a conta de demonstração com senha escrita |

## Tarefas

- [x] T001 Teste que falha: `new --template app` + `add users` + `check`
- [x] T002 O template sem login próprio, e o `new` aplicando a receita em `app/`
- [x] T003 O store sem a metade de usuários, e as telas usando `sessao`
- [x] T004 O `app_test.go` do template entrando pela receita
- [x] T005 Documentação (en + pt)
- [x] T006 `CHANGELOG.md`, `version`, `ROADMAP.md`, `make test`, `scripts/release.sh 0.91.0`

## Aceitação

- **SC-001** `trilha new x --template app && trilha check` passa, com um único `internal/sess*`.
- **SC-002** `trilha new x --template app && trilha add users && trilha check` passa.
- **SC-003** `--with ""` continua devolvendo o esqueleto, agora sem login nenhum.
- **SC-004** Nenhuma senha de demonstração no que o template escreve.
