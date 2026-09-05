---
title: Desempenho e comparação
description: Quanto o Trilha custa sobre a biblioteca padrão, como medir você mesmo, e como ele se posiciona frente a outras abordagens.
---

## Metodologia

O único número que faz sentido publicar é o **custo do framework sobre a biblioteca
padrão**, que é a alternativa real em Go. Os benchmarks ficam em `bench/` (módulo separado,
para o Trilha continuar sem dependências) e medem, em processo (`httptest`, sem rede), o
mesmo trabalho feito de dois jeitos: com o Trilha e com `net/http` + `html/template` puros.

```bash
git clone https://github.com/emersonjoe/trilha && cd trilha
make bench            # roda; make bench-results regrava bench/RESULTS.md
```

Cenários: página com layout e 20 itens (`h` × `html/template`), resposta JSON, arquivo
estático (`Public` × `http.FileServer`), 200 rotas com parâmetro (`ServeMux` nos dois lados)
e cadeia de 5 middlewares.

## Resultados de referência

Apple M2, Go 1.25, 2026-09-05 (mediana de 3 execuções; `bench/RESULTS.md` tem a saída
completa). Valores por requisição.

| Cenário | Stdlib | Trilha | Diferença |
|---|---|---|---|
| Página (20 itens, layout) | 29,4 µs · 270 allocs | 19,4 µs · 482 allocs | `h` é ~34 % mais rápido que `html/template` aqui, com mais alocações |
| JSON (20 itens) | 4,2 µs | 7,6 µs | +3,4 µs |
| Estático (1,4 KB) | 1,4 µs | 4,3 µs | +2,9 µs |
| 200 rotas + parâmetro | 0,72 µs | 4,0 µs | +3,3 µs |
| 5 middlewares | 0,64 µs | 4,1 µs | +3,4 µs |

Leitura honesta: o Trilha tem um **custo fixo de ~3 µs e ~40 alocações por requisição**,
independente da rota. Ele paga por: id de requisição (aleatório), nonce da CSP, cabeçalhos
de segurança, `Ctx` com mapa de valores, limite de corpo, medição e **log estruturado** de
cada requisição (`slog`, que formata a linha mesmo descartada). Em um servidor real uma
consulta ao banco custa de 100 µs a alguns ms, e a rede, mais; a diferença desaparece. Se
um dia isso importar para você, o caminho é reduzir alocações no `Ctx` e tornar o log
opcional por rota — e o benchmark está aí para provar o ganho.

### Observabilidade

| Cenário | Sem métricas | Com métricas | Diferença |
|---|---|---|---|
| Rota trivial (`c.Text`) | 4,1 µs · 50 allocs | 4,1 µs · 50 allocs | dentro do ruído; **zero alocações** |
| Sonda `/_trilha/health/live` | — | 0,9 µs · 18 allocs | não passa pelo roteador nem pela cadeia de middleware |

A instrumentação só existe quando `Observability.Metrics` está configurado; desligada, é
uma comparação de ponteiro. Ligada, a chave da série é montada num buffer de pilha e
procurada como `map[string(bytes)]`, forma que o compilador resolve sem alocar — por isso
a contagem de alocações não muda.

O ciclo **editar → ver** do `trilha dev` fica em ~1,2 s no exemplo do blog (recompilação
do Go) e ~30 ms para mudanças só em `public/` (`make reload` mede na sua máquina).

## Comparação de abordagem

Sem números de terceiros: versões mudam, configurações diferem e cada projeto otimiza para
coisas diferentes. O que dá para comparar com segurança é a **abordagem**. Confira sempre
a documentação de cada um; nomes citados são marcas dos respectivos donos e não há
afiliação.

| | Trilha | `net/http` puro | Roteadores Go (chi, echo, gin, fiber) | templ + htmx | Next.js |
|---|---|---|---|---|---|
| Rotas | por pastas em `app/` (`page.go`, `route.go`) | registradas à mão | registradas à mão | registradas à mão (com o roteador que você escolher) | por pastas em `app/` |
| Layouts aninhados | `layout.go` por pasta | manual | manual | componentes | `layout.tsx` |
| HTML | DSL tipado `h` (escape por padrão) ou `html/template` | `html/template` | `html/template` ou libs | `templ` (compilado) | JSX/React |
| Interatividade no cliente | HTML + `ui.js` (200 linhas) ou htmx; sem hidratação | você escolhe | você escolhe | htmx | React (hidratação, RSC) |
| Dependências no runtime | nenhuma | nenhuma | o roteador (+ deps) | `templ` (+ gerador) | Node, React, Next |
| Dev | `trilha dev`: recarga ~1 s, erro de compilação na página | `go run` manual | `air`/manual | `templ generate --watch` + reload | `next dev` (HMR) |
| Produção | um binário estático com `public/` embutido | binário | binário | binário | Node ou edge; build |
| Export estático | `trilha export` | manual | manual | manual | `output: 'export'` |
| Segurança padrão | CSP com nonce, HSTS, CSRF, rate limit, cookies assinados, timeouts | nada (você configura) | varia | nada (você configura) | cabeçalhos básicos; CSRF em Server Actions |
| IA | `ai` (OpenAI-compatível), `ai/mcp` | — | — | — | Vercel AI SDK (pacote) |

Quando **não** usar o Trilha: apps que precisam de interface altamente interativa no cliente
(editores, dashboards em tempo real com estado complexo) são melhor servidos por React/Next
ou por um SPA; e projetos que já têm um roteador Go e templates maduros ganham pouco ao
trocar. O Trilha brilha em apps de negócio renderizados no servidor, sites de conteúdo e
APIs com painel, onde um binário sem dependências e convenções fortes pesam mais que
interatividade fina.
