# Feature Specification: Redirect seguido, formulário na região e URL declarada

**Feature Branch**: `172-redirect-form-e-url` | **Created**: 2026-10-06 | **Status**: Entregue (0.153.0)
**Input**: issues #291 (seguir o `Trilha-Location`), #292 (formulário dentro de `ui.Navigate`),
#293 (`c.PushURL`/`c.ReplaceURL`). As issues são a fonte do escopo e dos critérios.

## Contexto

Com a 171, um link dentro de `ui.Navigate` troca a região sem recarregar. O resto da tela
ainda recarrega: o POST com `c.Flash` + `c.Redirect` — o PRG que o Trilha ensina — desfaz
qualquer `ui.Swap` (#291); um formulário comum na região, GET ou POST, navega de verdade (#292);
e quem responde o fragmento no próprio POST para economizar a segunda ida não tem como pôr na
barra o endereço do que desenhou (#293). No Acervo, o botão "Filtrar" e o "Publicar" recarregam
ao lado de links que não recarregam: pior que antes, porque a diferença fica visível.

Antes disso, a primeira tarefa: a CI da `main` (job `test (1.25)`) reprovava no `bench/`
(`TestMeasureSeries`), escondida até a 0.151.0 pelo gofmt do Go 1.22. `countWritten` perguntava
"o mtime é depois do início?", e o kernel carimba o arquivo com um relógio grosso que fica
alguns ms atrás do `time.Now()`. Agora compara carimbos antes e depois da execução, com
regressão (`TestCountWrittenIgnoresClockGranularity`).

## Jornada e risco

"Preencher e salvar" e "navegar e listar" de quem usa o app. Pior impacto: um POST enviado de
novo pelo Voltar, um aviso que some ou aparece duas vezes, a barra mostrando um endereço que
não desenha a tela (F5 e link compartilhado mostram outra coisa).

## Decisões

1. **Seguir é o padrão onde já se navega no lugar** (página com região `ui.Navigate`); fora,
   opt-in por `ui.Follow()`, e `ui.NoFollow()` desliga. Quem não manda `Trilha-Follow`
   (cliente antigo, `curl`, teste) recebe a resposta de sempre.
2. **Flash no modo seguir vai no cabeçalho, sem cookie** — o toast mostra depois da troca. Se o
   cliente desiste no caminho, guarda a lista em `sessionStorage` e a página que carrega mostra
   uma vez. `RedirectReload` volta ao cookie, porque a página carrega inteira.
3. **O POST da região vai sem `Trilha-Fragment`**: a rota responde o 303 ou o 422 de sempre,
   nenhuma rota muda. Desvio da #292: quando o POST redirecionou e o destino não tem a região, o
   cliente carrega `res.url` (o destino) em vez de `f.submit()` — o POST já foi aceito, e
   reenviar seria gravar duas vezes. `f.submit()` fica para 5xx, erro de rede e resposta que não
   é HTML ou é download.
4. **`PushURL`/`ReplaceURL` aceitam um caminho como `Redirect`**, sem acrescentar o `BasePath`:
   é o mesmo contrato do `Redirect` (o app prefixa com `c.Base()`), e dois verbos de URL com
   regras diferentes seriam a pegadinha. Endereço de fora é descartado com aviso em dev.
5. **Quem escreve o histórico é o `ask`, depois da troca**: o gatilho diz o padrão (`push` no
   link, `replace` no GET, nada no POST); `Trilha-Push-Url` empurra no lugar dele,
   `Trilha-Replace-Url` reescreve a entrada que o gatilho fez, `"false"` não mexe. Ao empurrar,
   a entrada que fica para trás ganha estado, e o Voltar a reconstrói — antes, o Voltar para a
   primeira entrada de uma página com `ui.Swap` não refazia nada (achado pelo cenário da #293).
6. **A detecção da região fica no `ui.nav.js`** (`ui.navRegion`), para o `ui.js` continuar sem
   nada de navegação (`TestNavigateIsOptIn`).

## User Scenarios & Testing

Os cenários estão em `uitest/follow_test.go` e no catálogo `uitest/JORNADAS.md`:
`TestUISwapFollowsRedirect`, `TestUISwapRedirectReload`, `TestUISwapFollowFallsBack`,
`TestUIUploadFollowsRedirect`, `TestUIRegionFormsNavigate`, `TestUIRegionForm422`,
`TestUIRegionLinkRedirect`, `TestUIServerDeclaresURL`. No servidor, `follow_test.go`:
`TestFlashEmFragmentoSeguidoVaiNoCabecalho`, `TestRedirectReload`, `TestPushURLEReplaceURL`; e
os testes antigos de flash e fragmento continuam valendo para quem não segue.

## Requisitos

- **FR-001** `Trilha-Follow: 1` no pedido + `Trilha-Location` na resposta → flash em
  `Trilha-Flash`, sem cookie; sem o cabeçalho, nada muda.
- **FR-002** `trilha.RedirectReload`/`c.RedirectReload`: 303 sem fragmento; com fragmento, 204 +
  `Trilha-Location` + `Trilha-Reload: 1`; recusa endereço de fora como `Redirect`.
- **FR-003** `c.PushURL`/`c.ReplaceURL` escrevem `Trilha-Push-Url`/`Trilha-Replace-Url` só em
  fragmento e só para caminho local; `ReplaceURL("")` escreve `false`.
- **FR-004** Cliente: segue `Trilha-Location` de mesma origem pela região (`ui.navigate`) ou
  pelo mesmo alvo, empurra o destino, mostra o flash depois; recua com o flash guardado.
  `ui.UploadTo` igual.
- **FR-005** `ui.nav.js` intercepta `submit` de formulário da região sem `Swap`/`UploadTo`,
  com `formaction`/`formmethod`/`formtarget`, um POST por vez, GET abortável; link redirecionado
  é um GET só, com os toasts do destino.
- **FR-006** `ui.PushHistory()`, `ui.Follow()`, `ui.NoFollow()`.

## Segurança e privacidade (NIST SSDF 1.1 · OWASP ASVS 5.0 N2)

- **Fronteiras**: o cliente só segue `Trilha-Location`, `Trilha-Push-Url` e `Trilha-Replace-Url`
  de mesma origem; o servidor só escreve caminho local (`localPath`, o mesmo do `Redirect`) —
  nada de redirect aberto pelo histórico (ASVS V3.4, V5.1.5; Top 10 A01:2025).
- **CSRF**: o POST da região vai pelo `fetch` com `credentials: "same-origin"` e o token no
  corpo; a checagem do servidor não mudou e os cenários passam com ela ligada (V3.5).
- **Integridade**: POST nunca é abortado nem reenviado pelo cliente (V2.3, lógica de negócio).
- **Dados**: o flash guardado em `sessionStorage` é o texto que a própria página mostraria,
  da mesma origem, apagado na leitura; nada de dado novo.
- **Exceções**: nenhuma.

## Success Criteria

- **SC-001** Salvar com `c.Flash` + `c.Redirect` numa página que navega no lugar não recarrega,
  leva a barra ao destino e mostra o aviso uma vez; Voltar não reenvia.
- **SC-002** Filtrar e salvar com formulário comum dentro da região não recarregam.
- **SC-003** O endereço declarado pelo servidor é o da barra, e Voltar reconstrói a tela.
- **SC-004** Tudo nos três motores; a CI da `main` verde no `bench`.

## Evidências

| Nível | Comando | Passaram | Falharam | Pularam |
|---|---|---|---|---|
| Unidade + integração | `make test` | 1590 | 0 | 2 |
| Navegador, 27 cenários × 3 motores + 5 do módulo | `UITEST_REQUIRED=1 UITEST_BROWSERS=all make test-ui` | 32 | 0 | 0 |
| Bench (o job que estava vermelho) | `cd bench && go test ./...` | ok | 0 | 0 |

Pulos: `TestS3AoVivo` e `TestOIDCAoVivo` (serviço real, por desenho). Sem teste: o risco de
"moldura velha" (contador do menu) depois de um POST seguido é documentado e tem
`RedirectReload` como saída, não um teste — depende do que cada app põe fora da região.
