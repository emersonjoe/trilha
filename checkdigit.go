package trilha

// CheckDigit returns the two check digits of base under ISO 7064 MOD 97-10,
// the scheme behind the IBAN: append them to base and the whole code is a
// number that leaves remainder 1 when divided by 97.
//
// A code somebody types — a protocol number, the code printed on a receipt —
// should reach the database only when it is a code that could exist. Two check
// digits catch every single-character typo and every swap of two adjacent
// characters, and they cost the person nothing: they are part of what was
// printed. What they cost whoever is guessing is the point: only one code in
// ninety-seven passes, so a guess is refused before any query and enumeration
// is ninety-seven times more expensive before the rate limit is consulted.
//
//	code := base + trilha.CheckDigit(base) // "2026000123" + "45"
//
// Letters count as in the IBAN (A=10 … Z=35), case does not matter, and spaces
// and hyphens are skipped, so a base printed as "2026-0001" and typed as
// "20260001" get the same digits. Any other character makes the base invalid
// and the answer is "": there is no right check digit for a code that cannot
// exist.
//
//	see: HasCheckDigit
func CheckDigit(base string) string {
	rem, ok := mod97(base, true)
	if !ok {
		return ""
	}
	// Multiplying by 100 makes room for the two digits, and 98 - rem is what
	// puts the whole at remainder 1. It is always two digits: rem is at most
	// 96, so check runs from 2 to 98.
	check := 98 - (rem*100)%97
	return string([]byte{'0' + byte(check/10), '0' + byte(check%10)})
}

// HasCheckDigit reports whether code ends with the two check digits CheckDigit
// computes for the rest of it.
//
// It is the first thing a public lookup does with what was typed, and a false
// answer is refused with the same words as a code that does not exist: the
// check digits are a filter against typos and guessing, not a secret, and a
// screen that distinguishes "malformed" from "not found" has told a stranger
// which half of the guess was right.
//
//	if !trilha.HasCheckDigit(code) {
//		return naoEncontrado(c) // never reached the database
//	}
//
// Case, spaces and hyphens are ignored the way CheckDigit ignores them.
//
//	see: CheckDigit
func HasCheckDigit(code string) bool {
	rem, ok := mod97(code, false)
	if !ok {
		return false
	}
	return rem == 1
}

// mod97 walks the alphanumerics of s and returns their ISO 7064 remainder,
// with letters expanded to two digits. It is done one character at a time so
// the number never has to fit anywhere: a code has no length limit here.
//
// ok is false for a code with nothing in it or with a character that has no
// value and, when verifying, for one whose last two characters are not digits
// — a letter there is a code from some other scheme, not a typo of ours.
func mod97(s string, computing bool) (rem int, ok bool) {
	n := 0
	prev, last := -1, -1
	for i := 0; i < len(s); i++ {
		ch := s[i]
		var v int
		switch {
		case ch == ' ' || ch == '-':
			continue
		case ch >= '0' && ch <= '9':
			v = int(ch - '0')
			rem = (rem*10 + v) % 97
		case ch >= 'a' && ch <= 'z':
			ch -= 'a' - 'A'
			fallthrough
		case ch >= 'A' && ch <= 'Z':
			v = int(ch-'A') + 10
			rem = (rem*100 + v) % 97
		default:
			return 0, false
		}
		n++
		prev, last = last, v
	}
	if n == 0 {
		return 0, false
	}
	if !computing && (n < 3 || prev > 9 || last > 9) {
		return 0, false
	}
	return rem, true
}
