# Plano: 018-fragmentos

## Superfície pública

```go
// trilha
func (c *Ctx) Fragment() string // alvo pedido, "" numa navegação comum

// ui (o kit já é onde mora o ui.js)
func Swap(id string) h.Node // data-trilha-target="id"
```

Sem `ui`, o atributo é `h.Data("trilha-target", "lista")` — o kit é conveniência, não
requisito.

## Protocolo

| Direção | Cabeçalho | Significado |
|---|---|---|
| pedido | `Trilha-Fragment: lista` | quero só o pedaço `lista` |
| resposta | `Vary: Trilha-Fragment` | caches: a resposta depende disso |
| resposta | `Trilha-Location: /x` (204) | navegue de verdade para `/x` |

## Arquivos

| Arquivo | Papel |
|---|---|
| `ctx.go` | `Fragment()` com leitura única do cabeçalho |
| `render.go` | pular layouts, `<!doctype>` e script de dev; `Vary` |
| `errors.go`/`render.go` | redirecionamento → 204 + `Trilha-Location` |
| `ui/assets/ui.js` | interceptação de clique e envio, troca, foco, histórico, recuo |
| `ui/ui.go` | `Swap` |
| `ui/assets/ui.css` | estado de ocupado (`[aria-busy]`) |
| `fragment_test.go` | camada do servidor |
| `examples/cadastro/*` | busca na lista e envio sem recarga |
| `site/.../aprender/interatividade.md`, `.../referencia/ctx.md`, `.../referencia/ui.md` | documentação |

## Decisões

1. **A mesma rota, uma pergunta a mais.** Nada de registrar rotas de fragmento: duplicar
   endereço é duplicar autorização, middleware e teste. `c.Fragment()` é um `if` no lugar
   onde a página já decide o que mostrar.
2. **Troca por `outerHTML`, não por diferenciação de DOM.** O fragmento é o elemento com o
   mesmo `id` que a página inteira renderiza — a mesma função Go nos dois caminhos, então
   não há como divergirem. *Morphing* preservaria estado do DOM, mas custa uma biblioteca e
   um modelo mental; quem precisa disso está pedindo ilhas (#22).
3. **O cabeçalho é a fronteira de segurança.** Um cabeçalho personalizado obriga preflight
   de CORS numa requisição de outra origem, e o Trilha não responde a preflight — então
   nenhum site consegue pedir um fragmento em nome de quem está logado. O CSRF do
   formulário continua igual, sem caminho novo.
4. **Redirecionamento vira 204 + cabeçalho.** Seguir o redirecionamento dentro do `fetch`
   renderizaria a página de destino como fragmento (trabalho jogado fora) e ainda poderia
   enfiar uma página inteira dentro de uma `<div>`.
5. **Recuo sempre para a navegação.** Rede caiu, 500, alvo sumiu: `location.assign` ou
   `form.submit()`. Um botão que não faz nada é pior do que uma página que recarrega.
6. **O cliente mora no `ui.js`.** É o arquivo que o `trilha new` já copia e o `trilha ui`
   atualiza; um segundo script obrigatório seria uma dependência nova com outro nome.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| I | nenhuma rota nova; o fragmento sai da rota que já existe |
| II | zero dependências: `ui.js` cresce ~60 linhas de JavaScript sem biblioteca |
| III | `Vary` e status HTTP de verdade, sem protocolo paralelo |
| IV | uma função no `Ctx` e um atributo no HTML |
| V | funciona no `dev` sem passo de build; o script de recarga não entra no fragmento |
| VI | teste do servidor, teste do exemplo com e sem script, teste de ausência de manipulador inline |
| VII | cabeçalho personalizado (sem CORS), CSRF intacto, sem `unsafe-inline`, sem redirecionamento aberto |

## Complexity Tracking

O risco é o cliente virar um framework por acidente. Limite explícito: só `<a>` e `<form>`,
um alvo por elemento, troca do elemento inteiro, nenhum estado guardado no JavaScript.
Qualquer coisa além disso é a issue #22, não esta.
