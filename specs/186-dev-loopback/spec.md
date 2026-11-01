# Spec 186 — O trilha dev fica nesta máquina por padrão

**Feature Branch**: `186-dev-loopback` | **Created**: 2026-11-01 | **Status**: Entregue (0.164.1)

- **Issue**: #318 — a fonte do escopo.
- **Branch**: `186-dev-loopback`
- **Versão**: 0.164.1

## Por quê

O `trilha dev` escutava em `:3000`, ou seja, em todas as interfaces. A porta do dev serve o que
produção esconde: a página de erro com detalhes, o erro de compilação com trecho do código, o
inspetor de rotas, o `/_trilha/report` e o app com os dados de desenvolvimento. Num café ou
num coworking, qualquer máquina da mesma rede abria `http://<ip-do-notebook>:3000`. O Vite
mudou para `localhost` por padrão pelo mesmo motivo.

## Jornada e risco

Jornada: quem escreve o app roda `trilha dev` e abre o navegador. Pior impacto do problema:
código-fonte e dados de desenvolvimento vistos por quem está na mesma rede. Pior impacto da
mudança: quem roda o dev dentro de um container (`docker run -p 3000:3000`) ou abre o app no
celular deixa de alcançar a porta. O primeiro caso é tratado pela detecção de container; o
segundo, pelo `-addr` e pelo aviso.

## O que muda

- Padrão do `-addr`: `127.0.0.1:3000`.
- Dentro de um container (`/.dockerenv`, `/run/.containerenv` ou `KUBERNETES_SERVICE_HOST`),
  sem `-addr` explícito, o dev continua em `:3000`, porque só assim a porta publicada chega
  nele. A CLI diz que detectou o container.
- `-addr :3000` ou `-addr 0.0.0.0:3000` continua funcionando para o celular na mesma rede, a
  VM ou o container.
- Sempre que o endereço não é de loopback, a CLI avisa antes de subir que a porta está aberta
  para a rede e o que fica exposto. A mensagem segue o `TRILHA_LANG`.

```text
$ trilha dev -addr :3000
⚠ the dev server listens on :3000, beyond this machine: anyone on this network can open its
  error pages with source, the route inspector and /_trilha/report.
  Keep it local with -addr 127.0.0.1:3000.
→ http://localhost:3000
```

## Fora de escopo

- O endereço do binário de produção (`PORT`/`ADDR`, padrão `:3000`): em produção, o processo
  dentro de um container precisa de todas as interfaces, e não serve página de dev.
- Autenticação no dev server: com loopback por padrão, não se justifica.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `net`, `os` apenas. |
| VI — teste primeiro | Os testes do endereço padrão e do aviso reprovam antes da mudança. |
| VII — segurança por padrão | O padrão passa a ser o que não expõe; expor é uma escolha explícita e avisada. |

## Segurança e privacidade

- Ativos e fronteiras de confiança afetados: o código-fonte e os dados de desenvolvimento
  servidos pelo dev; a fronteira entre esta máquina e a rede local.
- Controles OWASP ASVS 5.0 nível 2 aplicáveis: V13.4 (não expor informação de depuração e
  funcionalidade de desenvolvimento).
- Riscos OWASP Top 10:2025 considerados: A02 — configuração insegura.
- Segredos, dados pessoais, logs, limites e exceções: exceção registrada para container, onde o
  padrão continua `:3000` com aviso.

## Tarefas

- [x] T001 Testes que falham: padrão em loopback; container mantém `:3000`; `-addr`
  explícito é respeitado; tabela de endereço exposto; aviso nas duas línguas; `trilha dev -h`
  mostra o padrão novo (e2e)
- [x] T002 Implementação em `cmd/trilha/dev.go` e mensagens em `cmd/trilha/i18n.go`
- [x] T003 Exemplo: `make dev-example` usa o padrão, sem mudança
- [x] T004 Documentação nas duas locales (referência da CLI, solução de problemas)
- [x] T005 `CHANGELOG.md`, `version` em `cmd/trilha/main.go`, item do `ROADMAP.md`
- [x] T006 Suíte inteira e os números em *Evidências*

## Aceitação

- **SC-001** `trilha dev` sem `-addr`, fora de container, escuta só em `127.0.0.1`.
- **SC-002** Em container, sem `-addr`, escuta em `:3000` e diz por quê.
- **SC-003** Com `-addr :3000`, escuta em todas as interfaces e avisa o que fica exposto, em
  inglês ou português conforme o `TRILHA_LANG`.

## Evidências

| Nível | Comando | Passaram | Falharam | Pularam |
|---|---|---|---|---|
| Unidade + integração + e2e da CLI | `make test` (duas rodadas) | 1654 | 0 | 2 |

Os dois pulados são `TestS3AoVivo` e `TestOIDCAoVivo`, que dependem de serviço externo.

Sem teste e por quê: nenhum e2e sobe o `trilha dev` e confere de fora que a porta recusa
conexão pela rede. Seria preciso um processo com `go build` filho e uma segunda interface de
rede no CI (inclusive no job Windows), o que é frágil. A escolha do endereço, a detecção de
exposição e o aviso nas duas línguas são testados em unidade (`cmd/trilha/dev_test.go`), e o
e2e confere o padrão no `trilha dev -h`. O `net.Listen` com o endereço escolhido é a
biblioteca padrão. A detecção de container lê só arquivos e variável de ambiente; a função é
trocável nos testes, mas não há teste que a rode dentro de um container real.
