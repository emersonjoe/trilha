# Custo por feature de um agente

Gerado em 2026-09-06 por `make bench-agent` — Trilha v0.33.0-1-g926a20e-dirty, agente claude 2.1.212 (Claude Code), modelo claude-opus-4-8[1m], go1.25.1, darwin/arm64.

### Sem `AGENTS.md`

| Cenário | Entrada | Cache lido | Saída | Rodadas | Negados | Tempo (s) | Custo (US$) | Passou |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| `comments` | 67.1k | 2241.4k | 26.8k | 39 | 1 | 421 | 2.54 | 3/3 |
| `contact-form` | 48.5k | 1074.0k | 10.5k | 30 | 2 | 185 | 1.28 | 3/3 |
| `cognito` | 23.6k | 442.1k | 4.6k | 18 | 0 | 73 | 0.57 | 3/3 |
| `pagination` | 30.2k | 496.2k | 6.3k | 16 | 1 | 101 | 0.71 | 3/3 |

### Com `AGENTS.md` (`trilha new --agents`)

| Cenário | Entrada | Cache lido | Saída | Rodadas | Negados | Tempo (s) | Custo (US$) | Passou |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| `comments` | 51.7k | 1573.1k | 20.1k | 32 | 3 | 328 | 1.81 | 3/3 |
| `contact-form` | 23.6k | 592.1k | 7.2k | 25 | 1 | 120 | 0.81 | 3/3 |
| `cognito` | 18.6k | 384.7k | 4.2k | 13 | 0 | 73 | 0.48 | 3/3 |
| `pagination` | 16.7k | 331.7k | 3.9k | 15 | 1 | 70 | 0.43 | 3/3 |

### Diferença

| Cenário | Rodadas | Cache lido | Tempo | Custo |
|---|---:|---:|---:|---:|
| `comments` | -18% | -30% | -22% | -29% |
| `contact-form` | -17% | -45% | -35% | -37% |
| `cognito` | -28% | -13% | -1% | -15% |
| `pagination` | -6% | -33% | -31% | -40% |

Negativo é menos: menos rodadas, menos tokens, menos tempo, menos dinheiro.

As duas tabelas medem a **mesma** fixture, com o `AGENTS.md` como única diferença. Quando o exemplo muda de forma, a comparação morre e a linha de base tem que ser remedida — uma tabela contra outro exemplo mediria a troca do exemplo.

Mediana de cada coluna sobre as execuções; *Passou* é quantas execuções deixaram o teste escondido verde.

## Metodologia

- **O que conta.** Tokens que o agente mandou (*Entrada*: novos, incluindo o que foi escrito
  no cache) e leu de volta do cache (*Cache lido*), tokens que produziu (*Saída*), rodadas de
  modelo (*Rodadas*), chamadas de ferramenta recusadas (*Negados*), tempo de parede e o custo
  que o próprio agente informa. Tudo vem do JSON de `claude -p --output-format json`.
- **O que não conta.** Nada é descontado: um comando repetido, um arquivo aberto à toa e uma
  assinatura errada são exatamente o que a régua quer ver.
- **Isolamento.** O agente roda com cwd numa cópia do exemplo em diretório temporário, com
  `go.mod` próprio e `replace` para uma cópia somente-leitura do repositório no mesmo
  diretório (o agente lê o framework como leria o cache de módulos, mas não o altera), a CLI
  `trilha` compilada no `PATH`, sem servidores MCP, sem plugins, sem memória do usuário
  (`--setting-sources project`): só o que está dentro do projeto conta, que é o que a #46
  (`AGENTS.md`) muda.
- **Negados.** A lista de comandos liberados cobre o que um ciclo Go precisa (`go`, `gofmt`,
  `trilha`, `make` e utilitários de leitura). Uma recusa é o agente pedindo algo fora dela; a
  coluna existe para que um vão na lista apareça em vez de virar custo do framework, e
  `results.json` guarda o comando recusado.
- **Passou.** Depois do agente, um teste escondido é copiado para a cópia e `go vet ./...` +
  `go test ./...` rodam. Verde é passou; o resto, não. O teste falha na fixture intocada, e
  `go test ./...` do módulo `bench` prova isso sem agente nenhum.
- **Três execuções, mediana.** Cada cenário roda três vezes; a tabela mostra a mediana de
  cada coluna e quantas execuções passaram.
- **Antes × depois.** A comparação é sempre Trilha contra Trilha: mesma tarefa, mesmo
  agente, mesmo modelo, versões diferentes. Nunca contra outro framework (spec 011).
- **Duas fixtures.** `make bench-agent` mede o projeto cru; `make bench-agent-agents` mede o
  mesmo projeto com o `AGENTS.md` que a 0.36.0 escreve (`trilha new --agents`). Cada uma tem
  sua tabela e sua mediana; a seção *Diferença* é a segunda contra a primeira.
- **Reproduzir.** `claude auth login`, depois `make bench-agent` (12 execuções, dezenas de minutos e
  custo real). `make bench-agent-dry` monta os cenários sem gastar token.

## Cenários

### `comments` — rota de API com Bind, validação e 404

Fixture: `examples/blog`.

> Adicione comentários à API deste blog: POST /api/posts/{id}/comments recebe JSON {"author": "...", "body": "..."} (os dois obrigatórios, body com no máximo 500 caracteres), responde 201 com o comentário criado em JSON (campos author, body, created), 422 quando o corpo é inválido e 404 quando o post não existe; GET /api/posts/{id}/comments lista os comentários do post em JSON (array). Guarde em memória, como os posts. Escreva um teste da rota com os auxiliares de teste do próprio Trilha e deixe go vet ./... e go test ./... verdes.

### `contact-form` — página com formulário do kit ui no layout raiz

Fixture: `examples/blog`.

> Adicione a página /contato a este blog, dentro do layout raiz que já existe, com um formulário de contato feito com o kit ui do Trilha: campos nome, email e mensagem (todos obrigatórios, email válido). O POST vai para a própria página: com erro, a página volta com as mensagens nos campos e status 422; válido, mostra um agradecimento. Deixe go vet ./... e go test ./... verdes.

### `cognito` — trocar o provedor de login de Keycloak para Cognito

Fixture: `examples/sso`.

> Este app faz login com Keycloak. Troque o provedor para AWS Cognito usando o pacote auth do Trilha: a região vem de SSO_REGION, o user pool de SSO_USER_POOL_ID e o domínio de logout de SSO_LOGOUT_DOMAIN (as variáveis SSO_URL e SSO_REALM deixam de existir). Atualize a mensagem que explica o que falta configurar e o README. Deixe go vet ./... e go test ./... verdes.

### `pagination` — paginar a lista de posts

Fixture: `examples/blog`.

> A página /blog lista todos os posts de uma vez. Faça-a mostrar 5 posts por página: ?page=N escolhe a página (1 por padrão), e abaixo da lista aparecem os links para a página anterior e a próxima quando existem, com a página atual indicada, usando o componente de paginação do kit ui ou a receita do cookbook do Trilha. Deixe go vet ./... e go test ./... verdes.

