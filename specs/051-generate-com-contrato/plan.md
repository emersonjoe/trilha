# Plano: `generate` com contrato

Spec: [spec.md](./spec.md) · Issue [#49](https://github.com/emersonjoe/trilha/issues/49)

## Os fatos que decidem o desenho

1. **O `scaffold` não conhece o módulo.** `Generate(root, GenOptions)` recebe só a raiz, e
   importar um tipo de outro pacote exige o caminho de importação. `GenOptions` ganha
   `Module`, que o `cmd/trilha` já tem em `project.Module`.
2. **Procurar um tipo é parsing, não compilação.** O `internal/openapi` monta o índice de
   tipos do projeto lendo os fontes com `go/parser` desde a 031, e pula as mesmas pastas
   (`.`, `_`, `testdata`, `vendor`, `node_modules`, com a exceção `scan.WellKnown`). O que o
   `--bind` precisa é bem menos: nome → (caminho de importação, nome do pacote, campos). Vale
   uma função curta no `scaffold` em vez de exportar a máquina do `openapi`, que resolve
   expressão de tipo para virar JSON Schema e carrega escopo de arquivo junto.
3. **O teste mora no pacote da rota.** `page_test.go` ao lado de `page.go`, `package sobre` —
   assim ele chama `Page` e `GET` direto, sem importar nada do projeto, e não precisa saber
   layout nenhum: `TestPage` devolve o nó da página, e afirmar sobre o nó sobrevive a troca
   de layout (é o que a página de testes da documentação ensina).
4. **O alvo do teste sai do padrão.** `trilha.TestRoute` registra a rota com o `Pattern`, e é
   o roteador do `net/http` que resolve `{id}`. Com o parâmetro virando o próprio nome no
   alvo, o esqueleto — que só devolve `c.Param` — responde 200 para qualquer valor.
5. **`generate test` acha a rota pelo scanner.** `scan.Scan(root)` já devolve padrão, arquivo,
   métodos e se é página; usar isso em vez de reparsear a URL é o que faz o teste bater com o
   que existe no disco, inclusive quando a pasta foi escrita à mão.
6. **Idioma é dado, não template.** O `scaffold` já tem `texts[lang]` para os textos do
   projeto novo. Os comentários do esqueleto entram na mesma tabela, e o template os lê por
   chave — nenhum template duplicado por idioma.

## Corte

- **Um tipo, um pacote.** Nome repetido em dois pacotes é recusa com os dois caminhos na
  mensagem, e o conserto é passar `posts.Comment`. Escolher por proximidade seria adivinhar,
  e adivinhar errado só aparece na compilação.
- **Corpo do teste montado das tags.** `required` decide se o campo entra; `min`/`max` de
  string decidem o tamanho; `oneof` dá o primeiro valor. Regra que não cobrir o caso deixa o
  campo de fora, e a validação avisa — melhor do que um corpo que passa por acaso.
- **`--layout` só grava o que falta.** Um `layout.go` que já existe nunca é tocado, nem com
  `--force`: `--force` é sobre o arquivo que o comando veio escrever.

## Arquivos

| Arquivo | O que entra |
|---|---|
| `internal/scaffold/generate.go` | as opções novas, a recusa de flag em `kind` errado, o desvio para os templates |
| `internal/scaffold/contract.go` | os templates com contrato e o que eles precisam escrito (novo) |
| `internal/scaffold/types.go` | achar o tipo no projeto e ler os campos (novo) |
| `internal/scan/scan.go` | `Methods` exportado, para o gerador não repetir a lista |
| `internal/scaffold/gentest.go` | o esqueleto de teste a partir do `scan.Result` (novo) |
| `internal/scaffold/texts.go` | os comentários do esqueleto, en e pt |
| `cmd/trilha/generate.go` | as flags, o `kind` `test`, o `Module` |
| `cmd/trilha/i18n.go` | ajuda das flags e `usage` |
| `internal/scaffold/agents/AGENTS.*.md` | a linha do `generate` com as flags |
| `testdata/golden/generate/*.golden` | uma por combinação |
| `Makefile` | `internal/scaffold` no alvo `golden` |
