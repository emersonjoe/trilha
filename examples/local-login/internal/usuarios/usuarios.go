// Package usuarios is the users table this app owns: e-mail, password hash and
// role. It stands for the table an app being migrated already has, hashes and
// all — which is the reason the hash format is the one Python writes.
package usuarios

import (
	"errors"
	"strings"
	"sync"

	"github.com/emersonjoe/trilha/auth"
)

// Usuario is one row.
type Usuario struct {
	ID    string
	Email string
	Nome  string
	Papel string
	// Hash is pbkdf2_sha256$iter$salt$hash, the format hashlib writes.
	Hash string
	// Token is what the API upstream expects in Authorization. In a real app
	// it comes from the API at login; here it stands still.
	Token string
}

// ErrCredencial is the one answer to a wrong e-mail and to a wrong password:
// saying which of the two was wrong tells an attacker who has an account.
var ErrCredencial = errors.New("e-mail ou senha inválidos")

// Store is the table. A real one is a database; the shape is the same.
type Store struct {
	mu   sync.RWMutex
	rows map[string]Usuario
}

// New seeds the two users of the example.
func New() *Store {
	s := &Store{rows: map[string]Usuario{}}
	s.Add("u-1", "ana@exemplo.com", "Ana", "analista", "segredo-da-ana", "jwt-da-ana")
	s.Add("u-2", "bia@exemplo.com", "Bia", "admin", "segredo-da-bia", "jwt-da-bia")
	return s
}

// Add hashes the password and stores the row.
func (s *Store) Add(id, email, nome, papel, senha, token string) {
	hash, err := auth.HashPBKDF2(senha)
	if err != nil {
		panic(err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rows[strings.ToLower(email)] = Usuario{ID: id, Email: email, Nome: nome, Papel: papel, Hash: hash, Token: token}
}

// Verify answers the row when the password checks out. The comparison runs
// even for an e-mail that does not exist, so the answer takes the same time
// either way.
func (s *Store) Verify(email, senha string) (Usuario, error) {
	s.mu.RLock()
	u, ok := s.rows[strings.ToLower(strings.TrimSpace(email))]
	s.mu.RUnlock()
	if !ok {
		auth.CheckPBKDF2(semUsuario, senha)
		return Usuario{}, ErrCredencial
	}
	if !auth.CheckPBKDF2(u.Hash, senha) {
		return Usuario{}, ErrCredencial
	}
	return u, nil
}

// semUsuario is a valid hash of nothing anybody knows, for the timing of a
// login attempt against an e-mail that is not in the table.
var semUsuario = func() string {
	h, err := auth.HashPBKDF2("nenhuma senha corresponde a este hash")
	if err != nil {
		panic(err)
	}
	return h
}()
