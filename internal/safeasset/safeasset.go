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
	base := strings.ToLower(path.Base(filepath.ToSlash(rel)))
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
	return unsafeEmbeddedFile(rel, true)
}

// PublicGeneratedOutputFile reports whether rel is part of the browser-facing
// generated-output surface. Unknown files fail closed so compiler-private
// metadata and user-added files under an output directory are not served or
// embedded by default.
func PublicGeneratedOutputFile(rel string) bool {
	rel = cleanRelativeSlashPath(rel)
	if rel == "" || unsafeEmbeddedFile(rel, false) || privateGeneratedMetadata(rel) {
		return false
	}
	if strings.HasPrefix(rel, "assets/") {
		return true
	}
	switch path.Base(rel) {
	case "sitemap.xml", "robots.txt", "openapi.json", "asyncapi.json":
		return true
	}
	switch strings.ToLower(path.Ext(rel)) {
	case ".html", ".css", ".js", ".wasm", ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".ico", ".avif", ".woff", ".woff2", ".ttf", ".otf":
		return true
	default:
		return false
	}
}

// EmbeddableGeneratedOutputFile reports whether rel should be copied into a
// generated app's embedded filesystem. Runtime-private manifests are embedded
// for generated code but remain non-public in the runtime handler.
func EmbeddableGeneratedOutputFile(rel string) bool {
	rel = cleanRelativeSlashPath(rel)
	if rel == "" || unsafeEmbeddedFile(rel, false) {
		return false
	}
	return PublicGeneratedOutputFile(rel) || runtimePrivateGeneratedMetadata(rel)
}

func unsafeEmbeddedFile(rel string, blockMetadata bool) bool {
	rel = filepath.ToSlash(rel)
	base := path.Base(rel)
	normalizedBase := strings.ToLower(base)
	ext := path.Ext(normalizedBase)
	switch {
	case normalizedBase == ".env" || strings.HasPrefix(normalizedBase, ".env."):
		return true
	case blockMetadata && privateGeneratedMetadata(rel):
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

func privateGeneratedMetadata(rel string) bool {
	switch strings.ToLower(path.Base(filepath.ToSlash(rel))) {
	case "gowdk-security.json", "gowdk-build-report.json", "gowdk-build-timings.json", "gowdk-routes.json", "gowdk-assets.json":
		return true
	default:
		return false
	}
}

func runtimePrivateGeneratedMetadata(rel string) bool {
	switch strings.ToLower(path.Base(filepath.ToSlash(rel))) {
	case "gowdk-routes.json", "gowdk-assets.json":
		return true
	default:
		return false
	}
}

func cleanRelativeSlashPath(rel string) string {
	rel = strings.TrimPrefix(filepath.ToSlash(strings.TrimSpace(rel)), "/")
	if rel == "" {
		return ""
	}
	clean := path.Clean(rel)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return ""
	}
	return clean
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
