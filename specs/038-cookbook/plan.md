# Plano: Cookbook, checklist de produção e guia de migração

**Spec**: [spec.md](./spec.md) · **Issue**: #38

## Decisões

1. **Terceira seção, não um capítulo novo em "Aprender".** Aprender é linear e se lê uma vez;
   receita se procura. Misturar as duas coisas piora as duas.
2. **O código mora em `examples/cookbook/`, não em blocos soltos.** É um pacote do módulo:
   `go vet ./...` e `gofmt` passam por ele, então uma receita que deixa de compilar quebra a
   suíte antes de chegar ao site.
3. **A verificação é literal, não por compilação de trecho.** Compilar cada bloco exigiria
   montar um módulo temporário por bloco; casar o texto do bloco com o arquivo compilado dá a
   mesma garantia com um `strings.Contains`. O custo é disciplina: o bloco é copiado do
   arquivo, com a indentação do arquivo, e portanto é declaração de topo.
4. **Sem driver de banco.** `database/sql` compila sem driver registrado; o `sql.Open("pgx",
   dsn)` da receita é código legítimo que falharia só em tempo de execução, e a linha do
   `import _ "…/pgx/v5/stdlib"` aparece como texto na página.
5. **A pt ganha `/pt/receitas` e o atalho antigo `/receitas`**, como as outras seções, porque o
   `setup.go` do site declara o caminho sem prefixo para toda página pt.

## Fases

1. **Teste** — `TestCookbookSnippetsAreReal` em `site/site_test.go`: para cada página das
   seções `cookbook`/`receitas`, todo bloco ```go` tem de aparecer em algum `.go` do repositório.
2. **Código** — `examples/cookbook/`: `db.go`, `sessions.go`, `uploads.go`, `pagination.go`,
   `email.go`, `jobs.go`, com um `doc.go` dizendo que o pacote existe para a documentação.
3. **Conteúdo** — as dez páginas em `en/cookbook/` e `pt/receitas/`.
4. **Ligação** — seção em `docs.Locales`, rotas `site/app/cookbook/**`, `site/app/pt/receitas/**`
   e o atalho `site/app/receitas/**`; `trilha gen` no site.
5. **Fechamento** — `CHANGELOG.md` (0.29.0), `version`, ROADMAP item 20, `make test`, release.

## Riscos

- **Bloco que envelhece.** O teste casa texto: mudar o arquivo sem mudar a página quebra a
  suíte — que é exatamente o efeito desejado.
- **Página longa demais.** Cada receita responde uma pergunta; o que for explicação de conceito
  fica em "Aprender", com link.
