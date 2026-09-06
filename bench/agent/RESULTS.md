# Custo por feature de um agente

**Ainda sem medição.** Rode `claude auth login` e depois `make bench-agent`; este arquivo é regravado a partir de `results.json`.

## Metodologia

- **O que conta.** Tokens que o agente mandou (*Entrada*: novos, incluindo o que foi escrito
  no cache) e leu de volta do cache (*Cache lido*), tokens que produziu (*Saída*), rodadas de
  modelo (*Rodadas*), chamadas de ferramenta recusadas (*Negados*), tempo de parede e o custo
  que o próprio agente informa. Tudo vem do JSON de `claude -p --output-format json`.
- **O que não conta.** Nada é descontado: um comando repetido, um arquivo aberto à toa e uma
  assinatura errada são exatamente o que a régua quer ver.
- **Isolamento.** O agente roda com cwd numa cópia do exemplo em diretório temporário, com
  `go.mod` próprio e `replace` para o repositório, a CLI `trilha` compilada no `PATH`, sem
  servidores MCP, sem plugins, sem memória do usuário (`--setting-sources project`): só o que
  está dentro do projeto conta, que é o que a #46 (`AGENTS.md`) muda.
- **Passou.** Depois do agente, um teste escondido é copiado para a cópia e `go vet ./...` +
  `go test ./...` rodam. Verde é passou; o resto, não. O teste falha na fixture intocada, e
  `go test ./...` do módulo `bench` prova isso sem agente nenhum.
- **Três execuções, mediana.** Cada cenário roda três vezes; a tabela mostra a mediana de
  cada coluna e quantas execuções passaram.
- **Antes × depois.** A comparação é sempre Trilha contra Trilha: mesma tarefa, mesmo
  agente, mesmo modelo, versões diferentes. Nunca contra outro framework (spec 011).
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

