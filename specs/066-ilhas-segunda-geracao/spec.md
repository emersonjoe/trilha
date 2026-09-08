# Spec 066 — Ilhas, segunda geração

Issue: [#70](https://github.com/emersonjoe/trilha/issues/70). A issue é a fonte do escopo;
aqui fica só a decisão.

## Por que as quatro partes juntas

Contrato de props, canal de volta, vendor e a receita de React são quatro pedaços do mesmo
buraco: a ilha existe desde a spec 022, mas quem tenta portar uma tela de verdade para dentro
dela para na primeira curva. Entregar só o `.d.ts` deixa a ilha sem como salvar; entregar só o
canal deixa o agente abrindo `.go` para descobrir o nome do campo. As quatro se provam na
mesma receita.

## Decisões

1. **O objeto `island` é o terceiro argumento do `export default`, e mora no loader.** Nada de
   `import` de um módulo do framework: a ilha já é um `import()` dinâmico, e um segundo
   arquivo para buscar seria uma volta de rede antes de montar. O `ui.js` não cresce — quem
   não usa ilha não baixa nada disso (princípio do orçamento do `ui.Head`).
2. **O token do CSRF chega pelo atributo, não pelo cookie.** O cookie do double-submit é
   `HttpOnly` de propósito. `c.Island` grava `data-trilha-csrf` no elemento, que é o mesmo
   token que `CSRFInput` já põe em todo formulário da página — nem mais nem menos exposto.
   É o que fecha a #44 para ilhas.
3. **`island.post` fala JSON e devolve o que o formulário receberia.** 2xx com JSON vira o
   objeto; 422 vira `IslandInvalid` com `.fields`; `Trilha-Location` vira navegação; qualquer
   outro erro vira `IslandError` com o `problem+json` já parseado. No servidor não muda nada:
   é um `route.go` com `BindJSON` e `FieldErrors`.
4. **`island.signal` aborta quando o elemento sai do DOM.** Uma ilha dentro de um fragmento
   que é trocado (spec 018) precisa parar de escrever no que não existe mais; um
   `MutationObserver` no loader é a única forma de saber, e ele é um só para a página.
5. **O `.d.ts` sai do `trilha gen`, não de um comando novo.** Já é o comando que lê `app/` e
   grava um arquivo determinístico e commitado; um segundo comando seria um segundo passo para
   esquecer. `gen --check` confere os dois arquivos.
6. **O tipo das props sai da declaração Go, por leitura de AST.** `c.Island("/x.js", Props{…})`
   com tipo nomeado vira uma interface; `map[string]any`, uma variável, uma chamada — qualquer
   coisa que não seja um literal de tipo nomeado — vira `unknown` e uma linha de aviso. O
   gerador não adivinha, e o aviso é o que faz alguém nomear o tipo.
7. **O `.d.ts` também descreve o objeto `island`.** É o contrato da parte que não está em Go
   nenhum; deixá-lo fora obrigaria a documentação a ser a fonte de verdade de uma API que o
   editor deveria completar sozinho.
8. **`trilha vendor` baixa, fixa e confere; não resolve dependências.** Sem árvore, sem
   `node_modules`, sem semver: um nome, uma versão exata, um arquivo, um sha256 no
   `vendor.lock`. Dependência transitiva é decisão de quem escreve o app, e um resolvedor
   seria um gerenciador de pacotes — que é exatamente o que a tese diz não precisar.
9. **O download é da CDN e o hash é a prova.** `--from` (e `TRILHA_VENDOR_BASE`) trocam a base,
   que é como o teste roda sem rede; `--check` refaz o sha256 do arquivo em disco contra o
   lock. Um arquivo em `public/vendor/` fora do lock é um achado do `trilha audit`, porque é
   código de terceiro que ninguém revisou entrando na página.
10. **Nada disso é dependência do framework.** O que a CLI baixa é do app, vive no `public/`
    dele e é servido pelo `c.Asset` como qualquer estático. `TestNoExternalDeps` continua
    valendo palavra por palavra.
11. **A receita de React monta o componente que já existe.** Preact + htm vendorizados, o
    componente inalterado, e `island.post` no lugar do `fetch`: o que muda no port é o
    transporte, não o estado. Reescrever contra o DOM é fase 2, e a receita diz isso.

## Critérios de aceitação

- SC-001 `c.Island` grava `data-trilha-csrf` e o loader monta com `(el, props, island)`.
- SC-002 `island.post` manda o token no header e devolve o JSON de um 2xx.
- SC-003 422 vira `IslandInvalid` com `.fields`; `Trilha-Location` navega; erro traz o `detail`.
- SC-004 `island.swap` troca o alvo e dispara `trilha:swap`, como o `ui.Swap`.
- SC-005 `island.signal` está abortado depois que o elemento sai do DOM.
- SC-006 `trilha gen` grava `public/islands.d.ts` com uma interface por tipo de props.
- SC-007 Props sem tipo nomeado viram `unknown` e uma linha de aviso na saída do `gen`.
- SC-008 O `.d.ts` declara `TrilhaIsland` e `IslandMount<S>` com o mapa por `src`.
- SC-009 `gen --check` falha quando o `.d.ts` está desatualizado.
- SC-010 `trilha vendor pacote@versao --from <base>` grava o arquivo e a linha do lock.
- SC-011 `trilha vendor --check` falha com hash trocado e passa com o arquivo intacto.
- SC-012 `trilha audit` avisa arquivo em `public/vendor/` que não está no lock.
- SC-013 `examples/blog` usa `island.post` e tem o `islands.d.ts` commitado.
- SC-014 Receita nas duas línguas e referência de ilhas atualizada.
- SC-015 `make test` verde, `api/current.txt` regravado.
