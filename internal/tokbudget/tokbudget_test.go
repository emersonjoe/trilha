package tokbudget

import (
	"strings"
	"testing"
)

// The estimator is a contract, not a heuristic to tune: three facts the
// callers rely on — zero in, zero out; rounding up; one number per text — and
// the rate itself, which every "est." label in every output answers for.
func TestEstimate(t *testing.T) {
	for _, tc := range []struct {
		name, in string
		want     int
	}{
		{"empty", "", 0},
		{"one char rounds up", "a", 1},
		{"exactly one token", "abcd", 1},
		{"one over rounds up", "abcde", 2},
		{"ascii sentence", "hello world, this is thirty chars..", 9},
	} {
		if got := Estimate(tc.in); got != tc.want {
			t.Errorf("%s: Estimate(%q) = %d, want %d", tc.name, tc.in, got, tc.want)
		}
	}
	// Multi-byte runes count as bytes: the estimate errs high on unicode,
	// which is the safe side of a budget.
	unicode := "çãáéíóú" // 7 runes, 14 bytes
	if got := Estimate(unicode); got != len(unicode)/CharsPerToken+1 {
		t.Errorf("unicode: Estimate = %d, want the byte-based %d", got, len(unicode)/CharsPerToken+1)
	}
	// The rate is what the doc-comment says, and doubling the text at least
	// doubles the cost.
	if CharsPerToken != 4 {
		t.Fatalf("CharsPerToken = %d, want 4", CharsPerToken)
	}
	long := strings.Repeat("x", 100)
	if Estimate(long+long) < 2*Estimate(long) {
		t.Error("doubling the text may not halve the cost")
	}
}
