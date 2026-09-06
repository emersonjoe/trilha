# local-login — sessão própria, API de fora

O app que mais aparece numa migração: os usuários são dele (e-mail, hash de senha,
papel), e a API continua onde está. Duas coisas do Trilha se encontram aqui.

**`auth.Sessions`** — o mesmo `*Auth` do OIDC, sem provedor. O login confere a senha
com `auth.CheckPBKDF2` no formato `pbkdf2_sha256$iter$salt$hash`, o que o Django e o
`hashlib` do Python gravam: a tabela de senhas que já existe continua valendo.
`Require()`, `RequireRole()`, a `Store`, a rotação do id e a janela idle são os mesmos.

**`Config.Upstreams`** — `/api/` é encaminhado para `API_URL` com o token da sessão no
`Authorization`. É o `rewrites` do `next.config.ts`, mais o que um rewrite não tem onde
guardar: a credencial e o CSRF.

```sh
API_URL=http://localhost:8801 TRILHA_SECRET=$(openssl rand -hex 32) go run .
```

Sem `API_URL` o proxy não é registrado e `/api/` é 404 — o exemplo roda sozinho.

Usuários semeados no `Setup`: `ana@exemplo.com` / `segredo-da-ana` (analista) e
`bia@exemplo.com` / `segredo-da-bia` (admin).

`app/kind.go` diz que a árvore inteira é de páginas, então o `POST /sair`, escrito
como `route.go`, exige o token do CSRF sem que o arquivo dele diga nada.
