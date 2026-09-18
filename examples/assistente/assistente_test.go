package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/ai"
	apivoz "github.com/emersonjoe/trilha/examples/assistente/app/api/voz"
	"github.com/emersonjoe/trilha/examples/assistente/internal/config"
	"github.com/emersonjoe/trilha/examples/assistente/internal/ferramentas"
)

// The audio half of the fake provider keeps what arrived, and audioStatus
// scripts a refusal.
var (
	audioStatus int
	audioSent   int
	audioLang   string
	spokenText  string
)

// fakeModel is a scripted OpenAI-compatible server: on the first call it asks
// for the "calcular" tool, on the second it answers with the tool output.
func fakeModel(t *testing.T) *httptest.Server {
	calls := 0
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The audio half of the protocol: the same fake server answers the
		// transcription and the speech, so the voice route is exercised
		// end to end without a provider.
		switch r.URL.Path {
		case "/audio/transcriptions":
			if audioStatus != 0 {
				w.WriteHeader(audioStatus)
				_, _ = w.Write([]byte(`{"error":{"message":"scripted failure"}}`))
				return
			}
			if err := r.ParseMultipartForm(8 << 20); err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			f, _, err := r.FormFile("file")
			if err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			defer f.Close()
			b, _ := io.ReadAll(f)
			audioSent = len(b)
			audioLang = r.FormValue("language")
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"text":%q}`, "onde fica o guichê?")
			return
		case "/audio/speech":
			if audioStatus != 0 {
				w.WriteHeader(audioStatus)
				_, _ = w.Write([]byte(`{"error":{"message":"scripted failure"}}`))
				return
			}
			var in map[string]any
			_ = json.NewDecoder(r.Body).Decode(&in)
			spokenText, _ = in["input"].(string)
			w.Header().Set("Content-Type", "audio/mpeg")
			_, _ = w.Write([]byte("ID3fake-mp3"))
			return
		}
		var req ai.Request
		_ = json.NewDecoder(r.Body).Decode(&req)
		calls++
		if !req.Stream {
			// The request that did not ask for a stream: one answer, whole.
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(ai.Response{ID: "1", Model: "fake",
				Choices: []ai.Choice{{Message: ai.Message{Role: "assistant", Content: "O resultado é 20."}, FinishReason: "stop"}},
				Usage:   ai.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15}})
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		chunk := func(delta string) {
			fmt.Fprintf(w, "data: %s\n\n", delta)
		}
		if calls == 1 {
			chunk(`{"choices":[{"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"c1","type":"function","function":{"name":"calcular","arguments":"{\"expressao\":"}}]}}]}`)
			chunk(`{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"(2+3)*4\"}"}}]}}]}`)
			chunk(`{"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`)
		} else {
			last := req.Messages[len(req.Messages)-1]
			for _, piece := range []string{"O resultado ", "é " + last.Content + "."} {
				b, _ := json.Marshal(piece)
				chunk(`{"choices":[{"delta":{"content":` + string(b) + `}}]}`)
			}
			chunk(`{"choices":[{"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`)
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
}

func newClient(t *testing.T) *trilha.TestClient {
	t.Helper()
	t.Setenv("TRILHA_ENV", "dev")
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	fm := fakeModel(t)
	t.Cleanup(fm.Close)
	ferramentas.Client = &ai.Client{BaseURL: fm.URL, Model: "fake"}
	ferramentas.Reset()
	audioStatus, audioSent, audioLang, spokenText = 0, 0, "", ""
	ferramentas.Now = func() time.Time { return time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC) }
	return trilha.NewTestClient(t, newApp())
}

func TestPageRenders(t *testing.T) {
	res := newClient(t).Get("/").WantStatus(200).WantContains(`class="ui-chat"`, `data-trilha-chat="/api/chat"`, "calcular")
	if csp := res.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "script-src 'self'") {
		t.Fatal(csp)
	}
}

func TestChatStreams(t *testing.T) {
	c := newClient(t)
	res := c.Request("POST", "/api/chat",
		trilha.WithHeader("Accept", "text/event-stream"),
		trilha.WithJSON(map[string]string{"message": "quanto é (2+3)*4?"}),
	).WantStatus(200)
	if !strings.HasPrefix(res.Header().Get("Content-Type"), "text/event-stream") {
		t.Fatal(res.Header())
	}
	res.WantContains(
		`event: tool_call`,
		`"tool":"calcular"`,
		`event: tool_result`,
		`"output":"20"`,
		"event: text\ndata: O resultado \n\n",
		"event: text\ndata: é 20.\n\n",
		"event: done",
		`"total_tokens":15`,
		// The finished answer comes rendered: ui.ChatHTML is wired in the route.
		`\u003cp\u003eO resultado é 20.\u003c/p\u003e`,
	)
	// Without the header the same route answers the whole thing at once — the
	// request that arrives when the script is not there.
	c.PostJSON("/api/chat", map[string]string{"message": "quanto é (2+3)*4?"}).
		WantStatus(200).WantContains(`"output":"O resultado é 20."`)
	// Validation.
	c.PostJSON("/api/chat", map[string]string{"message": "  "}).WantStatus(422)
}

func TestMCPEndpoint(t *testing.T) {
	c := newClient(t)
	post := func(body, session string) *trilha.TestResponse {
		opts := []trilha.TestOption{trilha.WithBody("application/json", body)}
		if session != "" {
			opts = append(opts, trilha.WithHeader("Mcp-Session-Id", session))
		}
		return c.Request("POST", "/mcp", opts...)
	}
	res := post(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`, "")
	res.WantStatus(200).WantContains(`"trilha-assistente"`)
	sid := res.Header().Get("Mcp-Session-Id")
	if sid == "" {
		t.Fatal("sem sessão o cliente não consegue seguir")
	}
	post(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"salvar_nota","arguments":{"titulo":"compras","texto":"leite"}}}`, sid).
		WantStatus(200).WantContains("nota salva: compras")
	post(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"listar_notas"}}`, sid).
		WantContains("compras: leite")
	post(`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"hora_atual","arguments":{"fuso":"UTC"}}}`, sid).
		WantContains("05/09/2026 12:00 (UTC)")
	// Sem sessão não há endpoint.
	post(`{"jsonrpc":"2.0","id":5,"method":"tools/list"}`, "").WantStatus(404)
}

func TestCalcular(t *testing.T) {
	for in, want := range map[string]string{"1+2*3": "7", "(1+2)*3": "9", "-4/2": "-2", "10/4": "2.5"} {
		out, err := ferramentas.Tools[1].Func(context.Background(), json.RawMessage(`{"expressao":"`+in+`"}`))
		if err != nil || out != want {
			t.Fatal(in, out, err)
		}
	}
	for _, bad := range []string{"1/0", "2+", "abc"} {
		if _, err := ferramentas.Tools[1].Func(context.Background(), json.RawMessage(`{"expressao":"`+bad+`"}`)); err == nil {
			t.Fatal("expected error for", bad)
		}
	}
}

// #107 — a tela de configuração vem da struct: um campo por campo, do tipo que
// as tags pediram. E o POST é uma linha.
func TestConfiguracaoVemDaStruct(t *testing.T) {
	c := newClient(t)

	tela := c.Get("/config")
	tela.WantStatus(200).WantContains(
		`name="modelo"`, "Modelo", "o nome que o provedor conhece",
		`name="temperatura"`, `type="number"`, `min="0"`, `max="2"`,
		`name="ferramentas"`, `type="checkbox"`, `name="_csrf"`)

	// O que não passa na validação da própria struct não é gravado.
	ruim := c.Request("POST", "/config", trilha.WithBody("application/x-www-form-urlencoded",
		"modelo=&temperatura=9&max_tokens=1024&ferramentas=on"))
	ruim.WantStatus(422).WantContains("required", "must be 2 or less")
	if config.Cfg.Get().Temperatura == 9 {
		t.Fatal("gravou o que não passou")
	}

	// O que passa vale na próxima mensagem, sem reiniciar nada.
	bom := c.Request("POST", "/config", trilha.WithBody("application/x-www-form-urlencoded",
		"modelo=llama3.1&temperatura=0.2&max_tokens=512&ferramentas=on"))
	if bom.Code != 303 {
		t.Fatalf("POST bom = %d %s", bom.Code, bom.Body.String())
	}
	if got := config.Cfg.Get(); got.Modelo != "llama3.1" || got.Temperatura != 0.2 || got.MaxTokens != 512 {
		t.Fatalf("%+v", got)
	}
	c.Get("/config").WantStatus(200).WantContains(`value="llama3.1"`, `value="0.2"`)
}

// #108 — a chave do provedor é um trilha.Secret: o campo nunca volta
// preenchido, o vazio mantém o que está guardado, e nada disso aparece no JSON
// nem no log.
func TestChaveDoProvedorEhSegredo(t *testing.T) {
	c := newClient(t)

	// Um campo de senha, vazio, com a frase que faz o padrão funcionar.
	c.Get("/config").WantStatus(200).WantContains(`name="chave"`, `type="password"`, "leave blank")

	c.Request("POST", "/config", trilha.WithBody("application/x-www-form-urlencoded",
		"modelo=llama3.1&temperatura=0.2&max_tokens=512&ferramentas=on&chave=sk-abcdef123456")).WantStatus(303)
	if got := config.Cfg.Get().Chave.Reveal(); got != "sk-abcdef123456" {
		t.Fatalf("chave = %q", got)
	}

	// A tela de volta não traz a chave — nem inteira, nem mascarada dentro do
	// campo: máscara no campo é máscara que volta como valor no próximo envio.
	tela := c.Get("/config").WantStatus(200)
	if strings.Contains(tela.Body.String(), "abcdef123456") {
		t.Fatal("a chave voltou para o navegador")
	}
	if strings.Contains(tela.Body.String(), `name="chave" type="password" value="`) {
		t.Fatal("o campo de senha voltou preenchido")
	}

	// E salvar de novo com o campo vazio mantém o que estava lá.
	c.Request("POST", "/config", trilha.WithBody("application/x-www-form-urlencoded",
		"modelo=llama3.1&temperatura=0.5&max_tokens=512&ferramentas=on&chave=")).WantStatus(303)
	if got := config.Cfg.Get().Chave.Reveal(); got != "sk-abcdef123456" {
		t.Fatalf("o vazio apagou a chave: %q", got)
	}
	if config.Cfg.Get().Temperatura != 0.5 {
		t.Fatal("o resto do formulário devia ter mudado")
	}
}

// ---- #269: voz -------------------------------------------------------------

// gravacao builds the multipart a browser sends: the recording, the language
// and the CSRF field the form carries.
func gravacao(t *testing.T, bytes int, idioma string) (string, string) {
	t.Helper()
	var body strings.Builder
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("audio", "recording.webm")
	if err != nil {
		t.Fatal(err)
	}
	// A WebM header, so the type detected in the content is a real one.
	if _, err := fw.Write(append([]byte{0x1A, 0x45, 0xDF, 0xA3}, make([]byte, bytes)...)); err != nil {
		t.Fatal(err)
	}
	if err := mw.WriteField("idioma", idioma); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	return mw.FormDataContentType(), body.String()
}

// A tela de voz é um formulário que posta um arquivo, e a política de
// permissões é a linha que deixa o microfone existir.
func TestPaginaDeVozLiberaOMicrofone(t *testing.T) {
	res := newClient(t).Get("/voz").WantStatus(200).WantContains(
		`data-ui-recorder=""`, `accept="audio/*"`, `capture=""`,
		`data-trilha-upload="resposta"`, `name="idioma"`, "ui.recorder.js", "ui.upload.js")
	if got := res.Header().Get("Permissions-Policy"); !strings.Contains(got, "microphone=(self)") {
		t.Fatalf("Permissions-Policy = %q", got)
	}
	if csp := res.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "script-src 'self'") {
		t.Fatal("a CSP mudou:", csp)
	}
}

// Gravou, transcreveu, respondeu no idioma pedido e o pedaço volta com o
// player apontando para a rota que fala.
func TestVozTranscreveEResponde(t *testing.T) {
	c := newClient(t)
	ctype, body := gravacao(t, 2048, "en")
	res := c.Request("POST", "/api/voz", trilha.WithBody(ctype, body))
	res.WantStatus(200).WantContains("onde fica o guichê?", "O resultado é 20.", "/api/voz/fala?", "<audio")
	if audioSent == 0 {
		t.Fatal("a gravação não chegou ao provedor")
	}
	if audioLang != "en" {
		t.Fatalf("idioma enviado = %q", audioLang)
	}

	// A fala é servida como áudio, sem cache e com o texto da resposta.
	fala := c.Get("/api/voz/fala?texto=hello+there&idioma=en").WantStatus(200)
	if got := fala.Header().Get("Content-Type"); got != "audio/mpeg" {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := fala.Header().Get("Cache-Control"); !strings.Contains(got, "no-store") {
		t.Fatalf("Cache-Control = %q", got)
	}
	if fala.Body.String() != "ID3fake-mp3" || spokenText != "hello there" {
		t.Fatalf("falou %q com o corpo %q", spokenText, fala.Body.String())
	}
	c.Get("/api/voz/fala").WantStatus(422)
	c.Get("/api/voz/fala?texto=" + strings.Repeat("a", 700)).WantStatus(413)
}

// O provedor que falha não vira 500 nem mensagem dele na tela.
func TestVozComProvedorFora(t *testing.T) {
	c := newClient(t)
	audioStatus = 500
	ctype, body := gravacao(t, 512, "pt-BR")
	res := c.Request("POST", "/api/voz", trilha.WithBody(ctype, body)).WantStatus(502)
	if strings.Contains(res.Body.String(), "scripted failure") {
		t.Fatal("a mensagem do provedor vazou para a tela:", res.Body.String())
	}
	c.Get("/api/voz/fala?texto=oi").WantStatus(502)
}

// A gravação grande demais para antes do provedor, no limite da própria rota.
func TestVozRecusaGravacaoGrande(t *testing.T) {
	c := newClient(t)
	ctype, body := gravacao(t, apivoz.MaxAudio+1024, "pt-BR")
	c.Request("POST", "/api/voz", trilha.WithBody(ctype, body)).WantStatus(413)
	if audioSent != 0 {
		t.Fatal("a gravação grande chegou ao provedor")
	}
}
