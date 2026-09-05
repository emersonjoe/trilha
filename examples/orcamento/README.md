# Exemplo: orçamento (dificuldade complexa)

Controle orçamentário com plano de contas hierárquico, orçado × realizado por mês,
drill-down e componentes aninhados.

O que ensina:

- **Domínio com árvore** (`internal/plano`): contas sintéticas agregam as analíticas;
  orçamento mensal e lançamentos em memória, com seed determinístico.
- **Componentes aninhados e recursivos** (`internal/componentes`): `Linha` renderiza a
  conta e chama a si mesma para as filhas (`ui.Depth` indenta); `Tabela`, `Cartao`,
  `ResumoCards`, `Variacao`, `Barra`, `Trilha`, `FormLancamento`, `DialogoLancamento`.
- **Drill-down por rota dinâmica**: `/contas/{codigo}?mes=` com breadcrumb, filhas ou
  lançamentos, e 404 para código inexistente.
- **Um formulário, dois lugares**: o mesmo `FormLancamento` dentro de `ui.Dialog` (na
  visão geral e no drill-down) e na página `/lancamentos`; o `POST` valida com
  `c.Bind` + `trilha.FieldErrors`, devolve 422 com os erros no campo (o diálogo reabre
  sozinho) ou redireciona para onde o formulário foi aberto com um aviso que some.
- **Filtro por período** na URL (`?mes=2026-08`) e **exportação CSV** em
  `/api/relatorio.csv` (pasta com ponto no nome).

```bash
cd examples/orcamento && trilha dev
```

Teste: `go test ./examples/orcamento/`.
