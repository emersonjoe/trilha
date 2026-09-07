# Spec 063 — O catálogo do `ui` e o guia de quem vem do Next

Issue: [#73](https://github.com/emersonjoe/trilha/issues/73). A issue é a fonte do escopo;
aqui fica só a decisão.

## Por que as duas metades juntas

As duas são a mesma leitura, feita antes da primeira linha de uma migração: *que componente eu
uso* e *em que isto que eu tinha se transforma*. O catálogo sem a tabela deixa o agente
adivinhando o idioma; a tabela sem o catálogo manda ele abrir `ui/ui.go` para achar a
assinatura. Separadas, as duas custam o mesmo token duas vezes.

## Decisões

1. **A fonte é o doc comment, e só ele.** O `describe` não tem texto próprio: lê o pacote `ui`
   com `go/doc`, tira o resumo da primeira frase e o exemplo do bloco indentado que o `go doc`
   já mostra. Um catálogo escrito à mão fica velho no primeiro componente novo.
2. **O catálogo é gerado e commitado**, como o `api/current.txt`: `internal/uidoc/catalog.json`,
   regravado por `go test ./internal/uidoc -update` e embutido na CLI. O `go:embed` não
   alcança `../../ui`, e a CLI não pode depender do código-fonte estar por perto.
3. **JSON é o formato guardado; o texto é derivado.** O `--json` sai do arquivo sem
   reformatar, e a saída de texto é montada na hora. Guardar o texto obrigaria a um segundo
   golden para o `--json`.
4. **Exemplo é obrigatório onde a assinatura não basta.** A régua é mecânica: função com mais
   de dois parâmetros ou com um `…Opts` na assinatura precisa de um bloco de exemplo no doc
   comment. As de um argumento (`ui.Muted`, `ui.Outline`) precisam só da frase. Exigir exemplo
   das 112 seria escrever 112 exemplos que ninguém lê.
5. **Todo símbolo exportado do `ui` tem doc.** Quando um grupo compartilha o comentário
   (`Secondary`, `Outline`, `Ghost`…), o resumo do grupo serve os membros — o teste aceita o
   comentário do grupo, e não aceita silêncio.
6. **`describe` sem argumento lista; com argumento descreve.** O nome é aceito com ou sem o
   prefixo `ui.`, e um nome desconhecido responde com os parecidos, não com um erro seco.
7. **O guia do Next é uma página nova do cookbook**, não um capítulo do `migration.md`: quem
   vem do `net/http` e quem vem do Next leem coisas diferentes, e a página existente já é
   longa. A tabela "padrão React → idioma Trilha" é o coração da página.
8. **O guia não inventa API.** Cada linha da tabela aponta para o que já existe (`ui.Poll`,
   `c.Flash`, `ui.Dialog`, `c.Inline`, `Auth.Require`, `ListParams`, `ui.Shell`,
   `ui.Markdown`, `c.Island`); o que ainda não existe entra como aviso, não como promessa.
9. **O `AGENTS.md` aponta para o comando.** Uma linha: antes de escrever `ui.X`, rode
   `trilha ui describe X`. É o mesmo lugar onde o agente já lê as outras regras.

## Critérios de aceitação

- SC-001 `trilha ui describe` sem argumento lista todos os componentes, um por linha, agrupados.
- SC-002 `trilha ui describe Field` mostra assinatura, resumo, exemplo e os símbolos citados.
- SC-003 `trilha ui describe field` e `trilha ui describe ui.Field` acham o mesmo componente.
- SC-004 Nome desconhecido sai com sugestão de nomes parecidos e status ≠ 0.
- SC-005 `--json` devolve o catálogo (ou o componente) em JSON estável.
- SC-006 `internal/uidoc/catalog.json` é determinístico e o teste falha quando está velho.
- SC-007 Um símbolo exportado do `ui` sem doc reprova o teste.
- SC-008 Uma função com mais de dois parâmetros ou com `…Opts` sem exemplo reprova o teste.
- SC-009 A página do cookbook existe nas duas línguas com a tabela React → Trilha.
- SC-010 Os blocos `go` da página nova são trechos reais de `examples/cookbook`.
- SC-011 `AGENTS.md` (as duas línguas) cita o `trilha ui describe`.
- SC-012 `make test` verde, `api/current.txt` regravado, docs de CLI nas duas línguas.
