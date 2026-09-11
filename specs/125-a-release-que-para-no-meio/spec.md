# Spec 125 — A release que para no meio

- **Issue**: [#172](https://github.com/emersonjoe/trilha/issues/172) — a issue é a fonte do
  escopo, e ela nasceu de duas execuções reais.
- **Branch**: `claude/relaxed-keller-v1pwuf`
- **Versão**: 0.104.0

## Por quê

A 0.102.0 e a 0.103.0 pararam no mesmo lugar, com a mesma saída:

```
==> main pelo remoto
   ebe140b..2513d82  HEAD -> main
==> tag e push
error: RPC failed; HTTP 403 curl 22 The requested URL returned error: 403
```

A credencial daquelas execuções empurra branch — a `main` inclusive — e não cria
`refs/tags/`. O `set -e` mata o script ali, e o que sobra é a **`main` fundida, sem tag, sem
release e com as issues abertas**. Não é irreversível; é uma release pela metade sobre a qual
o script não diz nada. Nas duas vezes o conserto foi ler o script para descobrir onde ele
tinha parado e reconstruir a segunda metade à mão.

O problema não é a tag. É que tudo depois do `git push origin HEAD:main` acontece com um
passo sem volta já dado, e qualquer falha ali — rede no `gh release create`, `gh` sem sessão —
deixa o mesmo buraco.

## O que muda

**A ordem não muda.** O comentário do script já registra por que a tag vem depois:
*"para não sobrar tag local apontando para um commit que a fusão recusou"*. Inverter troca
este problema por aquele.

**E não dá para prever a falha.** `git push --dry-run` não serve: no mesmo ambiente em que o
push real devolve 403, o ensaio sai com `0` e anuncia `* [new tag]`. Ele negocia com o remoto
e não chega na autorização da escrita do ref. Está registrado na issue para ninguém tentar de
novo.

Sobra não deixar a falha sair cara, que são duas coisas:

**1. Dizer o que ficou para trás.** Um `trap` de saída que só fala depois do push da `main`:

```
release.sh: a main já foi fundida em 2513d82, e o ritual parou.

Falta, e é só rodar de novo — o que já foi feito é pulado:
  scripts/release.sh 0.103.0 --issues "164 165"

Ou à mão:
  git push origin v0.103.0
  gh release create v0.103.0 --title v0.103.0 --notes-file -   # a seção 0.103.0 do CHANGELOG
  gh issue close 164 --comment "Entregue na v0.103.0."
```

**2. Poder retomar.** Uma tag que já existe **neste commit** deixa de ser recusa e passa a ser
passo pulado; apontando para outro commit continua sendo recusa, porque é a versão sendo
remarcada em cima de outro código. Os outros passos já eram quase idempotentes — o push da
`main` em dia responde `Everything up-to-date` e `gh issue close` numa issue fechada não
reclama. Faltava o `gh release create`, que erra quando a release existe: agora é
`gh release view` decidindo entre `create` e `edit`.

O `edit` resolve de quebra um caso que também aconteceu de verdade: a release publicada com o
corpo vazio, porque `gh release create --notes-file -` com a entrada vazia publica assim
mesmo. Rodar o script de novo escreve as notas nela.

## Fora de escopo

- **Exigir `gh` só onde ele é usado.** Hoje a verificação está no topo e aborta antes de
  qualquer escrita, que é o comportamento certo — fazer metade do ritual e pedir o resto à mão
  é o que esta spec existe para evitar. (Nas duas releases quem passou por cima da verificação
  fui eu, com um `gh` de mentira no `PATH`; o script estava certo.)
- **Assinar a tag.** A tag da 0.102.0 saiu leve porque o `gh release create` a criou; as
  outras são anotadas. Uniformizar isso é outra conversa.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | Bash e Git, como o script já era. O teste é `os/exec` sobre os dois. |
| VI — teste primeiro | Os quatro testes falham contra o script de antes e passam contra o de agora — conferido rodando `git show HEAD:scripts/release.sh` por cima. O caso principal é o incidente reproduzido: um remoto que recusa `refs/tags/`. |

## Tarefas

- [x] T001 Testes que falham: ensaio, falha depois da fusão, retomada, tag em outro commit
- [x] T002 `scripts/release.sh`: o `trap`, a tag tolerante, `release view` → `create|edit`
- [x] T003 Provar que os quatro falham contra o script anterior
- [x] T004 `CHANGELOG.md`, `version` em `cmd/trilha/main.go`, `ROADMAP.md`
- [x] T005 `make test` verde e `scripts/release.sh 0.104.0 --issues "172"`

## Aceitação

- **SC-001** Com um remoto que recusa `refs/tags/`, o script sai mal **e** imprime que a `main`
  já foi fundida, em qual commit, e os comandos que faltam.
- **SC-002** Rodar de novo depois dessa falha termina o ritual — anuncia que está retomando,
  empurra a tag, cria a release e fecha as issues — em vez de recusar por causa da tag local.
- **SC-003** Uma tag da mesma versão apontando para outro commit continua sendo recusa, e a
  recusa acontece **antes** de qualquer escrita no remoto.
- **SC-004** O ensaio (`--dry-run`) percorre o ritual inteiro e não cria a tag local.
- **SC-005** Os quatro testes falham contra o `scripts/release.sh` anterior a esta spec.
