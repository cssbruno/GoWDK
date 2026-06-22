// Package i18n provides dependency-free typed message catalog helpers.
package i18n

import (
	"fmt"
	"sort"
	"strings"
)

// Catalog maps typed keys to localized messages for one locale.
type Catalog[K comparable] struct {
	Locale   string
	Messages map[K]string
}

// NewCatalog creates a catalog with a defensive message copy.
func NewCatalog[K comparable](locale string, messages map[K]string) Catalog[K] {
	return Catalog[K]{
		Locale:   strings.TrimSpace(locale),
		Messages: cloneMessages(messages),
	}
}

// Message returns the localized message for key.
func (catalog Catalog[K]) Message(key K) (string, bool) {
	value, ok := catalog.Messages[key]
	return value, ok
}

// Must returns the localized message for key or panics when the key is missing.
// It is intended for build-time data functions and tests where missing keys
// should fail fast.
func (catalog Catalog[K]) Must(key K) string {
	value, ok := catalog.Message(key)
	if !ok {
		panic(fmt.Sprintf("i18n catalog %q missing key %v", catalog.Locale, key))
	}
	return value
}

// Format replaces {name} placeholders in the keyed message. Missing variables
// are left intact so callers can detect incomplete data in tests.
func (catalog Catalog[K]) Format(key K, vars map[string]string) (string, bool) {
	value, ok := catalog.Message(key)
	if !ok {
		return "", false
	}
	return Format(value, vars), true
}

// MustFormat is the panic-on-missing-key variant of Format.
func (catalog Catalog[K]) MustFormat(key K, vars map[string]string) string {
	value := catalog.Must(key)
	return Format(value, vars)
}

// Bundle stores catalogs by locale code.
type Bundle[K comparable] struct {
	DefaultLocale string
	Catalogs      map[string]Catalog[K]
}

// NewBundle creates a catalog bundle with defensive copies.
func NewBundle[K comparable](defaultLocale string, catalogs map[string]Catalog[K]) Bundle[K] {
	out := make(map[string]Catalog[K], len(catalogs))
	for locale, catalog := range catalogs {
		key := strings.TrimSpace(locale)
		if key == "" {
			key = strings.TrimSpace(catalog.Locale)
		}
		if key == "" {
			continue
		}
		out[key] = NewCatalog(key, catalog.Messages)
	}
	return Bundle[K]{
		DefaultLocale: strings.TrimSpace(defaultLocale),
		Catalogs:      out,
	}
}

// Catalog returns the requested locale catalog or the default catalog.
func (bundle Bundle[K]) Catalog(locale string) (Catalog[K], bool) {
	locale = strings.TrimSpace(locale)
	if locale != "" {
		if catalog, ok := bundle.Catalogs[locale]; ok {
			return catalog, true
		}
	}
	if bundle.DefaultLocale != "" {
		catalog, ok := bundle.Catalogs[bundle.DefaultLocale]
		return catalog, ok
	}
	return Catalog[K]{}, false
}

// MissingKeys reports typed keys absent from the catalog.
func (catalog Catalog[K]) MissingKeys(keys []K) []K {
	var missing []K
	for _, key := range keys {
		if _, ok := catalog.Messages[key]; !ok {
			missing = append(missing, key)
		}
	}
	return missing
}

// Format replaces literal {name} placeholders with values.
func Format(message string, vars map[string]string) string {
	if len(vars) == 0 || !strings.Contains(message, "{") {
		return message
	}
	keys := make([]string, 0, len(vars))
	for key := range vars {
		key = strings.TrimSpace(key)
		if key != "" {
			keys = append(keys, key)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		if len(keys[i]) != len(keys[j]) {
			return len(keys[i]) > len(keys[j])
		}
		return keys[i] < keys[j]
	})
	out := message
	for _, key := range keys {
		out = strings.ReplaceAll(out, "{"+key+"}", vars[key])
	}
	return out
}

func cloneMessages[K comparable](messages map[K]string) map[K]string {
	if len(messages) == 0 {
		return nil
	}
	out := make(map[K]string, len(messages))
	for key, value := range messages {
		out[key] = value
	}
	return out
}
