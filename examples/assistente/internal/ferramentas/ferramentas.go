// Package ferramentas holds the tools, agents and MCP server of the example.
package ferramentas

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/emersonjoe/trilha/ai"
	"github.com/emersonjoe/trilha/ai/mcp"
)

// Client is the model client (OpenAI-compatible; set OPENAI_BASE_URL to use
// Ollama, LM Studio, vLLM, Groq...).
var Client = ai.NewFromEnv()

// ---- notas: a tiny in-memory store shared by the tools and the MCP server --

var (
	mu    sync.Mutex
	notas = map[string]string{}
)

// Reset clears the notes (tests).
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	notas = map[string]string{}
}

// ---- tools -----------------------------------------------------------------

var horaAtual = ai.NewTool("hora_atual", "Retorna a data e hora atuais no fuso informado (padrão America/Sao_Paulo).",
	ai.Schema(`{"type":"object","properties":{"fuso":{"type":"string","description":"nome IANA, ex. Europe/Lisbon"}}}`),
	ai.Typed(func(ctx context.Context, in struct{ Fuso string }) (string, error) {
		if in.Fuso == "" {
			in.Fuso = "America/Sao_Paulo"
		}
		loc, err := time.LoadLocation(in.Fuso)
		if err != nil {
			return "", fmt.Errorf("fuso desconhecido: %s", in.Fuso)
		}
		return Now().In(loc).Format("02/01/2006 15:04 (MST)"), nil
	}))

// Now is replaceable in tests.
var Now = time.Now

var calcular = ai.NewTool("calcular", "Avalia uma expressão aritmética simples com + - * / e parênteses.",
	ai.Schema(`{"type":"object","properties":{"expressao":{"type":"string"}},"required":["expressao"]}`),
	ai.Typed(func(ctx context.Context, in struct{ Expressao string }) (string, error) {
		v, err := eval(in.Expressao)
		if err != nil {
			return "", err
		}
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	}))

var salvarNota = ai.NewTool("salvar_nota", "Guarda uma nota curta identificada por um título.",
	ai.Schema(`{"type":"object","properties":{"titulo":{"type":"string"},"texto":{"type":"string"}},"required":["titulo","texto"]}`),
	ai.Typed(func(ctx context.Context, in struct{ Titulo, Texto string }) (string, error) {
		t := strings.TrimSpace(in.Titulo)
		if t == "" {
			return "", fmt.Errorf("título obrigatório")
		}
		mu.Lock()
		notas[t] = in.Texto
		mu.Unlock()
		return "nota salva: " + t, nil
	}))

var listarNotas = ai.NewTool("listar_notas", "Lista as notas guardadas.", nil,
	func(ctx context.Context, _ json.RawMessage) (string, error) {
		mu.Lock()
		defer mu.Unlock()
		if len(notas) == 0 {
			return "nenhuma nota", nil
		}
		keys := make([]string, 0, len(notas))
		for k := range notas {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var b strings.Builder
		for _, k := range keys {
			fmt.Fprintf(&b, "- %s: %s\n", k, notas[k])
		}
		return strings.TrimSpace(b.String()), nil
	})

// Tools are the tools shared by the agent and the MCP server.
var Tools = []*ai.Tool{horaAtual, calcular, salvarNota, listarNotas}

// ---- agents ----------------------------------------------------------------

// Tradutor answers only in the requested language; it receives handoffs.
var Tradutor = &ai.Agent{
	Name:         "Tradutor",
	Instructions: "Você é um tradutor. Traduza a última mensagem do usuário para o idioma pedido, sem comentários.",
}

// Assistente is the front agent: tools + handoff to Tradutor.
var Assistente = &ai.Agent{
	Name: "Assistente",
	Instructions: "Você é o assistente de exemplo do Trilha. Responda em português, de forma curta. " +
		"Use as ferramentas quando precisar de hora, contas ou notas. " +
		"Se o usuário pedir uma tradução, transfira para o Tradutor.",
	Tools:    Tools,
	Handoffs: []*ai.Agent{Tradutor},
}

// MCP exposes the same tools to MCP hosts at /mcp.
var MCP = mcp.NewServer("trilha-assistente", "0.1", Tools...)

// ---- a small expression evaluator (no deps) ---------------------------------

func eval(s string) (float64, error) {
	p := &parser{s: strings.ReplaceAll(s, " ", "")}
	v, err := p.expr()
	if err != nil {
		return 0, err
	}
	if p.i != len(p.s) {
		return 0, fmt.Errorf("expressão inválida perto de %q", p.s[p.i:])
	}
	return v, nil
}

type parser struct {
	s string
	i int
}

func (p *parser) peek() byte {
	if p.i < len(p.s) {
		return p.s[p.i]
	}
	return 0
}

func (p *parser) expr() (float64, error) {
	v, err := p.term()
	if err != nil {
		return 0, err
	}
	for p.peek() == '+' || p.peek() == '-' {
		op := p.s[p.i]
		p.i++
		r, err := p.term()
		if err != nil {
			return 0, err
		}
		if op == '+' {
			v += r
		} else {
			v -= r
		}
	}
	return v, nil
}

func (p *parser) term() (float64, error) {
	v, err := p.factor()
	if err != nil {
		return 0, err
	}
	for p.peek() == '*' || p.peek() == '/' {
		op := p.s[p.i]
		p.i++
		r, err := p.factor()
		if err != nil {
			return 0, err
		}
		if op == '*' {
			v *= r
		} else {
			if r == 0 {
				return 0, fmt.Errorf("divisão por zero")
			}
			v /= r
		}
	}
	return v, nil
}

func (p *parser) factor() (float64, error) {
	switch c := p.peek(); {
	case c == '(':
		p.i++
		v, err := p.expr()
		if err != nil {
			return 0, err
		}
		if p.peek() != ')' {
			return 0, fmt.Errorf("falta ')'")
		}
		p.i++
		return v, nil
	case c == '-':
		p.i++
		v, err := p.factor()
		return -v, err
	case c >= '0' && c <= '9' || c == '.':
		j := p.i
		for p.i < len(p.s) && (p.s[p.i] >= '0' && p.s[p.i] <= '9' || p.s[p.i] == '.') {
			p.i++
		}
		return strconv.ParseFloat(p.s[j:p.i], 64)
	default:
		return 0, fmt.Errorf("expressão inválida")
	}
}
