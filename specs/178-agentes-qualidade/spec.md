# Spec 178 — As regras de qualidade no projeto que um agente edita

**Feature Branch**: `178-agentes-qualidade` | **Created**: 2026-10-18 | **Status**: Pronta para release (0.158.0)

- **Issue**: #310 — a fonte do escopo.
- **Branch**: `178-agentes-qualidade`
- **Versão**: 0.158.0

## Por quê

Quem desenvolve um app Trilha com um agente de IA repete a mesma revisão em toda mudança: o
script carregado no layout, a consulta dentro do laço, a lista sem `LIMIT`, a imagem sem
tamanho, o `http.DefaultClient`, a função copiada, o `h.Raw` com entrada do usuário. O
`AGENTS.md` que o `trilha agents` grava ensina as convenções e o que não fazer, mas não diz
como o código deve ser escrito — então cada time escreve (ou esquece) as próprias regras, e o
agente só as segue quando alguém as cola na conversa.

## Jornada e risco

Jornada de quem escreve o app com um agente. Pior impacto: o agente entrega código que funciona
e é lento, repetido ou inseguro, e a revisão humana é a única barreira. Não há tela nem
navegador envolvidos; o risco é de conteúdo (uma regra errada ensinada a todo projeto), coberto
por teste de conteúdo e de comandos.

## O que muda

- `trilha agents` (e `trilha new --agents`) grava um terceiro arquivo,
  `.claude/skills/trilha-quality/SKILL.md` (`scaffold.SkillPath`), em inglês ou português: o
  checklist de qualidade em dez seções — renders; bundle e carregamento sob demanda;
  requisições e cache; consultas e N+1; imagens, fontes e scripts; padrões de projeto (GoF)
  onde tiram código; código limpo; reúso; revisão pelo tamanho e pela leitura; segurança por
  OWASP ASVS 5.0 nível 2, Top 10:2025 e NIST SSDF 1.1 — cada item com o que fazer no Trilha
  (`cache.Once`, `c.Fragment`, `c.ETag`, `ui.Defer`, `c.Asset`, `cache.Do`, `store.Paginate`,
  `h.Width`, `middleware.go`, `c.Bind`…) e o que procurar com `grep`. O Claude Code a descobre
  como a skill `trilha-quality`; o frontmatter diz quando usá-la.
- O `AGENTS.md` ganha a seção **How code is written here** / **Como o código é escrito aqui**:
  as dez regras em uma linha cada e o caminho da skill, para o agente que não carrega skills.
- A skill segue a regra do `AGENTS.md`: é do framework, leva carimbo com o hash do corpo (numa
  linha de comentário YAML dentro do frontmatter, que precisa abrir o arquivo), é atualizada em
  silêncio quando intocada e recusada sem `--force` quando editada. O `CLAUDE.md` continua do
  projeto.

## Fora de escopo

- Portão automático para cada regra (por exemplo, `trilha audit` reprovar `http.DefaultClient`
  ou consulta dentro de laço): heurística estática com falso positivo; fica para depois, com
  medida em app real.
- Copiar a skill para os apps de `examples/`: eles não versionam `AGENTS.md` hoje.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `embed` e `text/template`, como o `AGENTS.md` |
| IV — API pública | nenhum símbolo coberto; `scaffold` é `internal/` |
| VI — teste primeiro | `TestAgentsWritesTheQualitySkill` (não compilava: sem `SkillPath`), `TestAgentsMatchesUsage` estendido à skill, orçamento de tokens dos dois arquivos |
| VII — segurança por padrão | a seção 10 ensina os controles do framework, não substitutos |

## Segurança e privacidade

- Ativos e fronteiras: arquivos de texto no repositório do usuário, escritos só por comando
  explícito; nenhum dado do usuário entra neles além do nome do projeto.
- ASVS 5.0 nível 2 e Top 10:2025: a skill aponta para os controles existentes (validação de
  entrada, escape de saída, autorização no middleware, CSRF, SQL por parâmetro, segredos do
  ambiente); NIST SSDF 1.1 PW.1/PW.7 (prática de codificação segura e revisão) e PS.1 (nenhuma
  dependência sem necessidade).
- Exceções: nenhuma.

## Tarefas

- [x] T001 Testes: `TestAgentsWritesTheQualitySkill` (criação, carimbo no frontmatter, as dez
  seções, link no `AGENTS.md`, mantida, atualizada, recusada, orçamento), `TestWriteAgents` com
  três arquivos, `TestAgentsMatchesUsage` cobrindo a skill.
- [x] T002 `internal/scaffold/agents/SKILL.{en,pt}.md`, seção nova nos dois `AGENTS`,
  `skillStamp`, `writeStamped` com o formato do carimbo, mensagens da CLI.
- [x] T003 Documentação en/pt: referência da CLI, IA e agentes, migração, README.
- [x] T004 CHANGELOG, ROADMAP, versão.
- [ ] T005 Suíte e `scripts/release.sh 0.158.0 --issues "310"`.

## Aceitação

- **SC-001** `trilha agents` num projeto grava `AGENTS.md`, `CLAUDE.md` e
  `.claude/skills/trilha-quality/SKILL.md`; a skill abre com `---`, tem `name: trilha-quality`
  e as dez seções.
- **SC-002** Rodar de novo mantém; a cópia intocada de uma versão anterior é atualizada; a
  editada só é sobrescrita com `--force`.
- **SC-003** Todo comando `trilha …` citado na skill e no `AGENTS.md` existe na CLI; os dois
  arquivos ficam abaixo de 2.500 tokens estimados.

## Evidências

| Nível | Comando | Passaram | Falharam | Pularam |
|---|---|---|---|---|
| Unidade + integração | `make test` | | | |

Sem cenário de navegador: nada aqui é visto pelo navegador.
