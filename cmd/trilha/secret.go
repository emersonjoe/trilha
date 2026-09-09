package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// cmdSecret prints a signing key. It exists because "generate a secret" is
// half an instruction: the other half is how long, from where, and in what
// encoding — and every answer somebody improvises to that is worse than this
// one line.
//
// Thirty-two bytes from crypto/rand, base64: the length HMAC-SHA256 uses as a
// key without folding it, in an encoding that survives a shell, a YAML file
// and a copy into somebody's deployment console.
func cmdSecret(args []string) error {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return err
	}
	fmt.Println(base64.StdEncoding.EncodeToString(b[:]))
	return nil
}
