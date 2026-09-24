# AGENTS.md

Instruções para agentes de código trabalhando no `{{.Name}}`, um app web construído com o
[Trilha](https://github.com/emersonjoe/trilha) — um framework Go com roteamento por arquivos e
nenhuma dependência externa.

## Leia estreito, nesta ordem

1. **O mapa primeiro, os arquivos depois.** `trilha ctx --json` é o mapa deste projeto —
   rotas, contratos de API, tipos, setup — em uma leitura, com o custo em tokens no rodapé.
   `trilha ctx --pack <receita>` fatia para um trabalho e `--budget N` limita o que volta.
   Abra arquivos só depois do mapa.
2. **Procure, não leia.** Grep com número de linha (`grep -n -A5`) e leia a janela, nunca o
   arquivo inteiro.
3. **Consulte o catálogo.** `trilha ui components --json` lista todo componente do kit antes
   de você montar uma tela; `trilha add` lista as receitas antes de você escrever uma à mão.

## Receitas instaladas

O mapa (`trilha ctx`) lista as receitas que este projeto instalou, cada uma com o link da
documentação, e `trilha ctx --pack <receita>` responde "o que esta receita toca?". O catálogo
do que existe: <https://emersonjoe.github.io/trilha/cookbook>.

## Os portões

| Comando | O que faz |
|---|---|
| `trilha check` | o portão único — gen, gofmt, vet, test, audit, openapi, parando na primeira falha, cada problema com a linha dele e o conserto. Rode antes de dizer que terminou |
| `trilha check --fix` | o mesmo, reescrevendo `trilha_gen.go` e a formatação no caminho |
| `make test` | a suíte inteira; o CI roda ela |
| `trilha gen` | reescreve `trilha_gen.go` depois de adicionar ou remover rota |

Rota respondendo 404 é quase sempre um `trilha gen` que faltou; o `trilha check` pega antes do
navegador.

## As três convenções

- **Uma pasta dentro de `app/` é uma URL.** `app/blog/page.go` responde `/blog`; o nome do
  arquivo diz o que ele é — `page.go` renderiza, `route.go` é API, `layout.go` envolve a
  subárvore, `middleware.go` roda antes dela.
- **`slug_` é parâmetro.** `app/blog/slug_/page.go` responde `/blog/{slug}`, lido com
  `c.Param("slug")`. Nome terminando em `-` é grupo que não soma segmento na URL.
- **HTML é Go, com escape por padrão.** `h.Div(h.Class("card"), h.Text(titulo))` — não há
  templates, e nada chega à página sem escape a menos que você chame `h.Raw`, o que quase
  nunca deve.

## Não faça

- **Não edite `trilha_gen.go`** — gerado e commitado; o próximo `trilha gen` sobrescreve.
- **Não adicione dependência.** A resposta está na biblioteca padrão ou no framework.
- **Não ponha segredo no código.** Leia do ambiente; o `trilha audit` falha em literal que
  parece chave.
- **Não escreva CSRF, sessão ou escape por conta** — os três existem e já estão ligados.
  Escrita em `route.go` precisa de `var Kind = trilha.KindPage` num `kind.go` ao lado.
- **Não escreva listagem à mão.** `trilha.ListParams` + `ui.DataTable`: ordenação, filtro,
  busca e paginação vivem na URL e viram fragmento.
- **Não monte à mão o molde de app interno.** `ui.Shell` é a barra lateral e o menu do
  usuário; `ui.Stat`/`ui.Bars` desenham painéis em SVG no servidor. Esconder item de menu é
  cosmética — o middleware é que tranca.
- **Não escreva à mão autocomplete, fila de upload ou chat.** `ui.Combobox`, `ui.Dropzone` +
  `ui.UploadTo`, e `ai.Serve` + `ui.Chat` existem; leia `trilha ui describe <Nome>` antes.

## Onde procurar

- Receitas: <https://emersonjoe.github.io/trilha/cookbook> · Referência:
  <https://emersonjoe.github.io/trilha/reference> · Texto em massa:
  <https://emersonjoe.github.io/trilha/llms.txt>
