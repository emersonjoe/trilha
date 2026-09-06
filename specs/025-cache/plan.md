# Plano — Spec 025

**Branch**: `025-cache` · **Spec**: [spec.md](./spec.md) · uma rodada de `make test` por bloco.

## Os fatos que decidem o desenho

1. **Subpacote importa o runtime, nunca o contrário.** `auth` já faz isso (`auth.New(...)`,
   métodos que recebem `*trilha.Ctx`). Então `cache` pode receber `*trilha.Metrics` e
   `*trilha.Ctx` sem ciclo, e o `App` não ganha campo nenhum: quem não usa cache não carrega
   cache.
2. **Genérico não pode ser método.** Go não aceita parâmetro de tipo em método, então
   `Get[T]`, `Do[T]` e `Once[T]` são funções de pacote com o `*Cache` (ou o `*trilha.Ctx`)
   como primeiro argumento. É por isso que a superfície tem duas camadas: os métodos
   `Set`/`Get`/`Invalidate` (sem tipo, `any`) e as funções tipadas em cima deles.
3. **`Ctx` já tem `Set`/`Get` por requisição.** O `Once` não precisa de campo novo no `Ctx`
   nem de `context.WithValue`: guarda um mapa sob uma chave privada do pacote `cache`, criado
   na primeira chamada. Custo zero para quem não chama, e morre com a requisição.
4. **O `Ctx` é de uma requisição, mas não é de uma goroutine.** O handler pode disparar
   goroutines que também chamam `Once`; o mapa por requisição leva mutex.
5. **LRU precisa de duas estruturas.** `map[string]*list.Element` para achar em O(1) e
   `container/list` para saber quem é o mais velho. As tags são um terceiro mapa,
   `map[string]map[string]struct{}`, e toda remoção passa por um único `drop(key)` — é o
   lugar onde o repositório vai errar se as tags e a lista saírem de sincronia.
6. **Voo único é um mapa de chamadas em andamento.** `map[string]*call` com
   `sync.WaitGroup`; quem chega depois espera e lê o resultado da mesma `call`. O mutex do
   cache **não** pode ficar seguro durante o `fn` (é uma ida ao banco).
7. **HELP das métricas está em português** (`trilha_requests_total` e as outras quatro, em
   `trilha.go:353`). São saída pública do framework e a regra do repositório manda inglês;
   como esta spec acrescenta métricas ao mesmo `/metrics`, as cinco viram inglês aqui, senão
   a saída sai metade em cada língua. Nome de métrica não muda — painel de ninguém quebra.

## Fases

1. **Runtime do cache** — `cache/cache.go`: `Options`, `New`, `Key`, `Set`, `Get`, `Delete`,
   `Invalidate`, `Clear`, `Len`, `Stats`, LRU e tags. Teste primeiro.
2. **Camada tipada** — `cache/typed.go`: `Get[T]`, `Do[T]` com voo único, `Once[T]`.
3. **Métricas** — quatro séries em `New`, e o HELP das cinco existentes em inglês.
4. **Exemplo** — `examples/blog`: a lista de posts sai do cache, `Create`/`Delete` invalidam
   a tag, e o layout usa `Once`. Teste de integração no `blog_test.go`.
5. **Documentação e fechamento** — capítulo novo em `aprender/`, referência do pacote,
   CHANGELOG, `version`, ROADMAP, release.

## Riscos

- **Tag órfã.** Invalidar por tag e despejar por LRU mexem nas mesmas duas estruturas; um
  caminho que esquece de limpar o mapa de tags vaza memória sem falhar teste nenhum. Mitigado
  por um `drop(key)` único e por um teste que confere o mapa de tags vazio depois do `Clear`.
- **Deadlock no voo único.** `fn` roda fora do mutex; se um `fn` chamar o mesmo cache (e vai
  acontecer: cache dentro de cache), o mutex reentrante seria o bug. O teste inclui um `Do`
  aninhado com chave diferente.
- **`Once` prometendo demais.** É por requisição, e a documentação tem de dizer isso na
  primeira linha, senão vira cache de usuário vazando entre pessoas — o pior erro possível
  nesta área.
