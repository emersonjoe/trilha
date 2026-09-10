# Spec 100 — rodar o `generate crud` de novo diz o que faltou

- **Issue**: [#115](https://github.com/emersonjoe/trilha/issues/115) — a issue é a fonte do escopo.
- **Branch**: `100-crud-rodar-de-novo`
- **Versão**: 0.80.0

## Por quê

A issue escreve a regra em uma linha: *"gerador que sobrescreve é gerador que ninguém roda duas
vezes"*. A 0.68.0 entregou a primeira metade — o `generate crud` recusa arquivo que já existe, e
não sobrescreve nada. Falta a segunda: **o que ele diz quando recusa.**

Hoje ele diz que o arquivo já existe, que é verdade e não ajuda. O struct mudou desde então — foi
por isso que a pessoa rodou o comando de novo — e o que ela quer saber é qual campo entrou e onde
ele falta.

## O que muda

```
$ trilha generate crud docs.Tipo
já gerado — nada foi escrito.

  Descricao não está no formulário
    acrescente ui.Field("descricao", …) em app/tipos/new/page.go:34
  Descricao não está no formulário de edição
    acrescente ui.Field("descricao", …) em app/tipos/id_/page.go:37
  Descricao não está na lista
    acrescente {Key: "descricao", …} em app/tipos/page.go:21
```

E, quando as telas já têm tudo o que o struct tem, uma linha só:

```
$ trilha generate crud docs.Tipo
já gerado — nada foi escrito. As telas têm todos os campos do struct.
```

Como ele sabe: o gerador refaz o plano — os mesmos campos, com o mesmo nome de formulário — e
procura, em cada tela que já está lá, o `ui.Field("<nome>"` e o `{Key: "<nome>"`. O que não
aparece é o que falta, e a linha impressa é a do último campo do arquivo, que é onde o próximo
entra.

**O comando passa a sair com 0 nesse caso.** Rodar de novo é uma coisa normal de se fazer, e não
um erro; o que ele imprime é informação. Uma recusa continua sendo recusa quando **parte** das
telas existe — aí o diretório está pela metade, e escrever o resto por cima seria misturar duas
gerações.

## Fora de escopo

- **Aplicar o campo que falta.** Escrever dentro de um arquivo que a pessoa já editou é a mesma
  coisa que sobrescrever, com mais passos. A linha impressa é o que ela cola.
- **`--store sqlite|postgres`, `--tenant`, `--policy`, `--schema`.** Continuam na issue.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | leitura de arquivo e busca de texto |
| VI — teste primeiro | um projeto gerado, um campo novo no struct, e o que o comando diz |
| Determinismo | a ordem dos campos é a do struct, e a das telas é fixa |

## Tarefas

- [x] T001 Teste que falha: campo novo no struct vira três linhas com arquivo e número
- [x] T002 `scaffold.CrudMissing`, com a linha de cada tela
- [x] T003 A saída do `trilha generate crud` quando tudo já existe, en + pt
- [x] T004 Documentação (en + pt)
- [x] T005 `CHANGELOG.md`, `version`, `ROADMAP.md`, `make test`, `scripts/release.sh 0.80.0`

## Aceitação

- **SC-001** Rodar de novo com o struct igual imprime a linha de "tem tudo" e sai com 0.
- **SC-002** Um campo novo no struct vira uma linha por tela, com arquivo e número.
- **SC-003** Nenhum arquivo é escrito nem alterado em nenhum dos dois casos.
- **SC-004** Com parte das telas no lugar, continua sendo recusa.
