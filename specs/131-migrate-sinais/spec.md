# Spec 131 — Os sinais do `migrate next`: o import sem uso, a Server Action invisível e o microfone

- **Issues**: [#167](https://github.com/emersonjoe/trilha/issues/167),
  [#181](https://github.com/emersonjoe/trilha/issues/181),
  [#182](https://github.com/emersonjoe/trilha/issues/182) — as issues são a fonte do escopo;
  cada uma traz o trecho que reproduz.
- **Branch**: `feat/mig-s04-migrate-sinais`
- **Versão**: 0.110.0

## Por quê

As três issues vêm de duas migrações reais — o Verba (#167) e o Prosa (#181, #182) — e todas
as três dizem a mesma coisa de ângulos diferentes: **a classe que o relatório imprime é uma
ordem de trabalho**, e quem lê começa pela tela que ela indica. Quando a classe está errada, o
erro não custa uma linha de relatório: custa a primeira meia hora de quem porta.

**#167 — o import puxa o sinal, o uso não.** Hoje a página é classificada pelos sinais de todo
módulo que ela importa. No Verba, `portal/convite/[token]/page.tsx` importa dois nomes de um
`lib/portalApi.ts` grande e herda `upload` (linha 71, `portalUpload`, que ela nunca chama) e
`storage`. A tela é um formulário que lê um convite e define uma senha — **A** — e o relatório
manda escrever ilha. O `import { a, b } from "x"` já diz quais nomes entraram: seguir só esses
resolve o caso sem análise de fluxo.

**#181 — a metade de escrita é invisível.** O analisador conhece sinais de cliente e o
`'use client'`; `"use server"` não aparece em lugar nenhum do pacote. Numa aplicação de App
Router com Server Actions — o formato padrão desde o Next 14 — o relatório descreve
corretamente o que a página **desenha** e não diz uma palavra sobre o que ela **escreve**. No
Prosa são 57 ações exportadas em 15 arquivos e zero `fetch(` nas páginas: um inventário feito
só com o relatório mecânico começaria a migração sem saber que metade do produto não tem
contrato nenhum. O `A` impresso está certo quanto ao formato e é enganoso quanto ao tamanho do
trabalho.

**#182 — falso negativo é mais caro que falso positivo.** Não há sinal de captura de mídia.
Uma tela com `getUserMedia`/`MediaRecorder`/`SpeechRecognition` e nenhum `<canvas>` sai
**A — no island signal**: um formulário. Gravar voz é o caso mais puro de "o navegador está
fazendo o trabalho" que existe — permissão de dispositivo, stream, objeto com ciclo de vida,
`Blob` no fim — e não há PRG que substitua. A #156 corrigiu um falso positivo (um `<svg>` de
logo lido como desenho), que custa uma conferência; este é um falso negativo, que faz alguém
portar meia tela como formulário e descobrir o microfone no meio.

## O que muda

### 1. A classe conta o que a página importa **pelo nome** (#167)

Um módulo alcançado por `import { a, b } from "x"` é lido **mascarado**: as declarações de topo
que não são `a`, `b` — nem alcançadas por elas dentro do arquivo — saem da análise. O
mascaramento preserva as linhas (cada caractere fora da região vira espaço, os `\n` ficam), de
modo que o motivo impresso continua carregando a linha de verdade do arquivo.

| Forma do import | O que conta |
|---|---|
| `import { a, b } from "x"` | as declarações `a` e `b`, mais as declarações do arquivo que elas mencionam (fecho transitivo), mais tudo o que roda ao importar: o topo do módulo e as declarações de topo que são **valor** e não função — `const canal = new EventSource("/api/eventos")` é avaliado no instante em que alguém importa o arquivo, e é trabalho de toda tela que importa qualquer coisa dele |
| `import * as x` / `import X from "x"` | o módulo inteiro, e o motivo diz `, whole module` |
| nome que o arquivo não declara (reexport, tipo) | o módulo inteiro, e o motivo diz `, whole module` — sinal perdido é tela portada como formulário, e a issue #182 é sobre o preço disso |

A regra vale em cada salto: um `import` cujos nomes não aparecem no que ficou do arquivo não é
seguido. Um `export … from` é sempre seguido, porque reexportar é para fora.

O relatório do Verba passa a sair assim (a linha 71 continua sendo `portalUpload`, e ela não é
mais desta tela):

```
| `portal/convite/token_/page.go` | `/portal/convite/{token}` | yes (5 useState, …) | A — no island signal |
```

O tamanho do trabalho (`Origem (110 + 75)`) continua contando o arquivo importado inteiro: é o
arquivo que alguém vai abrir. O mascaramento é sobre **a quem o comportamento pertence**, não
sobre quantas linhas o módulo tem.

### 2. Seção *Server Actions* no relatório (#181)

`"use server"` **não é sinal de ilha** e não muda a classe: uma Server Action é o oposto de
JavaScript no cliente. Ela entra em duas coisas novas.

- **A linha da tela** ganha a contagem, para o `A` não ser lido como "nada a fazer":
  `A — no island signal · 2 server actions`.
- **Uma seção nova**, com a mesma disciplina das outras — o que foi achado, onde, e o que isso
  implica:

```markdown
## Server Actions

O que as telas escrevem: 2 ações em 1 arquivo. Uma Server Action não tem URL — é uma função
que o formulário chama, e o `"use server"` é o contrato inteiro. Aqui cada uma vira um handler
de escrita (um `route.go`, ou o `POST` da própria rota da página) e um formulário que posta
nele: o handler grava e redireciona, a página desenha — o PRG que o kit já faz. A classe ao
lado de uma tela é sobre o que ela desenha; estas são o que ela muda.

| Ação | Origem | Telas |
|---|---|---|
| `registrarResposta` | `lib/actions.ts:4` | `/estudo` |
| `criarBaralho` | `lib/actions.ts:9` | `/estudo` |
```

Regra de detecção, a da issue: `"use server"` no topo do arquivo → **toda função exportada é
uma ação**; `"use server"` no corpo de uma função → **só ela**. A travessia por nome da #167
serve igual aqui: a seção lista as ações que as telas **importam**, e uma ação que ninguém
importa não aparece (é código morto ou é chamada de um componente que a análise não alcançou).

O comentário do arquivo Go gerado ganha a lista, no lugar onde ela vai ser lida:

```go
// Page renders GET /estudo, ported from app/estudo/page.tsx.
//
// Source: 14 lines, server component.
// Server actions: registrarResposta (lib/actions.ts:4), criarBaralho (lib/actions.ts:9).
// Suggested: A — no island signal · 2 server actions.
```

E um `--actions` imprime **só** essa tabela e não grava nada — é a lista que quem escreve o
contrato usa como ponto de partida:

```bash
trilha migrate next ../web --actions
```

### 3. Captura de mídia é **C**, reprodução é **B** (#182)

Dois sinais novos, e a distinção é o ponto: **capturar** depende de permissão e de um objeto
vivo, então é sempre ilha de cliente; **reproduzir** é um elemento que o servidor desenha.

| Sinal | Classe | O que casa |
|---|---|---|
| `media capture` | **C** (SPA) | `getUserMedia(`, `new MediaRecorder(`, `SpeechRecognition`, `webkitSpeechRecognition`, `new AudioContext(` |
| `media playback` | **B** (ilha) | `new Audio(`, `<audio`, `<video`, `HTMLMediaElement` |

`<video` entra junto de `<audio` pelo mesmo motivo (é o mesmo elemento com imagem), e nenhum
dos dois puxa para C: o kit cobre reprodução sem bundle (#180). Ficar em `editor` não serve —
`editor` é sobre entrada de texto rica, e juntar as duas coisas faria o motivo impresso mentir
sobre o que a tela é, que é justamente o que torna a conferência barata.

## Fora de escopo

- **"Sinal que a migração elimina"** (nota da #167: `sessionStorage` guardando sessão vira
  cookie HttpOnly do servidor, então não deveria puxar a tela para B). É uma categoria nova no
  relatório e mexe no sinal mais comum que existe em app React; vai como issue à parte, com a
  medição de quantas telas mudam de classe.
- **Escrever o `route.go` de cada ação** na árvore gerada: não há URL para inventar, e o
  relatório sugerir a linha é o que a issue pede.
- **Parser de TypeScript.** O mascaramento é léxico e anda por declaração de topo, como o resto
  do pacote anda por expressão regular.
- **Traduzir JSX**: continua sendo o trabalho de quem porta.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| I — convenção sobre configuração | Nada muda em `app/`: o comando lê Next e escreve a árvore que o `internal/scan` já aceita. |
| II — só biblioteca padrão | `regexp`, `strings`, `sort`, `os`, `path/filepath`. `TestNoExternalDeps` continua verde. |
| III — geração explícita | A saída continua determinística: `TestScanIsDeterministic` e os golden de `testdata/next.golden` regravados por `make golden`. |
| IV — contrato pequeno e estável | `internal/`: fora da superfície vigiada por `api/current.txt`. A CLI ganha uma flag e não perde nenhuma. |
| V — recarga e binário | Não toca em `dev`/`build`. |
| VI — teste primeiro | Três árvores sintéticas novas em `internal/migrate/testdata/` (uma por issue) com os testes que falham antes da implementação, mais o golden do relatório completo em `testdata/next`. |
| VII — segurança por padrão | Só leitura de arquivos abaixo da raiz informada; nada é executado, nada é escrito fora de `--out`/`--report`. |

## Aceitação

- **SC-001** (#167) Uma página que importa dois nomes de um módulo que também tem
  `new FormData(`/`localStorage` em outra função sai **A**, e a página que importa o módulo com
  `import * as` continua saindo **B** com o motivo dizendo `, whole module`.
- **SC-002** (#167) Um `import` cujos nomes não aparecem no que ficou do módulo não é seguido:
  o arquivo não entra em `Deps`. E um valor de topo do módulo — `const canal = new
  EventSource(...)` — continua contando para quem importa só uma função de lá, porque ele roda
  ao importar.
- **SC-003** (#181) Um arquivo com `"use server"` no topo dá uma ação por função exportada e
  importada; um `"use server"` no corpo de uma função dá só ela. Cada ação aparece no relatório
  com arquivo, linha e as telas que a importam.
- **SC-004** (#181) A linha da tela traz `· N server actions` e a classe **não** muda por causa
  delas.
- **SC-005** (#181) `trilha migrate next <dir> --actions` imprime só a tabela e não grava
  arquivo nenhum.
- **SC-006** (#182) Uma tela com `getUserMedia`/`MediaRecorder`/`SpeechRecognition` sai **C**
  com o motivo `media capture (…:N)`; uma tela que só toca áudio sai **B** com
  `media playback`.
- **SC-007** O golden de `testdata/next.golden` mostra a seção nova e as três regras num
  relatório inteiro; `make test` verde.
