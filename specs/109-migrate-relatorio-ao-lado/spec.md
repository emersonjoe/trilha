# Spec 109 — o `MIGRATION.md` fica ao lado da árvore, como o texto dele promete

- **Issue**: [#146](https://github.com/emersonjoe/trilha/issues/146) — a issue é a fonte do escopo.
- **Branch**: `109-migrate-relatorio-ao-lado`
- **Versão**: 0.88.1

## Por quê

O relatório começa dizendo *"a árvore ao lado deste arquivo é o esqueleto"*. Com `--out
/tmp/x/mig/app`, a árvore vai para lá e o arquivo fica no diretório de onde o comando foi
rodado — porque o padrão de `--report` é `MIGRATION.md`, relativo ao cwd, independente do `--out`.

Quem roda de dentro deste repositório deixa um `MIGRATION.md` solto nele; quem roda de outro
lugar procura o relatório onde o texto diz que ele está e não acha.

## O que muda

O padrão de `--report` passa a ser `<pasta do --out>/MIGRATION.md`. Com o `--out app` de sempre o
resultado é o de hoje — `./MIGRATION.md` ao lado de `./app` —, e com `--out /tmp/x/mig/app` o
relatório vai para `/tmp/x/mig/MIGRATION.md`, que é o que a primeira linha dele diz.

`--report` dado explicitamente continua mandando, e o comando passa a imprimir os dois caminhos no
fim, absolutos:

```
esqueleto: /tmp/x/mig/app (47 telas)
relatório: /tmp/x/mig/MIGRATION.md
```

O `--dry-run` diz onde o relatório **seria** escrito, com o mesmo cálculo.

## Fora de escopo

- **O conteúdo do relatório.** Não muda uma linha.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `path/filepath` |
| VI — teste primeiro | os três casos do aceite, num teste da CLI |

## Tarefas

- [x] T001 Teste que falha: `--out` longe leva o relatório junto; sem `--out` fica como hoje
- [x] T002 O padrão calculado do `--report`, e as duas linhas no fim
- [x] T003 Documentação (en + pt)
- [x] T004 `CHANGELOG.md`, `version`, `make test`, `scripts/release.sh 0.88.1`

## Aceitação

- **SC-001** `--out /tmp/x/app` escreve `/tmp/x/MIGRATION.md`.
- **SC-002** Sem `--out`, escreve `./MIGRATION.md` como hoje.
- **SC-003** `--report outro.md` continua mandando.
- **SC-004** O fim do comando diz os dois caminhos.
