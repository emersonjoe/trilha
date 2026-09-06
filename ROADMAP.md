# Roadmap

Este arquivo é a resposta a uma avaliação externa do Trilha ("roadmap para 10/10", 24
seções). Ele separa três coisas que costumam vir misturadas: **o que já existe**, **o que
vamos fazer** e **o que decidimos não fazer, com o motivo**. Cada item aberto tem uma issue;
cada item que for implementado passa por uma spec em `specs/`, como manda a
[constituição](.specify/memory/constitution.md).

A tese não muda: *full-stack em Go, roteamento por arquivos, SSR primeiro, aprimoramento
progressivo, seguro por padrão, um binário no fim*. O risco de qualquer roadmap é virar uma
lista de features do Next.js; o critério de aceitação de cada item abaixo é **resolver um
problema real de quem escreve o app**, não empatar uma tabela comparativa.

## Onde o Trilha está (setembro de 2026, v0.18.0)

| Área da avaliação | Estado | Onde |
|---|---|---|
| Arquitetura | roteamento por arquivos, layouts aninhados, middleware por subárvore, erros como valores, `Setup`/`Config` (com erro)/`Shutdown` | specs 001, 007, 008, 021 |
| Simplicidade | zero dependências no runtime e na CLI, garantido por teste | princípio II |
| Coerência com Go | `http.ServeMux` 1.22, `context`, `log/slog`, `embed`, erros explícitos | princípio III |
| DX | `new`, `gen` (com `--check`), `dev` (recarga ~1 s, erro de build na página), `build`, `routes`, `export`, `audit`, `ui` | specs 001, 003, 004, 006, 021 |
| Frontend | HTML no servidor, `ui.js` (~200 linhas), SSE, formulários com `Bind`/`FieldErrors` e validação por tag, fragmentos, ilhas, navegação no cliente e upload com progresso | specs 006, 009, 018, 022, 023, 024, 027 |
| Dados | funções Go comuns, sem loader mágico; `cache` com prazo, tags, invalidação, voo único e memo por requisição; `ETag`/`Last-Modified`/`304` no `Ctx` e no estático | por decisão, specs 025 e 026 |
| Auth | cookies assinados, CSRF, limite de taxa; OIDC (Entra ID, Keycloak, Cognito) com PKCE, sessão, papéis e logout | specs 004, 016, 020 |
| Segurança | CSP com nonce, HSTS, COOP, `Permissions-Policy`, proxies confiáveis, timeouts, limite de corpo (global e por rota), `trilha audit` | specs 004, 024 |
| Observabilidade | sondas de vida e prontidão, métricas Prometheus, `traceparent`, eventos de segurança, log de requisição com filtro | specs 014, 021 |
| API | JSON, erros de API, SSE, `route.go` com `Kind` | specs 001, 005, 008 |
| SSG | `trilha export`, `AddExportPath`, `BasePath` | spec 003 |
| UI | kit `ui` com ~40 componentes, tema compatível com shadcn/ui, ícones Lucide | specs 006, 023, 024 |
| IA | `ai` (protocolo OpenAI: OpenAI, Ollama, OpenRouter, vLLM…), `ai/mcp` cliente e servidor | spec 005 |
| Testes | unitários, golden, integração por exemplo, e2e da CLI | princípio VI |
| Desempenho | módulo `bench/`, resultados publicados, metodologia | spec 011 |
| Comunidade | CONTRIBUTING, CODE_OF_CONDUCT, SECURITY, SUPPORT, GOVERNANCE, templates, CODEOWNERS | spec 004 |

Boa parte do que a avaliação lista como pendente já entrou entre a 0.4.0 e a 0.18.0 — a
avaliação enxergou o projeto num ponto anterior. O que sobra, sobra de verdade.

## O que vamos fazer

A ordem segue o retorno para quem escreve o app, não a ordem da avaliação.

### Fase 1 — Interatividade sem virar SPA (o maior buraco)

O Trilha entrega HTML e recarrega a página. Isso é honesto e rápido, mas hoje um filtro,
um modal com dados ou uma tabela paginada obrigam a escrever JavaScript à mão. A saída
**não** é adotar React: é fechar o degrau entre "formulário que recarrega" e "SPA".

1. ~~[#20](https://github.com/emersonjoe/trilha/issues/20) Atualização parcial de página: uma rota devolve um fragmento e o cliente troca um pedaço
   do documento, sem framework.~~ **Entregue na 0.10.0** (spec 018): `Ctx.Fragment()` e
   `ui.Swap`.
2. ~~[#21](https://github.com/emersonjoe/trilha/issues/21) Envio de formulário sem recarregar, com estado de carregamento e erro de campo vindos
   do mesmo handler que já responde HTML.~~ **Entregue na 0.10.0** (spec 018): mesmo
   mecanismo, com `aria-busy`, foco no campo inválido em 422 e recuo para o envio normal.
3. ~~[#22](https://github.com/emersonjoe/trilha/issues/22) Ilhas: um componente interativo isolado, com o resto da página estática.~~ **Entregue na
   0.13.0** (spec 022): `Ctx.Island` com módulo em `public/`, props escapadas e conteúdo de
   origem no servidor.
4. ~~[#23](https://github.com/emersonjoe/trilha/issues/23) Navegação no cliente, opcional e por atributo, preservando histórico e foco.~~
   **Entregue na 0.14.0** (spec 023): `ui.Navigate`, `ui.NoNavigate` e `ui.NavigateScript`,
   com `ui.nav.js` à parte — quem não usa não baixa.
5. ~~[#24](https://github.com/emersonjoe/trilha/issues/24) Upload com progresso e a decisão sobre WebSocket.~~ **Entregue na 0.15.0** (spec 024):
   `ui.UploadTo`/`ui.UploadBar`/`ui.UploadScript` com `ui.upload.js` à parte, e
   `Ctx.Hijack`/`AllowBody`/`NoReadDeadline` no runtime. **WebSocket fica fora do core por
   decisão**: é transporte, não encosta em rota nem em render, e o app pode pôr
   `coder/websocket` no go.mod dele — o `Hijack` é a porta.

**A Fase 1 está fechada.**

### Fase 2 — Dados, escrita e identidade

6. ~~[#25](https://github.com/emersonjoe/trilha/issues/25) Cache da aplicação com TTL, tags e invalidação explícita, mais deduplicação por
   requisição. Sem isso, "revalidação" não existe.~~ **Entregue na 0.16.0** (spec 025).
7. ~~[#26](https://github.com/emersonjoe/trilha/issues/26) Cache HTTP: `ETag`, `Last-Modified`, `304`.~~ **Entregue na 0.17.0** (spec 026).
8. ~~[#27](https://github.com/emersonjoe/trilha/issues/27) Validação declarativa no `Bind` (tags), com validadores próprios, mantendo
   `FieldErrors` como resposta.~~ **Entregue na 0.18.0** (spec 027).
9. ~~[#40](https://github.com/emersonjoe/trilha/issues/40) Autenticação de verdade: sessão, rotação, logout, middleware de autorização, RBAC e
    **OIDC** com atalhos para Microsoft Entra ID e Keycloak, sem acoplar o framework a
    nenhum provedor.~~ **Entregue na 0.7.0** (spec 016). Google e GitHub ficam de fora por
    ora: o primeiro já funciona pelo `auth.OIDC` genérico, e o GitHub fala OAuth2 puro,
    sem `id_token` — é outro fluxo, não um atalho.
10. [#28](https://github.com/emersonjoe/trilha/issues/28) Upload de arquivo com limites de tamanho e tipo.

### Fase 3 — Produção e API

11. [#29](https://github.com/emersonjoe/trilha/issues/29) CORS.
12. [#30](https://github.com/emersonjoe/trilha/issues/30) Negociação de conteúdo e erro de API padronizado (RFC 9457, `application/problem+json`).
13. [#31](https://github.com/emersonjoe/trilha/issues/31) Geração de OpenAPI a partir das rotas registradas.
14. [#32](https://github.com/emersonjoe/trilha/issues/32) Auxiliares de teste (`trilha.TestRequest`, `TestPage`, `TestRoute`).
15. [#33](https://github.com/emersonjoe/trilha/issues/33) `go test -race` e *fuzzing* no CI (roteador, `h`, `Bind`, cookies assinados).
16. [#34](https://github.com/emersonjoe/trilha/issues/34) Modelo de ameaças escrito e validação de `Host`.
17. [#35](https://github.com/emersonjoe/trilha/issues/35) Definição formal da API pública e política de depreciação, antes da 1.0.

### Fase 4 — DX e acabamento

18. [#36](https://github.com/emersonjoe/trilha/issues/36) `trilha generate page|route|component`.
19. [#37](https://github.com/emersonjoe/trilha/issues/37) Inspetor de rotas no `trilha dev`.
20. [#38](https://github.com/emersonjoe/trilha/issues/38) Cookbook, checklist de produção e guia de migração.
21. [#39](https://github.com/emersonjoe/trilha/issues/39) `Pagination` e `Tooltip` no kit `ui` (o resto da lista da avaliação já existe).
22. [#41](https://github.com/emersonjoe/trilha/issues/41) Atalhos de provedor no `auth` para **AWS Cognito** e **Clerk**. O Cognito foi
    **entregue na 0.11.0** (spec 020): `auth.Cognito` monta o emissor, lê os papéis de
    `cognito:groups` e `LogoutDomain` resolve o logout que a AWS implementa fora do padrão.
    O **Clerk continua aberto**: a documentação pública dele não descreve
    `/.well-known/openid-configuration`, de onde o `auth` tira todos os endereços, nem uma
    claim com o papel na organização. Falta confirmar contra uma instância real — atalho
    escrito em cima de suposição é pior que atalho nenhum.

## O que não vamos fazer, e por quê

| Item da avaliação | Decisão | Motivo |
|---|---|---|
| ORM, fila, runtime JavaScript no núcleo | não | princípio II; a avaliação também pede que não seja feito |
| Obrigar React, Vite, bundler | não | quebra "um binário, sem cadeia de build" |
| Publicar números contra Gin, Echo, Fiber, Next.js | não | decisão registrada na spec 011: comparação de abordagem é verificável, tabela de números entre projetos configurados de formas diferentes é briga, não informação |
| Repositórios separados (`trilha-ui`, `trilha-auth`…) | não agora | módulos Go separados **dentro deste repositório** (como `bench/`) dão o mesmo isolamento de dependências sem fragmentar versão, CI e issues. Reavaliar na 1.0 |
| Exportador OpenTelemetry no núcleo | não | o Trilha propaga `traceparent` e registra `trace_id`; exportar spans traz dezenas de dependências. Cabe um módulo opcional |
| ISR (regeneração incremental) | não | pressupõe estado compartilhado entre réplicas e invalidação distribuída; conflita com "um binário estático". Cache com tags (item 7) resolve o caso real |
| Criar um design system grande | não | o kit `ui` existe para compor, não para virar biblioteca de componentes |

## Como acompanhar

- **Issues**: [#20 a #41](https://github.com/emersonjoe/trilha/issues?q=is%3Aissue+label%3Aroadmap), com o rótulo `roadmap`, agrupadas por
  marco (`Fase 1 — Interatividade`, `Fase 2 — Dados e identidade`, `Fase 3 — Produção`,
  `Fase 4 — DX`).
- **Specs**: quando um item entra em execução, ganha `specs/NNN-nome/` (spec → plan →
  tasks → implement) e a issue passa a apontar para ela.
- **Versões**: toda spec fechada vira uma versão (`GOVERNANCE.md`), então a documentação do
  site e `go get ...@latest` nunca divergem.

A nota da avaliação não é a métrica. A métrica é a da própria avaliação, e essa vale:
*conseguir construir uma aplicação web moderna inteira em Go — SSR, rotas, formulários,
autenticação, API, cache, interatividade e observabilidade — sem montar um quebra-cabeça de
ferramentas, e poder integrar o resto sem lutar contra o framework.*
