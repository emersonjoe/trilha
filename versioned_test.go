package trilha

import (
	"context"
	"errors"
	"testing"
)

type modelo struct {
	Nome  string `json:"nome"`
	Corpo string `json:"corpo"`
}

// #148 — o ciclo inteiro: rascunho, gravar, publicar, e a recusa de escrever
// em cima do que está publicado.
func TestVersionedPublicadaNaoMuda(t *testing.T) {
	v := NewVersioned[modelo]("modelos", VersionedOpts{})
	ctx := context.Background()

	n, err := v.Save(nil, "m-1", modelo{Nome: "Contrato", Corpo: "primeira"}, "início")
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("primeira versão = %d", n)
	}
	// Sem publicar, gravar de novo continua na mesma versão: um rascunho é um
	// rascunho até alguém dizer que não é.
	if n, err = v.Save(nil, "m-1", modelo{Nome: "Contrato", Corpo: "segunda"}, ""); err != nil || n != 1 {
		t.Fatalf("n = %d, err = %v", n, err)
	}

	if err := v.Publish(nil, "m-1", 1); err != nil {
		t.Fatal(err)
	}
	// Publicada é imutável, e a recusa diz o que fazer.
	_, err = v.Save(nil, "m-1", modelo{Corpo: "terceira"}, "")
	if !errors.Is(err, ErrVersionFrozen) {
		t.Fatalf("err = %v", err)
	}
	hint := HintOf(err)
	if hint == nil || hint.Code != ErrFrozen || hint.Repair == "" {
		t.Fatalf("a recusa não ensina: %+v", hint)
	}

	// Com um rascunho aberto, grava — e a publicada continua a mesma.
	n, err = v.Draft(nil, "m-1")
	if err != nil || n != 2 {
		t.Fatalf("rascunho = %d, err = %v", n, err)
	}
	if _, err := v.Save(nil, "m-1", modelo{Nome: "Contrato", Corpo: "terceira"}, "revisão"); err != nil {
		t.Fatal(err)
	}
	atual, ver, err := v.Current(ctx, "m-1")
	if err != nil {
		t.Fatal(err)
	}
	if ver.N != 1 || atual.Corpo != "segunda" {
		t.Fatalf("a corrente mudou com o rascunho: v%d %+v", ver.N, atual)
	}

	// Publicar a 2 troca a corrente, e a 1 deixa de ser a publicada: "a atual"
	// tem de ser uma resposta só.
	if err := v.Publish(nil, "m-1", 2); err != nil {
		t.Fatal(err)
	}
	if _, ver, _ = v.Current(ctx, "m-1"); ver.N != 2 {
		t.Fatalf("corrente = %d", ver.N)
	}
	hist, err := v.History(ctx, "m-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(hist) != 2 || !hist[1].Published || hist[0].Published {
		t.Fatalf("histórico = %+v", hist)
	}
}

// Voltar é criar, não apagar: um histórico que perde uma linha é um histórico
// com que ninguém responde pergunta.
func TestVersionedRestaurarCria(t *testing.T) {
	v := NewVersioned[modelo]("modelos", VersionedOpts{})
	ctx := context.Background()

	if _, err := v.Save(nil, "m-1", modelo{Corpo: "um"}, ""); err != nil {
		t.Fatal(err)
	}
	if err := v.Publish(nil, "m-1", 1); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Draft(nil, "m-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Save(nil, "m-1", modelo{Corpo: "dois"}, ""); err != nil {
		t.Fatal(err)
	}
	if err := v.Publish(nil, "m-1", 2); err != nil {
		t.Fatal(err)
	}

	n, err := v.Restore(nil, "m-1", 1)
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("restaurou em %d", n)
	}
	restaurada, ver, err := v.At(ctx, "m-1", 3)
	if err != nil {
		t.Fatal(err)
	}
	if restaurada.Corpo != "um" || ver.Note == "" {
		t.Fatalf("v3 = %+v (%+v)", restaurada, ver)
	}
	// E as duas de antes continuam lá, com a publicada onde estava.
	hist, _ := v.History(ctx, "m-1")
	if len(hist) != 3 || !hist[1].Published {
		t.Fatalf("histórico = %+v", hist)
	}
}

// Um id sem versão nenhuma não é uma versão vazia: é a ausência dela, e a
// tela que pergunta precisa saber a diferença.
func TestVersionedSemVersao(t *testing.T) {
	v := NewVersioned[modelo]("modelos", VersionedOpts{})
	if _, _, err := v.Current(context.Background(), "nada"); !errors.Is(err, ErrNoVersion) {
		t.Fatalf("err = %v", err)
	}
	if _, _, err := v.At(context.Background(), "nada", 7); !errors.Is(err, ErrNoVersion) {
		t.Fatalf("err = %v", err)
	}
}
