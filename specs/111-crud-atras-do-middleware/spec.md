# Spec 111 — o CRUD gerado sob uma pasta fechada

- **Issue**: [#143](https://github.com/emersonjoe/trilha/issues/143) — a issue é a fonte do escopo.
- **Branch**: `111-crud-atras-do-middleware`
- **Versão**: 0.90.0

## Por quê

O `generate crud` promete telas que passam no `trilha check` sem uma edição. Isso é verdade num
projeto vazio e falso no lugar onde o iniciante mais vai usá-lo: dentro do `--template app`, que
tem um `middleware.go` na raiz exigindo sessão. O teste gerado bate em 401, e a mensagem que a
pessoa lê é "o CRUD que o framework gerou está quebrado".

O gerador escreve um teste que nunca vai passar ali porque não olha o que está acima do destino.

## O que muda

O gerador passa a olhar. Entre a raiz do `app/` e o `--at`, se existe um `middleware.go`, o teste
gerado precisa de uma sessão — e aí há dois casos:

- **O projeto tem um jeito conhecido de abrir uma sessão de teste** (a receita `login` passa a
  escrever `internal/sessao/sessaotest`, e o `--template app` ganha
  `internal/session/sessiontest`): o teste gerado abre a sessão na primeira linha e continua
  provando o que ele existe para provar.
- **Não tem** (middleware escrito à mão, autenticação de outro jeito): o teste vem com um `t.Skip`
  que diz qual arquivo fecha a pasta e o que fazer. Um `Skip` que explica é melhor que um 401 sem
  explicação — e o `check` fica verde, que é o que a promessa dizia.

E o comando imprime a linha, para a pessoa saber sem abrir o arquivo:

```
  aviso: app/middleware.go fecha esta pasta; o teste gerado abre uma sessão com internal/session/sessiontest
```

## Fora de escopo

- **O template `app` passar a usar a receita `login`.** É a #144, é maior, e esta correção não
  espera por ela.
- **O `Hint` do `trilha check` para um 401 de teste gerado.** Vale quando houver um caso que esta
  spec não cobre.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | leitura de arquivo, como o resto do gerador |
| VI — teste primeiro | um projeto com middleware acima do destino, e o que sai do gerador |
| Uma fonte | o helper de teste é escrito por quem escreve o login — a receita e o template |

## Tarefas

- [x] T001 Teste que falha: destino sob pasta com middleware gera teste que abre sessão
- [x] T002 O `sessaotest` da receita `login` e o `sessiontest` do template `app`
- [x] T003 A detecção no gerador, com o `Skip` explicativo quando não há helper
- [x] T004 e2e: `new --template app` + `generate crud --at app/admin/...` + `check` verde
- [x] T005 Documentação (en + pt)
- [x] T006 `CHANGELOG.md`, `version`, `ROADMAP.md`, `make test`, `scripts/release.sh 0.90.0`

## Aceitação

- **SC-001** No template `app`, gerar sob `app/admin/tipos` e rodar `check` passa sem edição.
- **SC-002** Num projeto sem middleware acima, o teste gerado é o de hoje.
- **SC-003** Com middleware e sem helper, o teste traz o `Skip` que nomeia o arquivo.
- **SC-004** O comando diz, no fim, o que encontrou.
