# Spec 087 — trilha generate crud: do struct à tela

- **Issue**: [#115](https://github.com/emersonjoe/trilha/issues/115) — a issue é a fonte do escopo.
- **Branch**: `087-generate-crud`
- **Versão**: 0.68.0

## Por quê

Entre o `trilha new --template app`, que traz **um** CRUD pronto de exemplo, e o
`trilha generate page`, que escreve uma página vazia, mora a tarefa que mais se repete: *tenho um
struct, quero a tela*. Dez vezes no mesmo projeto, cada uma com o mesmo esqueleto e um errinho
diferente — uma não confirma o excluir, outra não mostra o erro no campo, outra perde a página ao
voltar.

O que este gerador escreve não é mágica de runtime: é o código que a pessoa vai ler e editar, na
forma que o `templates/app` já provou ser idiomática. Ele existe para que a décima tela saia como
a primeira.

## O que muda

```
$ trilha generate crud docs.Tipo --at app/admin/tipos
  + internal/docs/tipo_store.go       TipoStore: List/Get/Create/Update/Delete + memória
  + app/admin/tipos/page.go           lista: ui.DataTable, busca, ordenação, paginação, ui.Empty, excluir
  + app/admin/tipos/new/page.go       formulário: ui.Field por campo, 422 volta com os erros no campo
  + app/admin/tipos/id_/page.go       editar: o mesmo formulário, preenchido
  + tipo_crud_test.go                 lista vazia, cria, edita, 422, exclui
  + app/setup.go                      o store, onde as páginas acham
  ✓ trilha_gen.go (3 rotas novas)
```

Como os campos viram tela:

- o struct é lido **por análise estática**, como o `--form` já faz; nada de compilador nem de
  download de módulo;
- `ID` é a chave, e um struct sem ela é uma recusa com essa frase — o gerador não inventa uma
  chave, porque a chave errada só aparece no primeiro `Update`;
- `CriadoEm`/`CreatedAt`/`AtualizadoEm`/`UpdatedAt` e `json:"-"` ficam fora do formulário: são
  do sistema, e um campo de data preenchido à mão é um bug esperando;
- `validate:"required,max=80"` vira `required` e `maxlength` no controle, e a mensagem no campo;
- o rótulo vem do nome do campo em título — `RetencaoAnos` vira `Retencao Anos` — e trocar isso
  é editar uma string no arquivo gerado, que é onde ela deve estar.

**Nada é sobrescrito.** Um arquivo que já existe é uma recusa nomeando-o, e o `--force` não
existe: gerador que sobrescreve é gerador que ninguém roda duas vezes, e o CRUD é justamente o
que se gera depois de já ter mexido.

A exceção é o `app/setup.go`, que o gerador **edita** em vez de recusar — uma linha, dentro de
uma função cuja forma o framework define. Sem ela o CRUD compila e responde 500 na primeira
requisição, que é o pior resultado possível para um gerador, porque parece que funcionou. O
comando nomeia esse arquivo na saída: quem mexe no que não foi pedido deve essa frase.

O teste gerado fica na raiz do projeto, e não na pasta da tela, porque é lá que o `newApp()`
mora: as telas são três pacotes, e o que vale testar é que os três concordam entre si.

O store gerado é **interface mais memória**, no pacote do próprio tipo. É a mesma escolha de todo
store deste repositório, e aqui ela vale duas vezes: o que sai do gerador **roda** — a tela abre,
o formulário grava, o teste passa — e trocar a memória por um banco é implementar cinco métodos
que já estão escritos como assinatura.

## Fora de escopo

A issue pede mais, e o resto fica para specs próprias:

- **`--store sqlite|postgres` e o `migrations/NNN_*.sql`.** Um store SQL gerado tem de escolher
  dialeto de placeholder e ser dono de um DDL — a coisa que este framework não faz em nenhum
  outro lugar. A receita de banco de dados mostra a implementação; gerar uma é uma decisão
  separada e maior que esta.
- **`--tenant` e `--policy`.** Os dois são uma linha a mais em cada arquivo gerado, e as duas
  linhas dependem de o projeto ter `auth` configurado de um jeito que o gerador teria de
  adivinhar. Depois de o gerador existir, elas são pequenas; antes, são chute.
- **`--schema`**, a versão compacta com `ui.SchemaForm`. O expandido é o que se pede primeiro:
  quem gera um CRUD quer ver o formulário, não uma chamada que o esconde.
- **Rodar de novo e imprimir o diff dos campos novos.** É a metade interessante da issue e pede
  ler o formulário que a pessoa editou para dizer o que falta — análise do código gerado depois
  de editado. Merece spec própria; até lá, a recusa nomeando o arquivo é honesta.
- **`c.Audit` no Create/Update/Delete.** O gerador não sabe se o projeto tem trilha de auditoria
  configurada, e escrever a linha em um projeto que não tem é escrever código que não compila.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `go/ast`, `text/template` — o mesmo caminho do `--form` |
| VI — teste primeiro | e2e que gera num projeto novo e roda `trilha check` sem uma edição |
| Convenção nova | as rotas geradas são as convenções que já existem; o e2e prova que o scanner as vê |

## Tarefas

- [x] T001 Teste que falha: `generate crud` sobre um struct escreve os arquivos e o scanner vê as rotas
- [x] T002 Leitura do struct: chave, campos do sistema, campos do formulário
- [x] T003 O store: interface e memória
- [x] T004 As três telas e o teste gerado
- [x] T005 Recusas: sem `ID`, arquivo existente, tipo inexistente
- [x] T006 e2e: gerar num projeto novo e `trilha check` verde sem edição
- [x] T007 Documentação: referência do `cli` en + pt
- [x] T008 `CHANGELOG.md`, `version`, `ROADMAP.md`, `make test`, `scripts/release.sh 0.68.0 --issues "115"`

## Aceitação

- **SC-001** Sobre um struct com `ID` e três campos, o comando escreve cinco arquivos, liga o
  store no `app/setup.go` e o `trilha_gen.go` passa a ter as três rotas.
- **SC-002** O que saiu compila e passa no `trilha check` sem nenhuma edição.
- **SC-003** O teste gerado passa: lista vazia, cria, edita, 422 com erro no campo, exclui.
- **SC-004** Struct sem `ID` é recusa com a frase, e não escreve arquivo nenhum.
- **SC-005** Arquivo que já existe é recusa nomeando-o, e o que já estava lá continua igual.
