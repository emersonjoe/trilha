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
| Preencher e salvar um formulário | dado perdido ou erro sem saber onde | `TestUIAsyncForm422Focus`, `TestUIBillingScreens`, `TestUINotifyPreferences` |
| Navegar e listar | a tela mostra outra coisa que a barra de endereço | `TestUIFragmentSwap`, `TestUIPaginationKeyboard`, `TestUIClientNavFocus` |
| Enviar arquivo | arquivo perdido sem aviso | `TestUIUploadProgress` |
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
- **Resultado esperado**: a URL é `/fluxos/nav/b`, a região mostra "Page B", o foco está em `#regiao` e o documento não recarrega.

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
