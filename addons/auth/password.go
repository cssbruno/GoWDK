package auth

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"hash"
	"strconv"
	"strings"
)

const (
	// DefaultIterations is the PBKDF2 iteration count for new password hashes.
	// It is encoded into each hash so stored credentials remain verifiable if
	// this default later increases.
	DefaultIterations = 600000
	// MinIterations is the minimum accepted PBKDF2 iteration count for new and
	// stored hashes. Keep this separate from DefaultIterations so existing
	// stored hashes remain verifiable if the default later increases.
	MinIterations = 600000
	// MaxIterations is the maximum accepted PBKDF2 iteration count for new and
	// stored hashes. It bounds CPU cost when verifying imported or corrupted
	// hashes while leaving room for future default increases.
	MaxIterations = 2000000

	pbkdf2SaltLength = 16
	pbkdf2KeyLength  = 32
	pbkdf2Prefix     = "pbkdf2-sha256"
)

// ErrInvalidHash reports that an encoded password hash is malformed.
var ErrInvalidHash = errors.New("gowdk auth: invalid password hash")

// PasswordHasher hashes and verifies stored password credentials.
type PasswordHasher interface {
	HashPassword(password string) (string, error)
	VerifyPassword(password, encoded string) bool
}

// PBKDF2Hasher is the default dependency-free password hasher used by this
// addon. Iterations defaults to DefaultIterations when omitted.
type PBKDF2Hasher struct {
	Iterations int
}

// HashPassword derives a PBKDF2-HMAC-SHA256 hash of password using a fresh
// random salt and the default iteration count. The returned value is
// self-describing and safe to store: pbkdf2-sha256$<iter>$<b64salt>$<b64hash>.
func HashPassword(password string) (string, error) {
	return PBKDF2Hasher{}.HashPassword(password)
}

// HashPasswordWithIterations is HashPassword with an explicit work factor.
func HashPasswordWithIterations(password string, iterations int) (string, error) {
	if err := validateIterations(iterations); err != nil {
		return "", err
	}
	return PBKDF2Hasher{Iterations: iterations}.HashPassword(password)
}

// HashPassword derives a PBKDF2-HMAC-SHA256 hash of password using a fresh
// random salt.
func (hasher PBKDF2Hasher) HashPassword(password string) (string, error) {
	iterations := hasher.iterations()
	if err := validateIterations(iterations); err != nil {
		return "", err
	}
	salt := make([]byte, pbkdf2SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("gowdk auth: read salt: %w", err)
	}
	key, err := pbkdf2SHA256(password, salt, iterations, pbkdf2KeyLength)
	if err != nil {
		return "", fmt.Errorf("gowdk auth: derive password hash: %w", err)
	}
	return strings.Join([]string{
		pbkdf2Prefix,
		strconv.Itoa(iterations),
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	}, "$"), nil
}

// VerifyPassword reports whether password matches encoded. Comparison is
// constant-time. A malformed encoding returns false rather than an error so
// callers cannot distinguish "wrong password" from "corrupt record" by timing
// or control flow.
func VerifyPassword(password, encoded string) bool {
	return PBKDF2Hasher{}.VerifyPassword(password, encoded)
}

// VerifyPassword reports whether password matches encoded.
func (hasher PBKDF2Hasher) VerifyPassword(password, encoded string) bool {
	iterations, salt, want, err := decodeHash(encoded)
	if err != nil {
		return false
	}
	got, err := pbkdf2SHA256(password, salt, iterations, len(want))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(got, want) == 1
}

func (hasher PBKDF2Hasher) iterations() int {
	if hasher.Iterations == 0 {
		return DefaultIterations
	}
	return hasher.Iterations
}

func validateIterations(iterations int) error {
	if iterations < MinIterations {
		return fmt.Errorf("gowdk auth: iterations must be at least %d", MinIterations)
	}
	if iterations > MaxIterations {
		return fmt.Errorf("gowdk auth: iterations must be at most %d", MaxIterations)
	}
	return nil
}

func decodeHash(encoded string) (iterations int, salt, key []byte, err error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != pbkdf2Prefix {
		return 0, nil, nil, ErrInvalidHash
	}
	iterations, err = strconv.Atoi(parts[1])
	if err != nil || validateIterations(iterations) != nil {
		return 0, nil, nil, ErrInvalidHash
	}
	salt, err = base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil || len(salt) != pbkdf2SaltLength {
		return 0, nil, nil, ErrInvalidHash
	}
	key, err = base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil || len(key) != pbkdf2KeyLength {
		return 0, nil, nil, ErrInvalidHash
	}
	return iterations, salt, key, nil
}

func pbkdf2SHA256(password string, salt []byte, iterations, keyLength int) ([]byte, error) {
	return pbkdf2.Key(func() hash.Hash { return sha256.New() }, password, salt, iterations, keyLength)
}
