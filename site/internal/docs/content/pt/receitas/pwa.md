---
title: App instalável (PWA)
description: Adicione manifesto, ícones e convite progressivo de instalação sem fingir que suporte offline é automático.
---

Rode a receita na raiz da aplicação:

```bash
trilha add pwa
trilha gen
```

Ela escreve `public/manifest.webmanifest`, ícones PNG substituíveis de 192 px e 512 px,
`public/pwa.js`, `internal/pwa/invite.go` e uma página de ajuda em `/install`. Ponha o convite
onde o produto precisa dele, em geral no shell ou na tela de configurações:

```go
func PWAInvite(c *trilha.Ctx) h.Node {
	return ui.InstallApp(c, ui.InstallAppOpts{Help: "/install"})
}
```

O componente é progressivo. Sem JavaScript, mantém uma instrução útil sobre o menu do
navegador e o link de ajuda. Com `pwa.js`, Chromium recebe o `beforeinstallprompt` nativo,
iPhone e iPad recebem os passos do Safari, outros navegadores do iOS explicam que a instalação
precisa começar no Safari, e um app já instalado não vê o convite. O navegador registra o modo
standalone num cookie não sensível, exposto no servidor como `c.Standalone()`; ele é dica de
apresentação, nunca autorização nem identidade do aparelho.

Troque os dois ícones gerados antes de publicar. O manifesto usa `display: "standalone"` e a
raiz do app como `start_url`; edite esses campos se o produto viver sob outro caminho.

A receita deliberadamente **não** adiciona service worker. Instalação e comportamento offline
são decisões diferentes de produto: só faça cache das rotas e assets cujas regras de
desatualização, logout e tenant você consegue definir.

Use [`ui.InstallApp`](/pt/referencia/ui#installapp) diretamente quando os arquivos já existem
ou o texto precisa ser específico da aplicação.

## Offline: a casca do app e um outbox de formulários

Dois produtos pediram a mesma coisa — coleta no campo e balcão com conexão ruim — então existe
uma segunda receita, escrita sobre esta:

```bash
trilha add pwa-offline
trilha gen
```

Ela escreve `public/sw.js` (o service worker), `internal/offline/offline.go`, uma tela
`/coleta` com o padrão inteiro e o teste que prova que o reenvio não duplica.

**O que fica guardado.** A própria página diz, como `var Kind` e `var CORS` dizem:

```go
var Offline = true
```

`a.OfflineRoutes()` devolve os padrões das páginas que declararam, em ordem, e
`ui.OfflineScript(c)` escreve essa lista na página (`data-ui-offline-routes`) com o hash do
conteúdo da folha de estilo do kit como versão do cache. O worker guarda a casca do app e
essas rotas, e nada mais: endereço que ninguém declarou vai sempre para a rede, e é assim que
a área privada de outro `Audience` nunca cai no aparelho. Resposta com `Set-Cookie` ou
`Cache-Control: no-store` nunca é guardada, e uma versão nova apaga o cache velho no activate.

**O outbox.** Um formulário marcado com `trilha.OfflineForm(c)` ganha uma chave de
idempotência gerada no servidor (o campo escondido `_idempotency_key`) e um `_queued_at`
vazio:

```go
func formulario(c *trilha.Ctx) h.Node {
	return h.Form(h.Method("post"), h.Action("/coleta"),
		trilha.CSRFInput(c),
		trilha.OfflineForm(c),
		ui.Field("texto", "Anotação", ui.Input(h.ID("texto"), h.Name("texto"), h.Required())),
		ui.Submit(h.Text("Enviar")),
	)
}
```

Com `ui.OfflineScript(c)` na página, um envio sem rede entra no IndexedDB (`trilha-outbox`)
com o carimbo do cliente em `_queued_at` e sai em ordem quando a rede volta — `POST`,
`credentials: same-origin`, a chave no cabeçalho `Idempotency-Key`, o campo do CSRF já no
corpo. `ui.Outbox(c)` mostra quantos esperam ("3 pendentes"), o último erro que o servidor
devolveu e um botão que tenta de novo agora.

**Do lado do servidor**, o reenvio é reconhecido e respondido do mesmo jeito:

```go
func POST(c *trilha.Ctx) error {
	texto := c.Form("texto")
	if texto == "" {
		return trilha.Errorf(http.StatusBadRequest, "escreva alguma coisa")
	}
	repetida, err := trilha.Idempotent(c, janela)
	if err != nil {
		return err
	}
	if !repetida {
		guardar(c, texto)
	}
	return c.Redirect("/coleta")
}
```

`trilha.Idempotent` lê a chave do cabeçalho `Idempotency-Key` ou do campo escondido
(`trilha.IdempotencyKey(c)` é a mesma busca; `trilha.NewIdempotencyKey()` gera uma, e
`trilha.OfflineKeyField` e `trilha.OfflineQueuedAtField` são os nomes dos campos), registra e
grava uma linha `offline.replay` com `c.Audit` quando já viu. As chaves vivem em
`Config.Idempotency` — um `trilha.IdempotencyStore`, e nil as guarda no processo, com teto e
com prazo, o que é honesto para uma réplica só.

`trilha.QueuedAt(c)` é quando a pessoa apertou o botão, lido de `_queued_at`. É relógio de
cliente: use como o carimbo que você registra ao lado do valor que ficou — último escreve,
último ganha, com o carimbo gravado — nunca como autoridade sobre a ordem. Essa é a história
de conflito aqui; um CRDT não é o que um caderno de campo precisa.

**O que não funciona offline**, e diz isso em vez de fingir:

- upload grande: formulário com arquivo nunca entra na fila, porque os bytes não sobrevivem à
  espera;
- ilha ou fragmento que depende do servidor para se desenhar (`ui.Defer`, `ui.Live`,
  `ui.Poll`, um alvo de `Swap`);
- qualquer tela que não declarou `var Offline = true`, e qualquer coisa sob a sessão de outro
  público;
- reenvio não é uma segunda submissão: a mesma chave responde a mesma coisa, então um handler
  que precisa ser repetível é a única coisa que a aplicação ainda deve.
