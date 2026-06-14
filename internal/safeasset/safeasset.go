// Package safeasset centralizes checks for files that must not be copied into
// generated public or embedded output.
package safeasset

import (
	"path"
	"path/filepath"
	"strings"
)

// UnsafeEmbeddedDirectory reports whether a directory should be skipped when
// copying generated output into a generated app.
func UnsafeEmbeddedDirectory(rel string) bool {
	base := path.Base(filepath.ToSlash(rel))
	switch base {
	case ".git", ".hg", ".svn", "node_modules", "tmp", "temp", ".tmp", "private", ".private", "secrets", ".secrets":
		return true
	default:
		return false
	}
}

// UnsafeEmbeddedFile reports whether a file must not be embedded or served as
// generated static output.
func UnsafeEmbeddedFile(rel string) bool {
	rel = filepath.ToSlash(rel)
	base := path.Base(rel)
	normalizedBase := strings.ToLower(base)
	ext := path.Ext(normalizedBase)
	switch {
	case normalizedBase == ".env" || strings.HasPrefix(normalizedBase, ".env."):
		return true
	case normalizedBase == "gowdk-security.json":
		return true
	case normalizedBase == ".npmrc" || normalizedBase == ".netrc":
		return true
	case PrivateKeyFile(normalizedBase):
		return true
	case ext == ".map" || ext == ".gwdk" || ext == ".go":
		return true
	case ext == ".tmp" || ext == ".temp" || strings.HasSuffix(normalizedBase, "~"):
		return true
	case ext == ".key" || ext == ".pem" || ext == ".p12" || ext == ".pfx":
		return true
	case strings.HasSuffix(normalizedBase, ".swp") || strings.HasSuffix(normalizedBase, ".swo"):
		return true
	default:
		return false
	}
}

// PrivateKeyFile reports common private key filenames without extension.
func PrivateKeyFile(base string) bool {
	switch strings.ToLower(base) {
	case "id_rsa", "id_dsa", "id_ecdsa", "id_ed25519":
		return true
	default:
		return false
	}
}
