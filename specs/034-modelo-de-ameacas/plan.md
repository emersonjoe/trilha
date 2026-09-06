# Plan 034 — Modelo de ameaças escrito e validação de `Host`

## Fatos que decidem o desenho

1. **`serveHTTP` já é a única borda.** CORS, observabilidade e roteador saem todos de
   `trilha.go:338`. A conferência de `Host` entra como a primeira linha dessa função: antes do
   preflight (um `Host` forjado não merece resposta de CORS), antes da métrica em voo e antes
   do `mux`.
2. **Nessa altura não existe `*Ctx`.** `securityEvent` recebe um `*Ctx` para tirar dele IP,
   caminho e id da requisição. A recusa por `Host` precisa de um caminho paralelo que monte o
   `SecurityEvent` a partir do `*http.Request` — mesmo log, mesma métrica, mesmo hook.
3. **`r.Host` inclui a porta e não é normalizado.** O Go entrega o que veio no cabeçalho (ou
   a autoridade da linha de requisição, em HTTP/2). `net.SplitHostPort` falha quando não há
   porta, então a normalização é: tirar a porta se houver, baixar a caixa, tirar o ponto final
   do FQDN (`exemplo.com.`). Um `Host` vazio nunca casa uma lista preenchida.
4. **Curinga só de um rótulo.** `*.exemplo.com` casando `a.b.exemplo.com` é a pegadinha que faz
   um subdomínio de cliente virar um host aceito. Regra: o sufixo tem de bater e o que sobra
   antes dele não pode conter ponto.
5. **Dev não pode herdar a lista de produção.** Quem copia o `Config` do exemplo e roda
   `localhost:3000` levaria 400 na cara sem entender por quê. Em `Env: Dev`, `localhost`,
   `127.0.0.1` e `[::1]` passam sempre. Em produção, não.
6. **O modelo de ameaças não é um capítulo do site.** Ele é documento de raiz, como o
   `SECURITY.md`: quem audita clona o repositório e procura na raiz. O site liga para ele.
7. **O documento envelhece se recopiar controle.** Cada linha da tabela STRIDE aponta o
   controle pelo nome e onde ele está configurado; a explicação continua na referência. O que
   não tem controle fica escrito como aberto — é a parte que dá valor ao documento.
8. **Tradução no mesmo commit.** Regra do repositório: público em inglês por padrão com pt-BR
   junto. São dois arquivos, e o `SECURITY.md` das duas línguas ganha o link.

## Fases

1. **Casamento de host** — testes das regras (porta, caixa, curinga, ponto final, vazio, dev)
   e a função que as implementa, isolada e sem `net/http`.
2. **Borda** — a conferência em `serveHTTP`, o evento `host` sem `*Ctx`, `TRILHA_ALLOWED_HOSTS`
   no `ConfigFromEnv`, teste de integração provando que a rota não roda.
3. **Auditoria** — item novo no `trilha audit`, nas duas línguas do CLI.
4. **Documento** — `SECURITY-MODEL.md` e a tradução, links no `SECURITY.md` (duas línguas) e no
   capítulo de segurança do site (duas locales), mais a referência do campo novo.
5. **Fechamento** — CHANGELOG 0.25.0, `version`, ROADMAP, release.

## Riscos

- **Ficar entre o proxy e o app.** Quem termina TLS num proxy costuma reescrever o `Host`; a
  documentação precisa dizer que a lista é do host que o app recebe, não do que o navegador
  digitou.
- **Documento que promete demais.** Um modelo de ameaças que lista controle que não existe é
  pior que nenhum. Cada linha vai ser conferida contra o código antes de entrar.
