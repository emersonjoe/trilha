# Spec 174 — `ui.Sheet` e diálogo sob demanda

**Feature Branch**: `174-sheet-e-dialogo-sob-demanda` | **Created**: 2026-10-06 | **Status**: Entregue (0.155.0)

- **Issue**: #296 — a fonte do escopo e dos critérios.
- **Versão**: 0.155.0

Forma curta: um componente do kit com um arquivo de comportamento próprio (`ui.sheet.js`, como
`ui.nav.js` e `ui.upload.js`), o `DialogTrigger` com endereço, e o uso no `examples/blog`.

## Por quê

"O leitor de PDF aberto numa lateral enquanto a pessoa percorre a lista" só saía com JavaScript
da app: o `ui.Dialog` é modal e não carrega nada sob demanda, `DialogTrigger` + `ui.Swap` no
mesmo gatilho abria o diálogo vazio (o listener do diálogo cancelava o clique antes do
fragmento), e com `ui.Navigate` na região o painel precisa ficar fora dela para sobreviver.

## Jornada e risco

"Interação rica": abrir o documento ao lado da lista. Pior impacto: perder o lugar na lista a
cada documento, o leitor recarregando a cada clique, ou quem usa teclado sem saber para onde o
foco foi.

## O que muda

```go
ui.Sheet(c, "leitor", ui.SheetOpts{Title: "Document"})          // no layout, fora da região
h.A(h.Href("/docs/7"), ui.SheetOpen("leitor"), h.Text("Doc 7"))   // na lista
if c.Fragment() == "leitor-body" { return ui.SheetBody("leitor", leitor(c, doc)), nil }
ui.DialogTrigger("ver", ui.Swap("ver-body"), h.Href("/docs/7"))  // diálogo sob demanda
```

- `SheetOpts{Side, Width, Title, Modal, Open, Push}`; `<aside role=complementary
  aria-labelledby>` ou `<dialog>`; corpo `#<id>-body` (`ui.SheetBody`); botão de fechar com o
  rótulo do locale. `SheetOpen`, `SheetClose`, `SheetScript`.
- Abrir: foco no título, `aria-expanded`/`aria-controls` no link, corpo pedido como fragmento
  pelo `ui.fragment` do `ui.js` (marcas de espera, redirect seguido ou recarga — sessão expirada
  vira navegação completa, #291 —, scripts que chegam rodam). Fechar (Esc, botão, Voltar com
  `Push`): esvazia o corpo, devolve o foco ao link. Telefone: tela cheia, fecha a gaveta.
- `DialogTrigger` com `h.Href` é um `<a>`; com `Swap`, o diálogo abre e o fragmento carrega, sem
  empurrar endereço.
- `ui.nav.js` e o prefetch deixam em paz os links de painel e de diálogo.

## Decisões

1. **Sem `SheetOpts.Src`.** A issue previa o servidor preencher o corpo a partir de outra rota.
   Com `Open`, o corpo são os `children`: a rota chama a mesma função que o fragmento usa. Pedido
   interno entre rotas seria um mecanismo novo para um caso que uma chamada de função resolve.
2. **Arquivo próprio** (`ui.sheet.js`, 3 KB): só quem tem painel baixa; o `ui.js` ganhou só o
   necessário (diálogo sob demanda e `ui.fragment`).
3. **O painel no `blog` fica no layout raiz**, fora do `<main id="conteudo">` que a navegação
   troca, e só na área `painel` — o lugar certo é sempre fora da região.
4. **Voltar com `Push` refaz a região** quando a entrada anterior é de navegação no cliente: o
   `ui.nav.js` não sabe que a entrada saída era só do painel. Custa um GET; fica registrado.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | nada novo |
| IV — API pública | `Sheet`, `SheetOpts`, `SheetBody`, `SheetOpen`, `SheetClose`, `SheetScript`; `DialogTrigger` aditivo |
| VI — teste primeiro | renderização (`TestSheetRendersItsContract`, `TestSheetTriggersAreLinks`) não compilava; os cinco cenários de navegador reprovaram contra o kit da 0.154.0 (rodados com os arquivos antigos) |
| VII — segurança por padrão | `Width` só como comprimento CSS (vai num `style`); o corpo chega pelo mesmo caminho de fragmento, com inline inerte |

## Segurança e privacidade

- **Injeção**: `SheetOpts.Width` validado por expressão de comprimento; testado com tentativa de
  sair do `style` (ASVS V1.2, Top 10 A05:2025).
- **Fronteira**: o corpo é fragmento da própria rota, mesma origem, com as regras da 171.
- **Recursos**: fechar destrói o quadro do `ui.Preview` (memória de PDF aberto).

## Tarefas

- [x] T001 Testes de renderização; cinco cenários (`TestUISheet*`, `TestUIDialogLoadsOnDemand`);
  `Session.Viewport` no `uitest`; fixture `/fluxos/docs`.
- [x] T002 `ui/sheet.go`, `ui.sheet.js`, CSS, `DialogTrigger`, `ui.fragment`, exclusões no `ui.nav.js`.
- [x] T003 `examples/blog`: painel na área `painel` e ramo de fragmento no post, com teste de integração.
- [x] T004 Catálogo, guia e referência nas duas locales, `api/current.txt`, `uidoc`, cópias do kit.
- [x] T005 `ui.css` 42 KB, com o motivo no teste; `trilha ui --js` escreve dez arquivos.
- [x] T006 CHANGELOG, ROADMAP, versão; suíte inteira com números.

## Aceitação

- **SC-001** Abrir o painel faz um GET do fragmento e mostra o quadro; outro link troca o corpo.
- **SC-002** Navegar na região mantém o painel e o quadro, sem novo GET do arquivo.
- **SC-003** Esc fecha, esvazia e devolve o foco; `aria-expanded` acompanha.
- **SC-004** Diálogo com `Swap` abre e carrega; a 375 px o painel ocupa a tela e fecha a gaveta.
- **SC-005** Sem script, o link do painel responde a página inteira (200).

## Evidências

| Nível | Comando | Passaram | Falharam | Pularam |
|---|---|---|---|---|
| Unidade + integração | `make test` | 1593 | 0 | 2 |
| Navegador, 37 cenários × 3 motores + 5 do módulo | `UITEST_REQUIRED=1 UITEST_BROWSERS=all make test-ui` | 42 | 0 | 0 |

Pulos: `TestS3AoVivo`, `TestOIDCAoVivo` (serviço real). Sem teste: o anúncio do leitor de tela
ao abrir o painel (o cenário prova foco e atributos, não a fala).
