package main

// The messages of trilha precommit and trilha hooks, kept apart from i18n.go
// so neither file grows past what the limit it enforces allows.
func init() {
	for k, v := range precommitMsgs {
		msgs[k] = v
	}
}

var precommitMsgs = map[string][2]string{
	"flag precommit all": {"check every tracked file, not only the staged ones", "confere todo arquivo versionado, não só os staged"},
	"flag force hooks":   {"overwrite a locally modified pre-commit hook", "sobrescrever um hook de pre-commit modificado localmente"},
	"precommit ok":       {"trilha precommit: every staged file within the limits (5,000 tokens; 500 lines of source)\n", "trilha precommit: todo arquivo staged dentro dos limites (5.000 tokens; 500 linhas de código)\n"},
	"precommit head": {"trilha precommit: %d file(s) over the size limits — the commit was stopped.\n\n",
		"trilha precommit: %d arquivo(s) acima dos limites de tamanho — o commit foi barrado.\n\n"},
	"precommit lines":  {"  %-44s %d lines (limit %d)\n", "  %-44s %d linhas (limite %d)\n"},
	"precommit tokens": {"  %-44s %d tokens (limit %d; tokens = bytes/4)\n", "  %-44s %d tokens (limite %d; tokens = bytes/4)\n"},
	"precommit how": {`
Split each file now, then git add the new files and commit again:

  1. Find the seams first: the types, the groups of functions that call each
     other, the parts a reader looks for separately. Each new file gets one of
     them and a name that says which (invoice_total.go, not page2.go).
  2. Go: the new files stay in the same package and folder — nothing to import,
     no call changes. Move each type with its methods and constructor to
     <type>.go; helpers of one topic to <file>_<topic>.go; in app/, a page.go
     keeps Page and its route functions, and its components go to
     components.go, its data access to internal/<domain>.
  3. JavaScript/TypeScript: one module per concern, exported and imported by
     name; the original keeps only what it is named after.
  4. SQL: a large migration becomes several numbered files, one change each;
     never edit a migration already applied.
  5. Markdown, tests and other files over 5,000 tokens: split by section or by
     subject into files that link to each other (guide.md → guide-install.md,
     guide-deploy.md); a test file follows the file it tests.
  6. Never split by line count or in the middle of a function, and do not
     shorten names or delete comments to fit: the limit asks for structure.
  7. Run trilha check, then commit.

Not checked: lockfiles, vendored code, binaries, goldens, and generated files —
a "Code generated ... DO NOT EDIT." header, or linguist-generated in
.gitattributes, marks one.
`, `
Divida cada arquivo agora, depois dê git add nos arquivos novos e faça o commit de novo:

  1. Ache as costuras primeiro: os tipos, os grupos de funções que se chamam,
     as partes que quem lê procura separadamente. Cada arquivo novo fica com
     uma delas e um nome que diz qual (total_da_fatura.go, não page2.go).
  2. Go: os arquivos novos ficam no mesmo pacote e na mesma pasta — nada a
     importar, nenhuma chamada muda. Mova cada tipo com seus métodos e seu
     construtor para <tipo>.go; os auxiliares de um assunto para
     <arquivo>_<assunto>.go; em app/, o page.go fica com a Page e as funções da
     rota, os componentes vão para components.go e o acesso a dados para
     internal/<domínio>.
  3. JavaScript/TypeScript: um módulo por assunto, exportado e importado pelo
     nome; o original fica só com o que dá nome a ele.
  4. SQL: uma migração grande vira vários arquivos numerados, uma mudança cada;
     nunca edite uma migração já aplicada.
  5. Markdown, testes e outros arquivos acima de 5.000 tokens: divida por seção
     ou assunto em arquivos que se ligam (guia.md → guia-instalacao.md,
     guia-deploy.md); o arquivo de teste acompanha o arquivo que testa.
  6. Nunca divida por contagem de linhas nem no meio de uma função, e não
     encurte nomes nem apague comentários para caber: o limite pede estrutura.
  7. Rode o trilha check e faça o commit.

Não são conferidos: lockfiles, código vendorizado, binários, goldens e
arquivos gerados — um cabeçalho "Code generated ... DO NOT EDIT." ou
linguist-generated no .gitattributes marca um.
`},
	"hooks enabled":   {"git now runs .githooks/pre-commit (core.hooksPath)", "o git passa a rodar .githooks/pre-commit (core.hooksPath)"},
	"hooks already":   {"core.hooksPath already points at .githooks", "o core.hooksPath já aponta para .githooks"},
	"hooks no repo":   {"not a git repository yet: after git init, run trilha hooks to turn the hook on", "ainda não é um repositório git: depois do git init, rode trilha hooks para ligar o hook"},
	"hooks elsewhere": {"core.hooksPath points at another folder, left alone: call .githooks/pre-commit from your own pre-commit", "o core.hooksPath aponta para outra pasta, que ficou como está: chame .githooks/pre-commit do seu pre-commit"},
	"hooks per clone": {"core.hooksPath is local to each clone: whoever clones runs trilha hooks once", "o core.hooksPath é local a cada clone: quem clonar roda trilha hooks uma vez"},
}
