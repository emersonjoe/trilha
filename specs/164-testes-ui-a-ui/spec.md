# Feature Specification: Testes UI a UI

**Feature Branch**: `164-testes-ui-a-ui` | **Created**: 2026-09-24 | **Status**: Entregue (0.145.0)
**Input**: Plano Tokens 70, spec 164 (`PLANO-TOKENS-70.md` §2) — duas camadas de teste de ponta a
ponta de UI: DOM servido (stdlib, no módulo, golden) e navegador (módulo próprio `uitest/`),
cobrindo as receitas e os padrões, para o agente provar que a tela funciona sem abrir um
navegador humano. Fecha o M3 junto com a 163.

## Contexto

`trilha.TestClient`/`TestResponse` já testam status e texto. Falta (a) um retrato do HTML
servido que seja estável entre execuções — nonce, token CSRF, request id, cookies e datas mudam
a cada requisição —, com asserções que dizem o que consertar, e (b) o que só o navegador vê:
foco depois de um 422, troca de fragmento, barra de upload, ilha montando, navegação no
cliente, tooltip no teclado. As rodadas de erro são ~25% do custo do agente (§1.2); uma
falha que diz "seletor, esperado, obtido, conserto" é uma rodada a menos.

### Decisões que vinculam a implementação

1. **Camada A no pacote raiz, só stdlib.** `PageSnapshot{Status, Header, Body}`,
   `CapturePage(t, app, method, path, opts...)` e `(*TestResponse).Snapshot()` (para páginas
   atrás de login, pelo `TestClient`). A normalização troca **os valores voláteis que a própria
   resposta revela** — o nonce do cabeçalho CSP, o token CSRF, o `X-Request-Id`, os valores de
   `Set-Cookie` — onde quer que apareçam, e datas (`Date`, RFC 3339, RFC 1123) por
   `{{NONCE}}`, `{{CSRF}}`, `{{RID}}`, `{{COOKIE}}`, `{{DATE}}`. Nada de regex sobre HTML
   genérico: o valor volátil é conhecido e é substituído literalmente.
2. **Asserções devolvem `error`** escrito para quem conserta: `HasCSRFToken()`,
   `HasAria(sel, attr)`, `FocusedOnError(sel)`, `HasCSPNonce()` (`E_CSP_NONCE`),
   `HasSafeCookies()`, `HasNoSecret(values...)`, e `MatchGolden(t, path, update)`. Os seletores
   são compostos simples (`tag`, `#id`, `.classe`, `[attr]`, `[attr=valor]`), o bastante para
   formulário e campo; nada de parser HTML de terceiros (princípio II).
3. **Camada B em `uitest/`, `go.mod` próprio** (precedente `otel/`), dependência `chromedp`
   v0.13.6 — a última que compila com `go 1.23`; a latest exige Go 1.26. O módulo raiz não ganha
   `require`.
4. **`Run` sobe o binário do app, não o `trilha dev`** (desvio do plano, registrado): o teste de
   UI prova o que vai para produção; o supervisor do dev acrescenta proxy, SSE de recarga e
   recompilação, que são ruído e tempo. `Run` compila uma vez por diretório (cache no
   processo), sobe numa porta livre, espera `/_trilha/health/live` e derruba no fim.
5. **Anti-flake por contrato**: sem screenshot; toda espera é por condição com timeout de 30 s;
   uma falha repete o cenário **uma vez** com navegador novo e log completo; a segunda falha
   grava `uitest/report/<cenário>.txt` (passo, seletor, esperado, obtido, conserto) e reprova.
6. **Fixture**: um projeto gerado no `TestMain` (`trilha new` + `trilha add login connections
   webhooks billing notify admin`) mais os seis padrões da 163 e três páginas de fluxo
   (`uitest/testdata/fixture/`): ilha, navegação no cliente, tooltip. Nada de app escrito à mão
   para imitar as receitas: é o que um usuário recebe.
7. **Sem Chrome, a camada B pula** (`t.Skip`), a menos que `UITEST_REQUIRED=1` — é assim que o
   job `ui` da CI fica opcional até o M3 e passa a bloqueante mudando uma variável.
8. **`E_CSP_NONCE`**: código novo no catálogo, erro de `HasCSPNonce` e regra do `trilha audit`
   (script ou style inline escrito com `h.Raw` numa página, que o nonce do kit não cobre).

## User Scenarios & Testing

### US1 - Retrato estável do HTML (P1)

`trilha.CapturePage(t, newApp(), "GET", "/")` duas vezes dá o mesmo texto; `MatchGolden`
compara com o arquivo e `-update` regrava.

### US2 - Asserções que dizem o conserto (P1)

`snap.HasCSRFToken()` falha nomeando o formulário e dizendo "adicione trilha.CSRFInput(c)";
`FocusedOnError("#email")` falha dizendo qual campo o kit vai focar.

### US3 - O navegador prova o fluxo (P1)

Os onze cenários do plano (`TestUILoginFlow` … `TestUIAdminDefaultDeny`) rodam contra o
fixture; uma falha deixa o relatório.

## Edge Cases

- Página sem CSP (CSP removida pela app): `HasCSPNonce` diz que não há nonce a conferir.
- Resposta 204/redirect: snapshot com corpo vazio, headers normalizados.
- Chrome ausente: pula com o motivo; `UITEST_REQUIRED=1` transforma em falha.

## Requirements

### Security and privacy impact (NIST SSDF 1.1 · OWASP ASVS 5.0 N2)

- **Fronteiras**: nenhuma nova no runtime — a camada A só lê a resposta que o teste recebeu;
  a camada B roda localmente, contra `127.0.0.1`, num módulo que nenhum app importa.
- **Controles verificados (evidência)**: V3.4 (cookies: `HasSafeCookies` exige `HttpOnly` e
  `SameSite`), V4 (CSRF: `HasCSRFToken` em todo POST de tela das receitas), V14/CSP (nonce em
  todo `<script>`/`<style>` inline: `HasCSPNonce`, `E_CSP_NONCE`, regra do audit), V13
  (segredos: `HasNoSecret` sobre o HTML da tela de conexões com o segredo do provedor salvo).
- **Cadeia de suprimentos** (SSDF PW.4, Top 10 A06): `chromedp` e dependências só em
  `uitest/go.sum`; o `TestNoExternalDeps` do módulo raiz continua valendo; versão fixada.
- **Segredos**: os de teste, literais nos testes. **Exceções**: nenhuma.

### Functional Requirements

- **FR-001**: `PageSnapshot`, `CapturePage`, `(*TestResponse).Snapshot`, as asserções e
  `MatchGolden` no pacote raiz, com superfície registrada.
- **FR-002**: `uitest.Run`/`RunWith` e `Session` com `Navigate`, `Click`, `Fill`, `Press`,
  `Upload`, `Text`, `Attr`, `Focused`, `WaitVisible`, `URL`, `Eval`.
- **FR-003**: relatório por cenário em `uitest/report/`.
- **FR-004**: `make test-ui` (B) e `make test-ui-a` (A, também no `make test`); job `ui` na CI.

## Success Criteria

- **SC-001**: `make test` verde com a camada A nos testes das receitas e dos padrões.
- **SC-002**: `make test-ui` verde localmente (Chrome instalado) com os onze cenários.

## Registro da implementação

### Desvios e decisões tomadas no caminho

- **Ações pelo `querySelector` do momento**, não pelo nó do chromedp: depois de um swap, o
  chromedp clicava num nó que já tinha saído da página (o clique caía no `<body>`). `Click`
  confere com `elementFromPoint` que nada cobre o ponto e clica com o mouse; `Fill`, `Focus` e
  `Upload` acham o elemento de novo a cada ação.
- **Foco emulado e movimento reduzido** no navegador de teste: sem foco emulado, uma aba
  headless não dispara `focusin` (a dica nunca abria); com animação, o overlay da view
  transition não terminava e cobria a página. Os cenários conferem onde a página termina.
- **O navegador anônimo em `/admin` vai para o login** (`/entrar?next=%2Fadmin`), não recebe
  401 — o `TestClient` recebe 401 porque não pede HTML. O cenário segue o navegador.
- **O 403 do backoffice sai sem layout** (sem `<main>`): registrado, fora do escopo desta spec.
- **Um 422 de formulário sem `ui.Swap`** (plano da cobrança) recarrega a página inteira, e o kit
  não põe o foco no campo numa carga completa — o `ui.js` está a 3 bytes do orçamento de 29 KB.
  O cenário confere a marca `aria-invalid`, não o foco. Fica como sugestão de issue.
- **O relatório** vai para `report/` relativo ao diretório do teste (`Config.ReportDir`), que no
  repositório é `uitest/report/` (ignorado pelo git).

### Defeitos que os cenários acharam (corrigidos na 0.145.0)

1. `applySwap` perdia o foco quando o controle focado não tinha id nem name (Enter no "Next"
   da paginação deixava o foco no `<body>`) — `TestUIPaginationKeyboard`.
2. `ui.upload.js` trocava o 204 com `Trilha-Location` de um envio aceito como corpo vazio,
   apagando o formulário — `TestUIUploadProgress`.
3. `ui.InvalidIf`/`ui.Errors` não viam `files[i]`, o nome que `c.Files` dá ao arquivo recusado:
   o padrão de envio da 163 não marcava o campo — `TestUIUploadProgress`.
4. O formulário de planos da cobrança e o de preferências do notify não marcavam o campo que
   falhou (`ui.InvalidIf`), então um 422 não tinha onde pôr o foco — `FocusedOnError` nos
   testes gerados.

A prova de que os cenários pegam os defeitos: com o `ui.js`, o `ui.upload.js` e o `ui/ui.go`
anteriores, `TestUIUploadProgress` e `TestUIPaginationKeyboard` falham com o relatório
(`aria-invalid` ausente; foco no `<body>`).

### Evidências

- `make test` verde (camada A dentro: `snapshot_test.go`, `TestPaginasContraOGolden` do blog,
  goldens dos padrões, testes gerados das receitas, `TestAuditoriaApontaScriptSemNonce`).
- `UITEST_REQUIRED=1 make test-ui` verde localmente, 16 testes em ~22 s (Chrome 154, macOS).
- `TestNoExternalDeps` verde: `chromedp` só em `uitest/go.mod`; `TestPacotesPublicosNaLista`
  trata `uitest/` como módulo próprio, como `otel/`.
- Job `ui` na CI: `continue-on-error: true` até o M3 fechar; a spec que fechar o M3 remove a
  linha e o job passa a bloquear.
