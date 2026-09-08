package trilha

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type csvRow struct {
	Quando time.Time `csv:"Quando"`
	Quem   string    `csv:"Usuário"`
	Valor  float64   `csv:"Valor"`
	Ativo  bool      `csv:"Ativo"`
	Fora   string    `csv:"-"`
	oculto string
}

func csvCtx(locale string) (*Ctx, *httptest.ResponseRecorder) {
	req := httptest.NewRequest("GET", "/exportar", nil)
	rec := httptest.NewRecorder()
	a := &App{cfg: Config{Locale: locale}}
	return newCtx(a, &responseWriter{ResponseWriter: rec, status: http.StatusOK}, req, kindAPI), rec
}

func csvRows() []csvRow {
	return []csvRow{
		{Quando: time.Date(2026, 9, 8, 15, 4, 0, 0, time.UTC), Quem: "ana", Valor: 1234.5, Ativo: true, Fora: "x", oculto: "y"},
		{Quem: "joão; o outro", Valor: 0, Ativo: false},
	}
}

// O arquivo inteiro, byte a byte: o BOM, o separador, a data e a palavra para
// verdadeiro são a metade da issue que ninguém confere de olho.
func TestCSVExportaEmIngles(t *testing.T) {
	c, rec := csvCtx("")
	if err := c.CSV("auditoria", csvRows()); err != nil {
		t.Fatal(err)
	}
	want := "\ufeff" +
		"Quando,Usuário,Valor,Ativo\r\n" +
		"2026-09-08 15:04,ana,1234.5,yes\r\n" +
		// Em inglês o ; é só um caractere: quem escolhe o separador é o locale,
		// e a aspa só entra quando a célula tem o separador de verdade.
		",joão; o outro,0,no\r\n"
	if got := rec.Body.String(); got != want {
		t.Fatalf("arquivo diferente:\n got %q\nwant %q", got, want)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/csv; charset=utf-8" {
		t.Fatalf("tipo %q", ct)
	}
	if d := rec.Header().Get("Content-Disposition"); !strings.Contains(d, `filename="auditoria.csv"`) {
		t.Fatalf("nome sem .csv: %q", d)
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("download sem nosniff")
	}
}

func TestCSVExportaEmPortugues(t *testing.T) {
	c, rec := csvCtx("pt-BR")
	if err := c.CSV("auditoria.csv", csvRows()); err != nil {
		t.Fatal(err)
	}
	want := "\ufeff" +
		"Quando;Usuário;Valor;Ativo\r\n" +
		"08/09/2026 15:04;ana;1234,5;sim\r\n" +
		";joão; o outro;0;não\r\n"
	// O ; dentro da célula obriga as aspas; sem elas o arquivo abre torto.
	want = strings.Replace(want, ";joão; o outro;", ";\"joão; o outro\";", 1)
	if got := rec.Body.String(); got != want {
		t.Fatalf("arquivo diferente:\n got %q\nwant %q", got, want)
	}
}

func TestCSVExportaDeUmCanal(t *testing.T) {
	ch := make(chan csvRow, 2)
	for _, r := range csvRows() {
		ch <- r
	}
	close(ch)
	c, rec := csvCtx("")
	if err := c.CSV("x.csv", (<-chan csvRow)(ch)); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(rec.Body.String(), "\r\n"); n != 3 {
		t.Fatalf("linhas do canal: %d", n)
	}
}

// O que a reflexão pode recusar é recusado antes do primeiro byte: senão o
// erro chega como um download pela metade, com 200 já enviado.
func TestCSVRecusaAntesDeEscrever(t *testing.T) {
	c, rec := csvCtx("")
	if err := c.CSV("x.csv", "não é uma lista"); err == nil {
		t.Fatal("aceitou uma string como linhas")
	}
	if rec.Body.Len() != 0 || rec.Header().Get("Content-Disposition") != "" {
		t.Fatalf("escreveu mesmo assim: %q", rec.Body.String())
	}
}

// ---- importação -------------------------------------------------------------

type csvNo struct {
	Codigo string  `csv:"codigo" validate:"required,max=5"`
	Nome   string  `csv:"nome"   validate:"required"`
	Peso   float64 `csv:"peso"`
}

func TestBindCSVLeVirgulaEPontoEVirgula(t *testing.T) {
	for _, tc := range []struct{ nome, arquivo string }{
		{"vírgula", "codigo,nome,peso\nA1,Documentos,2.5\nB2,Contratos,3\n"},
		{"ponto e vírgula", "codigo;nome;peso\nA1;Documentos;2,5\nB2;Contratos;3\n"},
		{"com BOM", "\ufeffcodigo;nome;peso\nA1;Documentos;2,5\nB2;Contratos;3\n"},
		{"CRLF", "codigo,nome,peso\r\nA1,Documentos,2.5\r\nB2,Contratos,3\r\n"},
	} {
		t.Run(tc.nome, func(t *testing.T) {
			var nos []csvNo
			res, err := BindCSV(strings.NewReader(tc.arquivo), &nos)
			if err != nil {
				t.Fatal(err)
			}
			if !res.OK() {
				t.Fatalf("erros: %+v", res.Errors)
			}
			if res.Rows != 2 || len(nos) != 2 {
				t.Fatalf("leu %d linhas, %d nós", res.Rows, len(nos))
			}
			if nos[0].Peso != 2.5 {
				t.Fatalf("peso %v", nos[0].Peso)
			}
		})
	}
}

// Quem mudou uma coluna de lugar não errou; quem acrescentou uma tem um aviso,
// não uma recusa.
func TestBindCSVCabecalhoForaDeOrdem(t *testing.T) {
	var nos []csvNo
	res, err := BindCSV(strings.NewReader("Nome ,peso,codigo,dono\nDocumentos,2.5,A1,ana\n"), &nos)
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK() {
		t.Fatalf("erros: %+v", res.Errors)
	}
	if nos[0].Codigo != "A1" || nos[0].Nome != "Documentos" {
		t.Fatalf("casou errado: %+v", nos[0])
	}
	if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "dono") {
		t.Fatalf("aviso da coluna desconhecida: %+v", res.Warnings)
	}
}

// A frase inteira da issue: qual linha e qual coluna, com o cabeçalho como
// linha 1 — inclusive depois de uma célula com quebra de linha dentro.
func TestBindCSVDizALinhaEAColuna(t *testing.T) {
	arquivo := "codigo,nome,peso\n" +
		"A1,\"Documentos\nda diretoria\",2.5\n" +
		"MUITOLONGO,,3\n"
	var nos []csvNo
	res, err := BindCSV(strings.NewReader(arquivo), &nos)
	if err != nil {
		t.Fatal(err)
	}
	if len(nos) != 1 {
		t.Fatalf("guardou linha ruim: %+v", nos)
	}
	want := []CSVError{
		{Line: 4, Column: "codigo", Message: message("maxlen", "5")},
		{Line: 4, Column: "nome", Message: message("required", "")},
	}
	if len(res.Errors) != len(want) {
		t.Fatalf("erros: %+v", res.Errors)
	}
	for i, e := range res.Errors {
		if e != want[i] {
			t.Fatalf("erro %d:\n got %+v\nwant %+v", i, e, want[i])
		}
	}
	if res.Rows != 2 {
		t.Fatalf("contou %d linhas", res.Rows)
	}
}

// Coluna obrigatória que não existe é uma mensagem no topo, não a mesma
// mensagem em cada uma de dez mil linhas.
func TestBindCSVColunaObrigatoriaFaltando(t *testing.T) {
	var nos []csvNo
	res, err := BindCSV(strings.NewReader("codigo,peso\nA1,2\nB2,3\n"), &nos)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Errors) != 1 || res.Errors[0].Line != 1 || res.Errors[0].Column != "nome" {
		t.Fatalf("erros: %+v", res.Errors)
	}
	if res.Rows != 0 {
		t.Fatal("leu as linhas mesmo sem a coluna")
	}
}

func TestBindCSVPulaLinhaEmBranco(t *testing.T) {
	var nos []csvNo
	res, err := BindCSV(strings.NewReader("codigo,nome\nA1,Documentos\n\n,\nB2,Contratos\n"), &nos)
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK() || res.Rows != 2 {
		t.Fatalf("res %+v", res)
	}
}

func TestBindCSVLimitaOArquivo(t *testing.T) {
	var b strings.Builder
	b.WriteString("codigo,nome\n")
	for i := 0; i < 5; i++ {
		b.WriteString("A,Documentos\n")
	}
	var nos []csvNo
	if _, err := BindCSV(strings.NewReader(b.String()), &nos, CSVRules{MaxRows: 3}); err == nil {
		t.Fatal("aceitou um arquivo acima do limite")
	}
}

func TestBindCSVArquivoVazio(t *testing.T) {
	var nos []csvNo
	if _, err := BindCSV(strings.NewReader(""), &nos); err == nil {
		t.Fatal("aceitou arquivo vazio")
	}
}

// A data como a planilha brasileira exporta. O input de data do HTML sempre
// manda ISO, então o bind nunca precisou saber desta.
func TestBindCSVLeDataBrasileira(t *testing.T) {
	type linha struct {
		Quando time.Time `csv:"quando"`
	}
	var ls []linha
	res, err := BindCSV(strings.NewReader("quando\n08/09/2026\n2026-09-08\n08/09/2026 15:04\n"), &ls)
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK() {
		t.Fatalf("erros: %+v", res.Errors)
	}
	if !ls[0].Quando.Equal(ls[1].Quando) {
		t.Fatalf("dd/mm/yyyy virou %v, ISO virou %v", ls[0].Quando, ls[1].Quando)
	}
	if h := ls[2].Quando.Hour(); h != 15 {
		t.Fatalf("hora %d", h)
	}
}

// A volta inteira: o que o c.CSV escreveu, o BindCSV lê — inclusive o BOM, o
// ponto e vírgula e a data em pt-BR, que é exatamente o arquivo que o usuário
// baixa, edita no Excel e sobe de novo.
func TestCSVIdaEVolta(t *testing.T) {
	c, rec := csvCtx("pt-BR")
	if err := c.CSV("x.csv", csvRows()); err != nil {
		t.Fatal(err)
	}
	type volta struct {
		Quando time.Time `csv:"Quando"`
		Quem   string    `csv:"Usuário"`
		Valor  float64   `csv:"Valor"`
		Ativo  bool      `csv:"Ativo"`
	}
	var vs []volta
	res, err := BindCSV(strings.NewReader(rec.Body.String()), &vs)
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK() || len(vs) != 2 {
		t.Fatalf("res %+v", res)
	}
	if vs[0].Quem != "ana" || vs[0].Valor != 1234.5 || !vs[0].Ativo {
		t.Fatalf("primeira linha: %+v", vs[0])
	}
	if !vs[0].Quando.Equal(time.Date(2026, 9, 8, 15, 4, 0, 0, time.UTC)) {
		t.Fatalf("data voltou %v", vs[0].Quando)
	}
	if vs[1].Quem != "joão; o outro" || vs[1].Ativo {
		t.Fatalf("segunda linha: %+v", vs[1])
	}
}

func TestBindCSVRecusaODestinoErrado(t *testing.T) {
	var n csvNo
	if _, err := BindCSV(strings.NewReader("codigo\nA1\n"), &n); err == nil {
		t.Fatal("aceitou um ponteiro que não é para lista")
	}
}
