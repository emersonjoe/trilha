# Spec 064 — `trilha migrate next`: o esqueleto e o mapa

Issue: [#59](https://github.com/emersonjoe/trilha/issues/59). A issue é a fonte do escopo;
aqui fica só a decisão. A receita "Do Next.js para a Trilha" (spec 063) ensina a traduzir uma
tela; esta spec entrega a parte que não exige inteligência nenhuma — a árvore de pastas e o
mapa do que existe.

## Decisões

1. **Ler o texto, não o TypeScript.** Nada de parser de TS: o comando classifica por análise
   léxica sobre o `.tsx`, como o `audit` já faz. Um parser de TypeScript sem dependências
   externas seria um projeto próprio, e o que a issue pede — endpoints, `'use client'`,
   contagem de hooks, sinais de ilha — se responde com cinco expressões regulares. A regra
   fica impressa no relatório, para ser contestável.
2. **A tradução de caminho é o inverso do `scan`, e vive em `internal/migrate`.**
   `[id]`→`id_`, `[...path]`→`path__`, `[[...path]]`→`path__`, `(grupo)`→`grupo-`. O nome de
   pasta que não é identificador Go (`[user-id]`) vira `user_id_`, e o relatório diz que o
   `.tsx` lia `params["user-id"]`.
3. **O que o Next tem e o Trilha não tem vira linha no relatório, não arquivo.**
   `loading.tsx`, `template.tsx`, rota paralela (`@slot`) e interceptadora (`(.)x`) são
   listadas como "sem equivalente". Escrever um arquivo vazio para elas seria fingir uma
   tradução que não existe.
4. **O esqueleto compila.** Cada `page.go` gerado devolve um `ui.Container` com o título da
   rota; cada `route.go` tem um handler por método exportado no `route.ts`, respondendo 501
   com a mensagem de que a tela ainda não foi portada. `trilha gen --check` e `go vet` passam
   sobre a saída sem edição — é isso que separa um esqueleto de um rascunho.
5. **O contexto mínimo vai como comentário no arquivo gerado**, não como código morto: origem,
   endpoints, classificação. É o que o agente leria de qualquer jeito, dez linhas em vez de
   trezentas.
6. **A classificação é A/B/C com a regra escrita.** `C` (SPA) quando há sinal de ponteiro,
   SVG, canvas ou editor; `B` (ilha pequena) quando há polling, modal, abas ou upload; `A`
   (formulário + lista) quando não há nenhum dos dois. A ordem é essa: o sinal mais forte
   ganha, e o relatório imprime o motivo da linha.
7. **`--dry-run` imprime e não escreve**, e sem `--force` o comando nunca sobrescreve um
   arquivo que já existe: quem roda o `migrate` uma segunda vez, depois de portar três telas,
   não perde as três.
8. **O `next.config.*` só é lido para os `rewrites`.** Um `rewrite` de `/api/:path*` vira uma
   linha comentada de `Config.Upstreams` no relatório, porque a credencial e o destino são
   decisão de quem migra, não do comando.
9. **O `app/layout.go` da raiz nunca é escrito.** Quem migra já rodou o `trilha new`, e o
   layout raiz dele é um documento `<html>` inteiro, com tema, `ui.Head` e flashes. Escrever
   um por cima seria trocar algo que funciona por um esqueleto; o relatório diz para portar o
   `<html>` do `layout.tsx` para o que já existe. Layout aninhado é escrito normalmente.
10. **O cenário novo da régua (#45) fica para a medição.** Rodar o `make bench-agent` custa
   uma hora e chamadas reais de modelo; o cenário entra na spec da régua, não nesta.

## Critérios de aceitação

- SC-001 Árvore Next sintética em `testdata/next/` com dinâmico, catch-all, grupo, `route.ts`
  com três métodos, `layout.tsx` aninhado, `middleware.ts` e `next.config.ts` com `rewrites`.
- SC-002 Golden do `app/` gerado e do `MIGRATION.md`, regravado por `make golden`.
- SC-003 Mesma árvore, mesmos bytes: a saída é determinística, o relatório também.
- SC-004 `[id]`, `[...path]`, `[[...path]]`, `(grupo)` e `[user-id]` caem nas pastas certas.
- SC-005 `route.ts` com `GET`, `POST` e `DELETE` vira um `route.go` com os três handlers.
- SC-006 `loading.tsx`, `template.tsx`, `@slot` e `(.)x` aparecem no relatório como sem
  equivalente, e nenhum arquivo é escrito para elas.
- SC-007 O relatório traz, por página: origem, URL, `'use client'`, hooks, endpoints,
  classificação e linhas.
- SC-008 A regra da classificação está impressa no relatório.
- SC-009 `--dry-run` não escreve nada e imprime a tabela.
- SC-010 Sem `--force`, um arquivo existente é mantido e contado como tal.
- SC-011 O `app/` gerado passa em `trilha gen --check` e `go vet` sem edição (e2e).
- SC-012 Mensagens novas em `msgs` (en + pt); docs de CLI nas duas línguas; CHANGELOG.
