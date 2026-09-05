# Feature Specification: Demos de formulário interativas

**Feature Branch**: `013-demos-interativas` | **Created**: 2026-09-05 | **Status**: Draft
**Input**: "os formulários de teste em https://emersonjoe.github.io/trilha/ não são interativos"

## O defeito

O cartão de demonstração "Formulário com CSRF em uma linha" (página inicial e capítulo
[Formulários](https://emersonjoe.github.io/trilha/aprender/formularios)) renderiza um
formulário que **não reage a nada**. A causa está em `site/internal/demos/demos.go`:

```go
h.Form(h.Method("post"), h.Action("#"), h.Onclick("return false"), …)
```

O `onclick="return false"` foi posto no `<form>` para impedir um envio que quebraria a
página estática, mas ele cancela **todo clique dentro do formulário**: o botão "Publicar"
não faz nada, sem mensagem nenhuma. Confirmado no site publicado: digitar no campo funciona,
clicar no botão não produz efeito nem erro no console.

Há um segundo problema no mesmo trecho: um manipulador **inline** contradiz o que o próprio
site ensina no capítulo de segurança. A CSP padrão do Trilha (`script-src 'self'
'nonce-…'`, sem `unsafe-inline`) bloqueia manipuladores inline; no export estático não há
cabeçalho e por isso ele "funciona", mas o mesmo código rodando com `trilha dev` seria
barrado. O exemplo de orçamento tem um resíduo parecido: `h.Attr("onchange", "")`.

As demos do kit `ui` (capítulo Interface com ui) **já são interativas** no site publicado —
campos condicionais, abas e diálogo respondem. O problema é só o cartão do formulário.

## User Scenarios & Testing

### US1 - O formulário de demonstração responde (P1)
Preencher "Nome do evento" e clicar em "Publicar" mostra, ali mesmo, o que o servidor faria:
`POST /eventos/novo` com o token conferido, `303 See Other` e o `GET` da página criada, com
o *slug* derivado do que foi digitado. Enviar de novo com outro nome atualiza a resposta.
**Acceptance**: no navegador, com o site exportado, digitar "Encontro Go" e enviar mostra
`… → GET /eventos/encontro-go`; o campo vazio é barrado pela validação nativa (`required`).

### US2 - Nenhum manipulador inline no site nem nos exemplos (P1)
A interceptação vive em `site/public/tema.js` (arquivo externo, já carregado em todas as
páginas), não em atributo. O resíduo `onchange=""` do exemplo de orçamento sai.
**Acceptance**: teste do site que varre **todas** as páginas e falha se encontrar
`onclick=`, `onchange=`, `onsubmit=` ou `onload=` no HTML; `grep` nos exemplos idem.

### US3 - Continua útil sem JavaScript (P2)
Sem JavaScript, o botão não pode simular nada; o formulário então só recarrega a própria
página (`method="get"`, `action="#"`), sem erro 405 do GitHub Pages. Uma legenda diz que o
envio é simulado no navegador e que o `POST` real está no código ao lado.
**Acceptance**: o HTML exportado tem `method="get"` no formulário renderizado e a legenda;
o painel de código ao lado continua mostrando o `POST` verdadeiro.

## Requirements

- **FR-001** `demos.go`: sem `h.Onclick`; formulário marcado com `data-demo="form"` e uma
  saída `data-demo-saida` para a resposta simulada.
- **FR-002** `tema.js`: interceptar `submit` dos formulários de demo, sem tocar nas demos do
  kit (que são governadas por `ui.js`).
- **FR-003** Estilo da legenda/saída em `site.css`.
- **FR-004** Testes: sem manipuladores inline; a demo tem `data-demo`; a legenda existe.
- **FR-005** CHANGELOG e nova versão de correção.

## Fora do escopo

Tornar as demos de lista, escape e layout interativas (são saída renderizada, e é isso que
elas ensinam) e simular o servidor de verdade no site estático.
