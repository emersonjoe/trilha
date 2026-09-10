# Spec 115 — `Deadlines`: o que vence, quando, e de que cor

- **Issue**: [#150](https://github.com/emersonjoe/trilha/issues/150) — a issue é a fonte do escopo.
- **Branch**: `115-deadlines`
- **Versão**: 0.94.0

## Por quê

Todo app com data de vencimento acaba com o mesmo cálculo: quantos venceram, quantos vencem em 30
e em 90, qual é o mais próximo, e qual cor cada faixa recebe. Medido no Acervo: 161 linhas de rota
de conformidade, 424 de prazos de guarda e 176 de tela, todas em cima de `today + 30` e
`today + 90`.

O framework já mostra uma data (`ui.Date`, `ui.Relative`) e um número (`ui.Stat`). O que falta é a
conta no meio.

## O que muda

```go
itens := []trilha.Deadline{
	{Title: "Certidão FGTS · ACME", Due: d1, URL: "/fornecedores/1", Kind: "certidao"},
	{Title: "Contrato 2023/12", Due: d2, URL: "/contratos/12", Kind: "contrato"},
}

resumo := trilha.Deadlines(itens, trilha.DeadlineOpts{Horizons: []int{7, 30, 90}, Now: agora})
// resumo.Overdue, resumo.Within[30], resumo.Next, resumo.ByKind
```

- **O dia acaba no fim do dia.** Um prazo é uma data, não um instante: vence hoje significa que
  até o último minuto de hoje ainda não venceu. É o erro que aparece toda vez que alguém compara
  `time.Time` direto, e o fuso que decide qual é "hoje" é o do `Config`.
- **`Done` sai da conta.** Um item resolvido não é um prazo, é história.
- **Dias úteis quando a regra é essa**: `DeadlineOpts{Business: ...}` conta pulando fim de semana
  e os feriados que o app passar. O calendário é do app: feriado nacional é uma lista que muda por
  país e por ano, e uma lista errada dentro do framework é pior do que nenhuma.
- **`ui.DeadlineCards`, `ui.DeadlineList`, `ui.DeadlineBadge`**: os quatro cartões, a lista
  ordenada com `ui.Relative` e a badge do menu. Cor por faixa, classe do `ui.css`, sem JavaScript.

## Fora de escopo

- **A tabela de feriados nacionais.** Ela muda por país e por ano; o que entra é o mecanismo, e o
  app passa as datas. Um calendário desatualizado dentro do framework seria uma conta errada que
  ninguém desconfia.
- **A receita e a página do cookbook.** O painel é uma tela do app, e as peças agora existem.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `time` |
| VI — teste primeiro | vence hoje, fuso, item resolvido, faixas próprias, dias úteis |
| Determinismo | `Now` é um parâmetro: um painel de prazos testável não pode depender do relógio |

## Tarefas

- [x] T001 Teste que falha: as faixas, o fim do dia, o `Done` e os dias úteis
- [x] T002 `Deadline`, `Deadlines` e `BusinessDays`
- [x] T003 `ui.DeadlineCards`, `ui.DeadlineList` e `ui.DeadlineBadge`
- [x] T004 Documentação (en + pt) e superfície de API
- [x] T005 `CHANGELOG.md`, `version`, `ROADMAP.md`, `make test`, `scripts/release.sh 0.94.0`

## Aceitação

- **SC-001** Um prazo que vence hoje não está vencido até o fim do dia.
- **SC-002** Um item `Done` não entra em faixa nenhuma.
- **SC-003** As faixas são as pedidas, e o `Next` é o mais próximo que ainda não venceu.
- **SC-004** `BusinessDays` pula fim de semana e os feriados passados.
- **SC-005** Os três componentes desenham com lista vazia sem erro.
