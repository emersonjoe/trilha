# Spec 090 — bench/agent: portar uma listagem .tsx

- **Issue**: [#94](https://github.com/emersonjoe/trilha/issues/94) — a issue é a fonte do escopo.
- **Branch**: `090-bench-port-listing`
- **Versão**: 0.71.0

## Por quê

A Fase 7 fechou inteira e o `bench/agent` continua com os quatro cenários de antes. A régua não
mede a fase que mais mexeu no que o agente escreve — `ListParams`, `ui.DataTable`, `ui.Poll`,
`c.Fragment` — e sem esse cenário o "antes × depois" que cada issue daquela fase pediu não
existe.

Uma régua que não mede a coisa que se mudou é uma régua que concorda com qualquer resultado.

## O que muda

Um quinto cenário, `port-listing`. O `Prepare` **troca** o `app/documentos/page.go` do
`examples/blog` por um stub e deixa ao lado o `.tsx` que a tela era, mais a linha
correspondente do `MIGRATION.md`. Stub e não remoção: apagar o arquivo deixa o pacote vazio e o
`trilha_gen.go` deixa de compilar, e aí o que falha é o projeto e não a tela. A tarefa dada ao agente é portar aquela tela.

O que o teste escondido verifica, tudo em processo e sem rede:

- a listagem responde, com as colunas que o `.tsx` tinha;
- **a ordenação é a da URL**: `?sort=tamanho&dir=desc` ordena de verdade, e a coluna ordenada
  traz `aria-sort` — sem o qual a tabela é ordenada para quem enxerga e não para quem escuta;
- **a paginação preserva o filtro**: os links de página carregam o `?q=`. É o erro clássico da
  porta de um `useSearchParams`, e o que faz a segunda página mostrar outra coisa;
- **a atualização automática existe e é fragmento**: a página traz `data-trilha-poll`, e pedir o
  fragmento devolve só o pedaço — não a página inteira dentro dela mesma;
- **nada de JavaScript próprio na página**: portar um `'use client'` para Go e trazer o
  JavaScript junto é não ter portado. O que conta é o que a página carrega — uma ilha, ou um
  `src` que não é do kit; script embutido fica de fora porque o shell escreve um, e distinguir
  o dele pelo texto é um teste que adivinha.

## Fora de escopo

- **O stub HTTP da API, o `Upstream` e o `trilha client`.** A issue pede que o cenário meça
  também por onde a chamada saiu. Isso pede um segundo serviço de pé durante a verificação, que
  nenhum dos cinco cenários tem hoje — o `examples/local-login` é quem tem `Upstreams`, e medir
  ali é um cenário próprio. Este mede a tela; o outro mediria o caminho até a API.
- **O contador de "quantas vezes o agente abriu o `.tsx`".** É uma medida de ferramenta, não de
  resultado, e o `bench` hoje conta tokens e turnos. Vale quando houver o que comparar.
- **Rodar a régua e publicar o número.** Cada execução chama um agente de verdade; o que esta
  spec entrega é o cenário e a prova de que ele é uma régua válida — que o teste escondido falha
  no fixture intocado e o fixture compila com ele ao lado.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | o cenário é um struct e um teste |
| VI — teste primeiro | o `TestFixturesFailWithoutTheAgent` já é esse teste: ele prova que a régua mede algo |

## Tarefas

- [x] T001 O `.tsx` de origem e a linha do `MIGRATION.md`, como fixture
- [x] T002 O `Prepare` que troca a página por um stub e deixa a origem ao lado
- [x] T003 O teste escondido: colunas, ordenação com `aria-sort`, paginação com filtro, fragmento
- [x] T004 O cenário na lista, com `TestFixturesFailWithoutTheAgent` e `TestPortListingEhAtingivel` verdes
- [x] T005 `CHANGELOG.md`, `version`, `ROADMAP.md`, `make test`, `scripts/release.sh 0.71.0`

## Aceitação

- **SC-001** Com o fixture intocado, o teste escondido **falha** — senão o cenário não mede nada.
- **SC-002** O fixture compila com o teste escondido ao lado, então uma falha é a medição e não a
  régua quebrada.
- **SC-003** O que o `Prepare` troca é só a página: o resto do exemplo continua compilando.
- **SC-004** O teste escondido passa contra a página que o `examples/blog` já tem — a régua é
  atingível, e a prova é a própria tela que foi trocada.
