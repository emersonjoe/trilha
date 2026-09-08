# Spec 061 — O MCP local

- **Issue**: #50 — a issue é a fonte do escopo. Esta spec entrega **a primeira das duas peças**
  dela; a segunda (MCP de documentação hospedado) fica aberta, e o motivo está abaixo.
- **Branch**: `061-mcp-local`
- **Versão**: 0.43.0

## Por quê

Para agente com shell, o `--json` dos comandos já entrega o valor — e é por isso que a #50 foi
deixada por último de propósito. Onde o MCP ganha é no **agente sem shell**: chat, editor que
só fala MCP. Esse agente, hoje, adivinha o que o projeto tem dentro.

## O que muda

`trilha mcp` sobe um servidor MCP por stdio sobre o pacote `ai/mcp` que já existe, expondo como
ferramenta o que os comandos já respondem:

| Ferramenta | Por trás |
|---|---|
| `describe_project` | `trilha ctx --json` |
| `routes` | `trilha routes` |
| `check` | `trilha check --json` |
| `ui_describe` | o catálogo que a CLI já embute — sem processo, sem projeto |
| `generate` | `trilha generate` — **só com `--write`** |

Nada de lógica nova: cada ferramenta é invólucro, e o e2e compara a saída dela com a do
comando byte a byte.

## Postura de segurança

Um servidor local roda com os privilégios de quem o iniciou e é dirigido por um modelo cujo
contexto pode ter vindo de qualquer lugar. As decisões, e por quê:

| Decisão | Contra o quê |
|---|---|
| Somente leitura por padrão; o `generate` **não é registrado** sem `--write` | Menor privilégio. Ferramenta que não está no `tools/list` não tem como ser pedida — não existe recusa para o modelo insistir contra. |
| Nunca shell: programa + lista de argumentos | Injeção de comando (CWE-78). Valor do modelo nunca vira linha de comando. |
| Todo argumento contra lista do que é permitido | Travessia de caminho (CWE-22) e contrabando de flag. `/../../etc/passwd`, `/x; rm -rf /` e `--force` param antes de rodar. |
| Diretório decidido na subida | Nada que o modelo mande muda de projeto. |
| Um comando por vez, com prazo e teto de saída | Exaustão de recurso (CWE-400). |
| Toda chamada registrada no stderr antes de rodar | Auditoria. O stdout é do protocolo; log lá corromperia o fluxo. |
| Nenhuma conexão de rede | O servidor local não busca nada. |

## Fora de escopo

- **O MCP de documentação hospedado** (a segunda peça da #50). Receita e referência são o mesmo
  Markdown do site — cerca de 1 MB — e embuti-lo no binário da CLI para responder sobre um
  projeto do qual ele não faz parte é a troca errada; o `internal/uidoc` já registra essa
  restrição ao gerar um catálogo em vez de embutir a fonte. Servir isso exige um lugar para
  rodar, e o site é estático no GitHub Pages: é decisão de hospedagem, não de código.
- **Ferramentas que editam arquivo existente** — o `generate` escreve esqueleto em lugar novo;
  editar é trabalho de quem tem shell.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `ai/mcp` é do próprio repositório; nada entra no `go.mod`. |
| IV — superfície pequena | Nenhum símbolo exportado novo; é um comando. |
| VI — teste primeiro | As recusas são testadas sem servidor, sem projeto e sem processo; o e2e prova que é invólucro. |
| VII — segurança por padrão | O padrão é somente leitura, e o modo que grava tem que ser pedido na linha de comando. |

## Tarefas

- [x] T001 `cmd/trilha/mcp.go`: comando, ferramentas e validação.
- [x] T002 Testes das recusas e da construção de argumentos.
- [x] T003 e2e: `tools/list` sem e com `--write`, e a saída igual à do comando.
- [x] T004 `usage` nas duas línguas e referência do MCP nas duas locales.
- [x] T005 `AGENTS.md` dizendo quando usar a CLI e quando usar o MCP.
- [ ] T006 `CHANGELOG.md`, `version`, `make test` verde e release.

## Aceitação

- **SC-001** Sem `--write`, `tools/list` traz quatro ferramentas e nenhuma grava.
- **SC-002** Com `--write`, aparece a quinta, e ela grava o arquivo.
- **SC-003** A saída de `routes` pela ferramenta é idêntica à do comando.
- **SC-004** Caminho que sobe, nome com separador e método inventado são recusados com o motivo.
