# Spec 151 — homologação documentada do Trilha Cloud

- **Issue**: [#244](https://github.com/emersonjoe/trilha/issues/244) — a issue é a fonte do escopo.
- **Branch**: `151-homologacao-trilha-cloud`
- **Versão**: 0.130.0

## Por quê

O capítulo do control plane explicava arquitetura, contrato e o aceite automatizado, mas ainda
deixava para o leitor unir sozinho a configuração do Cloud, a criação de uma aplicação real, o
protocolo, o runner e os fluxos de autenticação do template `app`. A documentação precisava ser
uma homologação reproduzível do ecossistema, com comandos executados e telas reais.

## O que muda

1. O capítulo `agentic-cloud` ganha, nas duas locales, um tutorial completo: iniciar o Cloud,
   criar um app de cadastro de usuários, configurar `make check`, gerar spec e task, versionar,
   registrar projeto, emitir chave, enfileirar, executar, revisar e aprovar.
2. O roteiro exercita login administrativo, convite, definição da primeira senha, login do
   convidado, troca da própria senha e novo login, e registra o limite dos stores em memória.
3. Dez screenshots reais da homologação entram em `site/public/docs/agentic-cloud/`.
4. O conversor Markdown do site passa a aceitar imagem de bloco com alt e legenda opcionais,
   aplicando o base path e escapando os atributos. O CSS mantém a figura responsiva.

## Fora de escopo

- Persistência de produção, envio real de e-mail e provisionamento público do `trilha-cloud`.
- Driver de IA: a homologação do protocolo usa `echo`, determinístico.
- Transformar o tutorial em um novo scaffold; ele documenta a composição dos componentes atuais.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | parser e renderização usam apenas `regexp`, `strings`, `html` e `fmt` já presentes |
| VI — teste primeiro | parser, base path, suíte do site, UI real e exportação do Pages foram exercitados |
| VII — segurança por padrão | `src`, `alt` e legenda são escapados; segredo e token aparecem só como placeholders |
| Idioma | tutorial e legendas existem em inglês e pt-BR no mesmo commit |

## Aceitação

- [x] Tutorial operacional nas duas locales, com todos os componentes do ecossistema.
- [x] Dez screenshots reais carregam no site e na exportação com `--base /trilha`.
- [x] Cloud executa `TASK-001` até `review`, mostra evidências e aprova para `done`.
- [x] Navegador real valida login, convite, primeira senha, troca de senha e novo login.
- [x] `go test ./site/...` e `git diff --check` passam.
