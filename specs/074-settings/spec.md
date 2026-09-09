# Spec 074 — A configuração que muda sem redeploy

- **Issue**: #107 — a issue é a fonte do escopo; aponte para ela, não a reescreva aqui.
- **Branch**: `074-settings`
- **Versão**: 0.56.0

## Por quê

Toda aplicação escreve isto à mão: uma tabela de settings, um GET que devolve JSON, um PUT, uma
tela com um formulário por seção — e validação em nenhum deles. É o padrão mais repetido que
sobrou, e o único em que a struct já tem toda a informação necessária para gerar o resto.

## O que muda

`trilha.Settings[T]` com `NewSettings`, `Bind`, `Get`, `Set`, `Update`; `SettingsStore`;
`trilha.SchemaOf[T]`; `ui.SettingsForm`. O contrato está na referência do App, nas duas línguas.
O que vale registrar:

**As tags fazem três trabalhos e são as mesmas de sempre.** `json` guarda, `form` nomeia o input
— o mesmo nome que o `Bind` já lê —, `validate` é a regra. Nenhum vocabulário novo: a tela sai
das tags que a struct teria de qualquer jeito.

**Um 422 não grava nada.** É a metade que toda tela feita à mão erra — valida na tela e salva
assim mesmo —, e o teste que mais vale aqui é o que confere que o store não foi tocado.

**Seção de outra versão não derruba o app.** Campo renomeado entre dois deploys fica no padrão;
valor ilegível cai nos padrões com uma linha no log. Configuração vazia em produção é pior que
configuração desatualizada.

**A auditoria diz o que mudou, nunca o valor.** Tela de configuração é onde mora um token; uma
trilha que o copia é um segundo lugar de onde ele vaza. É a mesma disciplina da spec 067, agora
do lado de quem escreve na trilha.

**Store nil é memória, e o log avisa uma vez.** Serve para teste e para uma primeira versão;
esquecer no restart em silêncio, não.

**`SchemaOf` explode no que nenhum formulário segura** — mapa, fatia, struct aninhada. A
alternativa seria uma tela que silenciosamente não edita parte da própria configuração, e o erro
aparece na hora de escrever a tela, não em produção.

## Fora de escopo, com o motivo

- **`trilha.Secret` e `ui.SecretField`** — são a [#108](https://github.com/emersonjoe/trilha/issues/108),
  que está aberta. O `Settings` já não copia valores para a auditoria, que era o ponto onde o
  segredo vazaria primeiro; o campo cifrado entra quando aquela issue entrar.
- **`settings.SQL(db)`** — DDL de dialeto nenhum entra num framework sem dependência de banco. É
  a mesma razão pela qual o `audit.SQL` ficou fora na 0.49.0, e a interface de dois métodos
  existe justamente para a aplicação escrever o seu.
- **`[]string` com `oneof` virando checkboxes** — o `Schema` não tem tipo para escolha múltipla;
  inventá-lo aqui seria decidir pela #69 no corredor. Por isso fatia é o caso que explode, com a
  mensagem dizendo o que a struct aceita.
- **`trilha ctx` listando as seções** — mesma dívida de scanner da
  [#124](https://github.com/emersonjoe/trilha/issues/124).

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `encoding/json`, `reflect`, `sync`. |
| III — rota no exemplo | `/config` no `assistente`, e o chat lendo a configuração por requisição. |
| IV — superfície pequena | Um tipo genérico com cinco métodos, uma interface de dois, uma função de reflexão e um componente. |
| V — inglês no código, pt-BR junto | Referência do App nas duas línguas no mesmo commit. |
| VI — teste primeiro | Padrão sem nada gravado, grava e lê, versão antiga da struct, valor ilegível, 422 que não grava, gravação que audita sem valores, `SchemaOf` lendo as tags e recusando o que não cabe. |
| VII — segurança por padrão | A trilha nunca copia valores; a tela é um formulário com CSRF como qualquer outro. |

## Tarefas

1. `settings.go`: `Settings[T]`, `SettingsStore`, `SchemaOf[T]`, valores do formulário. ✅
2. `ui/settings.go`: `SettingsForm` sobre o `SchemaForm`. ✅
3. `examples/assistente`: seção, tela e o chat lendo por requisição. ✅
4. Referência do App en e pt; linha do `ui`. ✅
5. `api/current.txt`, catálogo do `ui`. ✅
