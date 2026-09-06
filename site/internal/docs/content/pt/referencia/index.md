---
title: Visão geral
description: Os pacotes do Trilha e o que cada um faz.
---

| Pacote | Import | Papel |
|---|---|---|
| `trilha` | `github.com/emersonjoe/trilha` | runtime: `App`, `Ctx`, erros, CSRF, estáticos, export |
| `h` | `github.com/emersonjoe/trilha/h` | DSL de HTML |
| `tmpl` | `github.com/emersonjoe/trilha/tmpl` | adaptador para `html/template` |
| `cache` | `github.com/emersonjoe/trilha/cache` | cache com prazo, tags e memo por requisição |
| `ui` | `github.com/emersonjoe/trilha/ui` | kit de componentes (tema compatível com shadcn/ui) |
| `ai` | `github.com/emersonjoe/trilha/ai` | cliente OpenAI-compatível, ferramentas, agentes |
| `ai/mcp` | `github.com/emersonjoe/trilha/ai/mcp` | cliente e servidor MCP |
| CLI | `github.com/emersonjoe/trilha/cmd/trilha` | `new`, `gen`, `dev`, `build`, `routes`, `export`, `audit`, `ui` |

Nenhum deles depende de nada fora da biblioteca padrão. Compatível com Go 1.22 ou mais novo.

## Modelo mental em uma página

- **Convenções de arquivo** em `app/` definem rotas, layouts e middlewares
  ([Convenções](/pt/referencia/convencoes)).
- Toda função de rota recebe `*trilha.Ctx` ([Ctx](/pt/referencia/ctx)) e devolve `error` ou
  `(h.Node, error)`.
- Erros são valores com significado HTTP ([Erros](/pt/referencia/erros)).
- Entrada de formulário e de JSON é preenchida e conferida pelo `Bind`
  ([Validação](/pt/referencia/validacao)).
- O HTML é um `h.Node` ([h](/pt/referencia/h)), vindo do DSL ou de um template
  ([tmpl](/pt/referencia/tmpl)).
- `trilha_gen.go` liga tudo e é gerado pela [CLI](/pt/referencia/cli); `App`
  ([App](/pt/referencia/app)) é o que ele monta.

## Estabilidade

Versão 0.x: a API pode mudar entre versões menores. Mudanças incompatíveis são listadas no
`CHANGELOG.md` do repositório com instruções de migração.
