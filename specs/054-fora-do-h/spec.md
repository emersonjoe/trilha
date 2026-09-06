# Spec 054 — o que o framework guarda para quem não renderiza com o `h`

- **Issues**: [#44](https://github.com/emersonjoe/trilha/issues/44),
  [#57](https://github.com/emersonjoe/trilha/issues/57)
- **Branch**: `054-fora-do-h`
- **Versão**: 0.39.0 (com as specs 051, 052 e 053)

## Por quê

As duas issues são a mesma fronteira vista dos dois lados: um app cuja casca já existe em
`html/template` — 50 telas no Partiu, 20 no painel do Farol — e que está migrando tela por
tela, como o guia de migração manda.

1. **O nonce e o token de CSRF não chegam a quem só tem o `*http.Request`.** A Trilha sorteia
   os dois e os expõe em `c.Nonce()`, `c.CSRFToken()` e `trilha.CSRFInput(c)` — tudo em cima
   do `*Ctx`, e o `CSRFInput` já devolve um `h.Node`. Quem renderiza com `html/template`,
   `templ` ou um handler comum escreve um middleware com dois `context.WithValue` e um campo
   em cada estrutura de página. O custo de esquecer é assimétrico e silencioso: sem o nonce a
   CSP bloqueia os scripts da própria página (uma tela que "não faz nada", com erro só no
   console); sem o token todo formulário de uma rota de página responde 403 — e só na hora de
   salvar. O framework fez a metade difícil (sortear, comparar, expirar) e deixou a fácil.
2. **O `tmpl` só tem a ponte de ida.** `tmpl.Node(t, nome, dados)` põe um template dentro do
   `h`. O contrário — pôr um `h.Node` dentro de um `layout.html` que já existe — é o
   `layout.go` de todo mundo que está migrando, e hoje custa renderizar o filho num buffer,
   atravessar a fronteira do escape com um `template.HTML` escrito à mão e inventar um
   `{{define "conteudo"}}` postiço só para despejar o que já veio pronto. Um `template.HTML`
   no código do app é exatamente a linha em que uma revisão de segurança para.

## O que muda

### `trilha.NonceFrom(r)` e `trilha.CSRFTokenFrom(r)`

```go
func pagina(r *http.Request) dados {
	return dados{Nonce: trilha.NonceFrom(r), CSRF: trilha.CSRFTokenFrom(r)}
}
```

- O `*Ctx` passa a viajar no contexto da requisição, e os dois leitores o encontram lá. Não
  é um valor copiado no `newCtx`: os dois são preguiçosos (o token cria o cookie na primeira
  leitura), e adiantá-los poria um `Set-Cookie` em toda resposta do app.
- Fora de uma requisição da Trilha os dois devolvem `""` — que é o que o `Nonce()` já
  devolve quando o host é dono da política.
- O nome do campo escondido continua sendo `trilha.CSRFField`; quem renomeou sabe o nome que
  escolheu.

### `tmpl.Wrap(t, nome, slot)` e `tmpl.HTML(nó)`

```go
var casca = tmpl.Wrap(t, "layout.html", "conteudo") // no nível do pacote

func Layout(c *trilha.Ctx, children h.Node) (h.Node, error) {
	return casca.Node(pagina(c.Request()), children), nil
}
```

- `Wrap` prepara a casca uma vez, na carga do pacote: clona o conjunto de templates (o
  `html/template` só clona antes da primeira execução) e define ali o `slot` que o
  `{{template "conteudo" .}}` da casca chama.
- `Shell.Node(dados, filhos)` executa a casca com os dados do app e escreve os filhos no
  lugar do slot. Nada do app toca em `template.HTML`, e a casca não muda de forma.
- Uma casca que nunca chama o slot é um erro de renderização com o nome do template e o do
  slot — o silêncio aqui seria uma página sem conteúdo.
- `tmpl.HTML(nó) (template.HTML, error)` é a saída de baixo nível, para quem precisa do valor
  em vez da casca inteira.

## Superfície

| Onde | O quê |
| --- | --- |
| `ctx.go` | o `*Ctx` no contexto da requisição |
| `security.go` | `NonceFrom(r)` |
| `csrf.go` | `CSRFTokenFrom(r)` |
| `tmpl/tmpl.go` | `Wrap`, `Shell`, `Shell.Node`, `HTML` |
| `examples/blog` | grupo `legado-`: casca em `html/template`, miolo em `h`, formulário com o token vindo do request |

## Fora de escopo

- Um `tmpl.CSRFInput(r)` que devolva `template.HTML`: o `tmpl` não precisa importar o
  runtime para dois campos que o app já monta com os leitores acima.
- Fazer o `Wrap` funcionar depois da primeira execução do template: é o `html/template` que
  não clona um conjunto já executado, e a mensagem do pânico diz onde chamar.
- Slot em contexto de atributo ou de `<script>`: o miolo é HTML de corpo, como no `h`.

## Constitution Check

- **Convenção nova em `app/`**: nenhuma — o varredor não muda.
- **Zero dependências**; o `html/template` continua só no `tmpl`.
- **Inglês no código e no público, pt-BR na spec**; docs nas duas línguas no mesmo commit.
- Rota no `examples/blog` e teste de integração: o grupo `legado-`.

## Critérios

- **SC-001** `NonceFrom(r)` devolve o mesmo nonce que a CSP daquela resposta anuncia.
- **SC-002** `CSRFTokenFrom(r)` devolve o mesmo token do cookie, e um formulário montado só
  com ele passa pela verificação (303, não 403).
- **SC-003** Os dois devolvem `""` num `*http.Request` que não veio da Trilha, sem pânico.
- **SC-004** Os dois continuam valendo depois de um `SetContext` de middleware.
- **SC-005** `Wrap` + `Shell.Node` põem o `h.Node` no slot, com o HTML do filho intacto e o
  dos dados escapado.
- **SC-006** Casca sem slot, template inexistente e `Wrap` com template `nil` dão erro claro.
- **SC-007** `Wrap` depois de o template já ter executado dá pânico dizendo onde chamar.
- **SC-008** `Shell.Node` é seguro em requisições simultâneas (`-race`).
- **SC-009** O `examples/blog` serve uma tela de casca velha com miolo novo, e o teste de
  integração faz o `POST` do formulário dela.
- **SC-010** `make test` verde.
