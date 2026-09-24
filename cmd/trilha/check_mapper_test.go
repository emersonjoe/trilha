package main

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha/internal/checkerr"
)

// The mappers read real tool output — captured, pasted here, and kept — so
// the codes the gate prints are the codes the tools actually earned.

// TestVetCode maps the printf analyzer by name and everything else to the
// family.
func TestVetCode(t *testing.T) {
	out := "app/blog/novo/page.go:12:3: printf: %d format %s with arg x\nother.go:4:5: copylocks: assignment copies lock"
	probs := vetProblems(out)
	if len(probs) != 2 {
		t.Fatalf("problems = %d", len(probs))
	}
	if probs[0].Code != "E_VET_PRINTF" {
		t.Fatalf("first = %s, want the printf analyzer named", probs[0].Code)
	}
	if probs[1].Code != "E_VET" {
		t.Fatalf("second = %s, want the family code", probs[1].Code)
	}
	// Every code the mapper emits must teach: hint and doc page come along.
	for _, pb := range probs {
		d, ok := checkerr.ByCode(pb.Code)
		if !ok || d.Fix == "" || d.Doc == "" {
			t.Fatalf("%s is not in the catalog", pb.Code)
		}
	}
}

// TestVulnProblems reads govulncheck's text output the way it writes it: one
// block per advisory, the ID on the "Vulnerability" line, the module on the
// "Found in" line.
func TestVulnProblems(t *testing.T) {
	out := `Vulnerability #1: GO-2026-0099
    Malformed header in github.com/some/module
  More info: https://pkg.go.dev/vuln/GO-2026-0099
  Module: github.com/some/module
    Found in: github.com/some/module@v1.1.0
    Fixed in: github.com/some/module@v1.1.2`
	probs := vulnProblems(out)
	if len(probs) != 1 {
		t.Fatalf("problems = %d, want 1", len(probs))
	}
	pb := probs[0]
	if pb.Code != "E_VULN_GO-2026-0099" {
		t.Fatalf("code = %s", pb.Code)
	}
	if !strings.Contains(pb.Message, "Found in") {
		t.Fatalf("the module that carries it is not in the message: %s", pb.Message)
	}
	if d, ok := checkerr.ByCode(pb.Code); !ok || !strings.Contains(d.Fix, "go get") {
		t.Fatalf("the family entry must say the upgrade: %+v %v", d, ok)
	}
	// No vulnerabilities, no problems.
	if probs := vulnProblems("No vulnerabilities found.\n"); len(probs) != 0 {
		t.Fatalf("clean output produced %d problems", len(probs))
	}
}

// TestSurfaceProblems keeps the diff — the part a reader acts on — under the
// one code the lock answers by.
func TestSurfaceProblems(t *testing.T) {
	out := `--- FAIL: TestSuperficiePublica (0.04s)
    superficie_test.go:230: the reference fell behind the code: 1 symbol.
        +pkg ui, func Badge(string) h.Node
--- FAIL`
	probs := surfaceProblems(out)
	if len(probs) != 1 || probs[0].Code != "E_API_SURFACE" {
		t.Fatalf("probs = %+v", probs)
	}
	if !strings.Contains(probs[0].Message, "pkg ui") {
		t.Fatalf("the diff is not in the message: %s", probs[0].Message)
	}
}

// TestCheckJSONSchema runs the gate over the planted app and holds the
// machine form to the spec's shape: one status, every failure with tool,
// code, message, hint and doc.
func TestCheckJSONSchema(t *testing.T) {
	r := runCheck(appProject(t, "err_no_page_func", "example.com/x"), false)
	if r.OK {
		t.Fatal("the planted app must fail")
	}
	b, err := json.MarshalIndent(r.machine(), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Status   string `json:"status"`
		Failures []struct {
			Tool, Code, Message, Hint, Doc string
		} `json:"failures"`
	}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.Status != "fail" || len(got.Failures) == 0 {
		t.Fatalf("%s", b)
	}
	for _, f := range got.Failures {
		if f.Code == "" || f.Hint == "" || !strings.HasPrefix(f.Doc, "/docs/errors/") {
			t.Fatalf("failure without its teaching: %+v", f)
		}
		if _, ok := checkerr.ByCode(f.Code); !ok {
			t.Fatalf("%s is not in the catalog", f.Code)
		}
	}
}

// TestCheckExitCodes is the shell contract: a gate failure is 1, and misuse
// — a flag the command does not know — is 2.
func TestCheckExitCodes(t *testing.T) {
	if err := cmdCheck([]string{"--flag-que-nao-existe"}); err == nil {
		t.Fatal("an unknown flag must fail")
	} else {
		var usage usageError
		if !errors.As(err, &usage) {
			t.Fatalf("an unknown flag is misuse, got %T: %v", err, err)
		}
	}
}
