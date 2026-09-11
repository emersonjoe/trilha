# local-login — sessão própria, API de fora

O app que mais aparece numa migração: os usuários são dele (e-mail, hash de senha,
papel), e a API continua onde está. Três coisas do Trilha se encontram aqui.

**`auth.Sessions`** — o mesmo `*Auth` do OIDC, sem provedor. O login confere a senha
com `auth.CheckPBKDF2` no formato `pbkdf2_sha256$iter$salt$hash`, o que o Django e o
`hashlib` do Python gravam: a tabela de senhas que já existe continua valendo.
`Require()`, `RequireRole()`, a `Store`, a rotação do id e a janela idle são os mesmos.

**`Config.Upstreams`** — `/api/` é encaminhado para `API_URL` com o token da sessão no
`Authorization`. É o `rewrites` do `next.config.ts`, mais o que um rewrite não tem onde
guardar: a credencial e o CSRF.

**`trilha client`** — o outro caminho até a mesma API, e o que uma listagem usa. O
`openapi.json` na raiz é o documento que a API publica; `internal/acervo` é o cliente Go
gerado a partir dele, commitado como o `trilha_gen.go`:

```sh
trilha client openapi.json --out internal/acervo
```

A tela `/painel/documentos` lê o acervo por ele, no servidor. A credencial vai no `context`
e sai no `Authorization` de cada chamada; o `Authorization` que o browser mandar não é lido
em lugar nenhum, e o token não aparece na página — que é onde ele estava quando esta tela
era JavaScript.

```sh
API_URL=http://localhost:8801 TRILHA_SECRET=$(openssl rand -hex 32) go run .
```

Sem `API_URL` o proxy não é registrado, `/api/` é 404 e a tela de documentos diz o que
falta em vez de quebrar — o exemplo roda sozinho.

Usuários semeados no `Setup`: `ana@exemplo.com` / `segredo-da-ana` (analista) e
`bia@exemplo.com` / `segredo-da-bia` (admin).

`app/kind.go` diz que a árvore inteira é de páginas, então o `POST /sair`, escrito
como `route.go`, exige o token do CSRF sem que o arquivo dele diga nada.
