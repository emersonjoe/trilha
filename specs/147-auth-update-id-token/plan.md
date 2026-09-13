# Plano — Spec 147

**Branch**: `feat/auth-update-id-token` | **Spec**: [spec.md](spec.md) | **Issues**: #206, #208

## Summary

Duas portas que faltavam no `auth`: escrever numa sessão já aberta (`Auth.Update`) e ver o
`id_token` verificado durante o callback (`Options.OnLoginToken` + `auth.IDToken`). Um pacote,
nenhuma convenção nova em `app/`, nenhuma quebra de API — mas a forma completa porque são dois
contratos públicos novos e a decisão do #208 (callback em vez de campo no `Extra`) precisa ficar
escrita.

## Technical Context

Go 1.22+, stdlib. Arquivos:

- `auth/session.go`: `persist` (o `write` de hoje com uma decisão a mais) e `Update`;
- `auth/tenant.go`: `SwitchTenant` passa a usar o mesmo caminho;
- `auth/flow.go`: `IDToken`, `Options.OnLoginToken`, a chamada no `Callback`;
- `auth/update_test.go`, `auth/idtoken_test.go`: novos;
- `api/current.txt`: três linhas;
- `site/internal/docs/content/en/reference/auth.md` e `pt/referencia/auth.md`;
- `CHANGELOG.md`, `cmd/trilha/main.go` (`version`), `ROADMAP.md`.

## Constitution Check

Na spec. Sem violação; sem *Complexity Tracking*.

## Decisões

- **`Update` lê por `a.Session(c)`**, não por `a.User(c)`. `User` responde com o que está no
  contexto, que num POST longo é o que foi lido no começo da requisição; `Session` relê o store.
  É o que faz a forma de callback valer a pena: `fn` recebe o estado mais novo que o processo
  consegue ver.
- **`write` vira `persist(c, u, cookie bool)`**, com `write` como o caso `true`. Com store e
  identificador inalterado, o `Set-Cookie` voltaria com o mesmo valor e o mesmo prazo: barulho
  numa resposta que pode ser um 204 de `fetch`. Sem store não há escolha — a sessão é o cookie.
- **`SessionID` restaurado depois do `fn`**, em vez de `fn` receber uma cópia sem ele: a app
  precisa poder *ler* o identificador (é o que ela usa para reconhecer a própria sessão numa
  lista), só não pode trocá-lo.
- **`SwitchTenant` reescrito sobre `Update`**, sem mudar o que ele faz: a saída antecipada de
  `before == tenant` fica, porque mover para onde a sessão já está não é mover e uma linha de
  trilha "de acme para acme" seria ruído. O ganho é um caminho de escrita só; o que a #206
  pedia do `SwitchTenant` (ser veículo para mutar o `Extra`) agora tem nome próprio. O
  `challenge` para requisição anônima também fica: trocá-lo por `ErrNoSession` mudaria a resposta
  de quem já usa a chamada.
- **`OnLoginToken` depois de `OnLogin`, não em vez dele.** Substituir seria uma app que define os
  dois perder um sem aviso. A ordem é a útil: o `OnLogin` (a regra comum, a allow-list) decide se
  esta pessoa entra, e só então a app gasta uma ida ao backend com o token.
- **`IDToken` é struct e não dois parâmetros.** Um campo novo (o `access_token`, se algum dia
  alguém justificar) entra sem mexer na assinatura de quem já escreveu o callback.
- **`Raw` e não `Token`/`String`**: o campo é o JWS compacto do jeito que chegou, e `Raw` é o
  nome que o resto do kit usa para "o que veio antes de qualquer interpretação".
- **Um uso de cada símbolo em `examples/`** (princípio IV). O `Update` entra no
  `examples/local-login`: o acervo rotaciona a credencial e devolve a nova num cabeçalho, que o
  `acervo.WithResponse` (spec 146) lê e a sessão guarda — com teste de integração conferindo que a
  chamada da requisição seguinte já sai com o token novo, mesmo cookie. O `OnLoginToken` entra no
  `examples/sso`, que não tem backend para repassar o token: ele copia para o `Extra` as claims
  nomeadas em `SSO_EXTRA_CLAIMS`, e o comentário mostra onde o `t.Raw` iria.
- **Nada de log do token.** O `Callback` já loga por `a.fail`, que só recebe o erro; nenhuma
  linha nova, e o doc comment do `IDToken` diz para não logar.
