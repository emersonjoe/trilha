# Spec 026 — Cache HTTP: ETag, Last-Modified e 304

- **Issue**: [#26](https://github.com/emersonjoe/trilha/issues/26) — a issue é a fonte do
  escopo.
- **Branch**: `026-cache-http`
- **Versão**: 0.17.0

## Por quê

A spec 025 fez o servidor parar de refazer trabalho. Esta faz o servidor parar de
**mandar** de novo o que já está no navegador. São coisas diferentes: um cache de
aplicação com 100 % de acerto ainda serializa a página inteira e empurra 40 kB pela rede
a cada F5.

Hoje quem quer isso escreve à mão: monta o cabeçalho, lê o `If-None-Match`, compara,
lembra que a comparação de `ETag` tem aspas e vírgula e `*`, lembra que o `304` não pode
levar corpo, e descobre semanas depois que a página autenticada de um usuário foi parar
no cache de um proxy. É o tipo de código que o framework deve trazer pronto — cinco
linhas de RFC que ninguém deveria reler.

O arquivo estático já responde `304` por `If-Modified-Since`, porque `http.ServeFileFS`
faz isso sozinho. Falta o `ETag`: com data de modificação só, um deploy que reescreve o
arquivo com o mesmo conteúdo (é o que um `git clone` num container novo faz) invalida
tudo à toa.

## O que muda

Três métodos no `Ctx`. Nenhuma configuração nova, nenhuma convenção nova em `app/`.

```go
func (c *Ctx) ETag(tag string) bool
func (c *Ctx) LastModified(t time.Time) bool
func (c *Ctx) CacheControl(v string)
```

`ETag` declara **a versão do que a rota vai responder** e devolve `true` quando o
navegador já a tem — nesse caso o `304` já foi escrito e o handler não deve responder
mais nada:

```go
func Page(c *trilha.Ctx) (h.Node, error) {
	p, ok := posts.Get(c.Param("slug"))
	if !ok {
		return nil, trilha.ErrNotFound
	}
	if c.ETag(p.Rev) {
		return nil, nil // o navegador já tem esta versão
	}
	return pagina(p), nil
}
```

O que o framework faz por dentro, e que é justamente o que ninguém quer reescrever:

| Detalhe | Comportamento |
|---|---|
| Aspas | `c.ETag("abc")` manda `ETag: "abc"`; um valor que já venha `"abc"` ou `W/"abc"` passa como está |
| `If-None-Match` | lista separada por vírgula, com `*` casando com qualquer coisa; comparação fraca (RFC 9110 §8.8.3.2) |
| `Vary` | `Trilha-Fragment` entra no `304` também: um pedaço da página nunca é servido no lugar dela |
| Método | só `GET` e `HEAD` respondem `304`; nos outros o cabeçalho é escrito e o retorno é `false` |
| Vazio | `c.ETag("")` não escreve cabeçalho nenhum e devolve `false` |

`LastModified` é o mesmo contrato com data: escreve `Last-Modified`, compara com
`If-Modified-Since` na resolução de segundo do HTTP, e **não decide nada quando a
requisição trouxe `If-None-Match`** — validador forte ganha do fraco, como manda a RFC.
Tempo zero não escreve cabeçalho.

`CacheControl` é açúcar por simetria (`c.Header("Cache-Control", v)` faz o mesmo), e
existe para ter um lugar onde a documentação diga a única frase que importa: página com
dado de usuário pede `private`, ou o `ETag` vira o jeito de um proxy compartilhado
entregar a página de um para outro.

Nos estáticos, `serveStatic` passa a mandar o `ETag` com a impressão digital que a app já
calcula para o `?v=` — `http.ServeFileFS` cuida da comparação e do `304` a partir daí.

## Fora de escopo

- **`ETag` automático do corpo da página.** Tentador e errado: a CSP põe um *nonce* novo
  em cada resposta, então o corpo muda a cada requisição e a etiqueta nunca casaria. Quem
  sabe a versão do dado é o handler.
- **`If-Match`, `If-Unmodified-Since` e `412`.** É escrita condicional, outra história
  (perde-atualização em `PUT`), e sem demanda.
- **`Cache-Control` padrão para páginas.** O framework não vai adivinhar se a sua página é
  pública.
- **Invalidação em CDN.** Depende do fornecedor.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `net/http`, `strings`, `time` |
| III — coerente com Go | `http.ParseTime`/`TimeFormat`, e o `304` do estático continua sendo o do `http.ServeContent` |
| VI — teste primeiro | teste vermelho por bloco, incluindo o `trilha export` |
| VII — segurança por padrão | `Vary` no `304`; `Cache-Control` continua sem padrão para página, e a documentação diz por que `private` é assunto de quem escreve |

## Tarefas

- [x] T001 Teste que falha em `httpcache_test.go`: `304` com `If-None-Match` (valor único,
      lista e `*`), `200` com etiqueta diferente, aspas adicionadas uma vez só, `Vary` no
      `304`, `POST` não vira `304`, `ETag("")` não escreve nada; `LastModified` responde
      `304` por `If-Modified-Since` e se cala quando há `If-None-Match`.
- [x] T002 `httpcache.go`: `Ctx.ETag`, `Ctx.LastModified`, `Ctx.CacheControl`.
- [x] T003 Teste que falha em `static_test.go`: estático responde `ETag` e devolve `304`
      para quem o repete; e teste em `export_test.go` provando que o `trilha export`
      continua escrevendo o arquivo.
- [x] T004 `ETag` em `serveStatic` a partir de `assetVersion`.
- [x] T005 Teste que falha em `examples/blog/blog_test.go` + uso na página do post.
- [x] T006 Documentação nas duas locales: seção nova em `learn/data` / `aprender/dados` e
      os três métodos em `reference/ctx` / `referencia/ctx`.
- [x] T007 `CHANGELOG.md` (0.17.0), `version` em `cmd/trilha/main.go`, ROADMAP (Fase 2,
      item 7).
- [ ] T008 `make test` verde e `make release VERSION=0.17.0 ISSUES="26"`.

## Aceitação

- **SC-001** `GET` com `If-None-Match` igual à etiqueta responde `304` sem corpo e com o
  `ETag` repetido; com etiqueta diferente responde `200` com o corpo inteiro.
- **SC-002** `If-Modified-Since` posterior à data declarada responde `304`; anterior
  responde `200`. Havendo `If-None-Match`, quem decide é ele.
- **SC-003** Um arquivo em `public/` responde `ETag` estável entre reinícios e `304` na
  segunda requisição.
- **SC-004** `trilha export` continua gerando os mesmos arquivos (teste existente verde).
- **SC-005** A página do post no `examples/blog` responde `304` para quem já a tem, e
  volta a `200` depois que o post muda.
