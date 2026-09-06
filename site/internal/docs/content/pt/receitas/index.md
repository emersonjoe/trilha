---
title: Receitas
description: As partes que todo app precisa e nenhum framework decide por você — banco, sessão, upload, paginação, e-mail, tarefa agendada, Docker — com código que compila.
---

"Aprender" ensina o framework e "Referência" descreve cada símbolo. Esta seção responde a um
terceiro tipo de pergunta, a que aparece no segundo dia: *como eu faço a coisa que todo app
faz?* Abrir um banco, manter alguém logado, receber um arquivo, paginar uma lista, mandar um
e-mail, rodar uma tarefa de hora em hora, colocar tudo isso num contêiner.

Nada disso é decisão do framework. Trilha não tem ORM, não tem store de sessão e não tem
mailer — o que ele tem é um lugar para os seus, e é aqui que esse lugar está escrito.

| Receita | O que responde |
|---|---|
| [Banco de dados](/pt/receitas/banco-de-dados) | pool, consultas, transação, migrações, sqlc |
| [Sessões](/pt/receitas/sessoes) | login, cookie assinado, usuário atual, flash |
| [Uploads](/pt/receitas/uploads) | receber um arquivo, validar, guardar, devolver |
| [Paginação](/pt/receitas/paginacao) | página e cursor, e o rodapé que vem com eles |
| [E-mail](/pt/receitas/email) | SMTP em produção, o log em dev, um corpo vindo de template |
| [Tarefas agendadas](/pt/receitas/tarefas-agendadas) | um ticker que sobe com o app e para com ele |
| [Docker](/pt/receitas/docker) | uma imagem pequena, as variáveis, a sonda de saúde |
| [Checklist de produção](/pt/receitas/checklist-de-producao) | o que conferir antes de publicar, em ordem |
| [Migração](/pt/receitas/migracao) | de `net/http` puro para Trilha, e entre versões menores |

## De onde vem o código

Todo bloco Go destas páginas é copiado de um arquivo em
[`examples/cookbook`](https://github.com/emersonjoe/trilha/tree/main/examples/cookbook), que
faz parte do módulo do repositório: `go vet ./...` compila esse código a cada rodada, e um
teste do site confere que cada bloco continua aparecendo, caractere por caractere, no arquivo
de onde saiu. Uma receita que para de compilar quebra o build antes de enganar alguém.

Isso tem um preço que vale conhecer: o pacote usa só a biblioteca padrão, como o resto do
repositório. Então não há driver de banco, nem hash de senha, nem cliente de métricas dentro
dele. Onde um deles é necessário, a página diz qual linha acrescentar e por que ela não está
aqui.

:::nota
As receitas assumem as convenções de [Páginas e rotas](/pt/aprender/paginas-e-rotas) e o
`app/setup.go` de [App](/pt/referencia/app). Se um trecho fala em `Setup`, ele mora nesse
arquivo; se fala em `Config`, ele roda antes de o app existir.
:::
