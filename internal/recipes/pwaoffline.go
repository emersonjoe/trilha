package recipes

func pwaOfflineRecipe() Recipe {
	return Recipe{
		Name: "pwa-offline",
		Summary: map[string]string{
			"en": "service worker for the declared routes and an outbox that resends forms when the network is back",
			"pt": "service worker das rotas declaradas e outbox que reenvia formulários quando a rede volta",
		},
		Doc:   "/recipes/pwa",
		Needs: []Need{{Recipe: "pwa", File: "public/manifest.webmanifest"}},
		Files: []File{
			{Rel: "public/sw.js", Body: swScript},
			{Rel: "internal/offline/offline.go", Go: true, Body: offlineHelper},
			{Rel: "internal/offline/offline_test.go", Go: true, Body: offlineHelperTest},
			{Rel: "{{.At}}coleta/page.go", Go: true, Body: offlinePage},
			{Rel: "{{.At}}coleta/offline.go", Go: true, Body: offlineFlag},
			{Rel: "coleta_test.go", Go: true, Body: offlineProjectTest},
		},
		Next: map[string]string{
			"en": "Run `trilha gen` and open {{.URL}}coleta. Turn the network off: the form goes into the browser's outbox and is sent, in order, when it comes back. What does not work offline, and says so instead of pretending: a large upload (a form carrying a file is never queued), an island or a fragment that needs the server to draw itself, and any screen that did not declare `var Offline = true`. The service worker only caches the routes that declared it, so the private area of another audience is never kept on the device.",
			"pt": "Rode `trilha gen` e abra {{.URL}}coleta. Desligue a rede: o formulário entra no outbox do navegador e é enviado, em ordem, quando ela volta. O que não funciona offline, e diz isso em vez de fingir: upload grande (formulário com arquivo nunca entra na fila), ilha ou fragmento que depende do servidor para se desenhar, e qualquer tela que não declarou `var Offline = true`. O service worker só faz cache das rotas que declararam, então a área privada de outro público nunca fica guardada no aparelho.",
		},
	}
}

// swScript is the service worker: the app shell, the declared routes, and
// nothing else. The list of routes comes from the page (ui.OfflineScript
// writes what App.OfflineRoutes answered), so a path nobody declared is never
// kept on the device.
const swScript = `// Service worker de {{.Name}}, escrito por ` + "`trilha add pwa-offline`" + `.
//
// O que ele guarda: a casca do app e as rotas que declararam
// ` + "`var Offline = true`" + `. Nada mais — uma resposta com Set-Cookie ou
// Cache-Control: no-store, e qualquer endereço fora da lista, passa direto.
"use strict";

const SHELL = [
  "/", "/ui.css", "/ui.theme.css", "/ui.js", "/ui.offline.js",
  "/manifest.webmanifest", "/icon-192.png", "/icon-512.png",
];
const KIT = /\/ui\.[a-z.]+\.(css|js)$|\/ui\.(css|js)$/;
let version = "v0";
let routes = [];

const nome = () => "trilha-" + version;

const guardar = async (req, res) => {
  if (!res || !res.ok || res.type === "opaque") return;
  if (res.headers.get("set-cookie")) return;
  const cc = res.headers.get("cache-control") || "";
  if (cc.includes("no-store") || cc.includes("private")) return;
  const cache = await caches.open(nome());
  await cache.put(req, res.clone());
};

const daLista = (url) => {
  const p = new URL(url).pathname;
  return SHELL.includes(p) || routes.includes(p);
};

const encher = async () => {
  const cache = await caches.open(nome());
  await Promise.all([...SHELL, ...routes].map((p) => cache.add(p).catch(() => {})));
};

const limpar = async () => {
  const velhos = (await caches.keys()).filter((k) => k.startsWith("trilha-") && k !== nome());
  await Promise.all(velhos.map((k) => caches.delete(k)));
};

self.addEventListener("install", (e) => {
  e.waitUntil(encher().then(() => self.skipWaiting()));
});

self.addEventListener("activate", (e) => {
  e.waitUntil(limpar().then(() => self.clients.claim()));
});

// A página diz o que pode ser guardado: as rotas declaradas e a versão do
// kit, que é o hash do conteúdo. Versão nova, cache novo, e o velho sai.
self.addEventListener("message", (e) => {
  const dados = e.data || {};
  if (Array.isArray(dados.routes)) routes = dados.routes;
  if (dados.version) version = dados.version;
  e.waitUntil(encher().then(limpar));
});

self.addEventListener("fetch", (e) => {
  const req = e.request;
  if (req.method !== "GET") return; // o outbox cuida do resto
  const url = new URL(req.url);
  if (url.origin !== location.origin) return;

  // Os arquivos do kit têm o hash no endereço: cache primeiro, sem rede.
  if (KIT.test(url.pathname)) {
    e.respondWith(caches.match(req).then((hit) => hit || fetch(req).then(async (res) => {
      await guardar(req, res);
      return res;
    })));
    return;
  }
  if (!daLista(url.href)) return; // endereço que ninguém declarou: rede, sempre

  // Rede primeiro: offline é o caso raro, e uma tela velha só aparece quando
  // não há alternativa.
  e.respondWith(fetch(req).then(async (res) => {
    await guardar(req, res);
    return res;
  }).catch(async () => (await caches.match(req)) || (await caches.match("/")) ||
    new Response("offline", { status: 503, headers: { "Content-Type": "text/plain" } })));
});
`

const offlineHelper = `// Package offline is what a screen that accepts a form filled without the
// network calls: the same submission arriving twice is done once, and the
// moment the person pressed the button is written down.
package offline

import (
	"sort"
	"sync"
	"time"

	"github.com/emersonjoe/trilha"
)

// Item é uma coleta guardada: o texto, quem mandou e o carimbo do aparelho.
type Item struct {
	Chave    string
	Texto    string
	Carimbo  time.Time
	Recebido time.Time
}

var (
	mu    sync.Mutex
	itens = map[string]Item{}
)

// Recebido diz se esta submissão já foi feita antes. A chave vem do cabeçalho
// Idempotency-Key ou do campo escondido de trilha.OfflineForm, e o carimbo do
// cliente entra na trilha: conflito aqui é último-escreve-ganha, com o carimbo
// registrado para quem for ler depois — e não um CRDT.
func Recebido(c *trilha.Ctx, ttl time.Duration) (bool, error) {
	repetida, err := trilha.Idempotent(c, ttl)
	if err != nil {
		return false, err
	}
	carimbo, temCarimbo := trilha.QueuedAt(c)
	campos := trilha.Fields{"repetida": repetida}
	if temCarimbo {
		campos["queued_at"] = carimbo.UTC().Format(time.RFC3339)
	}
	c.Audit("coleta.recebida", trilha.IdempotencyKey(c), campos)
	return repetida, nil
}

// Guardar grava a coleta. A mesma chave sobrescreve o que estava lá: quem
// escreveu por último ganha, e o carimbo do cliente fica guardado junto.
func Guardar(c *trilha.Ctx, texto string) {
	chave := trilha.IdempotencyKey(c)
	carimbo, _ := trilha.QueuedAt(c)
	mu.Lock()
	defer mu.Unlock()
	itens[chave] = Item{Chave: chave, Texto: texto, Carimbo: carimbo, Recebido: time.Now()}
}

// Todas devolve as coletas, da mais nova para a mais velha.
func Todas() []Item {
	mu.Lock()
	defer mu.Unlock()
	out := make([]Item, 0, len(itens))
	for _, it := range itens {
		out = append(out, it)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Recebido.After(out[j].Recebido) })
	return out
}
`

const offlineHelperTest = `package offline

import (
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/emersonjoe/trilha"
)

// A mesma chave entra uma vez: o reenvio do outbox não vira uma segunda
// coleta.
func TestMesmaChaveGuardaUmItem(t *testing.T) {
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	a := trilha.New(trilha.Config{Env: trilha.Prod})
	a.Register(trilha.Route{Pattern: "/_coleta", Kind: trilha.KindAPI,
		Methods: map[string]trilha.HandlerFunc{
			"POST": func(c *trilha.Ctx) error {
				repetida, err := Recebido(c, time.Hour)
				if err != nil {
					return err
				}
				if !repetida {
					Guardar(c, c.Form("texto"))
				}
				return c.Text(http.StatusOK, c.Form("texto"))
			},
		}})
	cli := trilha.NewTestClient(t, a)
	campos := url.Values{"_idempotency_key": {"k-1"}, "texto": {"poste 4"}}
	cli.PostForm("/_coleta", campos).WantStatus(http.StatusOK)
	cli.PostForm("/_coleta", campos).WantStatus(http.StatusOK)

	var achados int
	for _, it := range Todas() {
		if it.Chave == "k-1" {
			achados++
		}
	}
	if achados != 1 {
		t.Fatalf("a mesma chave virou %d coletas", achados)
	}
}
`

const offlineFlag = `package coleta

// Offline põe esta tela na lista que o service worker guarda: ela abre sem
// rede, e o formulário dela espera no outbox até a rede voltar.
var Offline = true
`

const offlinePage = `// Package coleta is the screen filled where the network comes and goes.
package coleta

import (
	"net/http"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"

	"{{.Module}}/internal/offline"
)

// janela é quanto tempo uma chave é lembrada: o outbox de um aparelho que
// ficou um dia sem rede ainda encontra a chave dele do lado de cá.
const janela = 48 * time.Hour

// Page desenha o formulário e o que já foi recebido.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("{{if eq .Lang "pt"}}Coleta{{else}}Collection{{end}}")
	linhas := []h.Node{}
	for _, item := range offline.Todas() {
		linhas = append(linhas, h.Li(h.Text(item.Texto)))
	}
	return ui.Container(
		ui.PageHeader("{{if eq .Lang "pt"}}Coleta{{else}}Collection{{end}}"),
		ui.Outbox(c),
		h.Form(h.Method("post"), h.Action("{{.URL}}coleta"),
			trilha.CSRFInput(c),
			trilha.OfflineForm(c),
			ui.Field("texto", "{{if eq .Lang "pt"}}Anotação{{else}}Note{{end}}", ui.Input(h.ID("texto"), h.Name("texto"), h.Required())),
			ui.Submit(h.Text("{{if eq .Lang "pt"}}Enviar{{else}}Send{{end}}")),
		),
		h.Ul(linhas...),
		ui.OfflineScript(c),
	), nil
}

// POST guarda a coleta. O reenvio do outbox chega com a mesma chave e é
// respondido do mesmo jeito, sem virar uma segunda linha.
func POST(c *trilha.Ctx) error {
	texto := c.Form("texto")
	if texto == "" {
		return trilha.Errorf(http.StatusBadRequest, "{{if eq .Lang "pt"}}escreva alguma coisa{{else}}write something{{end}}")
	}
	repetida, err := offline.Recebido(c, janela)
	if err != nil {
		return err
	}
	if !repetida {
		offline.Guardar(c, texto)
	}
	return c.Redirect("{{.URL}}coleta")
}
`

const offlineProjectTest = `package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha"
)

// O reenvio do outbox chega com a mesma Idempotency-Key: uma coleta só, e a
// segunda resposta é igual à primeira.
func TestColetaReenviadaNaoDuplica(t *testing.T) {
	t.Setenv("TRILHA_ENV", "prod")
	t.Setenv("TRILHA_SECRET", "a-test-secret-with-more-than-32-bytes!!")
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	c := trilha.NewTestClient(t, newApp())

	campos := url.Values{
		"_idempotency_key": {"chave-do-aparelho"},
		"_queued_at":       {"2026-01-02T03:04:05Z"},
		"texto":            {"poste 4 sem placa"},
	}
	primeira := c.PostForm("{{.URL}}coleta", campos).WantStatus(http.StatusSeeOther)
	segunda := c.PostForm("{{.URL}}coleta", campos).WantStatus(http.StatusSeeOther)
	if primeira.Header().Get("Location") != segunda.Header().Get("Location") {
		t.Fatalf("o reenvio foi respondido de outro jeito: %q e %q",
			primeira.Header().Get("Location"), segunda.Header().Get("Location"))
	}

	tela := c.Get("{{.URL}}coleta").WantStatus(http.StatusOK).Body.String()
	if n := strings.Count(tela, "poste 4 sem placa"); n != 1 {
		t.Fatalf("a coleta aparece %d vezes na tela:\n%s", n, tela)
	}
}
`
