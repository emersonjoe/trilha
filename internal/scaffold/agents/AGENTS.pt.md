# AGENTS.md

Instruções para agentes de código que trabalham em `{{.Name}}`, uma aplicação web feita com
[Trilha](https://github.com/emersonjoe/trilha) — um framework Go com roteamento por arquivos e
sem dependências externas.

## As três convenções

- **Uma pasta dentro de `app/` é uma URL.** `app/blog/page.go` responde `/blog`.
- **O nome do arquivo diz o que ele é.** `page.go` renderiza uma página (`func Page(c
  *trilha.Ctx) (h.Node, error)`), `route.go` é uma API (`func GET`, `func POST`, ...),
  `layout.go` envolve tudo que está abaixo dele, `middleware.go` roda antes de tudo que está
  abaixo dele.
- **Uma pasta chamada `slug_` é um parâmetro.** `app/blog/slug_/page.go` responde
  `/blog/{slug}`, lido com `c.Param("slug")`. Já uma pasta com ponto no nome é caminho fixo
  (`app/api/relatorio.csv/route.go` responde `/api/relatorio.csv`). Pasta cujo nome
  *começa* com ponto é ignorada, menos a `.well-known`
  (`app/.well-known/security.txt/route.go` responde `/.well-known/security.txt`). Rota
  buscada de outra origem declara a própria política — `var CORS = trilha.CORS{...}` no
  `route.go` — e o framework responde o preflight.

O HTML é escrito em Go com o pacote `h`, não com template:
`h.Div(h.Class("card"), h.H1(nil, h.Text(titulo)))`. Tudo que ele renderiza sai escapado.

## Comandos

| Comando | O que faz |
|---|---|
| `trilha check` | o portão único: gen, gofmt, vet, test, audit, openapi, nesta ordem, parando na primeira falha. Rode antes de dizer que terminou — é também a única linha que a CI roda |
| `trilha check --fix` | o mesmo, regravando `trilha_gen.go` e a formatação pelo caminho |
| `trilha ctx` | o mapa do projeto — rotas, API, tipos de entrada e saída, setup — numa leitura só; `--json` para ferramenta, `--all` sem nada elidido. Leia antes de abrir arquivo por arquivo |
| `trilha dev` | servidor de desenvolvimento com recarga; deixe rodando enquanto trabalha |
| `trilha gen` | regrava `trilha_gen.go` a partir de `app/`; rode depois de criar ou remover rota |
| `trilha generate page /caminho` | grava o esqueleto na pasta certa (também `route`, `component`, `test`); `--methods`, `--bind` e `--form` escrevem o contrato junto |
| `trilha routes` | lista as rotas encontradas e o arquivo de onde vieram |
| `trilha audit` | verifica segurança e configuração: segredos, CSP, cookies, dependências |
| `trilha build` | gera e compila um binário único |
| `trilha export` | grava as páginas estáticas em HTML |
| `trilha openapi` | escreve o documento OpenAPI das rotas de API |
| `trilha ui` | regrava o kit ui em `public/` |
| `trilha ui describe [Nome]` | o catálogo do ui: todos os componentes, ou um com assinatura e exemplo; `--json` para ferramenta. Leia antes de escrever uma tela, em vez de chutar um nome |
| `trilha migrate next <dir>` | lê um projeto Next.js e grava a árvore do `app/` mais um `MIGRATION.md` dizendo, tela por tela, o que ela chamava e o que não tem equivalente aqui; `--dry-run` não grava nada |
| `trilha agents` | regrava este arquivo |
| `trilha new` | cria outro projeto |
| `trilha version` | a versão do framework |

Rota que responde 404 quase sempre é `trilha gen` que faltou; o `trilha check` pega isso antes
do navegador. Todo problema que ele reporta vem com a linha em que está e a frase que resolve —
leia essa frase em vez de adivinhar.

## O que não fazer

- **Não edite `trilha_gen.go`.** Ele é gerado e commitado, e o próximo `trilha gen` sobrescreve
  o que você escrever ali. Mexa em `app/`.
- **Não acrescente dependência.** O framework roda só com a biblioteca padrão; a resposta
  costuma estar em `net/http`, `database/sql` ou no próprio framework.
- **Não ponha segredo no código.** Leia do ambiente. O `trilha audit` falha em literal com cara
  de chave.
- **Não escreva seu próprio CSRF, assinatura de sessão ou escape de HTML.** Os três já existem e
  já vêm ligados. A exceção é a escrita que mora num `route.go`: `route.go` é API, e API não
  confere o token, então ponha `var Kind = trilha.KindPage` num `kind.go` na raiz daquele ramo —
  ele é herdado por tudo abaixo, e o `trilha audit` aponta a escrita que nenhum `Kind` alcança.
- **Não escreva um proxy reverso para alcançar uma API que já existe.** O `Config.Upstreams`
  encaminha um prefixo com a credencial da sessão injetada, o token do CSRF exigido, o corpo
  passando por cima do `MaxBodyBytes` e um 502/504 em `problem+json`. Um
  `httputil.ReverseProxy` escrito à mão num `route.go` catch-all esbarra em cada um desses
  sozinho.
- **Não copie uma receita de sessão para um app com usuários próprios.** O `auth.Sessions` é o
  mesmo `*Auth` sem provedor: confira a senha com `auth.CheckPBKDF2` e chame `Login(c, u)`. O
  resto — `Require`, `RequireRole`, a `Store`, a janela de ociosidade — já está escrito.
- **Não escreva uma tela de listagem na mão.** Página, ordem, filtro e busca moram na URL
  com a `trilha.ListParams` (embutida na struct que o `c.Bind` preenche) e são desenhados
  pelo `ui.DataTable` — cabeçalho ordenável em links de verdade, filtro em
  `<form method=get>`, paginação, estado vazio, e tudo isso trocado como fragmento quando
  tem `ID`. O `Restrict` é o que transforma o `sort` do endereço em nome de coluna, então
  nenhum repositório recebe uma que ninguém declarou.
- **Não escreva um `setInterval` para atualizar um pedaço da página.** O
  `ui.Poll("6s", src)` atualiza um fragmento pelo relógio — pausando na aba escondida,
  recuando no erro, parando quando a rota responde `c.PollStop()` — e o `ui.Live`/`ui.On`
  fazem o mesmo por um Server-Sent Event que carrega só o nome do que mudou. Carregue o
  `ui.LiveScript(c)` uma vez na página que observa alguma coisa.
- **Não monte a moldura de um app interno na mão, nem vá buscar uma biblioteca de
  gráficos.** O `ui.Shell` é a barra lateral, o cabeçalho e o menu do usuário, com o item
  ativo achado pelo prefixo mais longo; o `ui.PageHeader` é o título da tela dentro dele.
  `ui.Stat`, `ui.Bars`, `ui.Sparkline` e `ui.Donut` desenham um painel em SVG no servidor,
  com uma tabela invisível dos mesmos números para o leitor de tela. Esconder item de menu
  é enfeite: quem mantém alguém do lado de fora é o middleware na raiz da pasta.
- **Não numere na mão os inputs de uma linha que se repete, nem escreva um motor de
  formulário.** Uma lista de sub-registros é lida de `itens[0].nome`, `itens[1].nome`… para
  um `[]Linha`, uma matriz de `perm[docs]` para um mapa, e o nome do input é a chave da
  mensagem, então `ui.Errors(errs, "itens[1].qtd")` já aponta para a linha certa. Quando o
  formulário em si vem de dado, o `trilha.Schema` decodifica de JSON, o `trilha.BindSchema`
  valida como qualquer outro formulário e o `ui.SchemaForm` desenha.
- **Não escreva um autocomplete, nem leia campo de arquivo num laço.** O `ui.Combobox` é o
  campo de texto que busca numa lista: um input escondido carrega o valor escolhido, uma rota
  `Source` responde `ui.ComboboxOptions` com só os `<li>`, e o `With` leva os outros campos do
  formulário na query. O `ui.Dropzone` é a área de soltar, e com o `ui.UploadTo` a fila dele
  manda um arquivo por requisição. No servidor, o `c.Files(campo, regras)` aplica as regras do
  `c.File` a cada arquivo e nomeia a falha pela posição — `arquivos[2]` — então a mensagem cai
  na linha que a mereceu.
- **Não jogue texto de modelo na página com `h.Raw`, nem escreva um chat na mão.** O
  `ui.Markdown(texto, ui.MarkdownOpts{})` devolve nós, não string: uma tag no texto sai como
  texto, e não há como desligar isso. O `ai.Serve(c, cliente, agente)` é a rota de chat
  inteira — lê `{message, history}`, transmite `text`, `tool_call`, `tool_result`, `done` e
  `error`, e responde de uma vez quando ninguém pediu fluxo — e o `ui.Chat(c,
  ui.ChatOpts{Action: "/api/chat"})` com o `ui.ChatScript(c)` é a tela. O histórico é seu; o
  framework não guarda nenhum.
- **Não escreva na mão os cabeçalhos de um download.** O `c.Attachment(nome, corpo, tipo)`
  manda um arquivo e o `c.Inline` abre no navegador; os dois saneiam o nome, escrevem nas duas
  formas que os navegadores leem de verdade, detectam o tipo no conteúdo e mandam `nosniff`. O
  `Inline` recusa HTML, SVG e XML — é para isso que ele é uma segunda função. O `c.Pipe(res)`
  repassa a resposta de outro serviço, com lista fechada de cabeçalhos e sem `Set-Cookie`.
- **Não invente um cookie de flash nem um `onclick="return confirm()"`.** Depois de um `POST`,
  conte o que aconteceu com `c.Flash(ui.FlashSuccess, "…")` — o `ui.Flashes(c)` do layout mostra
  na página onde o redirect cai — e pergunte antes de destruir com `ui.Confirm(título,
  descrição)` no formulário. Script inline a CSP bloqueia de qualquer jeito.

## Onde procurar

- Receitas para os problemas de sempre (banco, sessões, uploads, paginação, email, Docker):
  <https://emersonjoe.github.io/trilha/pt/receitas>
- Cada função e cada tipo: <https://emersonjoe.github.io/trilha/pt/referencia>
- A documentação inteira em texto puro, mais barata de ler de uma vez:
  <https://emersonjoe.github.io/trilha/pt/llms.txt>
