# Spec 110 — o CRUD gerado mostra o que a pessoa lê

- **Issue**: [#145](https://github.com/emersonjoe/trilha/issues/145) — a issue é a fonte do escopo.
- **Branch**: `110-crud-rotulos`
- **Versão**: 0.89.0

## Por quê

Três coisas que o `generate crud` produz e que ninguém mostraria a um usuário: um rótulo sem
acento e sem unidade ("Retencao" no lugar de "Retenção (anos)"), um booleano escrito `true`, e uma
data que some da tela sem aviso.

As três têm o mesmo formato de erro: o gerador sabe menos do que o struct diz. A tag `label:` está
lá — a receita `settings` escreve uma, e o `ui.Field` a lê pelo `SchemaOf` —, o kit tem `ui.Status`
e `ui.Date`, e o campo de data existe no struct.

## O que muda

- **A tag `label:` é lida**, e vale para a coluna da lista e para o campo do formulário. Sem tag,
  continua o nome do campo separado em palavras.
- **Booleano vira Sim/Não** no idioma do `--lang`, com o tom do `ui.Status` — verde para ligado,
  cinza para desligado. `true` na tela é o valor de uma variável, não uma informação.
- **`time.Time` volta para a lista**, formatada pelo `ui.Date` (que respeita o fuso e o idioma do
  app), e continua fora do formulário: data que alguém digita é bug esperando. O store carimba.
- **O que ficou de fora é dito.** Um campo de tipo que o CRUD ainda não sabe desenhar vira uma
  linha no terminal e um comentário no arquivo, em vez de sumir.

## Fora de escopo

- **`--schema`, `--store` e o diff** — o diff saiu na 0.80.0, os outros continuam na #115.
- **Enum como campo do formulário** (select vindo do `RegisteredEnums`). É outra issue, e ela
  espera o `--schema`.
- **O cenário do `bench/agent`** com rótulos acentuados: vale quando os cenários voltarem a ser
  mexidos, e está registrado na #94.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | leitura de tag, como o resto do gerador |
| VI — teste primeiro | um struct com `label:`, `bool` e `time.Time`, e o que sai deles |
| Determinismo | golden do gerador continua valendo |

## Tarefas

- [x] T001 Teste que falha: rótulo com acento, bool como Sim/Não, data na lista
- [x] T002 A tag `label:` no `typeField` e nos dois lugares que a usam
- [x] T003 `ui.Status` para bool, `ui.Date` para data, e o aviso do que ficou fora
- [x] T004 Documentação (en + pt)
- [x] T005 `CHANGELOG.md`, `version`, `ROADMAP.md`, `make test`, `scripts/release.sh 0.89.0`

## Aceitação

- **SC-001** `label:"Retenção (anos)"` aparece na coluna e no campo.
- **SC-002** Um `bool` aparece como Sim/Não (pt) ou Yes/No (en).
- **SC-003** Um `time.Time` de sistema aparece na lista e não no formulário.
- **SC-004** Um campo de tipo não suportado vira aviso e comentário, e não silêncio.
- **SC-005** O projeto gerado continua passando `trilha check` sem edição.
