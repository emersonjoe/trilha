# Spec 182 — Revisão de segurança de outubro: os incidentes de PHP e Node.js no runtime

**Feature Branch**: `182-seguranca-outubro` | **Created**: 2026-10-25 | **Status**: Entregue (0.162.0)

- **Issue**: #313 (o `c.Asset` com `Mounts`) — a fonte do escopo dessa parte. O resto vem de
  uma revisão de segurança do runtime guiada pelos incidentes de outubro de 2026, descritos
  abaixo; os achados que pedem mudança de contrato ficaram fora (ver *Fora de escopo*).
- **Branch**: `182-seguranca-outubro`
- **Versão**: 0.162.0

## Por quê

Em 21/10/2026 saíram duas releases de segurança que interessam a quem escreve framework web:

- **PHP** (CVE-2026-109618 e 109619, 8.2.35/8.3.36/8.4.27/8.5.12): o parse de upload multipart
  com tamanhos escolhidos pelo atacante leva o interpretador à falta de memória, e estado de
  uma requisição sobrevive para a próxima no mesmo worker.
- **Node.js** (22.23.4/24.22.1/26.12.1, dez CVEs): guardas de SSRF e de TLS enganadas porque o
  endereço que a guarda leu não é o que foi discado — NUL que trunca o nome (105482), IPv4 em
  octal, hexa ou abreviado que a guarda não reconhece e o resolvedor do sistema liga
  (106069, 106072) —, e o decodificador Brotli escrevendo fora do buffer com entrada maliciosa.
- **npm**: a onda de worms (ChainDrop, Miasma/`binding.gyp`, IronWorm) que publica versões com
  token roubado e forja proveniência.

O Trilha é Go sem dependências, então nenhum desses CVEs o atinge diretamente; a pergunta é se
a **classe** de cada um existe no runtime. A revisão achou seis lugares onde existe, e a issue
#313 estava aberta.

## Jornada e risco

- **Upload de arquivo** (quem usa o app): um arquivo acima de `MaxFormMemory` ficava no disco
  para sempre — disco cheio derruba o app inteiro, e o arquivo do usuário sobra no servidor.
- **Webhook e Connections** (quem configura integrações): um endereço que aponta para dentro
  transforma o servidor em quem lê o metadata da nuvem e manda para fora.
- **Login com `?next=`** (quem usa o app): redirect para outro domínio é phishing com o domínio
  do app.
- **Limite de taxa e auditoria** (quem opera): IP escolhido pelo cliente anula o limite de
  tentativas de login e falsifica a trilha.
- **Exportar planilha** (quem usa o app): fórmula digitada por um visitante roda no Excel de
  quem exporta.
- **Estáticos por `Mounts`** (#313, quem escreve o app): todo deploy invalida o cache de todos
  os scripts.

## O que muda

- Os temporários de um formulário multipart são apagados quando a requisição termina.
- O cliente padrão do `webhook` confere o IP na discagem (`net.Dialer.Control`) com a mesma
  regra do cadastro; atrás de `HTTPS_PROXY` fica só a checagem por nome. Um `Options.HTTP`
  próprio não ganha a checagem na discagem.
- `ValidateExternalURL` trata como privado o host cujo último rótulo é número (decimal, octal ou
  `0x`), IPv6 com IPv4 mapeado e `100.64.0.0/10`.
- `localPath` (de `Redirect`, `ReplaceURL`, `PushURL`) recusa byte de controle em qualquer
  posição.
- `ClientIP` junta todas as linhas de `X-Forwarded-For` antes de procurar a entrada não
  confiável mais à direita.
- O `Config.RateLimit` roda antes do upstream (429 em `problem+json`).
- `c.CSV` põe apóstrofo na célula de texto que começa com `= + - @`, tab ou CR; `BindCSV` tira.
- `X-Request-ID` do cliente só é aceito com até 128 caracteres de `A-Za-z0-9._:-`.
- `c.Asset` procura em `Mounts` também; sem `Mounts` nem `Public` devolve o caminho calado.

## Fora de escopo

- **Encaminhar `Cookie` no upstream**: documentado como intencional (uma API com sessão
  própria precisa dele) e o `Headers` já remove; fica como está.
- **Achados que mudam contrato** (formato de valor assinado, nome do cookie de CSRF, endereço
  padrão do `trilha dev`): pedem janela de transição ou mudam o comportamento de quem já usa;
  seguem pelo canal privado do `SECURITY.md` e por issue própria, não nesta versão.
- **`Connections` com nome público que resolve para endereço privado**: documentado como
  pergunta de DNS; conferir na discagem pediria o ambiente dentro de `Connection.Client`.
- **Brotli, keep-alive e cadeia de suprimentos npm**: o runtime não descomprime corpo de
  requisição, o cliente HTTP é o da biblioteca padrão, e o repositório não tem dependência npm
  (o `uitest` usa `playwright-go`; as actions do CI já estão fixadas por SHA).

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `net.Dialer.Control`, `netip`, `strings`; nada novo no `go.mod`. |
| VI — teste primeiro | Cada correção tem um teste que reprovou antes dela (ver *Tarefas*). |
| VII — segurança por padrão | Tudo vale sem configuração; nenhum controle novo é opcional. |

## Segurança e privacidade

- **Ativos e fronteiras**: disco do servidor (navegador → app, upload); rede interna e metadata
  da nuvem (app → terceiro, webhook e Connections); sessão e reputação do domínio (redirect);
  limite de taxa e trilha de auditoria (proxy → app); planilha de quem exporta (app → Excel).
- **OWASP ASVS 5.0 nível 2**: V1.2 (injeção — CSV), V3.7 (redirect para destino não
  permitido), V5.2 (upload: armazenamento e limpeza), V12/V13 (SSRF e lista de destinos), V16
  (log sem valor controlado pelo cliente sem limite), limite de taxa contra adivinhação de
  senha (V6.3).
- **OWASP Top 10:2025**: A01 (redirect, SSRF), A05 (injeção), A06/A10 (falha de projeto e de
  condição excepcional: temporário que fica), A09 (log).
- **Segredos, dados pessoais, logs**: o arquivo enviado deixa de sobrar no `TMPDIR`; o
  `X-Request-ID` não leva mais texto arbitrário para o log. Exceção registrada: um
  `Options.HTTP` próprio no webhook não tem a checagem na discagem (documentado no campo).

## Tarefas

- [x] T001 Testes que reprovam: `TestMultipartTempFilesRemovedAfterRequest`,
  `TestAssetVersionsFileFromMount`, `TestValidateExternalURL` (IPv4 não canônico, mapeado,
  CGNAT), `TestADiscagemConfereOEnderecoDeVerdade`, `TestRedirectRecusaSairDoSite` (tab, LF,
  CRLF, NUL), `TestClientIPReadsEveryForwardedForLine`,
  `TestUpstreamHonoursTheGlobalRateLimit`, `TestCSVNeutralizaFormula`,
  `TestRequestIDFromClientIsBounded`.
- [x] T002 Implementação (`ctx.go`, `serve.go`, `assets.go`, `connections.go`, `errors.go`,
  `proxy.go`, `upstream.go`, `csv.go`, `webhook/address.go`, `webhook/webhook.go`).
- [x] T003 Documentação nas duas locales (referência de `ctx`, `app`, `upstreams`) e
  `SECURITY-MODEL.md` (en e pt-BR).
- [x] T004 `CHANGELOG.md`, `version` em `cmd/trilha/main.go`, item 150 do `ROADMAP.md`.
- [x] T005 Suíte inteira verde e os números em *Evidências*; `scripts/release.sh 0.162.0
  --issues "313"`.

## Aceitação

- **SC-001** Depois de um upload que passou de `MaxFormMemory`, o diretório temporário fica
  vazio.
- **SC-002** Um webhook cujo nome parecia público na checagem e que liga no loopback é recusado
  antes do handshake.
- **SC-003** `http://127.1`, `http://0x7f.0.0.1`, `http://0177.0.0.1`, `http://[::ffff:127.0.0.1]`
  e `http://100.100.100.200` são recusados como privados em produção.
- **SC-004** `Redirect("/\t/evil.example")` é recusado como os outros endereços de fora.
- **SC-005** Com duas linhas de `X-Forwarded-For`, o IP do cliente é o que o proxy acrescentou.
- **SC-006** Com `RateLimit{RPS: 1, Burst: 2}`, a terceira chamada ao upstream é 429 e não
  chega ao serviço.
- **SC-007** Uma célula `=HYPERLINK(…)` sai `'=HYPERLINK(…)` e volta igual pelo `BindCSV`.
- **SC-008** `c.Asset("/js/a.js")` de um `Mount` volta com `?v=` igual ao `ETag`, e o pedido
  com esse `?v=` recebe `immutable`.

## Evidências

| Nível | Comando | Passaram | Falharam | Pularam |
|---|---|---|---|---|
| Unidade + integração (testes e subtestes) | `make test` | 1645 | 0 | 2 |
| Navegador | `make test-ui` (motores: chromium) | 48 | 0 | 0 |
| Segurança | `make security` (race + `govulncheck` em Go 1.26.10) | verde | 0 | — |

Sem cenário de navegador novo: nenhuma correção muda o que a página mostra, e o redirect com
tab foi confirmado no Chromium durante a revisão e é provado no nível do `localPath`, que é
onde a decisão acontece.
