# Tasks: WebSocket (adaptador) e upload com progresso

**Input**: [spec.md](./spec.md), [plan.md](./plan.md) · uma rodada de `make test` por bloco.

## Bloco 1 — runtime (FR-001…FR-005)

- [x] T001 Teste que falha em `hijack_test.go`: handshake WebSocket na mão (chave, `Sec-WebSocket-Accept`, frame de texto mascarado) contra uma rota que chama `c.Hijack()`; prova também que o log não vira 500.
- [x] T002 Teste que falha em `ctx_test.go`: 2 MiB sem `AllowBody` → 413; com `c.AllowBody(4<<20)` → 200 e o handler lê o corpo inteiro; `NoReadDeadline` não devolve erro.
- [x] T003 `responseWriter.Hijack` + marca de sequestro (`Write`/`WriteHeader` no-op, status 101 no log) em `ctx.go`/`serve.go`.
- [x] T004 `Ctx.Hijack`, `Ctx.AllowBody`, `Ctx.NoReadDeadline`; corpo original guardado em `wrap`.

## Bloco 2 — kit (FR-006…FR-010)

- [x] T005 Teste que falha em `ui/ui_test.go`: `UploadTo`/`UploadBar` rendem os atributos, `UploadScript` respeita `BasePath` e hash, `ui.upload.js` está em `Files`, cabe em 4 KiB e o `ui.js` não fala de upload.
- [x] T006 `ui/assets/ui.upload.js` (XHR, `Trilha-Fragment`, `window.ui.swap`, `<progress>`, evento `trilha:upload`, recuo para envio normal) e os três símbolos em `ui/ui.go`.
- [x] T007 Ajustar `TestWriteUIStamp` para cinco arquivos.

## Bloco 3 — exemplo (SC-003)

- [x] T008 Teste que falha em `examples/blog/blog_test.go`: anexo de 2 MiB aceito na rota, fragmento da lista na resposta; sem o cabeçalho, página inteira.
- [x] T009 `internal/anexos` (memória), `app/anexos/page.go` e `app/anexos/middleware.go`; formulário com `ui.UploadTo`/`ui.UploadBar`/`ui.UploadScript`. **O limite subiu no `middleware.go`, não no `POST`**: o CSRF de formulário lê o corpo antes do handler, então a decisão tem de vir antes dele — e `middleware.go` já é o lugar onde uma rota decide o que vale nela.
- [x] ~~T010 `internal/ws` (eco mínimo, só texto) e a rota que o usa.~~ **Cortada.** O handshake e o frame já são exercidos ponta a ponta em `hijack_test.go`, contra um socket de verdade. Repetir aquilo dentro do `examples/blog` põe ali um meio-WebSocket — sem fragmentação, sem frame de controle, sem fechamento — que é justamente o que a spec argumenta que ninguém deve escrever à mão. O exemplo continua sendo modelo de cópia; o adaptador com biblioteca fica na documentação.
- [x] T011 `trilha gen` do exemplo (arquivo gerado é commitado).

## Bloco 4 — documentação e fechamento

- [ ] T012 `learn/interactivity` + `pt/aprender/interatividade`: upload com progresso.
- [ ] T013 `reference/ctx` + `pt/referencia/ctx`: `Hijack`, `AllowBody`, `NoReadDeadline` e a decisão sobre WebSocket; `reference/ui` + `pt/referencia/ui`: os três símbolos.
- [ ] T014 `CHANGELOG.md` (0.15.0), `version` em `cmd/trilha/main.go`, ROADMAP (Fase 1, item 5).
- [ ] T015 `make test` verde e `make release VERSION=0.15.0 ISSUES="24"`.
