# Tasks: Testes UI a UI

- [x] **T01** — `PageSnapshot` + normalização + `CapturePage` (+ `TestResponse.Snapshot`,
  `MatchGolden`) no pacote raiz; superfície registrada. *Aceite:* `make api` + golden do blog
  estável entre execuções (nonce muda, golden não).
- [x] **T02** — Asserções da camada A (`HasCSRFToken`, `HasAria`, `FocusedOnError`,
  `HasSafeCookies`, `HasNoSecret`) + testes. *Aceite:* usados nos testes das receitas da spec
  162 (billing, notify, admin) e nos goldens dos padrões.
- [x] **T03** — Módulo `uitest/` com `go.mod` próprio + `Session`. *Aceite:* `TestUILoginFlow`
  verde local (e no job `ui` da CI).
- [x] **T04** — Cenários de fluxo (form 422, swap, upload, ilha, navegação, tooltip,
  paginação). *Aceite:* os sete `TestUI*` correspondentes.
- [x] **T05** — Cenários por receita (billing, notify, admin) incluindo `TestUIAdminDefaultDeny`.
  *Aceite:* os três `TestUI<Receita>*`.
- [x] **T06** — Cenários de segurança (CSP nonce, CSRF por formulário, sem segredo no HTML).
  *Aceite:* testes + regra `E_CSP_NONCE` no `trilha audit` (+ `HasCSPNonce`, catálogo).
- [ ] **T07** — Makefile `test-ui`/`test-ui-a` + CI (job `ui` opcional; vira bloqueante ao fechar
  o M3 trocando `continue-on-error`, registrado aqui). *Aceite:* workflow válido + suíte verde.
- [ ] **T08** — Relatório de falha legível por agente (seletor + esperado + obtido + conserto).
  *Aceite:* fixture de falha produz o artefato em `uitest/report/`.
- [ ] **T09** — Docs bilíngues (referência de testes, camada A e B) + CHANGELOG + ROADMAP.
  *Aceite:* testes de conteúdo do site + `make test` verde.
