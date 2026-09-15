# Spec 155 — lote das issues abertas

## Problema

O repositório chegou à `v0.133.0` com vinte e uma issues abertas, incluindo épicos antigos já
quase concluídos e lacunas pequenas descobertas por uso real. Publicar uma versão por correção
fragmentaria o contrato e deixaria o roadmap dizendo que trabalho entregue ainda estava pendente.

## O que muda

- **Documentação e agentes (#50):** o site passa a servir MCP de documentação somente leitura em
  `POST /mcp`, com `search_docs`, `get_page` e `get_recipe`.
- **Scaffold e DX (#115, #117, #118, #119):** `generate crud` recebe `--tenant`, `--policy` e
  `--schema`; o catálogo de `Hint` ganha páginas exportáveis em `/docs/errors/<código>`; o bench
  mede tanto “adicione um cadastro” quanto “corrija este erro”.
- **Chat e ilhas (#179, #183):** mensagens carregam fontes seguras e o canal de ilha envia
  `Blob`, `File`, `FormData` e `ArrayBuffer` sem serialização JSON.
- **UI e PWA (#184, #185, #231, #233, #234, #235, #237):** entram `ui.Audio`, convite de
  instalação, abas endereçáveis, arredondamento decimal compatível com JS/ICU, estados reais da
  árvore, nós não selecionáveis e ação por linha em prazos.
- **CLI e HTTP (#200, #212, #213, #216, #229, #230):** UTF-8 no cliente, dependências obrigatórias
  do template, sinais modais ampliados, revoke correto de chave, memória multipart separada do
  teto do corpo e defaults opcionais representáveis.
- **Segredos (#240):** conexões podem informar presença sem devolver o segredo, e o campo mantém
  a credencial remota quando fica vazio.

## Decisões

- Uma única release `0.134.0` fecha o conjunto; não há uma tag por issue.
- O MCP documental é servido pelo app dinâmico. O export para GitHub Pages continua estático e
  não promete aceitar `POST`.
- `--tenant` põe o tenant na assinatura do store e em toda cláusula SQL, sem acrescentar um campo
  ao struct de domínio.
- `--policy módulo` usa a matriz da receita `permissions` no nível `administrar`; combinado com
  `--tenant`, os dois middlewares são aplicados.
- O catálogo de erros é a fonte usada por `NewHint` e pelas páginas do site.

## Segurança e compatibilidade

- Fontes do chat aceitam apenas URLs seguras; links HTTP(S) recebem `noopener nofollow ugc`.
- Upload multipart continua limitado por `MaxBodyBytes`, mas só `MaxFormMemory` permanece em RAM.
- O tenant participa de listagem, leitura, criação, atualização e exclusão, em memória e SQL.
- Defaults opcionais do cliente OpenAPI usam ponteiros para preservar valores zero explícitos.
- As APIs existentes continuam válidas; a release acrescenta apenas campos, funções e opções.

## Aceitação

- [x] As 21 issues abertas em 15 de setembro de 2026 têm implementação e documentação no lote.
- [x] O CRUD gerado combina `--store`, `--tenant`, `--policy`, `--schema` e `--lang`.
- [x] O site lista e exporta cada código de erro, e o bench contém os dois cenários pendentes.
- [x] Rotas, clientes, receitas e catálogo do kit são regenerados.
- [x] `make test`, `make race`, `make security` e `make bench-agent-dry` passam antes da publicação.
- [ ] A release `v0.134.0` é publicada e as 21 issues são fechadas pelo mesmo comando de release.
