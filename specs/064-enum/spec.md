# Spec 064 — O enum declarado uma vez

- **Issue**: #96 — a issue é a fonte do escopo; aponte para ela, não a reescreva aqui.
- **Branch**: `064-enum`
- **Versão**: 0.46.0

## Por quê

Um valor de domínio — status, tipo, estágio — existe em quatro lugares e é escrito quatro
vezes: a badge da tabela, as opções do select, a validação do formulário e um comentário na
tag. Escritos separados, eles derivam, e o quarto é onde o rótulo fica errado.

O `examples/cadastro` tinha exatamente isso antes desta spec: dois rádios com os valores à
mão, uma badge que imprimia o valor cru — quem escolhia "Mensal" via `mensal` na lista —, um
comentário `// semanal | mensal` na tag, e uma checagem à mão comparando as duas strings.

## O que muda

`trilha.Enum` é a lista declarada uma vez, e o contrato completo está na referência de
validação, nas duas línguas. O que vale registrar aqui é o que não é óbvio:

**`Tone` é nome, não cor.** Um entre seis do tema. Quem declara um status escolhe um
significado; escolher uma cor é como duas telas acabam com dois verdes diferentes.

**Valor desconhecido renderiza cru.** Linha gravada antes de alguém aposentar aquele valor não
pode derrubar a tela: mostrar `legado` em cinza é uma tela em que dá para agir.

**A mensagem cita rótulos, não valores.** Quem preencheu o formulário leu rótulos. Isso é
feito compondo a mensagem a partir do `ValidationMessages`, e devolvendo-a como chave — o
`message()` repassa chave desconhecida sem tocar, e é essa saída que permite dizer algo que a
tabela estática não sabe: a lista depende de qual enum a tag citou.

**`RegisterEnum` é idempotente para a mesma lista.** O `Setup` é onde ele mora, e uma suíte
sobe o app uma vez por teste — o pânico em duplicata que a issue herdou do `AddRule` deixaria
o símbolo inutilizável no lugar que a própria documentação indica. Duas listas **diferentes**
sob um nome continuam entrando em pânico.

**O `Options` devolve `h.Node`,** não uma fatia de algum tipo `Option`, para compor com o
`ui.Select` sem que nenhum dos dois pacotes precise conhecer o tipo do outro.

## Fora de escopo

- **`trilha ctx` listando os enums registrados.** A issue pede, e é trabalho de scanner: o
  registro é de tempo de execução e o `ctx` lê código. Vira issue própria, junto com a mesma
  pendência que a spec 063 deixou.
- **i18n dos rótulos** — a própria issue põe fora, e pelo motivo certo.
- **`ui.SchemaForm` aceitando o nome do enum como string** — depende do #69 e é outra spec.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | Nada novo no `go.mod`. |
| IV — superfície pequena | `LookupEnum` e `RegisteredEnums` saíram do plano: existiriam só para o `trilha ctx`, que não foi feito, e símbolo exportado sem consumidor é promessa que ninguém pediu. |
| VI — teste primeiro | Valor desconhecido, marcação do `selected`, a mensagem citando rótulos, o registro duplicado, e o exemplo ponta a ponta. |

## Tarefas

- [x] T001 `enum.go`: `Enum`, `EnumValue`, `Options`, `Label`, `Tone`, `Has`, `RegisterEnum`.
- [x] T002 A regra `enum=` e as mensagens nas duas línguas.
- [x] T003 `ui.Status` e os seis tons no `ui.css`.
- [x] T004 `examples/cadastro`: a frequência sai de uma declaração só.
- [x] T005 Referência de validação nas duas línguas; catálogo do `ui describe`.
- [ ] T006 `CHANGELOG.md`, `version`, `make test` verde e release.

## Aceitação

- **SC-001** Valor fora da lista renderiza cru, em `muted`, sem pânico.
- **SC-002** `Options` marca o atual, e o placeholder só quando nada foi escolhido.
- **SC-003** A tag recusa valor fora da lista com mensagem que cita os rótulos.
- **SC-004** No exemplo, a badge mostra "Mensal" e não "mensal".
