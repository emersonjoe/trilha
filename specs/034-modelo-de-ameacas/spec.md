# Spec 034 — Modelo de ameaças escrito e validação de `Host`

- **Issue**: [#34](https://github.com/emersonjoe/trilha/issues/34) (ROADMAP, §10 e §20)
- **Branch**: `034-modelo-de-ameacas`
- **Versão**: 0.25.0

## Por quê

A segurança do Trilha está documentada por **controle**: o capítulo de segurança conta o que
vem ligado, a referência lista os campos, o `trilha audit` confere o que o time esqueceu. Falta
o documento que a primeira auditoria de qualquer cliente pede, e que nenhuma dessas páginas
responde: **contra o quê**. Sem ativos, fronteiras e ameaças escritas, cada controle parece uma
escolha de gosto, e quem avalia o framework não tem como saber se o que falta é intencional ou
esquecimento.

Escrever o modelo também revela o que falta. Uma ameaça clássica de aplicação web não tem hoje
resposta no runtime: **o `Host` da requisição não é validado**. O cabeçalho é escolhido por
quem chama, e o app o usa para montar URL absoluta — link de redefinição de senha, e-mail de
convite, `Location` de redirecionamento — além de virar chave em qualquer cache na frente.
Um `Host: atacante.example` numa requisição bem-formada faz o app gerar um link para o
atacante, ou envenena o cache compartilhado. A defesa é uma linha de configuração que quase
todo framework tem e o Trilha não.

## O que muda

### `SECURITY-MODEL.md` (inglês) e `docs/pt-BR/SECURITY-MODEL.md`

Um documento novo na raiz, ligado do `SECURITY.md` e do capítulo de segurança, com:

- **Ativos**: sessão da pessoa usuária, dados da aplicação, segredo de assinatura, o próprio
  processo (disponibilidade) e a cadeia de build (arquivo gerado, dependências).
- **Fronteiras de confiança**: navegador ↔ app, proxy ↔ app, app ↔ banco/serviço, disco
  (`public/`, uploads) e o build.
- **Agentes**: visitante anônimo, pessoa autenticada, quem opera o deploy, o proxy à frente e
  o atacante na rede.
- **Ameaças por STRIDE**, cada linha apontando o controle que responde e onde ele mora — sem
  recopiar o que a referência já explica.
- **O que o framework não resolve**: autorização de domínio, segredo em repouso, WAF, DDoS
  volumétrico, segurança do que o app faz com o dado depois do `Bind`.

O documento é descritivo, não aspiracional: ameaça sem controle aparece como **aberta**, com o
item de roadmap ou a razão de não valer a pena.

### `Config.AllowedHosts []string`

```go
trilha.Config{AllowedHosts: []string{"exemplo.com", "*.exemplo.com"}}
```

- **Lista vazia = comportamento de hoje**: nada é conferido, nenhum `Host` é recusado.
- Ligada, a conferência acontece **na borda**, antes do CORS, da observabilidade e do
  roteamento: `Host` fora da lista responde **400** com corpo curto, sem tocar em rota,
  middleware ou banco.
- Casa o host **sem a porta** (`exemplo.com:8443` casa `exemplo.com`), sem diferenciar
  maiúsculas, e aceita um curinga de rótulo único no começo (`*.exemplo.com` casa
  `app.exemplo.com`, não casa `exemplo.com` nem `a.b.exemplo.com`).
- `localhost` e `127.0.0.1` são aceitos em `Dev` mesmo fora da lista — senão o dev server
  quebraria a cada `AllowedHosts` copiado da produção.
- Recusa emite evento de segurança de tipo `host`, que já vai para o log, a métrica
  `trilha_security_events_total` e o `OnSecurityEvent`.
- `TRILHA_ALLOWED_HOSTS=a,b` preenche o campo pelo `ConfigFromEnv`.
- O `trilha audit` avisa quando o app não declara a lista — aviso, não crítico: quebrar a
  auditoria de todo projeto existente por uma defesa nova seria trocar sinal por ruído.

## Fora de escopo

- Redirecionar para o host canônico (`www` → apex) — é política do app, não do framework.
- Validar `Origin`/`Referer`: quem faz isso é o CSRF, que já existe.
- Ligar `AllowedHosts` por padrão: quebraria todo app existente, e o valor certo só o app sabe.
- Modelo de ameaças do site de documentação ou da infraestrutura do projeto.

## Constitution Check

| Princípio | Como esta spec o respeita |
|---|---|
| I — convenção sobre configuração | A defesa é um campo só, com um padrão que não quebra ninguém. |
| II — só biblioteca padrão | `strings` e `net`; nada novo. |
| VI — teste primeiro | Cada regra de casamento vira teste antes do código. |
| VII — segurança por padrão | Ameaça conhecida ganha controle, e o que continua aberto fica escrito. |

## Aceitação

- **SC-001** `AllowedHosts` vazio: requisição com qualquer `Host` é atendida como hoje.
- **SC-002** Lista preenchida: `Host` fora dela responde 400 sem que a rota rode, e um `Host`
  da lista (com porta, com outra caixa, casando curinga) passa.
- **SC-003** A recusa emite `SecurityEvent{Kind: "host", Status: 400}`.
- **SC-004** `SECURITY-MODEL.md` existe nas duas línguas, ligado do `SECURITY.md` e do capítulo
  de segurança, e toda ameaça listada aponta um controle ou está marcada como aberta.
- **SC-005** `trilha audit` aponta (aviso, não crítico) o app que não declara `AllowedHosts`.
