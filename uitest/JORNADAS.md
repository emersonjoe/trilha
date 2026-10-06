# Catálogo de jornadas — testes de ponta a ponta no navegador

Cada cenário `TestUI*` deste módulo é um caso de teste documentado aqui, com título, jornada,
risco, pré-condições, dados, passos e resultado esperado. `TestCatalogoDeJornadas` reprova
quando um cenário não tem caso, quando um caso não tem cenário ou quando falta um campo:
mudar um cenário é mudar este arquivo no mesmo commit.

Os cenários rodam no app que um usuário do framework recebe (`trilha new` + `trilha add login
connections webhooks billing notify admin`, mais `testdata/fixture/`), compilado para produção
(`TRILHA_ENV=prod`), em Chromium, Firefox e WebKit (`UITEST_BROWSERS`). Os dados são sintéticos
(`example.com`, senhas literais do teste); cada execução sobe um servidor novo com stores em
memória e um contexto de navegador novo, então nada sobra entre cenários.

## Onde cada coisa é testada (pirâmide)

| Nível | Onde | O que prova |
|---|---|---|
| Unidade | `go test ./...` na raiz, um `_test.go` por arquivo | regras, bordas e exceções: scanner, gerador, DSL, validação, erros |
| Integração | `examples/blog` com `httptest`; `sqltest/` (SQLite e PostgreSQL); `cmd/trilha/*e2e_test.go` | cada convenção servida de verdade; stores num banco de verdade; a CLI gerando e compilando projetos |
| HTML servido | `trilha.CapturePage`/`PageSnapshot` (camada A, spec 164) | token CSRF, nonce, cookies, goldens — sem navegador |
| Navegador | este módulo (camada B) | só o que o navegador vê: foco, troca de fragmento, ilha, upload, teclado, CSP aplicada |

Caso extremo e exceção vão para a unidade ou a integração. Aqui fica a jornada que, quebrada,
tira de alguém o que ele veio fazer.

## Jornadas críticas

| Jornada | Pior impacto se quebrar | Casos |
|---|---|---|
| Entrar e sair | ninguém entra, ou entra quem não devia | `TestUILoginFlow`, `TestUIAdminDefaultDeny` |
| Preencher e salvar um formulário | dado perdido, gravado duas vezes, ou erro sem saber onde | `TestUIAsyncForm422Focus`, `TestUIBillingScreens`, `TestUINotifyPreferences`, `TestUISwapPostOnce`, `TestUISwapFollowsRedirect`, `TestUISwapRedirectReload`, `TestUISwapFollowFallsBack`, `TestUIRegionFormsNavigate`, `TestUIRegionForm422` |
| Navegar e listar | a tela mostra outra coisa que a barra de endereço; a página chega morta ou muda | `TestUIFragmentSwap`, `TestUIPaginationKeyboard`, `TestUIClientNavFocus`, `TestUIClientNavRunsRegionScripts`, `TestUIBeforeSwapEvent`, `TestUIClientNavAnnounces`, `TestUIClientNavFocusRegion`, `TestUISwapNewestClickWins`, `TestUIRegionLinkRedirect`, `TestUIServerDeclaresURL`, `TestUIPrefetchOnIntent`, `TestUIPrefetchNeedsIntent`, `TestUIPrefetchExpires`, `TestUIPrefetchSaveData`, `TestUIPrefetchRedirectNotKept` |
| Enviar arquivo | arquivo perdido sem aviso | `TestUIUploadProgress`, `TestUIUploadFollowsRedirect` |
| Interação rica | componente morto na tela | `TestUIIslandHydrates`, `TestUITooltipKeyboard` |
| Segurança percebida | script injetado roda; segredo vaza | `TestUISecurityEveryScreen`, `TestUISecretNeverInHTML` |

## Casos

### TestUILoginFlow — Entrar com a senha certa depois de errar

- **Jornada**: entrar e sair. **Risco**: ninguém entra no app.
- **Pré-condições**: administrador semeado por `ADMIN_EMAIL`/`ADMIN_PASSWORD`; navegador sem sessão.
- **Dados**: `admin@example.com`; uma senha errada e a senha certa do fixture.
- **Passos**: abrir `/entrar`; enviar a senha errada; enviar a senha certa; abrir `/admin`.
- **Resultado esperado**: a senha errada fica em `/entrar` com "Wrong e-mail or password"; a certa leva a `/`; `/admin` mostra "Admin".

### TestUIAdminDefaultDeny — Convidado entra, mas não é administrador

- **Jornada**: entrar e sair. **Risco**: quem não devia entra no backoffice.
- **Pré-condições**: administrador semeado; navegador sem sessão.
- **Dados**: convidado `leitor@example.com`, nome "Leitor", senha definida pelo link do convite.
- **Passos**: abrir `/admin` sem sessão; entrar como administrador; convidar o leitor; sair (limpar cookies); abrir o link do convite e definir a senha; entrar como leitor; abrir `/admin` e `/admin/usuarios`.
- **Resultado esperado**: o anônimo vai para `/entrar?next=%2Fadmin`; o convite mostra um link `/convite/…`; o leitor entra e recebe "No access" dentro do layout do app nas duas telas.

### TestUIAsyncForm422Focus — Formulário recusado volta no lugar com o foco no campo

- **Jornada**: preencher e salvar. **Risco**: a pessoa não sabe o que corrigir, ou perde o que digitou.
- **Pré-condições**: página `/padroes/formulario` (padrão da spec 163) com `ui.Swap`.
- **Dados**: e-mail `ana@example.com`, nome vazio; depois nome "Ana Lima".
- **Passos**: enviar sem nome; corrigir o nome e enviar de novo.
- **Resultado esperado**: o 422 troca o formulário sem recarregar, `#name` fica `aria-invalid="true"` e com o foco; corrigido, o envio navega de verdade para `/padroes/formulario`.

### TestUIBillingScreens — Cadastrar um plano recusado e depois aceito

- **Jornada**: preencher e salvar. **Risco**: plano cobrado errado ou formulário que não diz o erro.
- **Pré-condições**: receita `billing` instalada; administrador com sessão.
- **Dados**: plano "Pro", 4900 centavos, moeda `xx` (inválida) e depois `brl`, intervalo mensal.
- **Passos**: abrir `/billing`; em `/billing/planos` enviar com moeda `xx`; corrigir para `brl` e enviar; abrir `/billing/faturas`.
- **Resultado esperado**: a recusa marca `#moeda` inválido, mantém "Pro" no nome e põe o foco em `#moeda`; aceito, o plano aparece na tabela; as faturas oferecem o CSV.

### TestUINotifyPreferences — Escolher preferências de notificação e vê-las de volta

- **Jornada**: preencher e salvar. **Risco**: a escolha da pessoa não é guardada.
- **Pré-condições**: receita `notify` instalada; administrador com sessão.
- **Dados**: canal `mail`, resumo diário marcado, silêncio das 22 às 7.
- **Passos**: abrir `/notificacoes`; escolher o canal, marcar o resumo, preencher o silêncio; salvar.
- **Resultado esperado**: o redirect volta a `/notificacoes` com silêncio 22 e 7 e o resumo marcado.

### TestUIFragmentSwap — Ordenar a lista sem recarregar

- **Jornada**: navegar e listar. **Risco**: a barra de endereço e a tabela dizem coisas diferentes.
- **Pré-condições**: página `/padroes/lista` (spec 163) com `ui.Swap` no cabeçalho.
- **Dados**: os pedidos sintéticos do fixture.
- **Passos**: abrir a lista; clicar no cabeçalho de ordenação.
- **Resultado esperado**: a URL ganha `sort=`, a tabela mostra "Customer" e o documento não recarrega.

### TestUIPaginationKeyboard — Paginar pelo teclado

- **Jornada**: navegar e listar. **Risco**: quem usa teclado perde o lugar a cada página.
- **Pré-condições**: página `/padroes/lista` com paginador `ui.Swap`.
- **Dados**: os pedidos sintéticos do fixture.
- **Passos**: focar "Next"; apertar Enter.
- **Resultado esperado**: a URL tem `page=2`, a página atual marcada é "2", o foco continua em "Next" e o documento não recarrega.

### TestUIClientNavFocus — Navegar no cliente leva o foco ao conteúdo novo

- **Jornada**: navegar e listar. **Risco**: o leitor de tela fica no lugar antigo, ou a página recarrega inteira.
- **Pré-condições**: `/fluxos/nav` com `ui.Navigate` na região `#regiao`.
- **Dados**: nenhum.
- **Passos**: abrir `/fluxos/nav`; clicar no link `#ir`.
- **Resultado esperado**: a URL é `/fluxos/nav/b`, a região mostra "Page B", o foco está no `h1` da região e o documento não recarrega.

### TestUIClientNavRunsRegionScripts — A página navegada no cliente chega viva (#290)

- **Jornada**: navegar e listar. **Risco**: ilha, `ui.Defer` e scripts da página mortos até um F5; ou script inline vindo da resposta executando (XSS).
- **Pré-condições**: `/fluxos/nav` sem ilha; `/fluxos/nav/vivo` com ilha, `ui.LiveScript` + `ui.Defer`, o arquivo `/conta-execucoes.js` e um script inline, tudo dentro da região; `/fluxos/nav/vivo2` com o mesmo arquivo.
- **Dados**: contagem 41 da ilha; o fragmento adiado "Deferred part arrived".
- **Passos**: abrir `/fluxos/nav`; clicar `#ir-vivo`; clicar `#ir-vivo2`.
- **Resultado esperado**: a ilha monta com "count 41", o adiado chega, o arquivo roda uma vez (e não de novo na segunda página), o inline não roda, e o documento não recarrega.

### TestUIBeforeSwapEvent — O script da página sabe que vai ser trocado (#290)

- **Jornada**: navegar e listar. **Risco**: timer e listener de página ficam rodando sobre o que saiu.
- **Pré-condições**: `/fluxos/nav` (`ui.Navigate`) e `/padroes/lista` (`ui.Swap`).
- **Dados**: nenhum.
- **Passos**: ouvir `trilha:before-swap`; navegar no cliente para B; na lista, clicar na ordenação.
- **Resultado esperado**: um evento por troca, com o id do alvo e o elemento antigo ainda na página, nos dois caminhos.

### TestUIClientNavAnnounces — A troca de página é anunciada ao leitor de tela (#297)

- **Jornada**: navegar e listar. **Risco**: quem usa leitor de tela não sabe que a página mudou (eMAG, LBI).
- **Pré-condições**: `/fluxos/nav` (título "First page") e `/fluxos/nav/b` (título "Second page", `h1` "Page B").
- **Dados**: nenhum.
- **Passos**: clicar `#ir`; voltar com o histórico.
- **Resultado esperado**: o foco vai ao `h1` "Page B" e `#trilha-route-announcer` (`aria-live="assertive"`, visualmente oculto) diz "Second page"; no Voltar diz "First page", sem recarregar.

### TestUIClientNavFocusRegion — A região pode pedir o foco nela mesma (#297)

- **Jornada**: navegar e listar. **Risco**: app que dependia do foco na região muda de comportamento sem escolha.
- **Pré-condições**: `/fluxos/nav/foco` com `ui.NavigateFocus("region")`.
- **Dados**: nenhum.
- **Passos**: abrir `/fluxos/nav`; clicar `#ir-foco`.
- **Resultado esperado**: o foco fica em `#regiao`.

### TestUISwapNewestClickWins — O clique mais recente vence (#294)

- **Jornada**: navegar e listar. **Risco**: "cliquei na 3 e está mostrando a 2" — barra e conteúdo divergem.
- **Pré-condições**: `/fluxos/lenta` com links `ui.Swap("lista")`; a página 2 demora 900 ms e a 3, 400 ms.
- **Dados**: nenhum.
- **Passos**: clicar "2"; com a marca de espera já posta, clicar "3".
- **Resultado esperado**: conteúdo "page 3", URL `?p=3`, uma entrada nova no histórico, e a marca `data-trilha-pending` só sai uma vez, no fim.

### TestUISwapPostOnce — Dois cliques em salvar gravam uma vez (#294)

- **Jornada**: preencher e salvar. **Risco**: registro duplicado.
- **Pré-condições**: `/fluxos/lenta` com um formulário POST `ui.Swap("lista")` que demora 500 ms e conta as gravações no servidor.
- **Dados**: nenhum.
- **Passos**: clicar duas vezes seguidas em "Save"; recarregar a página.
- **Resultado esperado**: "saves: 1" na troca e depois da recarga, e a URL continua `/fluxos/lenta`.

### TestUIUploadProgress — Enviar arquivo com barra de progresso

- **Jornada**: enviar arquivo. **Risco**: arquivo perdido sem aviso, ou recusa sem explicação.
- **Pré-condições**: página `/padroes/envio` (spec 163) aceitando PDF.
- **Dados**: `notes.txt` (recusado) e `doc.pdf` de 512 KiB gerados no teste.
- **Passos**: enviar o `.txt`; enviar o `.pdf`.
- **Resultado esperado**: o `.txt` volta no mesmo painel com `#files` inválido e com o foco, sem recarregar; o `.pdf` navega para `/padroes/envio` e a barra informou todos os bytes.

### TestUIIslandHydrates — A ilha monta e responde

- **Jornada**: interação rica. **Risco**: componente interativo morto na tela.
- **Pré-condições**: `/fluxos/ilha` com `c.Island("/contador.js", …)`.
- **Dados**: contagem inicial 41 nas props.
- **Passos**: abrir a página; clicar no contador.
- **Resultado esperado**: a ilha fica `data-trilha-mounted`, mostra "count 41", depois "count 42", e o console não tem erro.

### TestUITooltipKeyboard — Dica abre pelo teclado e fecha com Escape

- **Jornada**: interação rica. **Risco**: informação só acessível com mouse.
- **Pré-condições**: `/fluxos/dica` com `ui.Tooltip` no botão `#copiar`.
- **Dados**: nenhum.
- **Passos**: focar `#copiar`; apertar Escape.
- **Resultado esperado**: a dica aparece com `role="tooltip"`, ligada por `aria-describedby`, com "Copies the address"; Escape a fecha.

### TestUISecurityEveryScreen — Nenhuma tela tem script sem nonce nem formulário sem token

- **Jornada**: segurança percebida. **Risco**: script injetado roda; POST forjado é aceito.
- **Pré-condições**: página de controle `/fluxos/semnonce`; administrador com sessão; a lista `screens` de telas.
- **Dados**: nenhum.
- **Passos**: abrir a página de controle; entrar; abrir cada tela da lista.
- **Resultado esperado**: a página de controle registra a recusa de `script-src`; nenhuma outra tela tem violação de CSP, e todo formulário POST tem `_csrf`.

### TestUISecretNeverInHTML — O segredo de uma conexão nunca volta no HTML

- **Jornada**: segurança percebida. **Risco**: credencial de terceiro exposta na página.
- **Pré-condições**: receita `connections` instalada; administrador com sessão.
- **Dados**: conexão "provedor", `https://api.example.com`, bearer `sk_uitest_…_never_shown` (sintético).
- **Passos**: cadastrar a conexão; abrir a lista e o formulário de edição.
- **Resultado esperado**: a conexão aparece na lista, e o segredo não está no HTML de nenhuma das duas páginas.

### TestUISwapFollowsRedirect — Salvar com `ui.Swap` segue o redirect sem recarregar (#291)

- **Jornada**: preencher e salvar. **Risco**: o PRG recarrega a página inteira, ou o aviso aparece duas vezes, ou Voltar reenvia o POST.
- **Pré-condições**: `/fluxos/itens/novo` com região `ui.Navigate` e o formulário `#form-segue` (`ui.Swap`), cuja rota faz `c.Flash` + `c.Redirect("/fluxos/itens")`.
- **Dados**: item "gamma".
- **Passos**: preencher e salvar; voltar; recarregar `/fluxos/itens`.
- **Resultado esperado**: a barra vai para `/fluxos/itens`, a lista tem "gamma", o toast aparece uma vez, o documento não recarrega; Voltar mostra o formulário vazio sem reenviar; a recarga não repete o aviso.

### TestUISwapRedirectReload — `RedirectReload` carrega a página inteira (#291)

- **Jornada**: preencher e salvar. **Risco**: a moldura (menu, usuário) fica velha depois de mudar de contexto.
- **Pré-condições**: `#form-recarrega`, cuja rota responde `c.RedirectReload("/fluxos/itens")`.
- **Dados**: item "delta".
- **Passos**: preencher e salvar.
- **Resultado esperado**: navegação completa para `/fluxos/itens`, com "Item added: delta" no toaster.

### TestUISwapFollowFallsBack — Destino sem a região carrega inteiro, com o aviso uma vez (#291)

- **Jornada**: preencher e salvar. **Risco**: o aviso se perde no recuo, ou aparece de novo depois.
- **Pré-condições**: `#form-fora`, cuja rota redireciona para `/fluxos/semregiao`, página sem `#regiao`.
- **Dados**: item "epsilon".
- **Passos**: preencher e salvar; recarregar o destino.
- **Resultado esperado**: navegação completa para `/fluxos/semregiao` com o aviso uma vez; a recarga não o mostra.

### TestUIUploadFollowsRedirect — Envio de arquivo segue o redirect (#291)

- **Jornada**: enviar arquivo. **Risco**: o envio recarrega a página inteira e perde o contexto.
- **Pré-condições**: `/fluxos/itens/anexo` com região `ui.Navigate` e formulário `ui.UploadTo`, cuja rota faz `c.Flash` + `c.Redirect`.
- **Dados**: `zeta.txt` gerado no teste.
- **Passos**: escolher o arquivo e enviar.
- **Resultado esperado**: a barra vai para `/fluxos/itens`, a lista tem "zeta.txt", o toast "Attached: zeta.txt" aparece e o documento não recarrega.

### TestUIRegionFormsNavigate — Formulário comum dentro da região navega no lugar (#292)

- **Jornada**: preencher e salvar; navegar e listar. **Risco**: o botão "Filtrar" e o "Salvar" recarregam enquanto os links não.
- **Pré-condições**: `/fluxos/itens` com filtro GET comum e `/fluxos/itens/novo` com `#form-simples` (POST sem `ui.Swap`), ambos dentro da região.
- **Dados**: filtro "alp"; item "eta".
- **Passos**: filtrar; voltar; ir a "New item"; salvar; voltar; avançar.
- **Resultado esperado**: o filtro vira `?q=alp` e Voltar desfaz; salvar leva a `/fluxos/itens` com "eta" e o toast uma vez, sem recarregar; Voltar não reenvia; Avançar refaz o GET do destino.

### TestUIRegionForm422 — Recusa de formulário comum na região volta com o foco no campo (#292)

- **Jornada**: preencher e salvar. **Risco**: a pessoa não vê o que errou.
- **Pré-condições**: `#form-simples`, cuja rota responde 422 com a página inteira e o campo marcado.
- **Dados**: nome vazio.
- **Passos**: salvar sem nome.
- **Resultado esperado**: o campo fica `aria-invalid="true"` e com o foco, a URL continua `/fluxos/itens/novo`, sem recarregar.

### TestUIRegionLinkRedirect — Link para endereço que mudou é um GET só, com o aviso (#292)

- **Jornada**: navegar e listar. **Risco**: pedido em dobro e aviso perdido.
- **Pré-condições**: `/fluxos/itens/antigo` faz `c.Flash` + `c.Redirect("/fluxos/itens")`; `/fluxos/itens` conta as visitas.
- **Dados**: nenhum.
- **Passos**: clicar `#ir-antigo`.
- **Resultado esperado**: a barra vai para `/fluxos/itens`, "This address moved" aparece, o contador sobe um e o documento não recarrega.

### TestUIServerDeclaresURL — O servidor diz qual endereço reconstrói a tela (#293)

- **Jornada**: navegar e listar. **Risco**: F5 e link compartilhado mostram outra coisa que a tela.
- **Pré-condições**: `/fluxos/itens/painel` com link `ui.Swap` cuja rota chama `c.ReplaceURL`, buscas GET com e sem `ui.PushHistory()`, e um POST que chama `c.PushURL("/fluxos/itens/painel?doc=7")`.
- **Dados**: documentos 1–5 e 7 (só números na query).
- **Passos**: clicar o link; buscar duas vezes em cada formulário; criar; voltar.
- **Resultado esperado**: o link deixa uma entrada nova com a URL canônica; a busca comum não cria entrada e a com `PushHistory` cria duas; o POST põe `?doc=7` na barra e Voltar reconstrói "no document".

### TestUIPrefetchOnIntent — Apontar para o link antecipa a página (#295)

- **Jornada**: navegar e listar. **Risco**: a espera de cada clique continua à vista; ou o clique pede a página duas vezes.
- **Pré-condições**: `/fluxos/pre` com `ui.Prefetch()` na região; `/fluxos/pre/alvo` conta GETs e prefetches.
- **Dados**: nenhum.
- **Passos**: parar o ponteiro em `#alvo`; clicar.
- **Resultado esperado**: um prefetch e nenhum GET no servidor; o clique troca a região sem pedido novo, o `trilha:swap` diz `prefetched: true` e o documento não recarrega.

### TestUIPrefetchNeedsIntent — Passar por cima não é intenção (#295)

- **Jornada**: navegar e listar. **Risco**: tráfego e carga no servidor por movimento de mouse.
- **Pré-condições**: `/fluxos/pre`; `#fora` com `ui.NoPrefetch()`.
- **Dados**: nenhum.
- **Passos**: passar o ponteiro por `#alvo` sem parar; parar em `#fora`; esperar 400 ms.
- **Resultado esperado**: nenhum pedido para `/fluxos/pre/alvo` nem para `/fluxos/pre/fora`.

### TestUIPrefetchExpires — Resposta velha é pedida de novo (#295)

- **Jornada**: navegar e listar. **Risco**: mostrar conteúdo desatualizado.
- **Pré-condições**: `#curto` com `ui.PrefetchTTL(300)`.
- **Dados**: nenhum.
- **Passos**: parar em `#curto`; sair; esperar 600 ms; clicar.
- **Resultado esperado**: um prefetch e um GET (o clique pediu de novo).

### TestUIPrefetchSaveData — Economia de dados desliga o prefetch (#295)

- **Jornada**: navegar e listar. **Risco**: gastar o plano de dados de quem pediu economia.
- **Pré-condições**: `navigator.connection.saveData` ligado.
- **Dados**: nenhum.
- **Passos**: parar em `#alvo`; esperar 400 ms.
- **Resultado esperado**: nenhum pedido.

### TestUIPrefetchRedirectNotKept — Prefetch redirecionado não é guardado (#295)

- **Jornada**: navegar e listar. **Risco**: a sessão expirada antecipada mostra a página errada no clique.
- **Pré-condições**: `#sai`, cuja rota redireciona para `/fluxos/semregiao`.
- **Dados**: nenhum.
- **Passos**: parar em `#sai`; clicar.
- **Resultado esperado**: um prefetch e um GET; a navegação segue o caminho normal até `/fluxos/semregiao`.
