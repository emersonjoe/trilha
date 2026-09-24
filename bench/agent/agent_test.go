package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

var update = flag.Bool("update", false, "rewrite prompts/CHECKSUMS.txt from the files on disk")

// TestScenarioPromptsFrozen is the anti-vise of spec 159: every scenario's
// prompt lives in a file under prompts/, the field the ruler uses is
// byte-for-byte that file, and CHECKSUMS.txt pins the hash of every prompt
// file — so a quiet edit cannot split the ruler into two rulers. Run with
// -update after *deliberately* changing or adding a prompt: the diff of
// CHECKSUMS.txt is the record that the contract moved.
func TestScenarioPromptsFrozen(t *testing.T) {
	if *update {
		if err := writeChecksums(); err != nil {
			t.Fatal(err)
		}
	}
	pins, err := checksums()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range Scenarios() {
		if s.PromptMD == "" {
			t.Errorf("%s: no frozen prompt file", s.Name)
			continue
		}
		if s.Prompt != mustPrompt(s.PromptMD) {
			t.Errorf("%s: the inline prompt is not byte-for-byte %s", s.Name, s.PromptMD)
		}
		if s.BaseDir == "" {
			continue // not in the savings series: no baseline side yet
		}
		// s8's baseline carries its own red tests in the fixture, like its
		// trilha side; the others get hidden tests copied in at verify time.
		if s.BaselinePromptMD == "" {
			t.Errorf("%s: in the series without a frozen baseline prompt", s.Name)
		}
	}
	files, err := promptFiles()
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for name := range pins {
		got = append(got, name)
	}
	sort.Strings(got)
	if !reflect.DeepEqual(files, got) {
		t.Fatalf("CHECKSUMS.txt pins %v, the prompts/ directory holds %v", got, files)
	}
	for _, f := range files {
		sum := sha256.Sum256([]byte(mustPrompt(f)))
		want := hex.EncodeToString(sum[:])
		if pins[f] != want {
			t.Errorf("%s: hash %s, CHECKSUMS.txt pins %s — the prompt moved; re-measure before publishing", f, want, pins[f])
		}
	}
}

// writeChecksums rewrites CHECKSUMS.txt from the prompt files, sorted by name.
func writeChecksums() error {
	files, err := promptFiles()
	if err != nil {
		return err
	}
	var sb strings.Builder
	for _, f := range files {
		sum := sha256.Sum256([]byte(mustPrompt(f)))
		name := strings.TrimPrefix(f, "prompts/")
		sb.WriteString(hex.EncodeToString(sum[:]) + "  " + name + "\n")
	}
	return os.WriteFile(filepath.Join("prompts", "CHECKSUMS.txt"), []byte(sb.String()), 0o644)
}

// TestBaselineBuilds is the stdlib half of the contract: every baseline of
// the savings series builds on its own (stdlib only, one go.mod, no replace),
// and the side's hidden test fails on the untouched fixture — a baseline
// whose test passes before the task is done measures nothing. s8 is the
// exception by design: its fixture is red as committed, and the test proves
// the canonical fixes take it green, so the bar is reachable.
func TestBaselineBuilds(t *testing.T) {
	if testing.Short() {
		t.Skip("builds every baseline")
	}
	repo, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	for _, sc := range SeriesScenarios() {
		t.Run(sc.Name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "base")
			if err := copyTree(filepath.Join(repo, filepath.FromSlash(sc.BaseDir)), dir, "", ""); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			if err := exec.CommandContext(ctx, "go", "build", "./...").Run(); err != nil {
				c := exec.CommandContext(ctx, "go", "build", "./...")
				c.Dir = dir
				out, _ := c.CombinedOutput()
				t.Fatalf("%s does not build: %s", sc.BaseDir, out)
			}
			if sc.Name == "s8-conserto" {
				// Red as committed, green under the canonical fixes: delete
				// the double registration and put the two checks in.
				if err := os.Remove(filepath.Join(dir, "extras.go")); err != nil {
					t.Fatal(err)
				}
				b, err := os.ReadFile(filepath.Join(dir, "main.go"))
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "main.go"),
					[]byte(strings.Replace(string(b), "\tregistrarExtras(mux)\n", "", 1)), 0o644); err != nil {
					t.Fatal(err)
				}
				b, err = os.ReadFile(filepath.Join(dir, "paginas.go"))
				if err != nil {
					t.Fatal(err)
				}
				fixed := strings.Replace(string(b), "\t// TODO(csrf): verificar o token do formulário contra o cookie antes de\n"+
					"\t// gravar qualquer coisa.\n"+
					"\t// TODO(validacao): recusar nome vazio com 422 em vez de gravar.\n",
					"\t"+`if r.FormValue("_csrf") == "" || r.FormValue("_csrf") != tokenDoCookie(r) {`+"\n"+
						"\t\thttp.Error(w, \"token csrf inválido\", http.StatusForbidden)\n\t\treturn\n\t}\n"+
						"\t"+`if nome == "" {`+"\n"+
						"\t\thttp.Error(w, \"nome é obrigatório\", http.StatusUnprocessableEntity)\n\t\treturn\n\t}\n", 1)
				if fixed == string(b) {
					t.Fatal("the fixture drifted; the planted TODO block is not where it was")
				}
				if err := os.WriteFile(filepath.Join(dir, "paginas.go"), []byte(fixed), 0o644); err != nil {
					t.Fatal(err)
				}
				if ok, why := VerifyBaseline(ctx, dir, sc); !ok {
					t.Fatalf("the canonical fixes do not make the baseline green:\n%s", why)
				}
				return
			}
			if ok, why := VerifyBaseline(ctx, dir, sc); ok {
				t.Fatalf("%s: the baseline hidden test passes on the untouched fixture; it measures nothing", sc.BaseDir)
			} else if !strings.Contains(why, "--- FAIL") {
				t.Fatalf("%s: the baseline does not compile with its hidden test: %s", sc.BaseDir, why)
			}
		})
	}
}

// for: each one passes `trilha check` as committed — except s8, which must
// TestScenarioApps proves the committed starting apps are what the plan asks
// for: each one passes `trilha check` as committed — except s8, which must
// fail at the first gate with the planted duplicate route, keep failing its
// own tests, and go green when the three canonical fixes land (the ruler is
// reachable; without this half a scenario can measure a bar nobody reaches).
func TestScenarioApps(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the CLI and runs a project check per app")
	}
	repo, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	if err := BuildCLI(repo, bin); err != nil {
		t.Fatal(err)
	}
	cli := filepath.Join(bin, "trilha")
	for _, sc := range Scenarios() {
		if sc.AppDir == "" {
			continue
		}
		t.Run(sc.Name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "app")
			if err := Build(repo, sc, dir, false); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			if sc.Name != "s8-conserto" {
				c := exec.CommandContext(ctx, cli, "check")
				c.Dir = dir
				out, err := c.CombinedOutput()
				if err != nil {
					t.Fatalf("trilha check on the untouched %s: %v\n%s", sc.AppDir, err, out)
				}
				return
			}
			// Planted: the first gate says the route is duplicated.
			c := exec.CommandContext(ctx, cli, "check")
			c.Dir = dir
			out, err := c.CombinedOutput()
			if err == nil {
				t.Fatalf("s8 passed check with three defects planted:\n%s", out)
			}
			if !strings.Contains(string(out), "already served by") {
				t.Fatalf("s8's first failure is not the duplicate route:\n%s", out)
			}
			// Planted: the app's own tests are red, and stay red after the
			// duplicate route is gone — the other two defects hold the line.
			s8 := sc
			fix1 := filepath.Join(t.TempDir(), "fix1")
			if err := Build(repo, s8, fix1, false); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(filepath.Join(fix1, "app", "admin-", "route.go")); err != nil {
				t.Fatal(err)
			}
			if ok, why := VerifyCLI(ctx, fix1, s8, cli); ok || (!strings.Contains(why, "_csrf") && !strings.Contains(why, "want 422")) {
				t.Fatalf("s8 with only the route fixed: ok=%v, want the tests still failing:\n%s", ok, why)
			}
			// Reachable: the three canonical fixes take it green.
			fix3 := filepath.Join(t.TempDir(), "fix3")
			if err := Build(repo, s8, fix3, false); err != nil {
				t.Fatal(err)
			}
			for _, fix := range []struct{ file, from, to string }{
				{"app/contas/page.go", `validate:"obrigatorio,email"`, `validate:"required,email"`},
				{"app/contas/page.go",
					`h.Form(h.Method("post"), h.Action("/contas"), h.Class("ui-stack"),`,
					`h.Form(h.Method("post"), h.Action("/contas"), h.Class("ui-stack"), trilha.CSRFInput(c),`},
			} {
				b, err := os.ReadFile(filepath.Join(fix3, filepath.FromSlash(fix.file)))
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(string(b), fix.from) {
					t.Fatalf("the fixture drifted; %s no longer holds %q", fix.file, fix.from)
				}
				if err := os.WriteFile(filepath.Join(fix3, filepath.FromSlash(fix.file)),
					[]byte(strings.Replace(string(b), fix.from, fix.to, 1)), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Remove(filepath.Join(fix3, "app", "admin-", "route.go")); err != nil {
				t.Fatal(err)
			}
			if ok, why := VerifyCLI(ctx, fix3, s8, cli); !ok {
				t.Fatalf("the canonical fixes do not make s8 green — the ruler measures a bar nobody reaches:\n%s", why)
			}
		})
	}
}

const successJSON = `{"type":"result","subtype":"success","is_error":false,"duration_ms":184000,"num_turns":23,"result":"Done.\nAdded the route.","total_cost_usd":0.42,"usage":{"input_tokens":1200,"cache_creation_input_tokens":30000,"cache_read_input_tokens":410000,"output_tokens":9800},"modelUsage":{"claude-sonnet-5":{}},"permission_denials":[{"tool_name":"Bash","tool_input":{"command":"go install ./..."}}]}`

const authErrorJSON = `{"type":"result","subtype":"success","is_error":true,"duration_ms":73,"num_turns":1,"result":"Failed to authenticate: OAuth session expired and could not be refreshed","total_cost_usd":0,"usage":{"input_tokens":0,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":0},"modelUsage":{},"permission_denials":[]}`

func TestParseResult(t *testing.T) {
	u, err := ParseResult([]byte(successJSON))
	if err != nil {
		t.Fatal(err)
	}
	want := Usage{Input: 31200, CacheRead: 410000, Output: 9800, Turns: 23, DurationMs: 184000, CostUSD: 0.42, Model: "claude-sonnet-5", Denials: 1, Denied: []string{"Bash: go install ./..."}}
	if !reflect.DeepEqual(u, want) {
		t.Fatalf("got %+v\nwant %+v", u, want)
	}
	u, err = ParseResult([]byte(authErrorJSON))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(u.Error, "authenticate") || u.Turns != 1 {
		t.Fatalf("auth failure must be recorded as Error: %+v", u)
	}
	if _, err := ParseResult([]byte("not json")); err == nil {
		t.Fatal("garbage must be an error")
	}
	if _, err := ParseResult([]byte(`{"type":"assistant"}`)); err == nil {
		t.Fatal("a non-result message must be an error")
	}
	if s := summary([]byte(successJSON)); s != "Done." {
		t.Fatalf("summary = %q", s)
	}
}

func TestMedian(t *testing.T) {
	for _, tc := range []struct {
		in   []float64
		want float64
	}{
		{nil, 0}, {[]float64{7}, 7}, {[]float64{9, 1, 5}, 5}, {[]float64{4, 1, 3, 2}, 2.5},
	} {
		if got := Median(tc.in); got != tc.want {
			t.Fatalf("Median(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestRender(t *testing.T) {
	r := Results{Trilha: "v0.33.0", Agent: "claude 2.1", Model: "claude-sonnet-5", Machine: "go1.25 darwin/arm64", Date: "2026-09-06"}
	for i, in := range []int{3000, 1000, 2000} {
		r.Runs = append(r.Runs, Run{Scenario: "comments", N: i + 1, Usage: Usage{Input: in, CacheRead: 100000, Output: 500 + i, Turns: 10 + i, DurationMs: 60000, CostUSD: 0.5}, Passed: i != 1})
	}
	md := Render(r, Scenarios())
	for _, want := range []string{
		"| `comments` | 2.0k | 100.0k | 501 | 11 | 0 | 60 | 0.50 | 2/3 |",
		"Trilha v0.33.0, agente claude 2.1, modelo claude-sonnet-5",
		"## Metodologia", "### `pagination`", "### `cognito`",
	} {
		if !strings.Contains(md, want) {
			t.Fatalf("RESULTS.md lacks %q:\n%s", want, md)
		}
	}
	if !strings.Contains(Render(Results{}, Scenarios()), "Ainda sem medição") {
		t.Fatal("empty results must say so")
	}
	// Scenarios are the contract, in order, each with a hidden test or a
	// gate of its own (s8 carries its proof inside the fixture).
	names := []string{"comments", "contact-form", "cognito", "pagination", "generate-crud", "fix-hint", "port-listing", "api-call",
		"s5-login", "s6-crud", "s7-tela", "s8-conserto"}
	for i, s := range Scenarios() {
		if s.Name != names[i] || (len(s.Tests) == 0 && len(s.Gate) == 0) || s.Prompt == "" || s.ID == "" || s.PromptMD == "" {
			t.Fatalf("scenario %d = %+v", i, s.Name)
		}
	}
}

// TestFixturesFailWithoutTheAgent is the proof that the ruler measures
// something: every fixture vets clean, and every hidden test fails on it.
func TestFixturesFailWithoutTheAgent(t *testing.T) {
	if testing.Short() {
		t.Skip("builds every fixture")
	}
	repo, _ := filepath.Abs(filepath.Join("..", ".."))
	for _, s := range Scenarios() {
		s := s
		t.Run(s.Name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "proj")
			if err := Build(repo, s, dir, false); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(dir, ".trilha")); err == nil {
				t.Fatal("the dev cache must not be copied")
			}
			if err := Vet(dir); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			ok, why := Verify(ctx, dir, s)
			if ok {
				t.Fatal("the hidden test passes on the untouched fixture; it measures nothing")
			}
			if BrokenRuler(why) {
				t.Fatalf("the fixture does not compile with the hidden test beside it, so a run would measure the agent repairing the ruler: %s", why)
			}
			// s8's proof is its gate going red on the planted defects, not a
			// copied-in test running; everything else must show "--- FAIL".
			if len(s.Gate) == 0 && !strings.Contains(why, "--- FAIL") {
				t.Fatalf("the hidden test did not run and fail; verify said: %s", why)
			}
		})
	}
}

// O que a régua lê é a linha "--- FAIL": é ela que separa "o teste escondido
// rodou e falhou", que é a medição, de "o fixture nem compila", que é a régua
// quebrada. Uma falha comprida o bastante empurrava essa linha para fora da
// janela — e um teste escondido que afirma sobre uma página recebe a página
// inteira na mensagem, que são dois quilobytes de HTML.
func TestTailGuardaOCabecalhoDaFalha(t *testing.T) {
	quebra := func(s string) string { return strings.ReplaceAll(s, "|", "\n") }
	saida := quebra("--- FAIL: TestBenchContato|") + strings.Repeat("x", 5000) + quebra("|FAIL|")

	got := tail(saida, 4000)
	if !strings.Contains(got, "--- FAIL: TestBenchContato") {
		t.Fatalf("o cabeçalho da falha sumiu: %q", got[:120])
	}
	if !strings.HasSuffix(strings.TrimSpace(got), "FAIL") {
		t.Fatal("o fim da saída, que é onde o go test diz o que quebrou, não sobreviveu")
	}
	if n := strings.Count(got, "--- FAIL: TestBenchContato"); n != 1 {
		t.Fatalf("o cabeçalho apareceu %d vezes", n)
	}
	// Saída curta volta inteira, sem cabeçalho repetido na frente.
	curta := quebra("--- FAIL: TestX|detalhe|FAIL|")
	if got := tail(curta, 4000); got != curta {
		t.Fatalf("saída curta veio mexida: %q", got)
	}
}

// #94 — a régua tem de ser atingível, e o cenário de portar tem a prova
// dentro dele: a tela que o Prepare apaga é uma resposta correta.
//
// Sem isto, um cenário pode medir uma barra que ninguém alcança — e uma barra
// dessas não distingue um agente ruim de um teste impossível.
func TestPortListingEhAtingivel(t *testing.T) {
	if testing.Short() {
		t.Skip("compila e roda um projeto inteiro")
	}
	repo, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	sc, ok := ScenarioByName("port-listing")
	if !ok {
		t.Fatal("o cenário sumiu")
	}
	// Sem o Prepare: o exemplo como está, que é a resposta.
	sc.Prepare = nil
	dir := t.TempDir()
	if err := Build(repo, sc, dir, false); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if ok, why := Verify(ctx, dir, sc); !ok {
		t.Fatalf("o teste escondido recusa a tela que o exemplo já tem:\n%s", why)
	}
}

// #94 — a outra metade: a régua que mede o caminho até a API também tem de ser
// atingível, e a prova é a mesma que a do port-listing — a tela que o Prepare
// troca por um esqueleto é uma resposta correta.
//
// Este é o único cenário com um serviço de pé durante a verificação, então ele
// prova duas coisas de uma vez: que a barra existe, e que o `Serve` chega ao
// teste escondido pelo ambiente.
func TestApiCallEhAtingivel(t *testing.T) {
	if testing.Short() {
		t.Skip("compila e roda um projeto inteiro")
	}
	repo, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	sc, ok := ScenarioByName("api-call")
	if !ok {
		t.Fatal("o cenário sumiu")
	}
	// Sem o Prepare: o exemplo como está, que é a resposta.
	sc.Prepare = nil
	dir := t.TempDir()
	if err := Build(repo, sc, dir, false); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if ok, why := Verify(ctx, dir, sc); !ok {
		t.Fatalf("o teste escondido recusa a tela que o exemplo já tem:\n%s", why)
	}
}

// O stub é uma API e um gravador: o que ele responde não depende da
// credencial, de propósito, porque uma API que recusasse o anônimo faria
// "saiu sem o token da sessão" e "não saiu" parecerem a mesma tela — e são
// esses dois que o cenário existe para separar.
func TestStubDoAcervoGravaOQueRecebe(t *testing.T) {
	env, stop := serveAcervo()
	defer stop()
	base := strings.TrimPrefix(env[0], "API_URL=")

	req, _ := http.NewRequest("GET", base+"/api/documents?q=contrato", nil)
	req.Header.Set("Authorization", "Bearer jwt-da-ana")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var page struct {
		Items []map[string]any `json:"items"`
		Total int              `json:"total"`
	}
	if err := json.NewDecoder(res.Body).Decode(&page); err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if len(page.Items) != 1 || page.Items[0]["filename"] != "contrato-2026.pdf" {
		t.Fatalf("o filtro q não filtrou: %+v", page.Items)
	}

	// Sem credencial a resposta é a mesma, e é isso que deixa a falha legível.
	res, err = http.Get(base + "/api/documents")
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 200 {
		t.Fatalf("chamada anônima = %d, queria 200", res.StatusCode)
	}
	res.Body.Close()

	res, err = http.Get(base + InspectPath)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var recs []Recebida
	if err := json.NewDecoder(res.Body).Decode(&recs); err != nil {
		t.Fatal(err)
	}
	if len(recs) != 2 {
		t.Fatalf("gravou %d chamadas, queria 2", len(recs))
	}
	if recs[0].Auth != "Bearer jwt-da-ana" || recs[0].Query != "q=contrato" {
		t.Fatalf("a primeira chamada veio errada: %+v", recs[0])
	}
	if recs[1].Auth != "" {
		t.Fatalf("a segunda chamada não era anônima: %+v", recs[1])
	}
	// O caminho de inspeção não entra no log: ele não é da API.
	for _, r := range recs {
		if r.Path == InspectPath {
			t.Fatal("o próprio gravador apareceu no log")
		}
	}
}

// A metade da aceitação que pedido nenhum mostra: a chamada pode levar a
// credencial certa e ainda ter sido escrita à mão sobre um documento que
// estava a um comando de virar tipos.
func TestClienteGeradoExigeOComando(t *testing.T) {
	dir := t.TempDir()
	mao := filepath.Join(dir, "app", "documentos")
	if err := os.MkdirAll(mao, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mao, "page.go"), []byte("package documentos\n// http.NewRequest e json.Decode na mão\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := clienteGerado(dir); err == nil {
		t.Fatal("um projeto sem cliente gerado passou")
	}

	gerado := filepath.Join(dir, "internal", "api")
	if err := os.MkdirAll(gerado, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gerado, "client.go"),
		[]byte("// Code generated by trilha client from openapi.json. DO NOT EDIT.\npackage api\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := clienteGerado(dir); err != nil {
		t.Fatalf("o cliente gerado não foi reconhecido: %v", err)
	}
}
