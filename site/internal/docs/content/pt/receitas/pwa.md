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
