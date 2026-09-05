# Feature Specification: Segurança por padrão (NIST CSF 2.0 · OWASP ASVS L2)

**Feature Branch**: `004-seguranca` | **Created**: 2026-09-05 | **Status**: Draft
**Input**: "adicionar segurança NIST e OWASP e configurações nesse sentido"

## Referências normativas

- **NIST CSF 2.0** (funções Govern, Identify, Protect, Detect, Respond, Recover). Um framework
  web só consegue implementar controles de *Protect* e *Detect* e facilitar *Respond*; o
  restante é documentado como responsabilidade de quem opera o app.
- **OWASP ASVS 4.0 nível 2**, capítulos V2 (autenticação: sessão/cookies), V3 (sessão), V4
  (controle de acesso: CSRF), V5 (validação/escape), V7 (log e erros), V8 (dados em
  trânsito: HSTS), V11 (lógica: rate limit), V12 (arquivos), V13 (API), V14 (configuração:
  cabeçalhos, CSP).
- **OWASP Top 10 2021**: A01 (acesso), A02 (criptografia), A03 (injeção), A05
  (configuração), A07 (autenticação), A09 (log).

## User Scenarios & Testing

### US1 - Cabeçalhos endurecidos por padrão, ajustáveis por config (P1)

Toda resposta sai com `Content-Security-Policy` (com nonce por requisição para scripts
inline, incluindo o de live-reload), `Strict-Transport-Security` quando a conexão é HTTPS
(direta ou via proxy confiável), `Permissions-Policy` restritiva, `Cross-Origin-Opener-Policy`
e os já existentes. O desenvolvedor ajusta ou desliga cada um em `Config.Security`.

**Acceptance**: (1) `GET /` responde com CSP `default-src 'self'; script-src 'self'
'nonce-…'; …` e o nonce da resposta é o mesmo do `<script nonce>` injetado em dev; (2) sem
TLS e sem proxy confiável não há HSTS; com `X-Forwarded-Proto: https` de um proxy confiável,
há; (3) `Security.CSP = ""` remove o cabeçalho; (4) `c.Nonce()` devolve o nonce e
`h.Script(h.Nonce(c), …)` o coloca no atributo.

### US2 - Limite de taxa por cliente (P1)

Um `Config.RateLimit{RPS, Burst}` ativa um limitador por IP (token bucket em memória) que
responde 429 com `Retry-After`. O IP vem de `RemoteAddr`, ou de `X-Forwarded-For` quando a
requisição chega de um `TrustedProxies`. Rotas podem ter limite próprio via `trilha.Limit`.

**Acceptance**: 10 req/s com burst 5: a 6ª requisição imediata responde 429; após 1 s volta
a 200; IPs diferentes têm baldes diferentes; `X-Forwarded-For` de IP não confiável é ignorado.

### US3 - Cookies assinados e sessão mínima (P1)

`c.SetSigned(nome, valor, ttl)` grava um cookie `HttpOnly; Secure (quando https); SameSite=Lax`
com HMAC-SHA256 e expiração embutida; `c.Signed(nome)` devolve o valor só se a assinatura e
o prazo conferem. A chave vem de `TRILHA_SECRET` (≥ 32 bytes, base64 ou texto). Em `prod`
sem `TRILHA_SECRET`, o app sobe com aviso e `SetSigned` devolve `ErrNoSecret` (apps sem
cookies assinados não podem ser derrubados por isso); `trilha audit` marca como crítico. Em
`dev` gera uma chave efêmera (uma por sessão do `trilha dev`) e avisa.

**Acceptance**: valor alterado, assinatura de outra chave ou cookie vencido → `("", false)`;
o exemplo de login usa `SetSigned("sessao", usuario, 8h)` e o middleware `Signed`.

### US4 - Detecção: eventos de segurança no log (P2)

Falha de CSRF, 401/403, 413, 429 e panics são registrados como `slog` com
`event=security`, `kind=csrf|auth|body|rate|panic`, `ip`, `path`, `request_id`, sem corpo
nem cookies. `Config.OnSecurityEvent` permite reagir (alertar, contar).

### US5 - `trilha audit` (P2)

O comando verifica a configuração do projeto e imprime um relatório: `TRILHA_SECRET` no
ambiente, CSP presente, versão do Go com suporte, `go vet`, `govulncheck` (se disponível via
`go run golang.org/x/vuln/cmd/govulncheck@latest`, fora do runtime) e dependências
externas no `go.mod`. Sai com 1 se houver item crítico.

### US6 - Documentação e mapeamento (P1)

Capítulo "Segurança" em Aprender e página "Segurança" em Referência com: tabela controle →
CSF 2.0 → ASVS → como ligar/desligar; checklist de implantação (TLS, segredos, proxies,
backups, resposta a incidente); o que o framework **não** faz (autenticação de usuários,
autorização de negócio, criptografia de dados em repouso).

### Edge Cases
- CSP e páginas que carregam fontes/CSS externos: `Security.CSP` aceita string completa;
  `Security.CSPExtra` acrescenta origens sem reescrever a política padrão.
- App atrás de proxy sem `TrustedProxies`: rate limit por IP do proxy (documentado) e HSTS
  desligado.
- Limitador em memória e várias réplicas: limite por réplica (documentado).
- `TRILHA_SECRET` rotacionado: cookies antigos deixam de validar (documentado; suporte a
  chave anterior via `TRILHA_SECRET_PREVIOUS`).

## Requirements
- **FR-001** `Config.Security{CSP, CSPExtra, HSTS, PermissionsPolicy, COOP, FrameOptions, Referrer}` com padrões endurecidos; `trilha.Off` em um campo remove o cabeçalho.
- **FR-002** Nonce por requisição (`c.Nonce()`, `h.Nonce(c)`), usado no script de dev.
- **FR-003** `Config.TrustedProxies []string` (CIDR): só então `X-Forwarded-For/Proto` valem; `c.ClientIP()`.
- **FR-004** `Config.RateLimit{RPS float64, Burst int}` global e `trilha.Limit(rps, burst) MiddlewareFunc` por subárvore; 429 + `Retry-After`; limpeza de baldes ociosos.
- **FR-005** `c.SetSigned/Signed`, `trilha.Signer`, `TRILHA_SECRET` e `TRILHA_SECRET_PREVIOUS`; recusa em prod sem segredo.
- **FR-006** Servidor com `ReadTimeout`, `WriteTimeout`, `IdleTimeout`, `MaxHeaderBytes` configuráveis com padrões.
- **FR-007** Eventos de segurança em `slog` + `Config.OnSecurityEvent(Event)`.
- **FR-008** `trilha audit` com saída legível e código de saída.
- **FR-009** Documentação (capítulo + referência + SECURITY.md atualizado) e CI com `govulncheck`.

## Success Criteria
- **SC-001** Cabeçalhos verificados por teste em dev e prod, com e sem proxy.
- **SC-002** Rate limit, cookies assinados e eventos cobertos por testes unitários e no exemplo.
- **SC-003** `examples/blog` migra o login para cookie assinado; `trilha audit` passa no exemplo.
- **SC-004** Zero dependências mantido; `make test` verde.
