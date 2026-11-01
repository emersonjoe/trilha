# Spec 185 — O cookie de CSRF com prefixo __Host-

**Feature Branch**: `185-csrf-host-prefix` | **Created**: 2026-11-01 | **Status**: Entregue (0.164.0)

- **Issue**: #317 — a fonte do escopo.
- **Branch**: `185-csrf-host-prefix`
- **Versão**: 0.164.0

## Por quê

O CSRF do Trilha é *double-submit*: o cookie `trilha_csrf` guarda o segredo, e o formulário ou o
cabeçalho repete esse valor. Sem o prefixo `__Host-`, o navegador aceita um `trilha_csrf`
gravado por um subdomínio irmão (`Domain=.exemplo.com`) ou por uma resposta HTTP em claro
antes de o HSTS valer. E o `csrfSecret` adotava qualquer valor com 32 caracteres ou mais. Quem
controla um `preview.exemplo.com` planta um valor que ele conhece e monta um formulário que o
app aceita. O `SameSite=Lax` não cobre esse caso, porque um subdomínio é *same-site*.

## Jornada e risco

Jornada: quem usa o app envia um formulário (login, cadastro, qualquer POST de página).
Pior impacto se o ataque funciona: uma ação feita em nome da vítima. Pior impacto se a mudança
erra: todo formulário em HTTPS passa a responder 403. As duas pontas são cobertas pelos testes
abaixo e pelo cenário de navegador que já envia formulários.

## O que muda

- Com HTTPS (TLS direto, ou `X-Forwarded-Proto: https` de um proxy confiável), o cookie padrão
  se chama `__Host-trilha_csrf` (nova constante `trilha.CSRFHostCookie`) e sai com `Secure`,
  `Path=/` e sem `Domain`. Assim o navegador recusa o mesmo nome vindo de subdomínio ou de HTTP.
- Em HTTP (desenvolvimento), o nome continua `trilha_csrf`: o navegador não grava cookie
  `__Host-` sem `Secure`.
- Um nome definido em `Config.CSRF.Cookie` continua como está. O app que quiser o prefixo
  escreve `__Host-` no nome.
- O valor do `trilha_csrf` antigo nunca vira o segredo de uma página servida em HTTPS: a
  primeira página depois do deploy emite um `__Host-trilha_csrf` novo.
- **Janela de transição, só nesta versão menor:** em HTTPS, uma requisição sem
  `__Host-trilha_csrf` ainda é conferida contra o `trilha_csrf`, para que o formulário aberto
  antes do deploy não dê 403. Basta uma página carregada depois do deploy para o cookie
  com prefixo passar a valer, e o antigo deixa de ser lido. A leitura do nome antigo sai na
  0.165.0.
- `TestClient` e `PageSnapshot` reconhecem o nome com prefixo quando o alvo é `https://`.

## Fora de escopo

- Amarrar o token à sessão (o *signed double-submit* com HMAC do id de sessão): pede uma
  sessão em toda página com formulário, o que é outra decisão.
- Renomear cookies de app que já escolheram o próprio nome.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `net/http` apenas. |
| VI — teste primeiro | Os testes de nome com prefixo, cookie plantado e transição reprovam antes da mudança. |
| VII — segurança por padrão | O padrão passa a ser o cookie que o navegador protege; nada a configurar. |

## Segurança e privacidade

- Ativos e fronteiras de confiança afetados: o segredo de CSRF no cookie; a fronteira entre o
  host do app e os subdomínios irmãos (e a rede antes do HSTS).
- Controles OWASP ASVS 5.0 nível 2 aplicáveis: V3.3 (atributos e prefixo `__Host-` de
  cookies) e V3.5 (defesa contra CSRF).
- Riscos OWASP Top 10:2025 considerados: A01 — controle de acesso quebrado (CSRF).
- Segredos, dados pessoais, logs, limites e exceções: o valor do cookie não vai para log. Exceção
  registrada: durante a janela de transição, quem ainda não carregou uma página depois do
  deploy segue exposto como antes.

## Tarefas

- [x] T001 Testes que falham: nome com prefixo em HTTPS com `Secure` e sem `Domain`; nome
  antigo em HTTP; cookie `trilha_csrf` plantado não vira segredo em HTTPS; com os dois
  cookies vale o prefixado; formulário anterior ao deploy passa na transição; nome próprio
  do app intacto em HTTPS; `TestClient` em `https://`
- [x] T002 Implementação em `csrf.go`, `testing.go` e `snapshot.go`
- [x] T003 `examples/blog` já cobre o fluxo (snapshot de login em HTTP, sem mudança)
- [x] T004 Documentação nas duas locales (formulários, referência do app) e `api/current.txt`
- [x] T005 `CHANGELOG.md`, `version` em `cmd/trilha/main.go`, item do `ROADMAP.md`
- [x] T006 Suíte inteira verde e os números em *Evidências*

## Aceitação

- **SC-001** Em HTTPS, a primeira página grava `__Host-trilha_csrf` com `Secure; Path=/` e
  sem `Domain`.
- **SC-002** Um `trilha_csrf` plantado por subdomínio não é adotado: a página em HTTPS emite
  um segredo novo, e o valor plantado não passa na checagem quando o cookie com prefixo existe.
- **SC-003** Um formulário aberto antes do deploy (só com `trilha_csrf`) ainda é aceito nesta
  versão.
- **SC-004** Em HTTP de desenvolvimento nada muda.

## Evidências

| Nível | Comando | Passaram | Falharam | Pularam |
|---|---|---|---|---|
| Unidade + integração | `make test` | 1649 | 0 | 2 |
| Navegador | `make test-ui` (local) | 0 | 0 | todos — Chromium do Playwright não instalado nesta máquina |
| Navegador | job `ui` do CI (Chromium, Firefox, WebKit, `UITEST_REQUIRED=1`) | ver PR | | |

Os dois pulados da suíte são `TestS3AoVivo` e `TestOIDCAoVivo`, que dependem de serviço externo.

Instabilidade registrada: numa de cinco rodadas de `go test ./...`, duas falhas apareceram e
não se repetiram nas quatro seguintes. A rodada não guardou os nomes. Ela coincidiu com um
erro transitório da instalação local do Go (`package encoding/json is not in std`), então a
causa provável é o ambiente, não um teste. Dono: quem fechar a 0.164.0 confere o job `test`
do CI.

Sem teste e por quê: nenhum cenário de navegador passa por HTTPS. O `uitest/` serve o app em
`http://127.0.0.1`, onde o nome continua `trilha_csrf` e o fluxo de formulário não muda.
Que o navegador recusa um `__Host-` vindo de subdomínio é garantia do navegador. O lado do
Trilha (nome, atributos, não adotar o valor plantado, transição) está coberto no nível de
integração (`csrf_test.go`). Um cenário em HTTPS pede TLS no harness, o que fica como
proposta à parte.
