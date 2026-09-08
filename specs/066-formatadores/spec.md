# Spec 066 — Data, tamanho, duração e contagem

- **Issue**: #99 — a issue é a fonte do escopo; aponte para ela, não a reescreva aqui.
- **Branch**: `066-formatadores`
- **Versão**: 0.48.0

## Por quê

O Trilha diz, com razão, que não tem locale e que dinheiro é da aplicação. Mas data, tamanho,
duração e contagem **não são domínio**: são iguais em toda aplicação, e toda aplicação escreve
de novo — quinze telas com `fmtDate` no app medido, cinco com `fmtBytes`, e todas com o mesmo
erro de iniciante: `toLocaleString` sem fuso, sem `<time datetime>`, sem tratar nulo.

## O que muda

`Config.Locale` e `Config.TimeZone`, e quatro formatadores no kit. O contrato está na
referência do `ui`, nas duas línguas. O que vale registrar:

**Eles recebem um `Ctx`, e a issue propunha sem.** `ui.Date(t)` exigiria idioma em estado de
pacote, e a spec 046 tirou o exemplo de estado de pacote exatamente porque duas aplicações num
processo — que é o que o `Provide` e o app embutido existem para permitir — passariam a
compartilhá-lo, e a segunda a subir mudaria a primeira em silêncio. O `Head`, o `Flashes` e o
`DataTable` recebem `Ctx` pelo mesmo tipo de razão.

**Ausente é travessão.** `time.Time` zero, `*time.Time` nil, tamanho zero. Data zero saindo
como `01/01/0001` é o bug que isto remove.

**O `datetime` é sempre o instante**, em RFC 3339 UTC, enquanto o texto é local e traduzido: a
máquina recebe o fato, a pessoa recebe a apresentação.

**O relativo não anda.** Escreve "há 3 min" e guarda o absoluto no `title`. Nada o atualiza —
o kit não tem relógio e a spec 057 já recusou um. Quem quer o número se mexendo põe num
`ui.Poll`.

**Fuso desconhecido cai para UTC e avisa no log, uma vez.** Cair em silêncio deslocaria todo
horário da tela sem nada parecer quebrado. A zona é resolvida uma vez e lembrada, inclusive a
falha: carregar zona lê o disco, e uma página com cinquenta datas leria cinquenta vezes.

## Fora de escopo

- **Dinheiro.** A moeda, o lugar do símbolo e como um negativo se lê são da aplicação, e um
  framework que chutasse estaria errado no país de alguém.
- **Tabela ICU / mais idiomas.** Dois, como o resto do framework.
- **Um quarto layout de data.** Três por idioma; oferecer o quarto convida o quinto, e o que
  isto existe para remover é justamente a string de layout na página.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `time` e `strconv`. |
| IV — superfície pequena | Quatro funções e cinco opções; dinheiro fora. |
| VI — teste primeiro | Golden nas duas línguas, o travessão, o fuso desconhecido ruidoso, e a leitura humana da duração. |
| Estilo | Referência nas duas línguas no mesmo commit. |

## Tarefas

- [x] T001 `Config.Locale`, `Config.TimeZone`, `Ctx.Locale`, `Ctx.Location` com resolução única.
- [x] T002 `ui/format.go`: `Date`, `Bytes`, `Duration`, `Number` e as opções.
- [x] T003 Testes: golden en/pt, ausentes, fuso desconhecido, relativo, duração.
- [x] T004 `trilha audit` avisa sobre `time.Format` com layout dentro de `app/`.
- [x] T005 Referência do `ui` nas duas línguas; catálogo do `ui describe`.
- [x] T006 Fase 9 registrada no `ROADMAP.md` — pendência de três releases.
- [ ] T007 `CHANGELOG.md`, `version`, `make test` verde e release.

## Aceitação

- **SC-001** A mesma chamada rende `Sep 8, 2026 3:04 PM` em `en` e `08/09/2026 12:04` em
  `pt-BR` com `America/Sao_Paulo`, e o `datetime` é o instante nos dois.
- **SC-002** Zero, nil e valor não numérico rendem `—`, nunca `01/01/0001`.
- **SC-003** Fuso inexistente mostra UTC **e** deixa linha no log.
- **SC-004** `1.400.000` sai `1,4 MB` (pt) e `1.4 MB` (en), com o exato no `title`.
