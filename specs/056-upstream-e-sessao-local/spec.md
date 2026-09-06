# Spec 056 — A porta para a API que já existe

Issues: [#60](https://github.com/emersonjoe/trilha/issues/60) (`Config.Upstreams`) e
[#62](https://github.com/emersonjoe/trilha/issues/62) (`auth.Sessions` sem OIDC).
A issue é a fonte do escopo; aqui fica só a decisão.

## Por que as duas juntas

São as duas metades de um mesmo movimento. Um app que migra para o Trilha com a API
onde ela está precisa encaminhar `/api/*` para ela **com a credencial da sessão** — e
a sessão dele não é OIDC, é uma tabela de usuários com hash de senha. O `Headers` do
upstream lê o que o `Login` guardou: sem o `User.Extra` o proxy não tem o que injetar,
e sem o proxy o `Extra` não tem para onde ir. Uma spec, um exemplo, uma receita.

## Decisões

1. **O upstream mora no fallback**, depois do estático e do 405, antes do redirect de
   barra e do 404. Assim uma rota local do app sempre ganha do prefixo — dá para migrar
   endpoint a endpoint sem tocar no front — e o corpo não passa pelo `MaxBodyBytes`,
   que só é aplicado no `wrap` das rotas tipadas.
2. **`httputil.ReverseProxy` da stdlib**, não um proxy escrito à mão: hop-by-hop,
   streaming e `Flush` já são dele. O que o Trilha acrescenta é a política —
   alvo fixo, credencial da sessão, CSRF, `problem+json` no erro.
3. **A resposta do upstream vence.** Os cabeçalhos de segurança já foram escritos pelo
   `applySecurity` antes; o `Set` da cópia sobrescreve o que o upstream mandou e deixa
   de pé o que ele não mandou. É o comportamento pedido, e sai de graça pela ordem.
4. **`Authorization` do cliente não é repassado.** A credencial é a da sessão, não a do
   browser: quem manda um `Bearer` de fora não fala com o upstream por tabela.
5. **CSRF ligado por padrão** nos métodos com corpo, como numa página — o `ui.js` já
   manda o header. `CSRF: trilha.Off` no upstream desliga para API por chave.
6. **`auth.Sessions(Options)` devolve o mesmo `*Auth`, sem provedor.** Nada de um segundo
   tipo: `Require`, `RequireRole`, `Optional`, `User`, `Session`, a `Store` e a rotação
   de id já estão escritos e valem igual. `Start` e `Callback` respondem erro claro.
7. **O predicado genérico chama-se `RequireFunc`**, não `Require` de pacote: o método
   `Require()` já existe e duas coisas com o mesmo nome no mesmo pacote é o tipo de
   economia que custa caro na leitura. A matriz módulo × nível fica no cookbook.
8. **PBKDF2 no formato do Django** (`pbkdf2_sha256$iter$salt$hash`), porque o ponto é
   não migrar a tabela de senhas. `bcrypt`/`argon2` continuam fora: dependência.

## Critérios de aceitação

- **SC-001** GET, POST, PUT e DELETE atravessam o upstream com corpo e resposta intactos.
- **SC-002** Corpo maior que o `MaxBodyBytes` do app passa (upload); a resposta grande não
  morre no write deadline.
- **SC-003** `Content-Type`, `Content-Disposition`, `Cache-Control` e `ETag` do upstream
  chegam ao cliente; os cabeçalhos de segurança do Trilha ficam onde o upstream calou.
- **SC-004** Rota local do app em `/api/x` ganha do upstream que cobre `/api/`.
- **SC-005** Alvo não é influenciável pela requisição (`..`, `//outro`, `@`, `\`).
- **SC-006** Hop-by-hop (`Connection`, `Upgrade`, `Proxy-Authorization`) não passam;
  `Authorization` do cliente não passa; `X-Request-ID` e `traceparent` passam.
- **SC-007** `Headers` injeta a credencial lida da sessão.
- **SC-008** Upstream fora do ar vira `502` e timeout vira `504`, ambos `problem+json`.
- **SC-009** POST sem token de CSRF é 403; com `CSRF: trilha.Off`, passa.
- **SC-010** `auth.Sessions` + `Login` cria sessão; `Require()` passa depois dela.
- **SC-011** `Logout` sem provedor limpa a sessão e redireciona, sem estourar.
- **SC-012** `User.Extra` sobrevive à rotação de id do login e à `Store`.
- **SC-013** `CheckPBKDF2` aceita um hash gerado pelo `hashlib.pbkdf2_hmac` do Python
  (vetor fixo) e recusa a senha errada em tempo constante.
- **SC-014** `RequireFunc` nega com 403 quem o predicado recusa.
- **SC-015** `OnLogin` roda dentro do `Login` e o erro dele impede a sessão.
- **SC-016** `trilha audit` avisa: alvo `http://` fora de `Dev`, upstream sem `Headers`
  num app com `Require()`, e `Login` numa rota sem `RateLimit`.
- **SC-017** `examples/local-login` sobe com login por senha, página protegida e o
  upstream injetando o token — teste de integração ponta a ponta.
- **SC-018** Referência e receita nas duas línguas; `SECURITY-MODEL.md` com a seção da
  sessão sem OIDC.

## Fora de escopo

Refresh do token do upstream, WebSocket através do proxy, `bcrypt`/`argon2`,
`RequirePermission` no runtime (fica como receita), e a régua do #45.
