# Feature Specification: Exemplos de dificuldade média e complexa

**Feature Branch**: `009-exemplos` | **Created**: 2026-09-05 | **Status**: Draft
**Input**: "acrescente um exemplo com regras de formulário mais complexas explorando a
responsividade dos formulários — mostrando e escondendo campos, mostrando mensagens de
feedback para o usuário e depois fazendo fading do componente da mensagem. Acrescente
exemplos de dificuldade média e de dificuldade complexa. Crie um exemplo para controle de
orçamento com contas contábeis e recursos desse tipo com drill down e aninhamento de
componentes. Se o código dos exemplos ficar complexo implemente melhorias no framework."

## Escada de exemplos

| Nível | Exemplo | Ensina |
|---|---|---|
| Básico | `examples/blog` (existente) | convenções de arquivo, layouts, API, middleware, sessão |
| Médio | `examples/cadastro` (novo) | formulário com regras: campos condicionais, validação por campo no servidor com valores preservados, seleção dependente via API, feedback que some sozinho, layout responsivo |
| Complexo | `examples/orcamento` (novo) | domínio com árvore (plano de contas), agregação, drill-down por rota dinâmica, componentes aninhados e recursivos, lançamentos com diálogo, filtros por período, exportação CSV |

## User Scenarios & Testing

### US1 - Cadastro com regras (médio, P1)
Formulário de cadastro de cliente em `/`: **Tipo** (pessoa física/jurídica) mostra CPF+data
de nascimento ou CNPJ+razão social; **Endereço de cobrança diferente** (switch) revela um
segundo bloco de endereço; **UF** carrega as cidades por `GET /api/cidades?uf=SP` e o
`<select>` de cidade fica desabilitado até a UF ser escolhida; **Quero receber novidades**
revela a frequência (rádio). No `POST`, o servidor valida (CPF/CNPJ com dígitos
verificadores, e-mail, obrigatórios conforme o tipo), devolve a página com os erros por
campo e os valores preservados, ou redireciona para `/?ok=1` que mostra um aviso "Cadastro
salvo" que some em 4 s e a lista de cadastrados. Em telas estreitas os campos empilham.

**Acceptance**: testes `httptest`: PJ sem CNPJ → 422 com erro no campo `cnpj` e valores
mantidos; PF com CPF inválido → erro; cidades por UF; POST válido → 303 e a lista tem o
registro; campos escondidos **não** chegam no POST (disabled) e, se chegarem por request
manual, são ignorados pela regra do tipo. Navegador: trocar tipo esconde/mostra; aviso some.

### US2 - Orçamento com plano de contas (complexo, P1)
Plano de contas hierárquico (`1 Receitas`, `1.1 Vendas`, `1.1.1 Produtos`…; `2 Despesas`,
`2.1 Pessoal`, `2.1.1 Salários`…) com orçamento mensal por conta analítica e lançamentos
reais. Página `/` mostra o resumo do mês (receita, despesa, resultado, % do orçamento) e a
árvore de nível 1 com orçado × realizado × variação; clicar numa conta abre `/contas/{codigo}`
com breadcrumb, os filhos (mesma tabela, recursiva) e, na conta analítica, os lançamentos.
Filtro de mês na URL (`?mes=2026-09`). Botão "Novo lançamento" abre um diálogo (conta,
data, valor, descrição) e, ao salvar, volta com aviso que some. `GET /api/relatorio.csv?mes=`
exporta. Contas sintéticas agregam os filhos; variação acima de 10 % ganha selo.

**Acceptance**: testes: agregação (pai = soma dos filhos), variação, drill-down 404 para
código inexistente, lançamento inválido (conta sintética, valor ≤ 0) → erro no campo,
CSV com cabeçalho e linhas; navegador: drill-down, diálogo, aviso.

### US3 - Melhorias no framework (P2)
O que se repetir nos dois exemplos vira API do Trilha ou do kit, com teste e doc:
- `c.Bind(&struct)`: decodifica formulário ou JSON em struct por tags `form:"nome"`
  (string, int, int64, float64, bool, []string, time.Time em `2006-01-02`), com erros de
  conversão por campo;
- `trilha.FieldErrors` (`map[string]string`) que implementa `error`, com `Add`, `Has`, `Get`
  e status 422 ao ser devolvido de um handler;
- `ui.Field` aceita `ui.Value(v)`/`ui.Errors(errs, "campo")` para preservar valor e marcar
  erro sem `if` repetido; `ui.Radio` já existe; `ui.SelectOptions([]Option, selecionado)`;
- `ui.Money(cents)`/`ui.Delta(pct)` no exemplo (não no kit) — formatação é do domínio.

## Requirements
- FR-001 Cada exemplo tem `README.md` curto (o que ensina, como rodar, o que olhar).
- FR-002 Testes de integração `httptest` cobrindo cada aceitação; `make test` verde.
- FR-003 Docs: capítulo "Exemplos" no Aprender (escada + o que cada um ensina) e as
  melhorias de US3 na referência (`ctx`, `erros`, `ui`); README; CHANGELOG.
- FR-004 Sem dependências externas nos exemplos (dados em memória com seed).
- FR-005 Sem JavaScript próprio além de um `app.js` pequeno para as cidades dependentes; o
  resto vem de `ui.js`.
