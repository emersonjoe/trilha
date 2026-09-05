# Feature Specification: Kit de UI padrão, customizável, inspirado no shadcn/ui

**Feature Branch**: `006-ui` | **Created**: 2026-09-05 | **Status**: Draft
**Input**: "adicione no spec kit que o padrão inicial, mas customizável, é utilizar
https://ui.shadcn.com/ — faça tudo dentro das licenças possíveis e edite os exemplos para
ficar assim"

## Contexto e licenças

O shadcn/ui é um conjunto de componentes React + Tailwind sob **licença MIT**
(Copyright (c) 2023 shadcn). O Trilha não roda React nem Tailwind e, pela constituição, não
adiciona dependências. O que se adota é a **filosofia** (o código dos componentes é copiado
para o projeto e pertence ao usuário) e o **contrato de tema**: os mesmos nomes de variáveis
CSS (`--background`, `--foreground`, `--primary`, `--radius`, `--chart-1`…, em `oklch`), de
modo que qualquer tema gerado para o shadcn/ui (ui.shadcn.com/themes, tweakcn e afins) possa
ser colado em `public/ui.css` e funcionar sem alterar Go.

O CSS e o JS são escritos do zero para o Trilha. Nomes de variáveis e valores numéricos do
tema padrão são reproduzidos com atribuição em `THIRD_PARTY_NOTICES.md` (MIT permite; a
atribuição é boa prática). Nenhum código React, Radix ou Tailwind é copiado. Ícones: um
conjunto pequeno de ícones do **Lucide** (licença ISC) embutido como SVG, também com aviso.
Nome e marca "shadcn" não são usados em identificadores públicos do Trilha; a documentação
cita o projeto como inspiração e compatibilidade de tema.

## User Scenarios & Testing

### US1 - Projeto novo já bonito e temável (P1)

`trilha new` cria `public/ui.css` (tokens + componentes) e `public/ui.js` (comportamentos),
e o layout gerado inclui `ui.Head(c)`. A primeira página usa `ui.Card`, `ui.Button` e
`ui.Input` e parece um app moderno, com modo claro/escuro automático.

**Acceptance**: (1) `trilha new x && cd x && go build .` compila; (2) `GET /` contém
`<link rel="stylesheet" href="/ui.css">` e classes `ui-card`/`ui-btn`; (3) colar o bloco
`:root{…} .dark{…}` de um tema shadcn v4 em `public/ui.css` muda as cores sem tocar em Go;
(4) `<html class="dark">` ou `prefers-color-scheme: dark` ativa o tema escuro.

### US2 - Componentes tipados no pacote `ui` (P1)

`import "github.com/emersonjoe/trilha/ui"` oferece funções que devolvem `h.Node`:
Button (variantes `Default/Secondary/Outline/Ghost/Destructive/Link`, tamanhos `Sm/Lg/Icon`),
Card (+Header/Title/Description/Content/Footer), Input, Textarea, Select, Checkbox, Switch,
Label, Field (rótulo + controle + ajuda + erro), Badge, Alert, Table, Tabs, Dialog,
Separator, Skeleton, Progress, Breadcrumb, Avatar, Toast (mensagem com desaparecimento),
Collapsible, Icon. Todo componente aceita nós/atributos extras do `h` (mesmo padrão do DSL).

**Acceptance**: teste unitário por componente verificando classes e atributos ARIA; nenhum
componente usa `h.Raw` com dado do usuário; `Icon` só aceita nomes do conjunto embutido.

### US3 - Comportamento sem framework (P1)

`public/ui.js` (vanilla, sem dependências, ~200 linhas) implementa: tabs com teclado,
abrir/fechar `<dialog>`, toast com desaparecimento (`data-ui-fade`), campos condicionais
(`data-ui-show-when="campo=valor"`, esconde/mostra e desabilita os controles ocultos),
alternância de tema (`data-ui-theme-toggle`, persistida em `localStorage`), *popover*
nativo para menus. Sem JS a página continua utilizável (progressive enhancement:
`<details>`, `<dialog open>`, formulários normais).

**Acceptance**: teste no navegador (exemplo) cobrindo tabs, dialog, toast que some,
campo condicional e tema; o CSP padrão (`script-src 'self'`) aceita `ui.js` sem nonce.

### US4 - Atualizar o kit em projetos existentes (P2)

`trilha ui` grava `public/ui.css` e `public/ui.js` no projeto atual. Se o arquivo existe e
foi modificado pelo usuário, avisa e só sobrescreve com `--force`; `--css-only` preserva o
JS e vice-versa. O tema (bloco `:root`/`.dark`) é separado em `public/ui.theme.css` para
que atualizar os componentes não apague o tema do usuário.

**Acceptance**: e2e: rodar `trilha ui` em projeto novo é no-op; após editar `ui.theme.css`
e rodar `trilha ui`, o tema permanece; `ui.css` modificado sem `--force` → mensagem e exit 1.

### US5 - Exemplos e scaffold reestilizados (P1)

`examples/blog`, `examples/assistente` e os templates de `trilha new` passam a usar o kit.
O site de documentação mantém o próprio design (é uma vitrine à parte), mas ganha a página
"Interface com ui" com demos vivas.

**Acceptance**: testes de integração dos exemplos continuam verdes; screenshots no
navegador em claro e escuro.

## Requirements

- **FR-001** Pacote `ui` na raiz do módulo, só stdlib; `ui.CSS`, `ui.ThemeCSS`, `ui.JS`
  embutidos (`embed`) e `ui.Head(c *trilha.Ctx) h.Node` (respeita `c.Base()`).
- **FR-002** Contrato de tema idêntico ao shadcn/ui v4: variáveis listadas em `ui.theme.css`
  com valores do tema neutro; `--radius` base e derivados; `.dark` e `prefers-color-scheme`.
- **FR-003** Classes com prefixo `ui-` (evita colisão com CSS do usuário); nada de estilos em
  elementos nus, exceto `body` (fonte, cores de fundo/texto) opt-in por `.ui-body`.
- **FR-004** Acessibilidade: foco visível (`--ring`), `aria-*` em tabs/dialog/switch,
  contraste do tema padrão AA.
- **FR-005** CLI `trilha ui [--force] [--css-only|--js-only]`; `trilha new` chama a mesma
  função.
- **FR-006** Documentação: capítulo "Interface com ui" (Aprender), referência `ui`, CLI;
  README; CHANGELOG; THIRD_PARTY_NOTICES (shadcn/ui MIT, Lucide ISC); constituição emenda
  "Estilo" registrando o kit como padrão customizável.
- **FR-007** Tamanho: `ui.css` ≤ 25 KB e `ui.js` ≤ 10 KB sem minificar.

## Fora do escopo

Componentes com dependência de JS pesado (combobox com busca, date picker, command
palette), animações complexas, port do CLI `npx shadcn add`. Podem vir depois em spec própria.
