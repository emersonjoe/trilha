# Plano — Spec 149

**Branch**: `feat/inline-range-svg` | **Spec**: [spec.md](spec.md) | **Issues**: #201, #210

## Summary

Uma porta com opções (`Ctx.Send` + `SendOpts`) onde hoje há duas de três argumentos, para que
os dois casos que obrigam a abandonar o envelope — o corpo de outro serviço que precisa
responder `Range`, e o SVG que precisa sair neutralizado — passem a caber dentro dele. Mais um
erro sentinela (`ErrCannotInline`) para a recusa deixar de ser indistinguível de um bug.

Um pacote, nenhuma convenção nova em `app/`, nenhuma quebra de API — mas a forma completa
porque são três símbolos públicos novos e porque a escolha entre as duas saídas que a #201
propõe (repassar o `Range` ao outro serviço, ou cortar o stream aqui) precisa ficar escrita.

## Technical Context

Go 1.22+, stdlib. Arquivos:

- `send.go`: `SendOpts`, `Ctx.Send`, `ErrCannotInline`, o corte de `Range`, a passagem do
  `Content-Range`, a CSP da neutralização, `parseSendRange`/`parseContentRange`;
- `send_test.go`: os casos novos;
- `examples/blog/internal/midia/midia.go`, `examples/blog/app/midia/{page.go,audio/route.go,
  marca/route.go}`, `examples/blog/trilha_gen.go` (regerado), `examples/blog/midia_test.go`;
- `api/current.txt`: três linhas (`ErrCannotInline`, `Send`, `SendOpts`);
- `site/internal/docs/content/en/reference/ctx.md` e `pt/referencia/ctx.md`;
- `CHANGELOG.md`, `cmd/trilha/main.go` (`version`), `ROADMAP.md`.

## Constitution Check

Na spec, princípio por princípio. Sem violação; sem *Complexity Tracking*.

## Decisões

- **As duas saídas da #201, e não uma.** Repassar o `Range` ao outro serviço é o caminho barato
  — o prefixo não viaja — mas só existe para quem controla a requisição de upstream e recebe um
  206 de volta; para esse caso basta o kit aceitar um `Content-Range` pronto (`ContentRange`).
  Cortar o stream aqui (`Size`) é o caminho que funciona com qualquer `io.Reader`, inclusive o
  serviço que ignora `Range`, e custa a banda do prefixo. Uma opção só forçaria metade dos apps
  a sair do envelope de novo, que é o problema da issue. O `Size` também é o que faz a **primeira**
  resposta trazer `Accept-Ranges` e `Content-Length` — sem ela o Safari nem chega a pedir um
  pedaço.
- **Sem `Size`, nada muda.** Adivinhar o tamanho lendo o corpo é a solução que a issue recusa
  (o arquivo inteiro na memória); responder `Content-Range: bytes 10-19/*` seria prometer o que
  o navegador não consegue usar para desenhar a barra. Um `io.Reader` sem tamanho continua
  saindo inteiro, com 200, exatamente como hoje.
- **`Size` e `ContentRange` juntos são erro**, e não uma precedência. São duas afirmações
  contraditórias sobre o mesmo corpo ("isto é o arquivo inteiro" e "isto é um pedaço"), e
  escolher uma em silêncio entrega bytes errados com status certo — a falha mais cara possível.
- **`Range` ilegível dá 416, não 200.** A RFC 9110 manda ignorar um `Range` de unidade
  desconhecida, mas `http.ServeContent` — que é quem responde pelo mesmo método quando o corpo
  busca posição — responde 416 a qualquer `Range` que não parseia. Duas respostas diferentes
  para o mesmo pedido dependendo do tipo do corpo seria pior do que seguir o vizinho de dentro
  de casa. Vários intervalos, esse sim, saem com o arquivo inteiro em 200: `multipart/byteranges`
  num stream que não busca posição é muito trabalho para o que nenhum `<audio>` pede.
- **Uma porta com opções, não quatro métodos.** `InlineRange` + `AttachmentRange` +
  `InlineNeutralized` + o par que falta seriam quatro nomes para combinações do mesmo envelope,
  e a quinta necessidade abriria o quinto. `SendOpts{Inline: …}` segue o `blob.ServeOpts` que o
  kit já tem, e `Inline`/`Attachment` continuam existindo porque são o que quase toda rota
  escreve. Assinatura variádica em `Inline` (`opts ...SendOpts`) foi descartada: mudaria a linha
  de `api/current.txt` de um símbolo que já está publicado, e o valor de função `c.Inline`
  deixaria de compilar na casa de quem usa.
- **A CSP da neutralização é do kit.** `default-src 'none'; style-src 'unsafe-inline';
  frame-ancestors 'self'`: sem script, sem busca de recurso nenhum, com estilo embutido (sem ele
  um SVG deixa de ser desenho) e enquadrável pela própria origem, que é o que `inline` quer
  dizer. Escrita **antes** de `allowSameOriginFrame`, para que ela ajuste o `X-Frame-Options` e
  não ache mais nada para relaxar na política; e escrita mesmo quando o app tem `Security.CSP`
  próprio, porque aqui a política não é hardening genérico, é a condição para o documento sair.
- **`NeutralizeScript` abre só a porta do script.** Ela tira a resposta de `inlineNever`; não
  toca em `inlineTypes`. Um `application/zip` inline continua recusado, com ou sem ela: aquilo
  não é uma questão de script, é um tipo que nenhum visor mostra.
- **`ErrCannotInline` embrulhado, não um tipo novo.** O que a tela precisa é distinguir "este
  conteúdo não se mostra" de "este código está errado", e `errors.Is` resolve isso sem ninguém
  ter de declarar um tipo para ler um campo. A mensagem continua dizendo o tipo recusado e as
  duas saídas, que é o que quem lê o log precisa.
- **Uso em `examples/` (princípio IV).** `examples/blog/app/midia`: a página junta `<audio>` e o
  logotipo; `/midia/audio` serve um corpo que não busca posição com `Size` (no comentário, o que
  no app de verdade é o `res.Body` de outro serviço); `/midia/marca` serve o SVG com
  `NeutralizeScript` e mostra ao lado o `errors.Is(err, trilha.ErrCannotInline)` que a tela
  trata. Teste de integração em `examples/blog/midia_test.go`.
- **`Pipe` fica como está.** Ele é a resposta inteira de outro serviço passada adiante, sem
  envelope e sem nome de arquivo; `Send` com `ContentRange` é o mesmo corpo **dentro** do
  envelope. Fundir os dois faria um método que às vezes saneia o nome e às vezes copia o
  `Content-Disposition` de terceiro.
