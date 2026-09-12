# Spec 145 — conviver com o protocolo trilha-spec

- **Issue**: [#232](https://github.com/emersonjoe/trilha/issues/232) — a issue é a fonte do escopo; aponte para ela, não a reescreva aqui.
- **Branch**: `145-protocolo-agentic`
- **Versão**: 0.124.0

## Por quê

A estratégia agentic saiu deste repositório e virou três produtos — `trilha-spec` (protocolo
público), `trilha-runner` (execução local, público) e `trilha-cloud` (control plane, privado);
o [ADR 001 do trilha-spec](https://github.com/emersonjoe/trilha-spec/blob/main/docs/adr/001-tres-repositorios.md)
registra a divisão e os conflitos com esta CLI. Dois deles só o framework resolve: o
`trilha dev` reescreve `.trilha/.gitignore` com `*`, escondendo do git o diretório que o
protocolo precisa commitar; e não há como `trilha spec …` chegar ao binário `trilha-spec`
sem colidir com `mcp`, `agents`, `check` e `ctx`. O terceiro pedido da issue é o site: a
trilha *Learn* termina em "IA e agentes" sem dizer como um agente trabalha *no* app.

## O que muda

1. **Cache em `.trilha/cache/`.** `internal/dev` compila em `.trilha/cache/app` e grava
   `.trilha/cache/.gitignore`; `export` compila em `.trilha/cache/export-app`; o scaffold e o
   `.gitignore` da raiz ignoram `.trilha/cache/` e `.trilha/runs/` em vez de `.trilha/`. O
   `trilha audit` continua conferindo `.trilha` no `.gitignore`; a mensagem de falta cita o
   caminho novo. A tabela de convenções do site descreve o diretório como o protocolo.
2. **Despacho externo.** `cmd/trilha/main.go`: comando desconhecido procura `trilha-<nome>` no
   `PATH` e o executa com o resto da linha, stdin/stdout/stderr ligados, devolvendo o código de
   saída; `TRILHA_PARENT_VERSION` vai no ambiente. Nome com separador ou ponto nunca é
   procurado. Sem binário, a mensagem de comando desconhecido é a de hoje. A linha entra no
   uso (en/pt).
3. **Três capítulos na trilha Learn**, depois de *AI and agents*: o protocolo
   (`agentic-protocol` / `agentico-protocolo`), o runner (`agentic-runner` /
   `agentico-runner`) e o control plane (`agentic-cloud` / `agentico-cloud`), cada um com
   desafio e solução, sobre a mesma agenda dos capítulos anteriores.

## Fora de escopo

- Um subcomando `trilha spec` embutido: seria vendorizar o protocolo dentro do framework e
  trazer a versão de um para dentro do outro; o despacho dá a mesma experiência sem acoplar.
- `trilha new --agentic` (inicializar `.trilha/` no scaffold): depende de o protocolo
  estabilizar em 1.0; até lá é `trilha-spec init`.
- Migrar apps existentes: quem tinha `.trilha/` no `.gitignore` continua funcionando; só quem
  adotar o protocolo precisa trocar a linha, e o `trilha-spec doctor` avisa.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `os/exec` no despacho; nenhuma dependência |
| III — geração explícita | não toca o gerador |
| V — dev < 2 s, um binário | o cache só muda de pasta; nada a mais no ciclo |
| VI — teste primeiro | `TestExternalSubcommand` (função e binário); testes do site cobrem os capítulos nas duas locales |
| VII — segurança por padrão | nome com `/`, `\` ou `.` não é despachado; só o `PATH` decide |
| Idioma | capítulos e uso da CLI em en e pt no mesmo commit |
