# Plano — Spec 148

**Branch**: `feat/rotas-public-especificidade` | **Spec**: [spec.md](spec.md) | **Issues**: #203, #204

## Summary

Duas correções no despacho, nenhuma convenção nova em `app/` e nenhum símbolo público novo: o
arquivo de `Public`/`Mounts` responde antes de uma rota com curinga, e os padrões que o
`http.ServeMux` recusa registrar passam a ser despachados pelo kit com a regra de
especificidade por segmento. Forma completa porque são dois contratos de roteamento (o que
responde primeiro, quem ganha quando dois padrões se cruzam) e porque o limite do despacho
próprio precisa ficar escrito.

## Technical Context

Go 1.22+, stdlib. Arquivos:

- `routing.go` (novo): padrão em segmentos, casamento, comparação por especificidade, despacho
  das rotas que o mux recusou;
- `serve.go`: `Register` (sondagem no `pathMux`, handlers na rota própria quando há conflito),
  `fallback` (405 e barra final enxergando essas rotas);
- `trilha.go`: `dispatch` — estático antes das rotas curinga, depois o despacho próprio, depois
  o mux; `New` põe `/_trilha/events` também no `pathMux`;
- `export.go`: `render` passa pelo `dispatch`, não pelo mux cru;
- `assets.go`: aviso quando o endereço do arquivo é de uma rota literal;
- `routing_test.go` (novo), `serve_test.go`;
- `examples/blog`: `public/blog/capa.svg`, `app/oficinas/slug_/inscricao/page.go`,
  `app/secao_/inscricao/id_/page.go`, `trilha_gen.go` regerado, teste de integração;
- `site/internal/docs/content/{en/reference/conventions.md,pt/referencia/convencoes.md}` e a
  seção de estáticos em `{en/reference/app.md,pt/referencia/app.md}`;
- `CHANGELOG.md`, `cmd/trilha/main.go` (`version`), `ROADMAP.md`.

## Constitution Check

Na spec. Sem violação; sem *Complexity Tracking*.

## Decisões

- **O mux do Go continua sendo o roteador.** O despacho próprio vale só para o conjunto que ele
  recusa, e num app típico esse conjunto é vazio: zero código novo no caminho quente. A
  alternativa — casar todos os padrões no kit — pagaria em risco (métodos, `{$}`, escapes,
  precedência) o preço de um caso de borda.
- **A sonda de conflito é o `pathMux`, que já existe**, com `recover`. Ele é registrado sem
  método, então é mais conservador que o mux de verdade: dois padrões que se cruzam só em
  métodos diferentes também vão para o despacho próprio. Conservador é o lado certo de errar —
  a resposta é a mesma, decidida pela regra de segmento.
- **Padrão repetido continua `panic`.** Hoje quem explode é o `pathMux`; com o `recover` no
  lugar, a repetição viraria silêncio. `Register` passa a checar `a.routes` antes e explodir
  com uma frase própria: é bug do arquivo gerado, não conflito de padrões.
- **Rota literal > estático > rota com curinga.** Inverter as duas primeiras trocaria um
  sombreamento silencioso por outro. A ordem escolhida é a que respeita quem escreveu o
  endereço à mão e resolve o caso da #203, que é sempre um arquivo contra um curinga.
- **O `fs.Stat` só acontece quando pode servir para alguma coisa**: `GET`/`HEAD`, app com
  `Public` ou `Mounts`, e rota casada com curinga. É por isso que a decisão consulta o
  `pathMux` (barato) antes de tocar no sistema de arquivos.
- **`dispatch` num lugar só.** `serveHTTP` e `export.render` chamavam `a.mux` direto; agora
  chamam o mesmo `dispatch`, senão o `export` renderizaria uma rota diferente da que o servidor
  responde.
- **Comparação sobre tipos de segmento, não sobre ordem de registro.** Dois padrões que casam o
  mesmo caminho diferem em alguma posição; comparar estático < `{param}` < `{path...}` dá uma
  ordem total e determinística, e o gerador não precisa emitir nada em ordem especial.
- **`{path...}` é o menos específico *da sua posição***, e só dela: em `/docs/{path...}` contra
  `/{secao}/guia/{id}`, quem decide é o `docs` literal da primeira posição, não o `{path...}` da
  segunda. Onde a posição empata, ele perde — inclusive para um padrão mais longo cheio de
  `{param}`, que é o que o Go já faz.
- **O despacho próprio passa pelo `wrap`.** Os handlers guardados na rota são os mesmos
  `http.Handler` que iriam para o mux, com CORS e cadeia — não há um segundo caminho de
  execução, só um segundo jeito de escolher a rota.
- **`SetPathValue` no despacho próprio**: `c.Param` lê `Request.PathValue`, e quem não passa
  pelo mux precisa preencher — inclusive o `{path...}`, com as barras, como o mux preenche.
