# Plano — spec 131

## Onde cada coisa entra

| Arquivo | O que muda |
|---|---|
| `internal/migrate/decls.go` (novo) | `declsOf` (as declarações de topo de um módulo), `usedSource` (o mascaramento por nome), `identsIn` |
| `internal/migrate/deps.go` | `deps` devolve `map[string]dep` (fonte mascarada + `whole`); `bindings` substitui `wanted` e passa a dizer também o nome local; um import cujos nomes não são usados no que ficou não é seguido |
| `internal/migrate/analyze.go` | `Analysis.Actions`; `actionsOf`; sinais `media capture` (SPA) e `media playback` (ilha); `hit.whole` e o `, whole module` no motivo; `analyze` recebe o caminho do próprio arquivo |
| `internal/migrate/migrate.go` | `Project.Actions` (união, com as telas de cada ação); `Action`; a linha `// Server actions:` do comentário gerado |
| `internal/migrate/report.go` | `ActionsTable`; a seção *Server Actions*; `· N server actions` na coluna Sugestão; o parágrafo da regra com os sinais novos, nas duas línguas |
| `internal/migrate/testdata/{unused-import,server-actions,media}/` (novo) | uma árvore sintética por issue |
| `internal/migrate/migrate_test.go` | um teste por issue sobre as árvores acima |
| `testdata/next/` | `lib/actions.ts`, `app/estudo/page.tsx` (ação + módulo com sinal não importado) e `app/gravar/page.tsx` (captura), para o relatório inteiro ter as três regras |
| `testdata/next.golden/` | regravado por `make golden` |
| `cmd/trilha/migrate.go`, `cmd/trilha/i18n.go` | `--actions` |
| `cmd/trilha/migrate_test.go` | `--actions` imprime a tabela e não grava nada |
| docs | `en/reference/cli.md` e `pt/referencia/cli.md`: os sinais e a seção nova |
| `CHANGELOG.md`, `cmd/trilha/main.go`, `ROADMAP.md` | a 0.110.0 |

## Ordem

O mascaramento primeiro, porque as outras duas regras andam sobre ele: a detecção de ação usa
a mesma lista de declarações de topo, e o sinal de mídia só é confiável depois que o sinal do
módulo para de vazar. Depois as ações (modelo, relatório, flag) e por fim os dois sinais, que
são duas linhas de expressão regular e um parágrafo de documentação em cada língua.

## Decisões

1. **Mascarar em vez de recortar.** A região não usada vira espaços com os `\n` preservados, e
   não uma cópia concatenada: assim o número de linha do motivo continua sendo o do arquivo, que
   é o que a #156 pagou para ter.
2. **Fecho transitivo dentro do arquivo.** `entrarNoPortal` pode chamar um `post()` local; parar
   no primeiro nível perderia o sinal de verdade. O fecho é por menção de identificador — sem
   parser, e do lado seguro.
3. **Corpo de função é de quem chama; valor de topo é de todo mundo.** Uma função só roda
   quando alguém a chama, então ela pertence a quem importou o nome; um `const canal = new
   EventSource(...)` é avaliado no `import` e pertence a toda tela que toca o arquivo. O
   `funcInitRe` separa os dois, e o que ele não tem certeza de ser função é lido como valor —
   o lado que guarda um sinal em vez de perder um.
4. **Nome não declarado → módulo inteiro.** Quando nenhuma das declarações casa com o que foi
   pedido (reexport, tipo, sintaxe que o léxico não viu), o arquivo é lido inteiro e o motivo
   diz `, whole module`. Falso negativo é o erro caro (#182).
5. **`DepLines` continua contando o arquivo inteiro.** O número é o tamanho do arquivo que
   alguém vai abrir; o mascaramento é sobre atribuição de comportamento.
6. **A seção lista o que as telas importam.** Um grep do repositório inteiro acharia mais ações
   (e `node_modules` junto); a travessia por nome é a que a #181 pede e é a que diz *de qual
   tela* a ação é.
7. **`"use server"` não é `islandSignal`.** Empurraria para B páginas que são A, que é
   exatamente o que a issue recusa.

## O que não entra

Ver *Fora de escopo* na spec: a categoria "sinal que a migração elimina", o `route.go` por ação,
parser de TypeScript, tradução de JSX.
