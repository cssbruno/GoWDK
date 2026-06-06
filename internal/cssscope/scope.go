// Package cssscope builds deterministic scope and hash metadata for scoped CSS.
package cssscope

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
)

// HashKey returns the canonical identity used for scoped CSS hashing.
func HashKey(ownerKind string, packageName string, ownerID string, source string, assetPath string) string {
	parts := []string{
		strings.TrimSpace(ownerKind),
		strings.TrimSpace(packageName),
		strings.TrimSpace(ownerID),
		filepath.ToSlash(strings.TrimSpace(source)),
		strings.TrimSpace(assetPath),
	}
	return strings.Join(parts, ":")
}

// ScopeID returns a stable CSS scope ID for a hash key.
func ScopeID(hashKey string) string {
	sum := sha256.Sum256([]byte(hashKey))
	return "gwdk-" + hex.EncodeToString(sum[:])[:12]
}
