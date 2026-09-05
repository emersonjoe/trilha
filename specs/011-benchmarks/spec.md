# Feature Specification: Benchmarks e comparação

**Feature Branch**: `011-benchmarks` | **Created**: 2026-09-05 | **Status**: Draft
**Input**: "acrescente benchmark e comparação com outros frameworks se isso fizer sentido e
não gere problemas"

## Decisão de escopo (o que faz sentido e não gera problema)

- **Números só contra a biblioteca padrão.** O custo que interessa a quem avalia o Trilha é
  o *overhead* sobre `net/http` + `html/template`, que é a alternativa real em Go. Medir
  Gin, Echo, Fiber ou Next.js aqui geraria comparações injustas (configurações diferentes,
  versões que envelhecem, marcas de terceiros) e é exatamente o tipo de tabela que vira
  briga. Quem quiser, roda o `bench/` e acrescenta o que quiser localmente.
- **Comparação com outros frameworks é qualitativa e verificável**: uma tabela de
  abordagem (roteamento por arquivos, HTML tipado, dependências, recarga, export estático,
  segurança padrão) com link para a documentação de cada projeto, nota de não afiliação e
  sem adjetivos.
- **Módulo separado** `bench/` (`github.com/emersonjoe/trilha/bench`, `replace ../`) para o
  módulo raiz continuar sem dependências e sem código de teste pesado.

## User Scenarios & Testing

### US1 - Benchmarks reprodutíveis (P1)
`cd bench && go test -bench . -benchmem` mede: página com layout (`h` × `html/template`),
resposta JSON (Trilha × `net/http`), arquivo estático (Trilha × `http.FileServer`), roteamento
com 200 rotas e parâmetro (Trilha × `http.ServeMux` puro) e a cadeia de 5 middlewares.
**Acceptance**: `make bench` roda; CI executa com `-benchtime=50ms` para garantir que
compila; `bench/RESULTS.md` tem os números da máquina de referência com data, CPU e
versão do Go, e é regravado por `make bench-results`.

### US2 - Página "Desempenho e comparação" no site (P1)
Metodologia, tabela de overhead (com o aviso de que o custo dominante em apps reais é I/O),
tempo do ciclo editar→ver, e a tabela qualitativa.
**Acceptance**: a página existe na Referência e passa nos testes do site.

## Requirements
- FR-001 Zero mudança no módulo raiz além do Makefile e CI.
- FR-002 Nenhum número sobre projetos de terceiros.
