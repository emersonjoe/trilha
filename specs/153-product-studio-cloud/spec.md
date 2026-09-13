# Spec 153 — Product Studio do Trilha Cloud no tutorial

- **Issue**: [#246](https://github.com/emersonjoe/trilha/issues/246) — a issue é a fonte do escopo.
- **Branch**: `153-product-studio-cloud-docs`
- **Versão**: 0.132.0

## Por quê

O capítulo do Trilha Cloud documenta o fluxo completo do ecossistema, mas ainda exige que a
pessoa crie o projeto, a spec, a task e o worker em terminais separados. O Product Studio do
Cloud 0.2.0 passa a orquestrar esses componentes por uma UI; sem documentá-lo, a experiência
publicada fica atrás do produto homologado.

## O que muda

O capítulo em inglês e pt-BR ganha um percurso zero-CLI para o usuário: o operador habilita o
workspace gerenciado, e a pessoa cria uma aplicação de cadastro de usuários, dispara a primeira
task, inspeciona evidências e aprova a execução pela UI. Duas capturas reais registram o formulário
e o resultado aprovado. O percurso manual permanece como referência operacional.

## Fora de escopo

- Publicar ou hospedar automaticamente a aplicação gerada; o Product Studio 0.2.0 encerra em
  branch, commit, evidências e aprovação.
- Ocultar a configuração inicial do operador; segredos e diretório gerenciado continuam definidos
  no ambiente do Cloud.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| VI — teste primeiro | as capturas vêm da homologação real e o site passa por sua suíte antes do release |
| VII — segurança por padrão | a documentação explicita opt-in, tokens, allow-list e limite do workspace |
| Idioma | inglês e pt-BR recebem o mesmo fluxo e as mesmas imagens |

## Segurança e privacidade

- Ativos e fronteiras: token administrativo, API key de revisão, workspace gerenciado e credenciais do provedor de IA.
- OWASP ASVS 5.0 L2: autenticação nas operações, validação de entrada, menor privilégio e trilha de auditoria.
- OWASP Top 10:2025: injeção, controle de acesso, configuração insegura e falhas de logging são explicitamente limitados.
- Segredos ficam no ambiente do operador ou no `sessionStorage`; não aparecem nas capturas nem no repositório.

## Tarefas

- [x] T001 Capturar o formulário e as evidências da homologação real do Product Studio.
- [x] T002 Documentar o fluxo zero-CLI em inglês e pt-BR.
- [x] T003 Registrar segurança, limites e comandos de homologação.
- [x] T004 Atualizar `CHANGELOG.md`, versão e `ROADMAP.md`.
- [x] T005 Executar testes e `make release VERSION=0.132.0 ISSUES="246"`.

## Aceitação

- [x] O tutorial mostra como habilitar e acessar o Product Studio.
- [x] O usuário cria, executa, revisa e aprova uma aplicação sem usar CLI.
- [x] Trilha, Trilha Spec, Trilha Runner e Trilha Cloud têm seus papéis explicados.
- [x] As duas locales exibem capturas da UI homologada sem segredos.
