# Spec 091 — migrate next: o que é da tela e o que é do shell

- **Issue**: [#140](https://github.com/emersonjoe/trilha/issues/140) — a issue é a fonte do escopo.
- **Branch**: `091-migrate-dep-global`
- **Versão**: 0.72.0

## Por quê

Medido no Verba: `trilha migrate next` classificou **20 de 20 telas como C**, quase todas com a
razão `components/Chat.tsx`. A ordem recomendada A → B → C é o produto do relatório, e uma coluna
que responde "C" para tudo não ordena nada.

A spec 062 passou a seguir imports transitivos justamente para não subestimar o componente
extraído — um `page.tsx` de cinquenta linhas na frente de um componente de trezentas não é uma
porta de cinquenta linhas. O que faltou foi a outra metade: **nem todo arquivo alcançável é
trabalho daquela tela.**

## O que foi reproduzido

Duas causas, e só a segunda estava na issue. Uma fixture com `layout.tsx → Shell.tsx → Chat.tsx`
e duas listagens simples:

- **o layout sozinho não contamina** — as páginas continuaram A, porque `deps` parte da página e
  a página não importa o layout;
- **o barril contamina** — bastou as páginas fazerem `import { DataTable } from '@/components'`,
  com um `components/index.ts` que reexporta `Shell`, `Chat` e `DataTable`, para as duas virarem
  `C — drawing (components/Chat.tsx)`. É a reprodução exata do que o Verba mediu.

A regra que faltava é a mesma nos dois casos: **reexportar não é usar, e envolver não é compor.**

## O que muda

**1. O barril é atravessado pelo nome, não em bloco.** `import { DataTable } from '@/components'`
segue a linha do `index.ts` que reexporta `DataTable` e mais nenhuma. Um arquivo cujo conteúdo é
só `export … from` é um barril: ele não é lido como dependência, ele é um índice. Um
`import * as x` continua trazendo tudo, porque aí a página realmente pediu tudo.

**2. O que vem do layout é global.** Os arquivos alcançáveis a partir de qualquer `layout.tsx` ou
`template.tsx` do app formam o conjunto global. Uma tela que alcança um deles **não muda de
classe por causa dele**: a razão dela passa a ser "no island signal **of its own**", e o arquivo
aparece uma vez na seção nova do relatório — o que se porta uma vez, como layout ou ilha, e não
vinte.

**3. O relatório diz de onde veio cada sinal.** A coluna `Sugestão` já nomeia o arquivo quando o
sinal é de um componente da tela; a seção nova lista cada dependência global com os sinais e o
tamanho dela, que sai da coluna de tamanho das telas — a coluna passa a contar só o que é da tela.

O que **não** muda: um componente que só aquela página importa continua contando para ela, com
sinal e linhas. É o caso que a spec 062 existe para resolver, e ele não é global.

## Fora de escopo

- **Um parser TypeScript.** A issue diz que não precisa. A heurística continua sendo expressão
  regular sobre o texto, e continua impressa no relatório para ser contestada.
- **Distinguir provider de shell.** `layout.tsx` e `template.tsx` são o que o App Router define
  como moldura; um `providers.tsx` que só o layout importa já entra pelo alcance, sem precisar de
  uma regra que adivinha pelo nome.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | expressão regular e leitura de arquivo |
| VI — teste primeiro | a fixture do barril e a do shell falham antes da mudança |
| Determinismo | o conjunto global sai do alcance dos layouts, e a saída continua ordenada |

## Tarefas

- [x] T001 Fixture: duas listagens simples sob um shell com chat, e um barril de componentes
- [x] T002 O barril atravessado pelo nome, em `deps`
- [x] T003 O conjunto global a partir dos layouts, e a classe vinda só do que é local
- [x] T004 A seção do relatório com as dependências globais, en + pt
- [x] T005 Golden e documentação (en + pt)
- [x] T006 `CHANGELOG.md`, `version`, `ROADMAP.md`, `make test`, `scripts/release.sh 0.72.0`

## Aceitação

- **SC-001** As duas páginas da fixture continuam **A**: nenhuma tem sinal B/C próprio.
- **SC-002** O `Chat` aparece uma vez, na seção de dependências globais, com o sinal que tem.
- **SC-003** Um componente importado e renderizado só por uma página continua mudando a classe
  daquela página.
- **SC-004** `import { X } from '@/components'` não traz os irmãos de `X`.
- **SC-005** Saída determinística: o mesmo projeto dá o mesmo relatório, e o golden prova.
