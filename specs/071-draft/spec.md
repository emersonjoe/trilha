# Spec 071 — Onde mora o passo 1

- **Issue**: #103 — a issue é a fonte do escopo; aponte para ela, não a reescreva aqui.
- **Branch**: `071-draft`
- **Versão**: 0.53.0

## Por quê

Formulário de várias telas tem uma pergunta difícil, e não é o HTML: onde mora o passo 1
enquanto a pessoa está no passo 2. O que se escreve no lugar é uma página cheia de
`<input type="hidden">` — que o primeiro upload quebra —, uma linha meio preenchida no banco —
que todo relatório depois aprende a ignorar — ou uma tela enorme com tudo.

## O que muda

`c.Draft(nome)` com `Load`, `Save` e `Clear`, mais o `ui.Steps`. O contrato está na referência
do `Ctx` e na receita, nas duas línguas. O que vale registrar:

**Uma struct por passo.** É a parte da receita que vale mais que o código do framework: uma
struct só, com todos os campos e todas as tags, não dá para validar pela metade — o passo 1
falharia no endereço que ninguém digitou. A saída que as pessoas acham é largar as regras até a
última tela, onde uma mensagem sobre um campo de três telas atrás não serve para nada.

**Cookie assinado até 2 KB, store acima disso.** O limite não é os 3 KB que a issue propôs: o
que vai no cookie é o base64 do rascunho mais o prazo e a assinatura, e 3 KB de JSON passam dos
4 KB do navegador — que descarta sem avisar. Com 2 KB o cookie fica folgado. Acima, o
`Config.Drafts` (três métodos) recebe, e **sem store o `Save` devolve erro citando o campo** em
vez de mandar um cookie que ia sumir; um formulário que perde o passo 1 em silêncio é o pior
resultado possível aqui.

**`ErrNoDraft` é resposta, não falha.** É o que manda alguém ao passo 1: nunca salvo, terminado,
vencido, mexido, ou de outro navegador. Rascunho escrito por uma versão anterior da struct
responde igual — o campo mudou de nome entre dois deploys, e recomeçar é melhor que um 500 no
meio do formulário de alguém.

**Rascunho é assinado e não é secreto.** Está escrito no doc, na referência e na receita, porque
é o erro que alguém vai cometer: preço, desconto e nome de terceiro vão para o store, com só a
chave no cookie.

**Sem `TRILHA_SECRET` o `Save` diz o que falta.** Foi o navegador que mostrou: em prod sem
segredo o passo 1 dava 500 sem explicação. Agora o erro cita o rascunho e a variável.

## Fora de escopo, com o motivo

- **`Attach` e `Attachment`** — o rascunho com arquivo. Exigem o que o framework não tem: um
  diretório temporário, um token, um faxineiro e a limpeza do arquivo quando o rascunho vence. A
  issue os descreve como se essa vida útil já existisse ("junto com a dos uploads temporários");
  ela não existe. E é exatamente o que a [#114](https://github.com/emersonjoe/trilha/issues/114)
  (`trilha/blob`) vai construir — fazer uma segunda cópia privada dentro do `Draft` agora é a
  duplicação que aquela issue existe para evitar. O exemplo e a receita mostram o passo 1
  guardando o arquivo e deixando o caminho no rascunho.
- **Limpeza de rascunhos vencidos** — no cookie ela é o próprio prazo, e no store é de quem
  implementa a interface (um `EXPIRE` no Redis, uma coluna com índice). O framework não tem onde
  varrer.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `encoding/json` e o assinador que já existia. |
| III — rota no exemplo | `examples/cadastro/app/assistente`, três telas, com e2e do fluxo inteiro. |
| IV — superfície pequena | Três métodos, uma interface de três, um componente. |
| V — inglês no código, pt-BR junto | Receita nova e referência nas duas línguas no mesmo commit. |
| VI — teste primeiro | Rascunho que atravessa passos, adulterado, vencido, de outro formato, `Clear`, dois nomes, grande com e sem store, nome torto; e2e do assistente e do rascunho invisível entre navegadores. |
| VII — segurança por padrão | Assinado; nome saneado para não virar atributo de cookie; o "não é secreto" escrito nos três lugares onde alguém vai ler. |

## Tarefas

1. `draft.go`: `Draft`, `Load`/`Save`/`Clear`, `DraftStore`, `ErrNoDraft`, `Config.Drafts`. ✅
2. `ui/steps.go` + CSS: o indicador, com link só para trás. ✅
3. `examples/cadastro/app/assistente`: três telas, uma struct por passo. ✅
4. Receita "Formulário em passos" e referência de rascunhos, en e pt. ✅
5. `api/current.txt`, catálogo do `ui`, cópias do kit nos exemplos. ✅
