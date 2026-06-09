package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const (
	// DefaultIterations is the PBKDF2 iteration count for new password hashes.
	// It is encoded into each hash so stored credentials remain verifiable if
	// this default later increases.
	DefaultIterations = 600000

	pbkdf2SaltLength = 16
	pbkdf2KeyLength  = 32
	pbkdf2Prefix     = "pbkdf2-sha256"
)

// ErrInvalidHash reports that an encoded password hash is malformed.
var ErrInvalidHash = errors.New("gowdk auth: invalid password hash")

// HashPassword derives a PBKDF2-HMAC-SHA256 hash of password using a fresh
// random salt and the default iteration count. The returned value is
// self-describing and safe to store: pbkdf2-sha256$<iter>$<b64salt>$<b64hash>.
func HashPassword(password string) (string, error) {
	return HashPasswordWithIterations(password, DefaultIterations)
}

// HashPasswordWithIterations is HashPassword with an explicit work factor.
func HashPasswordWithIterations(password string, iterations int) (string, error) {
	if iterations < 1 {
		return "", fmt.Errorf("gowdk auth: iterations must be at least 1")
	}
	salt := make([]byte, pbkdf2SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("gowdk auth: read salt: %w", err)
	}
	key := pbkdf2SHA256([]byte(password), salt, iterations, pbkdf2KeyLength)
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
	iterations, salt, want, err := decodeHash(encoded)
	if err != nil {
		return false
	}
	got := pbkdf2SHA256([]byte(password), salt, iterations, len(want))
	return subtle.ConstantTimeCompare(got, want) == 1
}

func decodeHash(encoded string) (iterations int, salt, key []byte, err error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != pbkdf2Prefix {
		return 0, nil, nil, ErrInvalidHash
	}
	iterations, err = strconv.Atoi(parts[1])
	if err != nil || iterations < 1 {
		return 0, nil, nil, ErrInvalidHash
	}
	salt, err = base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil || len(salt) == 0 {
		return 0, nil, nil, ErrInvalidHash
	}
	key, err = base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil || len(key) == 0 {
		return 0, nil, nil, ErrInvalidHash
	}
	return iterations, salt, key, nil
}

// pbkdf2SHA256 implements PBKDF2 (RFC 8018) with HMAC-SHA256 as the PRF. It is
// hand-rolled on crypto/hmac so the addon carries no dependency outside the
// standard library.
func pbkdf2SHA256(password, salt []byte, iterations, keyLength int) []byte {
	prf := hmac.New(sha256.New, password)
	hashLength := prf.Size()
	blocks := (keyLength + hashLength - 1) / hashLength

	derived := make([]byte, 0, blocks*hashLength)
	block := make([]byte, 4)
	for index := 1; index <= blocks; index++ {
		block[0] = byte(index >> 24)
		block[1] = byte(index >> 16)
		block[2] = byte(index >> 8)
		block[3] = byte(index)

		prf.Reset()
		prf.Write(salt)
		prf.Write(block)
		current := prf.Sum(nil)

		result := make([]byte, len(current))
		copy(result, current)
		for iteration := 1; iteration < iterations; iteration++ {
			prf.Reset()
			prf.Write(current)
			current = prf.Sum(current[:0])
			for offset := range result {
				result[offset] ^= current[offset]
			}
		}
		derived = append(derived, result...)
	}
	return derived[:keyLength]
}
