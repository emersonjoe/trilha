# Spec 019 — Fluxo enxuto: spec curta e release por script

> Escrita na própria forma curta que ela introduz. Se a forma não servisse para uma mudança
> deste tamanho, não serviria para nenhuma.

- **Issue**: nenhuma (mudança de processo, pedida na sessão de 05/09/2026)
- **Branch**: `019-fluxo-enxuto`
- **Versão**: nenhuma — não muda o framework, não gera release

## Por quê

O trabalho neste repositório é assistido por um agente, e o custo do agente não está no
código que ele escreve: cada chamada de ferramenta reenvia a conversa inteira, então o gasto
é aproximadamente *tamanho do contexto × número de turnos*. Duas consequências práticas:

1. **O ritual de release é o passo mais caro do projeto.** São ~10 chamadas mecânicas
   (testar, commitar, fundir, marcar, empurrar duas vezes, criar a release, fechar issue por
   issue) para produzir umas 200 palavras novas. Com o contexto na casa dos 100k, é da ordem
   de um milhão de tokens de entrada em um passo que não tem nenhuma decisão dentro.
2. **A cerimônia da spec-kit é desproporcional para mudança pequena.** Na 017 (cache busting,
   ~150 linhas de mudança), `plan.md` e `tasks.md` somaram 80 linhas que repetem a `spec.md`
   com outras palavras — três arquivos para uma decisão que cabe em um.

Some a isso o custo de duas sessões trabalhando no mesmo repositório ao mesmo tempo: o
encontro entre a 015 e a 018 custou um rebase, a renumeração da versão de 0.9.0 para 0.10.0,
quatro conflitos e uma passada de tradução — trabalho que não existiria em série.

O que **não** entra nesta spec: cortar teste antes do código ou a documentação nas duas
locales. É o que torna o framework verificável; o desperdício está no transporte, não no
conteúdo.

## O que muda

**`scripts/release.sh X.Y.Z [--issues "20 21"] [--dry-run]`** e o atalho
`make release VERSION=X.Y.Z ISSUES="20 21" [DRY_RUN=1]`. Rodado a partir do branch da spec,
com tudo commitado: verifica antes de mexer em qualquer coisa (branch não é `main`, árvore
limpa, `version` em `cmd/trilha/main.go` bate com o argumento, existe seção `## X.Y.Z — …`
no `CHANGELOG.md`, a tag ainda não existe, `gh` no `PATH`); depois roda `make test`, funde na
`main` por fast-forward, cria a tag anotada, empurra `main` e tag, publica a release **com as
notas extraídas do `CHANGELOG.md`** — não se escreve a mesma nota duas vezes — e fecha as
issues com "Entregue na vX.Y.Z". Ao final imprime o que nenhum script escreve: riscar o item
no `ROADMAP.md` e avisar a outra sessão, se houver.

**`.specify/templates/spec-curta-template.md`**: arquivo único com issue/branch/versão, *Por
quê*, *O que muda* (o contrato), *Fora de escopo*, *Constitution Check* só dos princípios
tocados, *Tarefas* e *Aceitação*. Vale para mudança de um pacote, sem convenção nova em
`app/`, sem quebra de API pública. Mesma numeração, mesmo branch, mesma release.

**Constituição 1.3.0**, seção *Fluxo de trabalho*: registra a spec curta e seu critério, a
regra de que **a issue é a fonte do escopo** (a spec aponta, não recopia; fato verificado
fora do repositório vai para a issue na primeira vez), **um dono da `main` por vez** e o
fechamento de spec por script.

**`CLAUDE.md`**, seção *Fluxo de trabalho*: as regras acima mais as duas de leitura — ler
estreito (`grep -n -A5`, `sed -n 'X,Yp'`) em vez de ler arquivo grande inteiro, e `make test`
por bloco de tarefas em vez de por arquivo.

## Fora de escopo

- **Automatizar o `ROADMAP.md`.** O item é prosa com o motivo da decisão; um `sed` produziria
  texto pior que o silêncio. O script lembra, a pessoa escreve.
- **Reservar número de spec por arquivo de trava.** Combinar antes resolve; um `.lock` no git
  seria mais cerimônia para o mesmo problema.
- **Gerar `CHANGELOG` a partir dos commits.** As notas atuais explicam o porquê, coisa que
  mensagem de commit não carrega.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `scripts/release.sh` é `bash` + `git` + `gh`, fora do módulo; não toca `go.mod` |
| VI — teste primeiro | o script **roda** `make test` antes de fundir, e falha cedo em cada verificação; a forma curta mantém "teste que falha" como primeira tarefa |
| Governança | emenda registrada com nova versão (1.3.0) e template correspondente criado, como a própria constituição exige |

## Tarefas

- [x] T001 `scripts/release.sh` com as verificações, o ritual e o `--dry-run`
- [x] T002 Alvo `release` no `Makefile`
- [x] T003 `.specify/templates/spec-curta-template.md`
- [x] T004 Constituição 1.3.0 (spec curta, issue como fonte, dono da `main`, release por script)
- [x] T005 `CLAUDE.md` e `CONTRIBUTING.md` + `docs/pt-BR/CONTRIBUTING.md`
- [x] T006 Esta spec, escrita na forma curta

## Aceitação

- **SC-001** `make release VERSION=X.Y.Z DRY_RUN=1` imprime o ritual inteiro sem executar
  nada, e recusa versão malformada, árvore suja, branch `main`, versão fora do
  `cmd/trilha/main.go`, seção ausente no `CHANGELOG.md` e tag repetida.
- **SC-002** As notas da release saem do `CHANGELOG.md` sem edição manual.
- **SC-003** Fechar uma spec passa de ~10 chamadas de ferramenta para 1.
- **SC-004** Uma mudança pequena tem um arquivo em `specs/NNN-nome/`, não três, sem perder
  issue, contrato, `Constitution Check`, tarefas e aceitação.
