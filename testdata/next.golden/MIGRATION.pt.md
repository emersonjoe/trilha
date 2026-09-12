# Relatório de migração

Lido de `app`: 15 arquivos com para onde ir. A árvore ao lado deste arquivo é o esqueleto — pastas, pacotes e assinaturas. O que cada tela faz continua sendo trabalho de quem porta; esta tabela existe para você não precisar abrir uma por uma para descobrir.

## Telas

| Origem | Aqui | URL | Cliente | Chama | Sugestão |
|---|---|---|---|---|---|
| `app/(marketing)/about/page.tsx` (11) | `marketing-/about/page.go` | `/about` | não | — | A — no island signal of its own |
| `app/api/documents/route.ts` (15) | `api/documents/route.go` | `/api/documents` | não | — | — |
| `app/api/health/route.ts` (2) | `api/health/route.go` | `/api/health` | não | — | — |
| `app/dashboard/layout.tsx` (4) | `dashboard/layout.go` | `/dashboard` | não | — | — |
| `app/dashboard/page.tsx` (15) | `dashboard/page.go` | `/dashboard` | sim (1 useState, 1 useMemo) | GET /api/metrics?range=:range | C — pointer (line 10) and live svg (line 10) |
| `app/docs/[[...slug]]/page.tsx` (4) | `docs/slug__/page.go` | `/docs/{slug...}` | não | — | A — no island signal |
| `app/documents/[id]/page.tsx` (24) | `documents/id_/page.go` | `/documents/{id}` | sim (2 useState, 1 useEffect, 1 useRef) | GET /api/documents/:id<br>POST /api/documents/:id/reprocess<br>GET /api/documents/:id/status | B — polling (line 12) |
| `app/documents/page.tsx` (15 + 12) | `documents/page.go` | `/documents` | sim (2 useState, 1 useEffect) | GET /api/documents?q=:query | A — no island signal |
| `app/error.tsx` (6) | `error.go` | `/` | sim | — | — |
| `app/estudo/page.tsx` (15 + 18) | `estudo/page.go` | `/estudo` | não | — | A — no island signal · 2 ações de servidor |
| `app/files/[...path]/page.tsx` (4) | `files/path__/page.go` | `/files/{path...}` | não | — | A — no island signal |
| `app/gravar/page.tsx` (16) | `gravar/page.go` | `/gravar` | sim (1 useState, 1 useRef) | — | C — media capture (line 9) |
| `app/not-found.tsx` (4) | `not_found.go` | `/` | não | — | — |
| `app/page.tsx` (11) | `page.go` | `/` | não | — | A — no island signal |
| `app/users/[user-id]/page.tsx` (6) | `users/user_id_/page.go` | `/users/{user_id}` | não | GET /api/users/:user_id | A — no island signal |

A sugestão é mecânica, e está aqui para ser contestada — o motivo ao lado de cada classe diz de que linha ela saiu: **C** quando o arquivo mostra tratador de ponteiro, superfície de desenho (`<canvas>`, ou um `<svg>` em que algo de fato desenha — ícone é ícone), editor ou captura de mídia (`getUserMedia`, `MediaRecorder`, reconhecimento de fala) — quem trabalha é o browser, então vira ilha; **B** quando faz polling, abre modal, tem abas, recebe arquivo ou toca áudio — o kit faz isso sem bundle; **A** no resto, que é formulário e lista, e cabe inteiro no servidor. Um módulo conta pelos nomes que a tela importou dele, e não por tudo o que ele faz: `import { a, b } from "x"` é lido como `a` e `b`, e um módulo alcançado por import default ou de namespace diz `whole module` ao lado do motivo.

## Server Actions

O que as telas escrevem: 2 ações em 1 arquivo. Uma Server Action não tem URL — é uma função que o formulário chama, e o `"use server"` é o contrato inteiro. Aqui cada uma vira um handler de escrita (um `route.go`, ou o `POST` da própria rota da página) e um formulário que posta nele: o handler grava e redireciona, a página desenha — o PRG que o kit já faz. A classe ao lado de uma tela é sobre o que ela desenha; estas são o que ela muda.

| Ação | Origem | Telas |
|---|---|---|
| `registrarResposta` | `lib/actions.ts:3` | `/estudo` |
| `criarBaralho` | `lib/actions.ts:7` | `/estudo` |

## Dependências globais

Alcançadas a partir de um `layout.tsx`: a moldura de todas as telas, e trabalho de nenhuma delas em particular. Porta-se uma vez — no layout, ou numa ilha só — e não muda a classe das telas que envolve.

- `components/Chat.tsx` (19 linhas): polling — sozinha seria B.

## Sem equivalente

- `app/dashboard/(.)modal/page.tsx`: rota interceptadora: sem equivalente — modal sobre uma página é ui.Dialog na página que abre ((.)modal)
- `app/dashboard/@sidebar/page.tsx`: rota paralela: não há equivalente — renderize os slots como partes da página (@sidebar)
- `app/dashboard/template.tsx`: template: layout que remonta não significa nada sem roteador no cliente
- `app/docs/[[...slug]]/page.tsx`: catch-all opcional: o endereço sem o segmento precisa de uma página própria ([[...slug]])
- `app/documents/[id]/loading.tsx`: estado de carregando: a página chega pronta, então não há instante para preencher
- `app/layout.tsx`: layout raiz: o seu já existe e é um documento <html> inteiro — porte o head e a moldura para ele
- `app/loading.tsx`: estado de carregando: a página chega pronta, então não há instante para preencher
- `app/users/[user-id]/page.tsx`: parâmetro renomeado para um identificador Go — a URL e o c.Param usam o nome novo ([user-id])
- `middleware.ts`: middleware: vira app/<ramo>/middleware.go, ou Auth.Require() — não há tradução automática
- `next.config.ts`: rewrite: vira uma linha de Config.Upstreams, com a credencial que o proxy injeta (/api/:path* → https://api.example.com/:path*)

## O que fazer agora

1. `trilha gen` e `go build ./...`: o esqueleto compila como está.
2. Porte primeiro as telas **A** — são formulários e listas, e fecham rápido.
3. `trilha ui describe` diz o que o kit tem, para você não chutar um nome.
4. A receita *Do Next.js para a Trilha* traz o padrão de React ao lado da linha que ocupa o lugar dele.
