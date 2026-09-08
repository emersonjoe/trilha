# Spec 056 — Trilha no Windows

- **Issue**: #79 — a issue é a fonte do escopo; aponte para ela, não a reescreva aqui.
- **Branch**: `056-trilha-no-windows`
- **Versão**: 0.39.2

## Por quê

Quem segue o "Aprender → Sua primeira página" no Windows não passa do terceiro comando.
`trilha new` funciona, `go build` funciona, e `trilha dev` morre com uma mensagem que culpa o
`%PATH%` por um arquivo de 11 MB que está ali no disco:

```console
✗ erro:
exec: "...\agenda\.trilha\app": executable file not found in %PATH%
```

O motivo é uma regra do sistema, não um bug de lógica: `go build -o <caminho>` escreve o nome
literal que recebeu, então no Windows a saída é `app` e não `app.exe`, e o `exec.LookPath` só
aceita executável cuja extensão esteja no `PATHEXT`. Três lugares constroem um binário e o
executam em seguida — `dev`, `export` e o padrão do `build` —, e os três estão errados do
mesmo jeito.

Isso passou porque o CI é só `ubuntu-latest`: nenhum trabalho nunca rodou a CLI no Windows. E
não é só o produto que está vermelho lá — o próprio e2e constrói `trilha-cli` sem extensão e
falha antes da primeira asserção, de modo que a suíte não conseguiria enxergar o bug mesmo se
alguém a rodasse. Enquanto o Windows não estiver no CI, a próxima regressão chega igual.

## O que muda

**Um binário construído pela CLI recebe a extensão que o sistema exige para executá-lo.** No
Windows, `.exe`; nos demais, nada. Vale para os três lugares e também para o `-o` que a pessoa
escreve à mão, porque um `-o bin/app` que produz arquivo inexecutável não ajuda ninguém:

```console
> trilha build
✓ bin\agenda.exe (1.4s)

> trilha build -o bin/app
✓ bin\app.exe (1.3s)

> trilha build -o bin/app.exe    # já tem extensão: fica como está
✓ bin\app.exe (1.3s)

> trilha dev
→ http://localhost:3000
✓ pronto (1.1s)
```

Em Linux e macOS nada muda: `trilha build` continua escrevendo `bin/agenda`.

O repositório também passa a ser conferido no Windows: um trabalho `windows-latest` no CI com
`go vet ./...` e `go test ./...`. Para que esse trabalho tenha chance de ficar verde, entra um
`.gitattributes` com `* text=auto eol=lf` — sem ele o `core.autocrlf=true` (padrão do Git no
Windows, e do `actions/checkout`) entrega os golden files com CRLF e todo teste golden falha
num clone recém-feito.

## Fora de escopo

- **Compilação cruzada (`GOOS`/`GOARCH` no `trilha build`)** — pedido legítimo e independente;
  aqui a extensão sai de `runtime.GOOS`, o sistema que está rodando.
- **Renomear o binário que já existe em `.trilha/`** — o diretório é ignorado e descartável;
  a próxima construção escreve o nome certo.
- **Melhorar a mensagem do `exec.LookPath`** — com a extensão certa ela deixa de aparecer, e
  reescrever erro de biblioteca padrão custa mais do que rende.
- **`arm64` no CI** — nenhum bug conhecido depende de arquitetura.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `runtime.GOOS` e `path/filepath`; nenhum `require` novo. |
| IV — superfície pública pequena e estável | Nada exportado muda; `api/current.txt` fica igual. O ajudante é privado em cada pacote que constrói binário. |
| V — dev em < 2 s, produção em um binário | É exatamente o princípio que estava quebrado no Windows: o ciclo não chegava a existir e o binário de produção não executava. |
| VI — teste primeiro | O e2e no Windows é o teste que falha hoje (`TestE2E`, `TestEmbeddedAppE2E`, `TestGenerateContratoE2E`) e passa depois; o trabalho `windows-latest` é o que impede a volta. |

## Tarefas

- [ ] T001 `.gitattributes` com `* text=auto eol=lf`: sem isto o golden falha num clone
      Windows e não dá para distinguir regressão de fim de linha.
- [ ] T002 Teste que falha: `go test ./...` no Windows — os três e2e param no `exec` do
      `trilha-cli` que eles mesmos constroem, e `TestFileSaveStaysInTheDirectory` cobra
      `0o600` de um sistema que não tem modo Unix.
- [ ] T003 `exeName` em `cmd/trilha` e em `internal/dev`, e os três locais de construção
      passando por ele (`dev`, `export`, `build` — padrão e `-o`).
- [ ] T004 e2e e `file_test.go` corrigidos: o harness constrói com extensão e a asserção de
      modo vale onde o modo existe.
- [ ] T005 Trabalho `windows-latest` no `.github/workflows/ci.yml`.
- [ ] T006 Documentação nas duas locales (`learn/troubleshooting.md`,
      `aprender/problemas-comuns.md`) e a nota do `-o` na referência da CLI.
- [ ] T007 `CHANGELOG.md`, `version` em `cmd/trilha/main.go`, `make test` verde e
      `scripts/release.sh 0.39.2 --issues "79"`.

## Aceitação

- **SC-001** Num Windows, `trilha new x && cd x && trilha dev` responde 200 em
  `http://localhost:3000` — o percurso do "Sua primeira página".
- **SC-002** `go test ./...` verde no Windows, no Linux e no macOS.
- **SC-003** `trilha build` no Windows escreve `bin\<projeto>.exe` e o arquivo executa;
  `trilha build -o bin/app` escreve `bin\app.exe`; em Linux continua `bin/<projeto>`.
- **SC-004** `trilha export` termina e escreve `out/index.html` no Windows.
- **SC-005** O CI tem um trabalho `windows-latest` verde no PR desta spec.
