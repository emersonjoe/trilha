# Demanda do Partiu: dez atritos encontrados adotando a Trilha num app real

> Cole isto na sessão que está trabalhando na Trilha.

## Contexto

O **Partiu** (PWA de viagem em grupo, Go + `html/template`, ~76 rotas, banco Postgres) está sendo
migrado do Next.js para Go, e a partir de agora passa a ser montado pela **Trilha** — a árvore
`servidor/app/**` é a fonte da URL, e o `trilha gen` escreve o `servidor/trilha_gen.go`.

A migração é medida por uma bateria de paridade que compara a versão Next e a versão Go **tela a
tela**, em cinco eixos: texto visível, árvore de acessibilidade, cabeçalhos HTTP, fronteiras de nó
de texto e pixels, nos dois motores (WebKit e Chromium). É por isso que várias das exigências
abaixo são de fidelidade byte a byte — não é preciosismo, é o que o teste mede.

A adoção **funcionou**: as 76 rotas subiram na Trilha na primeira tentativa, com os cabeçalhos de
segurança do Partiu reproduzidos por `Config.Security` e os estáticos servidos com os mesmos
`Cache-Control` de antes. As dez issues abaixo são o que doeu no caminho — todas com repro,
diagnóstico e sugestão, e nenhuma delas impediu a adoção.

## As issues

| # | Título | Tipo | Por que importa |
|---|---|---|---|
| [#5](https://github.com/emersonjoe/trilha/issues/5) | Publicar uma v0.2.0: a v0.1.0 não tem a API que a documentação descreve | bug | **Bloqueia qualquer adotante novo.** `go get @latest` traz uma versão sem `App.Config()` nem `trilha.Security`; tive de fixar a pseudo-versão `v0.1.1-0.20260905143038-ad9e3396dada`. |
| [#6](https://github.com/emersonjoe/trilha/issues/6) | Config: metade dos campos é ignorada quando mexida no Setup, sem avisar | bug | `Logger`, `Secret`, `RateLimit` e `TrustedProxies` são consumidos dentro do `New()`. O doc de `Config()` cita "rate limit" como exemplo do que dá para ajustar — e é justamente um dos que não valem. |
| [#7](https://github.com/emersonjoe/trilha/issues/7) | Timeouts: não existe forma de dizer "sem timeout" | proposta | Zero significa "use o padrão", então 30 s de `ReadTimeout` cortam upload de foto de 32 MB em 3G. Sugestão: `trilha.NoTimeout = -1`, na mesma convenção do `trilha.Off`. |
| [#8](https://github.com/emersonjoe/trilha/issues/8) | Estáticos: Cache-Control fixo em max-age=3600 | proposta | O Partiu versiona os estáticos na query e quer `immutable`. Sem gancho, tive de deixar `Public = nil` e servir tudo como rota — o trabalho que `Public` existe para poupar. |
| [#9](https://github.com/emersonjoe/trilha/issues/9) | Ctx: um middleware não consegue pôr valor no contexto da requisição | proposta | **O atrito maior.** Sem `SetContext`, um código Go existente que recebe `*http.Request` não enxerga nada que o middleware da Trilha guarde. Contornei com `*c.Request() = *c.Request().WithContext(ctx)`. Três linhas resolvem. |
| [#10](https://github.com/emersonjoe/trilha/issues/10) | Dev: a injeção do live reload não tem como ser desligada | proposta | Qualquer comparação de HTML (paridade, golden file, snapshot, `trilha export`) reprova em Dev. Tive de rodar `Env = Prod` em desenvolvimento e perder stack trace e `no-cache`. |
| [#11](https://github.com/emersonjoe/trilha/issues/11) | not_found.go/error.go: página que já respondeu ganha um segundo corpo | bug | `writeHTML` não consulta `c.w.wrote`, embora `handleError` consulte. Resultado: `superfluous WriteHeader` e dois corpos concatenados. |
| [#12](https://github.com/emersonjoe/trilha/issues/12) | route.go é sempre kindAPI: erro 4xx sai em JSON numa rota que serve HTML | proposta | Durante uma migração incremental **todas** as rotas são `route.go`, e nenhuma é API. O `fallback` já decide pelo `Accept`; a rota casada, não. |
| [#13](https://github.com/emersonjoe/trilha/issues/13) | trilha gen: main() fixo, public/ obrigatório e nenhum lugar para desmontar | proposta | O `Setup` abre o pool do Postgres e não há `Shutdown` para fechá-lo. `--no-main` sozinha já destrava. |
| [#14](https://github.com/emersonjoe/trilha/issues/14) | Documentar pasta com ponto no nome (app.css/ → /app.css) | documentação | **Funciona e não está documentado** — eu quase servi os estáticos por fora por achar que não dava. Merece teste em `internal/scan` para não morrer numa validação futura. |

## O que eu pediria primeiro, se fosse escolher

1. **#5** — sem tag, ninguém mais consegue começar pela documentação.
2. **#9** e **#11** — são as duas que obrigam a escrever código torto (mutar struct por ponteiro; não poder assumir a resposta), e as duas são pequenas.
3. **#6** — é a que mais engana: falha em silêncio, e o doc aponta para o campo errado.

O resto pode entrar por spec, sem pressa. O #14 é uma tarde de documentação e paga sozinho.

## Uma observação boa

O desenho de fazer o `findProject()` exigir `app/` no diretório atual e derivar o módulo do `go.mod`
mais próximo é o que tornou tudo isso possível: o Partiu já tinha um `app/` (o do Next), e a Trilha
coube em `servidor/app/` sem colisão nenhuma. Se o gerador exigisse a raiz do módulo, a adoção teria
parado antes de começar.
