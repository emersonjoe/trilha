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

## Onde o Trilha está (setembro de 2026, v0.41.0)

| Área da avaliação | Estado | Onde |
|---|---|---|
| Arquitetura | roteamento por arquivos, layouts aninhados, middleware por subárvore, erros como valores, `Setup`/`Config` (com erro)/`Shutdown` | specs 001, 007, 008, 021 |
| Simplicidade | zero dependências no runtime e na CLI, garantido por teste | princípio II |
| Coerência com Go | `http.ServeMux` 1.22, `context`, `log/slog`, `embed`, erros explícitos | princípio III |
| DX | `new`, `gen` (com `--check`), `dev` (recarga ~1 s, erro de build na página), `build`, `routes`, `export`, `audit`, `ui` | specs 001, 003, 004, 006, 021 |
| Frontend | HTML no servidor, `ui.js` (~200 linhas), SSE, formulários com `Bind`/`FieldErrors` e validação por tag, fragmentos, ilhas, navegação no cliente e upload com progresso | specs 006, 009, 018, 022, 023, 024, 027 |
| Dados | funções Go comuns, sem loader mágico; `cache` com prazo, tags, invalidação, voo único e memo por requisição; `ETag`/`Last-Modified`/`304` no `Ctx` e no estático | por decisão, specs 025 e 026 |
| Auth | cookies assinados, CSRF, limite de taxa; OIDC (Entra ID, Keycloak, Cognito) com PKCE, sessão, papéis e logout | specs 004, 016, 020 |
| Segurança | CSP com nonce, HSTS, COOP, `Permissions-Policy`, proxies confiáveis, timeouts, limite de corpo (global e por rota), upload com tipo pelo conteúdo e nome seguro, CORS configurável, `trilha audit` | specs 004, 024, 028, 029 |
| Observabilidade | sondas de vida e prontidão, métricas Prometheus, `traceparent`, eventos de segurança, log de requisição com filtro | specs 014, 021 |
| API | JSON, erro RFC 9457 (`problem+json`), negociação por `Accept`, SSE, `route.go` com `Kind` | specs 001, 005, 008, 030 |
| SSG | `trilha export`, `AddExportPath`, `BasePath` | spec 003 |
| UI | kit `ui` com ~40 componentes, tema compatível com shadcn/ui, ícones Lucide | specs 006, 023, 024 |
| IA | `ai` (protocolo OpenAI: OpenAI, Ollama, OpenRouter, vLLM…), `ai/mcp` cliente e servidor | spec 005 |
| Testes | unitários, golden, integração por exemplo, e2e da CLI | princípio VI |
| Desempenho | módulo `bench/`, resultados publicados, metodologia | spec 011 |
| Comunidade | CONTRIBUTING, CODE_OF_CONDUCT, SECURITY, SUPPORT, GOVERNANCE, templates, CODEOWNERS | spec 004 |

Boa parte do que a avaliação lista como pendente já entrou entre a 0.4.0 e a 0.21.0 — a
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
10. ~~[#28](https://github.com/emersonjoe/trilha/issues/28) Upload de arquivo com limites de tamanho e tipo.~~ **Entregue na 0.19.0** (spec 028).

### Fase 3 — Produção e API

11. ~~[#29](https://github.com/emersonjoe/trilha/issues/29) CORS.~~ **Entregue na 0.20.0** (spec 029).
12. ~~[#30](https://github.com/emersonjoe/trilha/issues/30) Negociação de conteúdo e erro de API padronizado (RFC 9457, `application/problem+json`).~~ **Entregue na 0.21.0** (spec 030).
13. ~~[#31](https://github.com/emersonjoe/trilha/issues/31) Geração de OpenAPI a partir das rotas registradas.~~ **Entregue na 0.22.0** (spec 031).
14. ~~[#32](https://github.com/emersonjoe/trilha/issues/32) Auxiliares de teste (`trilha.TestRequest`, `TestPage`, `TestRoute`).~~ **Entregue na 0.23.0** (spec 032): cliente com pote de cookies, CSRF que passa sozinho e `WithSigned`; os cinco exemplos deixaram de reimplementar o `httptest`.
15. ~~[#33](https://github.com/emersonjoe/trilha/issues/33) `go test -race` e *fuzzing* no CI (roteador, `h`, `Bind`, cookies assinados).~~ **Entregue na 0.24.0** (spec 033): teste de concorrência com 32 goroutines, seis alvos de fuzzing por invariante e dois jobs novos no CI.
16. ~~[#34](https://github.com/emersonjoe/trilha/issues/34) Modelo de ameaças escrito e validação de `Host`.~~ **Entregue na 0.25.0** (spec 034): `SECURITY-MODEL.md` nas duas línguas, com o que fica aberto escrito, e `Config.AllowedHosts` recusando na borda.
17. ~~[#35](https://github.com/emersonjoe/trilha/issues/35) Definição formal da API pública e política de depreciação, antes da 1.0.~~ **Entregue na 0.26.0** (spec 035): `API.md` nas duas línguas, `api/current.txt` com os 822 símbolos exportados e um teste que falha quando a superfície muda.

### Fase 4 — DX e acabamento

18. ~~[#36](https://github.com/emersonjoe/trilha/issues/36) `trilha generate page|route|component`.~~ **Entregue na 0.27.0** (spec 036): o comando recebe a URL e escreve a pasta da convenção, com esqueleto que compila e `trilha_gen.go` regerado no fim.
19. ~~[#37](https://github.com/emersonjoe/trilha/issues/37) Inspetor de rotas no `trilha dev`.~~ **Entregue na 0.28.0** (spec 037): `/_trilha/routes` com a tabela em ordem de precedência, layouts e middlewares por rota, e a caixa que diz quem atende um caminho; servido pelo supervisor, fora do binário.
20. ~~[#38](https://github.com/emersonjoe/trilha/issues/38) Cookbook, checklist de produção e guia de migração.~~ **Entregue na 0.29.0** (spec 038): terceira seção do site com nove receitas, checklist de produção e guia de migração; todo bloco Go vem de `examples/cookbook`, que o `go vet` compila e um teste do site confere caractere por caractere.
21. ~~[#39](https://github.com/emersonjoe/trilha/issues/39) `Pagination` e `Tooltip` no kit `ui` (o resto da lista da avaliação já existe).~~ **Entregue na 0.30.0** (spec 039): paginação em links de verdade (página atual em `<span>` com `aria-current`, janela de sete casas com reticências) e dica que nasce no `title` e vira bolha com `role=tooltip`, foco, toque e Escape.
22. ~~[#41](https://github.com/emersonjoe/trilha/issues/41) Atalhos de provedor no `auth` para **AWS Cognito** e **Clerk**.~~ **Entregue
    em duas partes**: o Cognito na 0.11.0 (spec 020) — `auth.Cognito` monta o emissor, lê os
    papéis de `cognito:groups` e `LogoutDomain` resolve o logout que a AWS implementa fora do
    padrão — e o Clerk na 0.31.0 (spec 041). As três dúvidas que seguravam o Clerk foram
    conferidas contra um documento de descoberta real: ele **tem**
    `/.well-known/openid-configuration` completo, o `issuer` **não** tem barra final, e **não
    há** claim de papel nem `end_session_endpoint`. Por isso o `auth.Clerk` monta o emissor a
    partir da Frontend API URL e diz a verdade sobre o resto: papéis caem no par genérico e o
    `Logout` avisa no log que a sessão do Clerk ficou de pé.

### Fase 5 — Agentes

A pergunta desta fase é outra: não "o que falta ao framework", mas **quanto uma ferramenta
de IA gasta para entregar uma feature nele**. Um agente paga em token cada arquivo que abre
para descobrir o que o projeto já tem, cada erro que não diz onde nem como consertar, cada
rodada de verificação separada. A ordem abaixo é de retorno por esforço, e o primeiro item
é a régua que mede os outros — sem ela a gente otimiza no escuro.

23. ~~[#45](https://github.com/emersonjoe/trilha/issues/45) **Régua: custo por feature.** Harness em `bench/agent/` que roda quatro cenários fixos (rota com `Bind`, página com formulário, troca de provedor, paginação) com um agente de linha de comando e publica tokens e rodadas no `RESULTS.md`. Comparação Trilha antes × Trilha depois, nunca contra outro framework.~~ **Entregue na 0.33.0** (spec 043): `make bench-agent`, fixture hermética (cópia do exemplo em módulo próprio + cópia somente-leitura do repositório), teste escondido decide passou/não passou. Primeira medição (Opus 4.8, 12/12 verdes, mediana): `comments` 39 rodadas / 421 s, `contact-form` 30 / 185 s, `cognito` 18 / 73 s, `pagination` 16 / 101 s.
24. ~~[#46](https://github.com/emersonjoe/trilha/issues/46) **`AGENTS.md` no scaffold e `llms.txt` no site.** `trilha new` grava as convenções, os comandos e o que não fazer em sessenta linhas; o site exporta `/llms.txt` e `/llms-full.txt` nas duas línguas a partir do mesmo Markdown.~~ **Entregue na 0.36.0** (spec 044): o `AGENTS.md` é opcional (`trilha agents`, `trilha new --agents`), carimbado como o kit de ui, com um `CLAUDE.md` que aponta para ele e nunca é reescrito; o site exporta `/llms.txt` e `/llms-full.txt` nas duas locales, e `App.Export` passou a gravar caminho com ponto como arquivo. Régua (mesma fixture, 12/12 verdes dos dois lados): rodadas -6% a -28%, cache lido -13% a -45%, custo -15% a -40%.
25. ~~[#47](https://github.com/emersonjoe/trilha/issues/47) **`trilha ctx --json`.** O mapa do projeto para máquinas: rotas, layouts, middlewares, o que cada handler recebe e devolve, tipos em `Bind`. Reaproveita `internal/scan`, a inferência do `openapi` e os dados do inspetor `/_trilha/routes`.~~ **Entregue na 0.37.0** (spec 047): Markdown compacto para ler e `--json` para ferramenta, com `--routes`, `--types` e `--all`; a seção de API sai da mesma inferência do `trilha openapi`, então mapa e documento não divergem, e a saída é determinística.
26. ~~[#48](https://github.com/emersonjoe/trilha/issues/48) **`trilha check`.** Um só portão (`gen --check`, `gofmt`, `vet`, `test`, `audit`, `openapi --check`) com `--json` e `--fix`, e toda mensagem com `file:line` e o conserto.~~ **Entregue na 0.37.0** (spec 047): os seis passos na ordem que falha mais barato primeiro, parando no primeiro erro, com `--fix`, `--json` e o conserto embaixo de cada problema — o mesmo que o scanner agora imprime no `gen` e no `dev`.
27. ~~[#49](https://github.com/emersonjoe/trilha/issues/49) **`trilha generate` com contrato.**~~ **Entregue na 0.39.0** (spec 051): `--methods`, `--bind Tipo`, `--form Tipo`, `--layout` e `generate test <url>` — o esqueleto nasce com a assinatura certa, o `Bind`, a resposta e um teste que passa sem edição.
28. [#50](https://github.com/emersonjoe/trilha/issues/50) **`trilha mcp` e MCP de documentação.** Por último, de propósito: um servidor stdio que embrulha os comandos acima sobre o pacote `mcp` da spec 005, e um MCP hospedado só de leitura (`search_docs`, `get_recipe`) para agente sem shell, servido por um app Trilha.

### Fase 6 — O que um app hospedeiro pediu

Esta fase não saiu de uma avaliação, e sim de pôr o framework dentro de um binário que já
existia. As issues 42–58 são o que doeu lá; a ordem aqui é a do que doeu mais.

29. ~~[#51](https://github.com/emersonjoe/trilha/issues/51) `trilha gen` só escrevia `package main`.~~ **Entregue na 0.32.0** (spec 042): o arquivo gerado assume o pacote que a pasta declara e exporta `NewApp()`, então o hospedeiro monta com `mux.Handle("/", crm.NewApp().Handler())` e ninguém mantém arquivo de registro à mão — o `gen --check` continua pegando a pasta criada sem gerar.
30. ~~[#56](https://github.com/emersonjoe/trilha/issues/56) `middleware.go` não distinguia método.~~ **Entregue na 0.32.0** (spec 042): `MiddlewareGET|POST|PUT|PATCH|DELETE` herdam pela subárvore como o `Middleware`, rodam depois dele, e um middleware de método que nenhuma rota serve é erro (`E_UNUSED_METHOD_MIDDLEWARE`).
31. ~~[#53](https://github.com/emersonjoe/trilha/issues/53) `error.go` não era chamado para 4xx.~~ **Entregue na 0.32.0** (spec 042): todo status que não é 404 passa pela página do app, com o layout raiz e o status certo; `trilha.StatusOf(err)` dá o código para o `switch`, o `problem+json` da API fica intocado e a página interna segue de rede.

32. ~~[#52](https://github.com/emersonjoe/trilha/issues/52) os cabeçalhos do app sobrescreviam os do hospedeiro.~~ **Entregue na 0.35.0** (spec 046): `Security.Delegated` não escreve nenhum dos sete, e `Security.Nonce` faz o `c.Nonce()` devolver o nonce que o hospedeiro já publicou na política dele.
33. ~~[#54](https://github.com/emersonjoe/trilha/issues/54) os nomes do CSRF eram constantes.~~ **Entregue na 0.35.0** (spec 046): `Config.CSRF` troca cookie, campo e cabeçalho, com as constantes como padrão; dois `_csrf` na mesma página deixa de ser possível sem querer.
34. ~~[#55](https://github.com/emersonjoe/trilha/issues/55) dependências só chegavam por `map[string]any`.~~ **Entregue na 0.35.0** (spec 046): `trilha.Provide`/`trilha.Use[T]` guardam e leem por tipo, do `*Ctx` ou do `*App`, e o exemplo do blog saiu das variáveis de pacote — que é o que quebra com dois apps no mesmo processo.

35. ~~[#75](https://github.com/emersonjoe/trilha/issues/75) pasta com ponto no começo do nome sumia em silêncio.~~ **Entregue na 0.38.0** (spec 048): `/.well-known/` virou a única exceção do ponto — é onde as RFCs mandam publicar documento — e qualquer `page.go` ou `route.go` escondido em outra pasta com ponto agora é `E_HIDDEN_ROUTE` na hora do `gen`, em vez de 404 sem explicação.
36. ~~[#57](https://github.com/emersonjoe/trilha/issues/57) o `tmpl` não tinha a ponte inversa.~~ **Entregue na 0.39.0** (spec 054): `tmpl.Wrap(t, nome, slot)` prepara a casca uma vez e `casca.Node(dados, filhos)` põe o `h.Node` no slot, sem `template.HTML` escrito pelo app — 8,3× mais barato do que clonar o conjunto por requisição, e casca que não chega ao slot é erro de render, não página vazia.
37. ~~[#44](https://github.com/emersonjoe/trilha/issues/44) o nonce e o token de CSRF eram inalcançáveis para quem não renderiza com o `h`.~~ **Entregue na 0.39.0** (spec 054): `trilha.NonceFrom(r)` e `trilha.CSRFTokenFrom(r)` respondem a partir do `*http.Request`, que é tudo o que um `html/template`, o `templ` ou um handler seu recebem.
38. ~~[#43](https://github.com/emersonjoe/trilha/issues/43) `Kind` era a única coisa da árvore que não se herdava — e é o que liga o CSRF.~~ **Entregue na 0.39.0** (spec 055): `var Kind` no pacote de uma pasta vale para ela e para tudo abaixo, com `kind.go` para a raiz de ramo sem rota própria, e o `trilha audit` aponta a escrita em `route.go` que nenhum `Kind` alcança.
39. ~~[#42](https://github.com/emersonjoe/trilha/issues/42) não dava para saber qual rota casou.~~ **Entregue na 0.39.0** (spec 055): `c.Pattern()` devolve o gabarito (`/blog/{slug}`), e o registro de acesso passou a gravar `path` e `route` — o concreto para quem investiga um caso, o gabarito para quem agrega.
40. ~~[#58](https://github.com/emersonjoe/trilha/issues/58) faltavam `maxlength`, `minlength`, `autocomplete` e `inputmode` no `h`.~~ **Entregue na 0.39.0** (spec 053): os quatro viraram funções ao lado de `Pattern` e `Required`, e `h.Attrs(...)` agrupa atributos num nó só.

### Fora das fases — o que apareceu usando

41. ~~[#66](https://github.com/emersonjoe/trilha/issues/66) o redirect comia a notícia.~~ **Entregue na 0.39.0** (spec 053): `c.Flash(tipo, texto)` sobrevive ao `POST → 303 → GET` num cookie assinado (ou no cabeçalho `Trilha-Flash`, quando a resposta é fragmento), o `ui.Flashes(c)` do layout mostra, e o `ui.Confirm(título, descrição)` pergunta antes de destruir sem script inline.
42. ~~[#76](https://github.com/emersonjoe/trilha/issues/76) e [#78](https://github.com/emersonjoe/trilha/issues/78) um `route.go` não conseguia responder a um preflight.~~ **Entregue na 0.39.0** (spec 052): `func OPTIONS` nasce da árvore como os outros métodos, e `var CORS = trilha.CORS{...}` dá a política de uma rota só, preflight incluído, sem abrir o app inteiro.
43. ~~[#77](https://github.com/emersonjoe/trilha/issues/77) o `trilha check` reprovava por `TRILHA_SECRET` num app que nunca assina cookie.~~ **Entregue na 0.39.0** (spec 052): a auditoria lê o código antes de cobrar — segredo ausente vira aviso quando nada assina, e continua crítico quando alguém assina ou quando está definido e curto.

44. ~~[#79](https://github.com/emersonjoe/trilha/issues/79) `trilha dev` não subia no Windows.~~ **Entregue na 0.39.2** (spec 056): o binário que a CLI constrói recebe a extensão que o sistema exige — `go build -o` escreve o nome literal, e o `exec.LookPath` do Windows só aceita o que estiver no `PATHEXT` —, e o CI ganhou um trabalho `windows-latest`, que é quem impede a volta: era a ausência dele que deixava uma CLI incapaz de subir passar verde.

### Fase 8 — O teto da interatividade

Esta fase saiu de um relato de campo: alguém que veio de anos de React, rodou Go + templ +
htmx seis meses em produção e escreveu o que aprendeu. Os elogios dele descrevem o que o
Trilha já é — HTML tipado no servidor, troca de pedaço em vez de store no cliente — e por
isso valem menos que as duas queixas. A primeira é que o teto de estado rico no cliente
existe e chega; a segunda é que, em conexão lenta, faltava resposta visual enquanto o
servidor responde, e ele resolveu com `hx-indicator` e transições de CSS.

A tese da fase é que **admitir o teto é o que dá confiança**. Um framework que finge não ter
um perde a pessoa exatamente no dia em que ela esbarra nele — e ela esbarra em produção, não
no tutorial.

45. ~~[#81](https://github.com/emersonjoe/trilha/issues/81) Enquanto o servidor responde: indicador em outro elemento que não o alvo, limiar de atraso para a resposta rápida não piscar, gatilho travado contra o segundo clique e troca sem pulo onde o `startViewTransition` existe. É a única queixa concreta do relato, e hoje o Trilha responde pela metade — `aria-busy` no alvo e nada mais.~~ **Entregue na 0.40.0** (spec 057): `ui.Indicator` com limiar de 120 ms, `ui.PendingAfter` para mudá-lo, `ui.Spinner`, guarda contra o segundo clique e *crossfade* onde o `startViewTransition` existe. O `aria-busy` passou a obedecer ao mesmo limiar — era ele que piscava.
46. ~~[#82](https://github.com/emersonjoe/trilha/issues/82) A ilha que chega dentro de um fragmento trocado não monta se a página não tinha nenhuma ilha antes: o `<script>` do loader entra pelo `outerHTML` e o DOM não executa script inserido assim. Falha em silêncio, e some quando você recarrega para conferir.~~ **Entregue na 0.40.0** (spec 057): quem monta o que a troca traz é o `ui.js`, que é por onde toda troca passa; os dois lados pulam o que já tem `data-trilha-mounted`, então a ilha monta uma vez só.
47. ~~[#83](https://github.com/emersonjoe/trilha/issues/83) Dizer onde o teto fica e como atravessá-lo: os sinais concretos (estado que sobrevive entre trocas, arrastar e soltar contínuo, edição colaborativa, canvas) e uma ilha com biblioteca de terceiros dentro, rodando em `examples/`, sem bundler.~~ **Entregue na 0.40.0** (spec 057): capítulo *O teto* nas duas línguas, com as regras que impedem a ilha de virar SPA, e a rota `/blog/ordem` com arrastar-e-soltar, botões ↑ ↓ para o teclado e a ordem salva pelo mesmo `POST → redirect → GET`.
48. ~~[#84](https://github.com/emersonjoe/trilha/issues/84) Chegando do htmx + templ: a página que traduz o que a pessoa já sabe em vez de ensinar do zero, inclusive o que **não** tem equivalente.~~ **Entregue na 0.40.0** (spec 057): capítulo *Vindo do htmx e do templ* nas duas línguas, com a tabela de tradução e as três faltas ditas sem rodeio — troca fora de banda, gatilho por tempo e gatilho por evento qualquer.

### Fase 7 — Migração de app Next.js

Esta fase saiu de um inventário: 15.701 linhas de TypeScript em 77 arquivos de um app real
(Acervo), 46 das 47 páginas em `'use client'`, 200 endpoints FastAPI que ficam onde estão. A
issue [#74](https://github.com/emersonjoe/trilha/issues/74) tem a medição e a classificação
das telas; o que segurava a migração não era a lógica das telas, era infraestrutura repetida.

Entregue quase inteira na 0.41.0, com dois itens que vieram antes e um que precisou de uma
segunda passada.

49. ~~[#59](https://github.com/emersonjoe/trilha/issues/59) `trilha migrate next`: o esqueleto de `app/` e o relatório A/B/C.~~ **Entregue na 0.41.0.**
50. ~~[#60](https://github.com/emersonjoe/trilha/issues/60) `Config.Upstreams`: o `rewrites` do Next com a credencial da sessão.~~ **Entregue na 0.41.0.**
51. ~~[#62](https://github.com/emersonjoe/trilha/issues/62) `auth.Sessions` sem OIDC, com `User.Extra` e PBKDF2.~~ **Entregue na 0.41.0.**
52. ~~[#61](https://github.com/emersonjoe/trilha/issues/61) `trilha client`: cliente Go tipado a partir do OpenAPI que já existe.~~ **Entregue na 0.41.0.**
53. ~~[#65](https://github.com/emersonjoe/trilha/issues/65) `ui.Shell` e `trilha new --template app`.~~ **Entregue na 0.41.0.**
54. ~~[#63](https://github.com/emersonjoe/trilha/issues/63) `ListParams` + `ui.DataTable`: a listagem que mora na URL.~~ **Entregue na 0.41.0.**
55. ~~[#64](https://github.com/emersonjoe/trilha/issues/64) `ui.Poll` e `ui.Live`: fragmento que se atualiza sozinho.~~ **Entregue na 0.41.0.**
56. ~~[#66](https://github.com/emersonjoe/trilha/issues/66) `c.Flash` sobrevivendo ao redirect e `ui.Confirm`.~~ **Entregue na 0.39.0** (spec 053) — chegou antes da fase que o pediu.
57. ~~[#69](https://github.com/emersonjoe/trilha/issues/69) `Bind` de listas e mapas, e `ui.SchemaForm`.~~ **Entregue na 0.41.0.**
58. ~~[#71](https://github.com/emersonjoe/trilha/issues/71) `c.Attachment`, `c.Inline` e `c.Pipe`.~~ **Entregue na 0.41.0.**
59. ~~[#72](https://github.com/emersonjoe/trilha/issues/72) `ui.Markdown` seguro e `ui.Chat` + `ai.Serve`.~~ **Entregue na 0.41.0.**
60. ~~[#67](https://github.com/emersonjoe/trilha/issues/67) `ui.Combobox` por fragmento e `ui.Dropzone`.~~ **Entregue na 0.41.0.**
61. ~~[#68](https://github.com/emersonjoe/trilha/issues/68) Gráficos SVG gerados no servidor, sem JS.~~ **Entregue na 0.41.0.**
62. ~~[#70](https://github.com/emersonjoe/trilha/issues/70) Ilhas de segunda geração: props tipadas, canal ilha→servidor, `trilha vendor`.~~ **Entregue na 0.41.0**, e endireitado na **0.42.0** (spec 060): o canal tinha nascido duplicado — uma cópia no script inline, outra no `ui.js` —, e as duas já divergiam no `swap`. Virou um arquivo do kit, alcançável pelos dois casos.
63. ~~[#73](https://github.com/emersonjoe/trilha/issues/73) `trilha ui describe` e o guia "de Next.js para o Trilha".~~ **Entregue na 0.41.0.**

### Fase 9 — O que o iniciante não faz sozinho

O épico é a [#119](https://github.com/emersonjoe/trilha/issues/119). A tese da fase não é
"faltam componentes": é que existe um conjunto de coisas que toda aplicação de gestão precisa,
que quem começa **não sabe que precisa** até doer, e que hoje cada projeto reinventa pior —
permissão por módulo, o estado vazio que diz o que fazer, o enum escrito em quatro lugares, a
data formatada à mão em quinze telas.

Cada item tem uma medição por trás, tirada do mesmo app real da Fase 7. A ordem aqui é a do
que destrava mais, e não a dos números.

64. ~~[#100](https://github.com/emersonjoe/trilha/issues/100) A permissão vivia em quatro lugares e o `RequireFunc` entregava uma folha em branco.~~ **Entregue na 0.45.0** (spec 063): `auth.Policy` é a matriz declarada uma vez, e ela responde no middleware, no botão, no 403 que diz o que faltou e na `ui.PolicyGrid` que edita. Achado no caminho: o `var Middleware = RequirePolicy(...)` que a issue propunha não passa no scanner, que exige função — o idioma que funciona está documentado.
65. ~~[#96](https://github.com/emersonjoe/trilha/issues/96) Um status era escrito quatro vezes e a badge imprimia o valor cru.~~ **Entregue na 0.46.0** (spec 064): `trilha.Enum` com `ui.Status`, `Options` e a tag `enum=`, cuja mensagem cita rótulos e não valores. O pânico em registro duplicado, herdado do `AddRule`, tornava o `RegisterEnum` inutilizável no `Setup` — agora a mesma lista pode, listas diferentes sob um nome não.
66. ~~[#97](https://github.com/emersonjoe/trilha/issues/97) Trinta e cinco estados vazios escritos à mão, e nove telas confundindo lista vazia com lista filtrada até o vazio.~~ **Entregue na 0.47.0** (spec 065): `ui.Empty`, `ui.EmptyError` e o `DataTable` distinguindo os dois casos — com um link que limpa o termo **e** volta à primeira página.

67. ~~[#99](https://github.com/emersonjoe/trilha/issues/99) Data, tamanho, duração e contagem escritos à mão em quinze telas, sem fuso e sem tratar nulo.~~ **Entregue na 0.48.0** (spec 066): `ui.Date`, `ui.Bytes`, `ui.Duration` e `ui.Number`, com `Config.Locale` e `Config.TimeZone`. Recebem um `Ctx` — a issue propunha sem, e idioma em estado de pacote seria compartilhado por duas aplicações num mesmo processo, com a segunda a subir mudando a primeira em silêncio.
68. ~~[#104](https://github.com/emersonjoe/trilha/issues/104) "Quem fez o quê" existia pela metade: o framework já tinha request id, IP e sessão, e faltava a frase.~~ **Entregue na 0.49.0** (spec 067): `c.Audit` em uma linha, com `Config.Audit` de um método só. Sink que falha não derruba a resposta — o documento foi excluído de qualquer jeito, e recusar-se a responder agora perderia a trilha e confundiria quem fez.
69. ~~[#109](https://github.com/emersonjoe/trilha/issues/109) A planilha, nos dois sentidos: exportar sem BOM e sem o separador daqui, e importar dizendo "erro no arquivo".~~ **Entregue na 0.50.0** (spec 068): `c.CSV` (fatia ou canal) e `trilha.BindCSV`, com o erro por célula em `ui.CSVErrors`. Sem `iter.Seq` — o módulo compila no Go 1.22, e o canal atende a exportação em stream, que era o que aquela parte da issue queria.
70. ~~[#98](https://github.com/emersonjoe/trilha/issues/98) A tela que precisa de sete consultas espera pela mais lenta das sete.~~ **Entregue na 0.51.0** (spec 069): `ui.Defer` serve a página agora e pede a parte cara depois — a máquina do `Poll` sem o relógio. Sem o `E_NESTED_DEFER` do `gen`: o aninhamento é dinâmico e o scanner não vê através de rotas; no lugar ficou o conjunto de ids já pedidos, que é o que impede o pedido infinito.
71. ~~[#102](https://github.com/emersonjoe/trilha/issues/102) Quem escrevia `h.Iframe` para mostrar um PDF ficava com um quadro em branco e nada no console.~~ **Entregue na 0.52.0** (spec 070): `ui.Preview` desenha, e o `c.Inline` passou a dizer que aquela resposta pode ser enquadrada pela mesma origem. O conselho que este repositório dava em quatro lugares — `frame-src` na página — estava errado: quem recusava era a resposta enquadrada, e a política padrão já deixava a página enquadrar a própria origem.
72. ~~[#103](https://github.com/emersonjoe/trilha/issues/103) Formulário de várias telas sem lugar para guardar o passo anterior: campo escondido, linha meio preenchida no banco, ou uma tela enorme.~~ **Entregue na 0.53.0** (spec 071): `c.Draft` em cookie assinado até 2 KB e `Config.Drafts` acima disso, com `ui.Steps`. Sem `Attach`: rascunho com arquivo precisa de uma vida útil de temporários que o framework não tem e que a [#114](https://github.com/emersonjoe/trilha/issues/114) vai construir.
73. ~~[#128](https://github.com/emersonjoe/trilha/issues/128) A tela da trilha ficou de fora da 0.49.0 por depender do `c.CSV`.~~ **Entregue na 0.54.0** (spec 072): `ui.AuditTable` sobre o `DataTable`, com o detalhe expansível e a exportação levando o recorte da tela. Ler a trilha continua sendo da aplicação — o `Config.Audit` é interface de escrita e assim fica.
74. ~~[#101](https://github.com/emersonjoe/trilha/issues/101) Hierarquia de milhares de nós era o componente que se ia buscar no npm.~~ **Entregue na 0.55.0** (spec 073): `ui.Tree` e `ui.TreePicker`, com o nó sendo um `<details>` e o seletor postando um radio — sem script, tudo continua de pé. De caminho, o `AddRule` deixou de explodir quando o `Setup` roda duas vezes, que é o que toda suíte de testes faz.
75. ~~[#107](https://github.com/emersonjoe/trilha/issues/107) A configuração que o administrador muda sem redeploy era tabela, GET, PUT, uma tela por seção e validação em lugar nenhum.~~ **Entregue na 0.56.0** (spec 074): `trilha.Settings[T]` com a tela vindo da struct pelo `SchemaOf`, 422 que não grava, e auditoria que diz o que mudou sem dizer o valor. Sem `settings.SQL` — mesma razão do `audit.SQL` — e sem `trilha.Secret`, que é a #108.
76. ~~[#108](https://github.com/emersonjoe/trilha/issues/108) O framework assinava e não cifrava: quem precisava guardar um token gravava em claro.~~ **Entregue na 0.57.0** (spec 075): `trilha.Seal`/`Open` em AES-256-GCM com chave derivada por HKDF, e o tipo `trilha.Secret`, que mascara em JSON, log e `%v`, e cujo `Value()` é o do driver — sem isso, passar o segredo a uma query gravaria em claro, em silêncio. Sem KMS, e o modelo de ameaças diz contra o que isto protege e contra o que não.
77. ~~[#105](https://github.com/emersonjoe/trilha/issues/105) O framework tinha cookie e bearer de terceiro; faltava a chave que a própria app emite.~~ **Entregue na 0.58.0** (spec 076): `auth.APIKeys` guardando só o hash com pimenta, comparação em tempo constante antes da revogação, limite por chave e escopo obrigatório — com `trilha.Pepper` e `trilha.Limiter` exportados em vez de uma segunda implementação dentro do `auth`.
78. ~~[#106](https://github.com/emersonjoe/trilha/issues/106) O formulário que alguém de fora preenche, o código que confere um documento: sem login, e escritos à mão como string aleatória numa tabela.~~ **Entregue na 0.59.0** (spec 077): `c.Link`/`c.Claim`, com `Uses: 0` não guardando estado nenhum, ler não gastando, e as quatro falhas respondendo o mesmo 404. O bloqueio por tentativa virou orçamento que se recompõe — bloquear por endereço derruba todo mundo atrás de um NAT.
79. ~~[#110](https://github.com/emersonjoe/trilha/issues/110) `tenant_id` em 26 de 28 routers, e o bug de esquecer a coluna numa consulta.~~ **Entregue na 0.60.0** (spec 078): o tenant viaja na sessão, entra na trilha e no log de acesso, o `RequireTenant` recusa quem não escolheu — e o `trilha audit` conta, por tabela, quais consultas filtram e qual esqueceu, com arquivo e linha. O framework carrega e aponta; a consulta é da aplicação.
80. ~~[#114](https://github.com/emersonjoe/trilha/issues/114) O framework recebia e devolvia arquivo, e não dizia onde guardar — o exemplo gravava em `./uploads` com o nome do cliente.~~ **Entregue na 0.61.0** (spec 079): módulo `trilha/blob` com disco, memória e S3 (assinatura escrita aqui, sem SDK). A chave é o digest do conteúdo, então travessia é impossível por construção e deduplicar sai de graça. A assinatura foi conferida depois contra um MinIO de verdade (0.62.0).
81. ~~Os módulos que falam com servidor externo — `blob.S3` e `auth` — só tinham servidor de teste escrito aqui, que confere o que este código faz do jeito que este código faz.~~ **Entregue na 0.62.0** (spec 080): suítes ao vivo contra MinIO e Keycloak, puladas sem variável de ambiente. O Keycloak achou um defeito de verdade: ele põe os papéis no access token, não no `id_token`, então `RequireRole` recusava todo mundo num Keycloak de fábrica.
82. ~~[#113](https://github.com/emersonjoe/trilha/issues/113) Todo app interno manda e-mail, e o que se escrevia era `net/smtp` dentro do manipulador — sem prazo, com o corpo em `<table>` na mão, e sem jeito de testar sem mandar e-mail para uma pessoa de verdade.~~ **Entregue na 0.63.0** (spec 081): módulo `trilha/mail` com o corpo em `h.Node`, texto alternativo gerado do mesmo nó, `Layout` que funciona no Outlook, STARTTLS e LOGIN, prazo do `context`, `.eml` em dev, `ErrNotConfigured` em produção e `Outbox` para o teste do app.
83. ~~[#111](https://github.com/emersonjoe/trilha/issues/111) Seis telas disparam algo longo e ficam perguntando "acabou?", e o que se escrevia era um `go func()` que perdia o erro, o contexto e o processo inteiro num panic.~~ **Entregue na 0.64.0** (spec 082): módulo `trilha/task` com dedupe por chave, panic virando erro, `interrupted` na subida, `Shutdown` que espera, e as duas telas no `ui`. Não é fila entre processos, e isso está na primeira linha do doc do pacote.
84. ~~[#112](https://github.com/emersonjoe/trilha/issues/112) Avisar outra aplicação era um `http.Post` no manipulador: o visitante esperando o servidor do parceiro, sem assinatura, sem retentativa, sem registro — e um formulário de URL que aceitava o endereço de metadados da própria nuvem.~~ **Entregue na 0.65.0** (spec 083): módulo `trilha/webhook` com assinatura HMAC do "timestamp.corpo", backoff de 1 min a 12 h, o corpo da resposta guardado, checagem de endereço no cadastro e na entrega, `Verify` para quem recebe, e o `ui.WebhooksPanel`. A seção `webhooks` do `trilha openapi` ficou de fora e a issue segue aberta para ela.
85. ~~[#124](https://github.com/emersonjoe/trilha/issues/124) Duas specs deixaram de fora a mesma coisa: o `trilha ctx` não dizia qual módulo e nível uma rota exige, nem quais enums existem — as duas moram numa declaração de pacote ligada ao uso, que é uma análise só.~~ **Entregue na 0.66.0** (spec 085): `needs docs:ver` por rota, com herança e por método, mais a seção de enums; e o `LookupEnum`/`RegisteredEnums` exportados, que era a condição escrita na 0.46.0.
86. [#118](https://github.com/emersonjoe/trilha/issues/118) Há erros que só aparecem em runtime, quando o iniciante já está no navegador sem entender por que "não funciona" — e a resposta era um 500 genérico. **Parcialmente entregue na 0.67.0** (spec 086): o `trilha.Hint`, a página de erro de dev que mostra o conserto, e as duas recusas que eram vulnerabilidade — `Redirect` para fora do site e segredo curto em produção — mais o `trilha secret` e o aviso de proxy sem `Timeout`. Falta o `trilha dev` observando o navegador (CSP e `Poll` devolvendo página inteira), que precisa de um canal que ainda não existe.
87. [#115](https://github.com/emersonjoe/trilha/issues/115) Entre o template com um CRUD pronto e o `generate page` que escreve uma página vazia, faltava a tarefa que mais se repete: tenho um struct, quero a tela. **Parcialmente entregue na 0.68.0** (spec 087): `trilha generate crud <Tipo>` escreve lista, criar, editar, excluir, o store e o teste, tudo formatado e passando no `trilha check` sem uma edição. Faltam as bandeiras `--store sqlite|postgres`, `--tenant`, `--policy`, `--schema` e o modo que roda de novo e imprime o diff dos campos novos.
88. [#116](https://github.com/emersonjoe/trilha/issues/116) Metade desta fase são padrões de app, e não primitivos: no núcleo ficam rígidos, como documentação viram trabalho de copiar. **Parcialmente entregue na 0.69.0** (spec 088): `trilha add` com as receitas `audit`, `api-keys` e `settings`, inserção marcada no `setup.go`, `--dry-run`, `--list --json`, e a CI aplicando cada receita num projeto novo e rodando `trilha check`. Faltam as outras receitas que a issue lista, cada uma meia hora agora que o mecanismo existe.
89. [#117](https://github.com/emersonjoe/trilha/issues/117) O `--template app` era um bom dia 1 e não tinha nada do mês 1: auditoria, chaves, configurações, usuários, permissões. **Parcialmente entregue na 0.70.0** (spec 089): o template passou a ser feito das receitas — as três telas que já existem, sob `app/admin/` e guardadas por papel — mais o `--with` no `new`. As outras quatro telas são receitas a escrever, e a de usuários é a que as outras esperam.
90. ~~[#94](https://github.com/emersonjoe/trilha/issues/94) A Fase 7 fechou inteira e a régua do `bench/agent` continuava com os quatro cenários de antes — nenhum deles tocava `ListParams`, `ui.DataTable`, `ui.Poll` ou o fragmento, que foi o que a fase mudou.~~ **Entregue na 0.71.0** (spec 090): o cenário `port-listing` dá ao agente o `.tsx` que a tela era em Next.js e mede a porta — colunas, ordenação vinda da URL com `aria-sort`, paginação que preserva o filtro, fragmento que responde só o pedaço e nenhum JavaScript próprio. Fica para uma spec própria o que a issue pede sobre o caminho até a API (o stub HTTP e o `Upstream`), que precisa de um segundo serviço de pé durante a verificação.
91. ~~[#140](https://github.com/emersonjoe/trilha/issues/140) Medido no Verba: o `trilha migrate next` classificou 20 de 20 telas como C, quase todas com a razão `components/Chat.tsx` — o chat que o shell carrega.~~ **Entregue na 0.72.0** (spec 091): reexportar não é usar, e envolver não é compor. O barril é atravessado pelo nome (`import { DataTable } from "@/components"` segue só a linha do `DataTable`), o que os layouts alcançam vira a seção *Dependências globais* — porta-se uma vez e não muda a classe de ninguém — e o componente que só uma página importa continua contando para ela.
92. ~~[#141](https://github.com/emersonjoe/trilha/issues/141) O `trilha client` reduzia um `multipart/form-data` a **um** campo binário: a assinatura recebia `file, filename` e o resto do formulário sumia — o documento sintético deste repositório já declarava um `folder` que nunca chegou na assinatura.~~ **Entregue na 0.73.0** (spec 092): multipart virou contrato. Um campo binário e mais nada continua com os dois argumentos; qualquer outra forma vira um struct tipado com `FilePart`, `[]FilePart` para lista com o mesmo nome na ordem do slice, e escalares obrigatórios e opcionais. Continua em stream, num `io.Pipe`.
93. ~~[#142](https://github.com/emersonjoe/trilha/issues/142) O `ui.Chat` resolvia a conversa dentro de uma página; o formato que todo app administrativo pede é o outro — um botão no canto que abre um painel sobre a tela, presente na área autenticada inteira, com o contexto da página indo junto com a pergunta.~~ **Entregue na 0.74.0** (spec 093): `ui.Assistant` sobre o `ui.Chat` (launcher que é link antes de ser botão, `<dialog>` nativo com foco e Escape do navegador, `aria-expanded`), `ui.ChatOpts.Context` como campos `ctx.*` e `ai.ServeOpts.Context` para lê-los — pelo JSON do script e pelo formulário sem script.
94. [#116](https://github.com/emersonjoe/trilha/issues/116) A receita `login` — a que a issue lista na primeira entrega e que ficou de fora da 0.69.0. **Mais uma parte entregue na 0.75.0** (spec 095): `trilha add login` escreve a tabela de gente, a sessão sem provedor, a tela de entrar, a saída e dois testes — o da tabela e o que entra de verdade no projeto. Sem senha padrão: o primeiro administrador sai de `ADMIN_EMAIL`/`ADMIN_PASSWORD`, e sem eles a tabela sobe vazia e diz isso. É a receita que a tela de usuários da [#117](https://github.com/emersonjoe/trilha/issues/117) espera.
95. [#116](https://github.com/emersonjoe/trilha/issues/116) e [#117](https://github.com/emersonjoe/trilha/issues/117) A tela de usuários — a que as outras três da fase esperam, porque permissões, organizações e perfil são todas sobre a mesma tabela de gente. **Mais uma parte entregue na 0.76.0** (spec 096): `trilha add users` escreve o convite com token que expira, a troca de papel, o desativar e o resetar, mais a página onde a pessoa define a própria senha. É a primeira receita escrita em cima de outra, e por isso estreia o `Needs`: sem a `login`, ela recusa e diz o que rodar antes.
96. ~~[#112](https://github.com/emersonjoe/trilha/issues/112) O módulo `webhook` entregou a entrega assinada, o retry e a tela, e ficou faltando a última linha do aceite: o documento descrevia só o que o app recebe.~~ **Entregue na 0.77.0** (spec 097): o `trilha openapi` gera a seção `webhooks` do OpenAPI 3.1 a partir das chamadas a `Emit` — o corpo é o schema do payload (por `$ref`, quando o struct já é componente de uma rota) e os quatro cabeçalhos da entrega vêm descritos. Um `Emit` com nome vindo de variável não vira evento nenhum.
97. [#118](https://github.com/emersonjoe/trilha/issues/118) Os erros que só o navegador via — o CSP que deixa o iframe cinza, o fragmento que volta como página inteira, o flash que aparece na tela seguinte. **Mais uma parte entregue na 0.78.0** (spec 098): em dev a política reporta e o `trilha dev` imprime a recusa com o conserto; o `ui.live.js` conta quando um fragmento voltou com `<html>`; e o servidor avisa sozinho o flash sem redirect. Nada disso existe em produção. Faltam os itens estáticos do `audit`, a pasta `docs/errors/` gerada e o cenário "corrija este erro" do bench.
98. [#118](https://github.com/emersonjoe/trilha/issues/118) As linhas da tabela que dá para ver sem rodar. **Mais uma parte entregue na 0.79.0** (spec 099): o `trilha audit` passou a apontar fluxo de eventos e escrita de auditoria em rota sem nada acima, e campo `string` vindo de fora sem `max=`. As três nomeiam as rotas (ou os campos) que as dispararam, e a do ator se cala quando a rota nomeia o ator sozinha. Faltam a pasta `docs/errors/` gerada e o cenário "corrija este erro" do bench.
99. [#115](https://github.com/emersonjoe/trilha/issues/115) "Gerador que sobrescreve é gerador que ninguém roda duas vezes" — a 0.68.0 entregou a recusa, e faltava o que ela diz. **Mais uma parte entregue na 0.80.0** (spec 100): rodar o `generate crud` de novo não escreve nada e imprime o que o struct ganhou desde então, com o arquivo e a linha de cada tela, saindo com 0. Faltam `--store sqlite|postgres`, `--tenant`, `--policy` e `--schema`.
100. [#116](https://github.com/emersonjoe/trilha/issues/116) e [#117](https://github.com/emersonjoe/trilha/issues/117) A tela de permissões, a segunda das quatro que a #117 lista. **Mais uma parte entregue na 0.81.0** (spec 101): `trilha add permissions` escreve a matriz como dado, as três funções que a leem, o `ui.PolicyGrid` e o middleware — e a tela que edita a matriz é guardada pela própria matriz, porque uma tela de permissões atrás de um if no papel é uma matriz com uma exceção do lado de fora.
101. [#116](https://github.com/emersonjoe/trilha/issues/116) e [#117](https://github.com/emersonjoe/trilha/issues/117) A tela da própria conta, a terceira das quatro. **Mais uma parte entregue na 0.82.0** (spec 102): `trilha add profile` escreve o nome e a troca de senha, com o id vindo da sessão e nunca do formulário — que é o bug que mais se escreve nessa tela. Trocar a senha pede a senha atual e fecha a sessão. Falta a `organizacoes`, que espera o tenant.
Os demais itens da fase estão nas issues #98 a #118, com a dívida de scanner que duas specs
deixaram registrada na [#124](https://github.com/emersonjoe/trilha/issues/124).

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

- **Issues**: [#20 a #50](https://github.com/emersonjoe/trilha/issues?q=is%3Aissue+label%3Aroadmap), com o rótulo `roadmap`, agrupadas por
  marco (`Fase 1 — Interatividade`, `Fase 2 — Dados e identidade`, `Fase 3 — Produção`,
  `Fase 4 — DX`, `Fase 5 — Agentes`).
- **Specs**: quando um item entra em execução, ganha `specs/NNN-nome/` (spec → plan →
  tasks → implement) e a issue passa a apontar para ela.
- **Versões**: toda spec fechada vira uma versão (`GOVERNANCE.md`), então a documentação do
  site e `go get ...@latest` nunca divergem.

A nota da avaliação não é a métrica. A métrica é a da própria avaliação, e essa vale:
*conseguir construir uma aplicação web moderna inteira em Go — SSR, rotas, formulários,
autenticação, API, cache, interatividade e observabilidade — sem montar um quebra-cabeça de
ferramentas, e poder integrar o resto sem lutar contra o framework.*
