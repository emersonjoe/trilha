# Implementation Plan: WebSocket (adaptador) e upload com progresso

**Branch**: `024-websocket-upload` | **Date**: 2026-09-05 | **Spec**: [spec.md](./spec.md)
**Versão**: 0.15.0

## Contexto técnico

**Linguagem**: Go 1.24, só biblioteca padrão. **Superfície tocada**: `ctx.go` (`Hijack`,
`AllowBody`, `NoReadDeadline`, `responseWriter`), `serve.go` (corpo original, status 101 no
log), `ui/` (três símbolos + `assets/ui.upload.js`), `internal/scaffold` (nada: o filtro já é
por sufixo desde a 0.14.0), `examples/blog` (rota de anexos + `internal/ws`), site nas duas
locales.

Três fatos que decidem a implementação:

1. `http.NewResponseController(w).Hijack()` segue a cadeia de `Unwrap`, e o
   `responseWriter` já tem `Unwrap`. Então o `Hijack` do writer é uma delegação de três
   linhas — o que faltava era existir, porque biblioteca de WebSocket faz asserção de tipo
   (`w.(http.Hijacker)`), não usa o controller.
2. A conexão sequestrada **herda os prazos** que o servidor pôs para ler a requisição
   (`ReadTimeout`), então `Hijack` tem de limpá-los ou a conexão morre em 30 s.
3. `MaxBytesReader` embrulha o corpo em `wrap`; para trocar o limite depois é preciso
   guardar o corpo original antes de embrulhar.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| I — roteamento por arquivos | nada muda no `app/`; upload e WebSocket são handlers comuns |
| II — só biblioteca padrão | nenhuma dependência nova; a decisão da spec é justamente **não** trazer transporte para o núcleo |
| III — coerente com Go | `http.Hijacker` é a interface da própria stdlib; `AllowBody` usa `http.MaxBytesReader` |
| IV — aprimoramento progressivo | o formulário de upload envia sem JavaScript e a rota responde a página inteira |
| VI — teste primeiro | handshake WebSocket falado na mão no teste; limite de corpo verificado nos dois sentidos |
| VII — segurança por padrão | o limite continua 1 MiB para todo mundo: `AllowBody` é por rota e explícito. Sequestrar a conexão não desliga CSRF nem autenticação, que rodam antes |

## Fases

**Fase 1 — runtime.** `responseWriter.Hijack`, marca de sequestro, `Ctx.Hijack`,
`Ctx.AllowBody`, `Ctx.NoReadDeadline`. Teste primeiro, incluindo o handshake completo.

**Fase 2 — kit.** `ui/assets/ui.upload.js`, `UploadTo`, `UploadBar`, `UploadScript`,
`ui.Files`. Teto de 4 KiB no teste, e a garantia de que o `ui.js` não fala de upload.

**Fase 3 — exemplo.** `examples/blog/internal/anexos` (memória), rota `app/anexos/route.go`
com `AllowBody`, formulário na página do blog, `internal/ws` com o eco mínimo e a rota `/ws`.
Teste de integração dos dois.

**Fase 4 — documentação e fechamento.** `learn/interactivity` (upload) e uma seção de
WebSocket com a decisão na referência; `reference/ctx` e `reference/ui`; CHANGELOG, versão,
ROADMAP; release.

## Riscos

- **Sequestro e logging.** Depois do `Hijack` o `wrap` continua rodando (log, métrica). Se
  algo tentar escrever no writer, o `net/http` reclama. Mitigação: a marca de sequestro faz
  `WriteHeader`/`Write` virarem no-op e o log registrar 101.
- **Dois handlers de `submit`.** O `ui.js` já escuta `form[data-trilha-target]`. Por isso o
  upload usa `data-trilha-upload`, atributo diferente: nada muda no `ui.js` e não há corrida.
- **Tamanho do exemplo de WebSocket.** Risco de virar meia biblioteca. Teto: só texto, uma
  mensagem por vez, sem fragmentação, com o aviso no topo do arquivo.
