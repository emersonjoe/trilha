# Spec 068 — A planilha, nos dois sentidos

- **Issue**: #109 — a issue é a fonte do escopo; aponte para ela, não a reescreva aqui.
- **Branch**: `068-csv`
- **Versão**: 0.50.0

## Por quê

Toda aplicação interna faz isto duas vezes: um botão que baixa a lista e uma tela que a recebe
de volta. As duas falham nos mesmos poucos lugares, e nenhum deles é interessante — é por isso
que ninguém acerta. Na ida: sem BOM, e todo acento abre como caractere estranho no Excel;
vírgula onde o Excel daqui espera ponto e vírgula, e o arquivo abre como uma coluna só. Na
volta: "erro no arquivo", que deixa alguém procurando uma data errada entre quatro mil linhas
no olho.

O framework já tinha as duas pontas — `c.Attachment` manda, `c.File` recebe, `Bind` valida e o
`FieldErrors` já sabe falar de campo. Faltava o CSV em cima delas.

## O que muda

`c.CSV(nome, linhas)` e `trilha.BindCSV(r, &destino)`, mais o `ui.CSVErrors` para a tela do
erro. O contrato está na referência do `Ctx` e na receita, nas duas línguas. O que vale
registrar:

**O BOM não é configurável.** É a única coisa aqui sem opção: sem ele o Excel lê todo acento
errado, e não existe aplicação para a qual isso seja o comportamento desejado. O separador, a
data e a palavra do booleano saem do `Config.Locale`, que a #99 já tinha introduzido — esta
spec é a segunda consumidora dele, e a primeira a fazê-lo mudar bytes de um arquivo.

**A linha vem do leitor, não de um contador.** Uma célula com quebra de linha dentro desloca
todas as mensagens seguintes se a contagem for nossa; o `csv.Reader.FieldPos` sabe a linha de
verdade. É a diferença entre a mensagem apontar a linha certa e apontar quase.

**A validação é a mesma dos formulários.** Cada linha é montada e passada pelo `validated` do
`bind.go` — a regra escrita uma vez vale na tela e na importação, e a tradução do
`UseValidationPTBR` vale nas duas. Foi por isso que a `csvColumn` carrega dois nomes: o
cabeçalho do arquivo e a chave que o bind usa, para a mensagem voltar na coluna certa.

**Coluna obrigatória faltando é uma mensagem, não dez mil.** A conferência acontece uma vez, no
cabeçalho, e a leitura nem começa. O contrário — deixar a regra `required` disparar por linha —
produz um relatório correto e ilegível.

**Coluna desconhecida é aviso.** Planilha ganha coluna o tempo todo; recusar o arquivo por isso
só ensina a apagar colunas antes de subir, que é pior para todo mundo.

**Só as linhas boas entram na fatia**, e o `MaxRows` é erro em vez de corte: metade de uma
importação dizendo que deu certo é pior que uma que falha.

**`dd/mm/aaaa` é lido na importação e em lugar nenhum mais.** O `input type=date` sempre manda
ISO, então o `bind` nunca precisou saber desta forma; uma planilha exportada do Excel daqui
precisa, e mandar alguém corrigir dez mil datas num editor de texto não é uma importação.

**O `Upload` virou `io.Reader`.** Uma linha, e é o que deixa `BindCSV(up, &linhas)` ser a
chamada que a issue escreveu, sem o chamador entrar na struct atrás do `.File`.

## Desvios da issue

- **`iter.Seq` não entra.** O `go.mod` declara `go 1.22` e a matriz do CI inclui 1.22; o
  `iter.Seq` é 1.23. A exportação em stream é atendida pelo canal, que é o que aquela parte da
  issue queria de fato, e o motivo está escrito no doc comment para ninguém reabrir a dúvida.
- **`c.Draft("importacao").Save(...)`** aparece no exemplo da issue e vem da #103, que não
  existe. A receita mostra o caminho sem ele: valida, e salva ou devolve os erros.
- **Erro de arquivo ilegível volta como `error` comum**, não como `FieldErrors`. O `BindCSV`
  recebe um `io.Reader` e não tem como saber o nome do campo do formulário; quem sabe é o
  handler, e a receita mostra a linha (`trilha.FieldErrors{"arquivo": err.Error()}`).
- **Teto de 500 erros coletados.** A issue não pede; guardar uma mensagem por célula de um
  arquivo de cem mil linhas é como um upload de 5 MB vira um gigabyte de processo. Passando
  disso, a leitura para e um aviso diz por quê.

## Fora de escopo

- **`ui.AuditTable`** (da #104) continua fora: agora que o `c.CSV` existe, ela é uma tabela com
  um botão, e cabe na issue dela e não nesta.
- **`trilha ctx` listando as rotas que exportam CSV** — mesma dívida de scanner da
  [#124](https://github.com/emersonjoe/trilha/issues/124).

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `encoding/csv`, `bufio`, `reflect`. Nada entra no go.mod. |
| III — rota no exemplo | `examples/blog/app/documentos/planilha`, com teste de ponta a ponta: o arquivo que o GET escreve é o que o POST aceita. |
| IV — superfície pequena | Um método, uma função, um componente. O `CSVRules` tem dois campos e ambos têm padrão. |
| V — inglês no código, pt-BR junto | Receita e referência nas duas línguas no mesmo commit. |
| VI — teste primeiro | Arquivo exportado byte a byte em en e pt; importação com `,` e `;`, com e sem BOM, cabeçalho fora de ordem, célula inválida na linha certa depois de uma quebra de linha dentro de célula. |
| VII — segurança por padrão | `nosniff` no download, nome saneado pelo `safeName`, teto de linhas, teto de erros coletados. |

## Tarefas

1. `csv.go`: exportação (`c.CSV`, `csvWriter`, colunas, células por locale). ✅
2. `csv.go`: importação (`BindCSV`, detecção de separador e BOM, cabeçalho, erro por célula). ✅
3. `ui/csv.go`: `CSVErrors` com o corte em vinte e os avisos embaixo. ✅
4. Mensagens novas no `ValidationMessages`, nas duas línguas; `Upload.Read`. ✅
5. `examples/cookbook/csv.go` e a rota do blog; `cfg.Locale` do exemplo. ✅
6. Receita e referência em en e pt; `api/current.txt` e o catálogo do `ui`. ✅
