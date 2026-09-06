# Spec 045 — Release sem trocar de branch

> Forma curta: um script, sem convenção nova, sem mudança na API pública.

- **Issue**: nenhuma (mudança de processo, saída do encontro das duas sessões em 06/09/2026)
- **Branch**: `045-release-sem-checkout`
- **Versão**: nenhuma — não muda o framework, não gera release

## Por quê

O `scripts/release.sh` funde a spec na `main` trocando o branch do worktree de onde ele roda:
`git checkout main`, `git pull --ff-only`, `git merge --ff-only <branch>`. Isso pressupõe que
a `main` não está checada em lugar nenhum, e neste repositório ela está: são dois worktrees
(`trilha` e `trilha-042`), e o Git recusa checar a mesma branch em dois. Quem acabou de soltar
uma release fica com o worktree parado na `main`, e a release seguinte — da outra sessão —
falha no primeiro passo que mexe em algo.

Aconteceu duas vezes no mesmo dia: a 0.32.0 deixou o worktree na `main`, e a 0.33.0 teve que
ser feita à mão, passo a passo. Um script de release que falha na metade é pior do que não
ter script: o que sobra é uma tag local sem push, ou uma `main` empurrada sem release, e a
pessoa tem que descobrir em que ponto parou.

O segundo problema é do mesmo tamanho e aparece no fim: o ruleset do GitHub recusa merge
commit na `main`, e o script só descobre isso quando o push já está sendo dado — com o
"bypassed rule violations" no log e a `main` já mexida.

## O que muda

**O script não troca mais de branch.** A fusão passa a ser feita pelo remoto, que é a fonte
de verdade: `git push origin HEAD:main` é o próprio fast-forward, e o servidor recusa se não
for um. O worktree fica onde está, em qualquer branch, e a `main` pode estar checada em outro
worktree sem que isso importe.

A ordem passa a ser: `make test` → conferências → `git push origin HEAD:main` → tag anotada e
`git push origin <tag>` → release no GitHub → issues. A tag nasce depois do push da `main`,
não antes: se a fusão for recusada, não sobra tag local apontando para um commit que não está
publicado.

Duas conferências novas, antes de qualquer escrita:

- **`git log --merges origin/main..HEAD` tem que estar vazio.** A mensagem diz o conserto
  (`git rebase origin/main`), porque o ruleset da `main` recusa merge commit e ninguém
  descobre isso lendo o script.
- **`git merge-base --is-ancestor origin/main HEAD`**, isto é, o branch está em cima da `main`
  de agora. Sem isso o push seria recusado pelo remoto no meio do ritual, e não no começo.

Ao final o script tenta atualizar a `main` local com `git fetch origin main:main`. Se ela
estiver checada em outro worktree o Git recusa, e aí o script diz — sem falhar, porque a
release já saiu — que a outra sessão dê `git pull` no worktree dela.

## Fora de escopo

- **Automatizar o `ROADMAP.md`** e o aviso à outra sessão: continuam no rodapé, pelo motivo
  da spec 019 (é prosa com decisão dentro).
- **Rodar `gh` para checar o ruleset** antes do push: a conferência de merge commit já cobre a
  regra que a gente esbarra; o resto é o remoto que diz.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | o script é `bash` com `git` e `gh`, como já era |
| VI — teste primeiro | não há teste automatizado de release; o `--dry-run` é a verificação, e a spec exige que ele imprima o ritual inteiro sem tocar em nada |

## Tarefas

- [x] T001 `scripts/release.sh`: conferências de merge commit e de ancestralidade, com
      mensagem que diz o conserto.
- [x] T002 `scripts/release.sh`: push para `origin HEAD:main`, tag depois do push, `fetch
      origin main:main` com aviso em vez de falha.
- [x] T003 `--dry-run` do começo ao fim num branch de verdade, com a `main` checada no outro
      worktree; `make test` verde.
- [x] T004 Constituição (1.4.1), seção *Fluxo de trabalho*: a release não troca de branch, e o
      branch entra rebasado. O `CLAUDE.md` não descreve o ritual, então não muda.

## Aceitação

- **SC-001** `scripts/release.sh X.Y.Z --dry-run` roda até o fim com a `main` checada em outro
  worktree, sem nenhum `git checkout`.
- **SC-002** Um branch com merge commit é recusado antes de qualquer escrita, com a mensagem
  mandando rebasar.
- **SC-003** Um branch atrás da `main` é recusado antes de qualquer escrita.
- **SC-004** Depois de uma release de verdade, o worktree continua no branch da spec.
