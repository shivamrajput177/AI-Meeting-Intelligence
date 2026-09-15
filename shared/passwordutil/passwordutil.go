// Package passwordutil hashes and verifies passwords with argon2id (the
// algorithm recorded in auth.credentials.algo — see
// docs/architecture/database-schema.md).
//
// golang.org/x/crypto/argon2 only computes the raw hash; it has no
// "verify this password" helper and no standard string encoding, so this
// package implements the common PHC-style encoding
// ($argon2id$v=19$m=...,t=...,p=...$salt$hash) so a hash can be stored as
// one self-describing TEXT column and later re-verified even if the cost
// parameters below are tuned over time.
package passwordutil

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Cost parameters. These are conservative defaults suitable for a
// general-purpose server (not tuned to specific hardware) — see the
// argon2 package docs for guidance on raising them.
const (
	memoryKiB   = 64 * 1024 // 64 MB
	iterations  = 3
	parallelism = 2
	saltLen     = 16
	keyLen      = 32
)

var ErrInvalidHash = errors.New("passwordutil: malformed hash")

// Hash returns an encoded argon2id hash of the given plaintext password.
func Hash(password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, iterations, memoryKiB, parallelism, keyLen)

	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Key := base64.RawStdEncoding.EncodeToString(key)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, memoryKiB, iterations, parallelism, b64Salt, b64Key), nil
}

// Verify reports whether password matches the given encoded hash,
// re-deriving the key with whatever cost parameters are embedded in the
// hash (so historical hashes stay verifiable even if the constants above
// change for newly created ones).
func Verify(password, encodedHash string) (bool, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, ErrInvalidHash
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false, ErrInvalidHash
	}
	var m, t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return false, ErrInvalidHash
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, ErrInvalidHash
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, ErrInvalidHash
	}

	got := argon2.IDKey([]byte(password), salt, t, m, p, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}
