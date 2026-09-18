package trilha

import (
	"math/big"
	"math/rand"
	"strings"
	"testing"
)

// expand writes s the way ISO 7064 reads it: digits as they are, letters as
// two digits (A=10 … Z=35), separators dropped.
func expand(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteString(big.NewInt(int64(r-'A') + 10).String())
		}
	}
	return b.String()
}

// oracle is the definition in arbitrary precision — 98 - (base*100 mod 97) —
// so the one-character-at-a-time arithmetic is checked against the whole
// number and not against itself.
func oracle(t *testing.T, base string) string {
	t.Helper()
	n, ok := new(big.Int).SetString(expand(base)+"00", 10)
	if !ok {
		t.Fatalf("oracle: %q is not a number", base)
	}
	rem := new(big.Int).Mod(n, big.NewInt(97)).Int64()
	check := 98 - rem
	return string([]byte{'0' + byte(check/10), '0' + byte(check%10)})
}

// #267: the digits are the IBAN's. GB82 WEST 1234 5698 7654 32 is the example
// every IBAN page carries; rotated the way IBAN validation rotates it — the
// country and the check digits moved to the end — it is a plain ISO 7064
// MOD 97-10 code, so the arithmetic is proven against a vector from outside
// this file.
func TestCheckDigitIBAN(t *testing.T) {
	if got := CheckDigit("WEST12345698765432GB"); got != "82" {
		t.Fatalf("CheckDigit(WEST…GB) = %q, want 82", got)
	}
	if !HasCheckDigit("WEST12345698765432GB82") {
		t.Fatal("the IBAN's own check digits do not verify")
	}
	// Case and separators are presentation, not value: what is printed with
	// spaces is typed without them.
	if !HasCheckDigit("west 1234-5698-7654-32 gb 82") {
		t.Fatal("lower case and separators changed the value")
	}
	if got := CheckDigit("west-1234 5698 7654 32 gb"); got != "82" {
		t.Fatalf("CheckDigit with separators = %q", got)
	}
}

// Every base agrees with the definition computed in big.Int, digits only and
// with letters, and the code that comes out of it verifies.
func TestCheckDigitMatchesOracle(t *testing.T) {
	const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	rng := rand.New(rand.NewSource(267))
	for i := 0; i < 500; i++ {
		n := 1 + rng.Intn(30)
		var b strings.Builder
		for j := 0; j < n; j++ {
			if i%2 == 0 {
				b.WriteByte(alphabet[rng.Intn(10)]) // a protocol number
			} else {
				b.WriteByte(alphabet[rng.Intn(len(alphabet))])
			}
		}
		base := b.String()
		want, got := oracle(t, base), CheckDigit(base)
		if got != want {
			t.Fatalf("CheckDigit(%q) = %q, the oracle says %q", base, got, want)
		}
		if !HasCheckDigit(base + got) {
			t.Fatalf("HasCheckDigit(%q) is false", base+got)
		}
	}
}

// One wrong character and two neighbours swapped are what check digits exist
// for, and every one of them is caught.
func TestCheckDigitCatchesTyposAndSwaps(t *testing.T) {
	base := "2026000123"
	code := base + CheckDigit(base)
	for i := 0; i < len(code); i++ {
		for d := byte('0'); d <= '9'; d++ {
			if code[i] == d {
				continue
			}
			typo := code[:i] + string(d) + code[i+1:]
			if HasCheckDigit(typo) {
				t.Fatalf("the typo %q verifies", typo)
			}
		}
		if i+1 < len(code) && code[i] != code[i+1] {
			swap := code[:i] + string(code[i+1]) + string(code[i]) + code[i+2:]
			if HasCheckDigit(swap) {
				t.Fatalf("the swap %q verifies", swap)
			}
		}
	}
}

// The edges: nothing to compute, a character with no value, a code too short
// to carry two digits, and letters where the digits should be.
func TestCheckDigitRefusesWhatCannotBeACode(t *testing.T) {
	for _, base := range []string{"", "  ", "--", "12.3", "ab_c", "ñ1"} {
		if got := CheckDigit(base); got != "" {
			t.Errorf("CheckDigit(%q) = %q, want empty", base, got)
		}
		if HasCheckDigit(base) {
			t.Errorf("HasCheckDigit(%q) is true", base)
		}
	}
	for _, code := range []string{"1", "12", "97", "1AB", "ABC", "2026000123AB"} {
		if HasCheckDigit(code) {
			t.Errorf("HasCheckDigit(%q) is true", code)
		}
	}
	// Both digits are always written, the leading zero included: a code whose
	// check is "07" is not a code whose check is "7".
	zero := false
	for i := 0; i < 300 && !zero; i++ {
		base := big.NewInt(int64(i)).String()
		d := CheckDigit(base)
		if len(d) != 2 {
			t.Fatalf("CheckDigit(%q) = %q, not two digits", base, d)
		}
		if d[0] == '0' {
			zero = true
		}
	}
	if !zero {
		t.Fatal("no base under 300 has a check starting with 0; the padding went untested")
	}
}
