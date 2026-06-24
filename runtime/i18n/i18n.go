// Package i18n provides dependency-free typed message catalog helpers.
package i18n

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Catalog maps typed keys to localized messages for one locale.
type Catalog[K comparable] struct {
	Locale   string
	Messages map[K]string
}

// MessageReference records one expected message key and, optionally, where it
// was declared or used. Apps can keep these references beside build helpers or
// generated extraction output and check catalogs in ordinary Go tests.
type MessageReference[K comparable] struct {
	Key    K      `json:"key"`
	Source string `json:"source,omitempty"`
	Line   int    `json:"line,omitempty"`
	Column int    `json:"column,omitempty"`
}

// Key records an expected message key without source metadata.
func Key[K comparable](key K) MessageReference[K] {
	return MessageReference[K]{Key: key}
}

// Ref records an expected message key with source metadata.
func Ref[K comparable](key K, source string, line int, column int) MessageReference[K] {
	return MessageReference[K]{
		Key:    key,
		Source: strings.TrimSpace(source),
		Line:   line,
		Column: column,
	}
}

// CatalogReport is the deterministic completeness report for one locale.
type CatalogReport[K comparable] struct {
	Locale         string                `json:"locale,omitempty"`
	MissingCatalog bool                  `json:"missingCatalog,omitempty"`
	Missing        []MessageReference[K] `json:"missing,omitempty"`
	Unused         []K                   `json:"unused,omitempty"`
}

// BundleReport summarizes catalog completeness across locales.
type BundleReport[K comparable] struct {
	DefaultLocale string                `json:"defaultLocale,omitempty"`
	Required      []MessageReference[K] `json:"required,omitempty"`
	Catalogs      []CatalogReport[K]    `json:"catalogs,omitempty"`
}

// CatalogTemplateEntry is one deterministic entry in a catalog template.
type CatalogTemplateEntry[K comparable] struct {
	Key    K      `json:"key"`
	Value  string `json:"value"`
	Source string `json:"source,omitempty"`
	Line   int    `json:"line,omitempty"`
	Column int    `json:"column,omitempty"`
}

// CatalogTemplate is a deterministic locale-specific starter catalog.
type CatalogTemplate[K comparable] struct {
	Locale  string                    `json:"locale"`
	Entries []CatalogTemplateEntry[K] `json:"entries"`
}

// PluralCategory is the bounded core plural category set.
type PluralCategory string

const (
	PluralZero  PluralCategory = "zero"
	PluralOne   PluralCategory = "one"
	PluralOther PluralCategory = "other"
)

// PluralForms holds the supported plural variants. Other is the fallback.
type PluralForms struct {
	Zero  string
	One   string
	Other string
}

// NumberFormatOptions controls dependency-free number formatting.
type NumberFormatOptions struct {
	MinFractionDigits int
	MaxFractionDigits int
}

// DateStyle selects a deterministic date format.
type DateStyle string

const (
	DateShort  DateStyle = "short"
	DateMedium DateStyle = "medium"
)

// TimeStyle selects a deterministic time format.
type TimeStyle string

const (
	TimeShort  TimeStyle = "short"
	TimeMedium TimeStyle = "medium"
)

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

// Template returns a deterministic starter catalog for the required keys. Any
// existing catalog values are copied into the template entries.
func (catalog Catalog[K]) Template(required []MessageReference[K]) CatalogTemplate[K] {
	entries := make([]CatalogTemplateEntry[K], 0, len(required))
	for _, ref := range uniqueReferences(required) {
		value := ""
		if catalog.Messages != nil {
			value = catalog.Messages[ref.Key]
		}
		entries = append(entries, CatalogTemplateEntry[K]{
			Key:    ref.Key,
			Value:  value,
			Source: ref.Source,
			Line:   ref.Line,
			Column: ref.Column,
		})
	}
	return CatalogTemplate[K]{
		Locale:  strings.TrimSpace(catalog.Locale),
		Entries: entries,
	}
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

// Check returns a deterministic completeness report for the required message
// references. Missing keys, unused keys, and missing default catalogs are
// reported without panicking so app tests can decide the failure policy.
func (bundle Bundle[K]) Check(required []MessageReference[K]) BundleReport[K] {
	refs := sortedReferences(required)
	requiredKeys := map[K]bool{}
	for _, ref := range refs {
		requiredKeys[ref.Key] = true
	}
	locales := sortedCatalogLocales(bundle.Catalogs)
	defaultLocale := strings.TrimSpace(bundle.DefaultLocale)
	if defaultLocale != "" && !containsString(locales, defaultLocale) {
		locales = append(locales, defaultLocale)
		sort.Strings(locales)
	}
	if len(locales) == 0 && len(refs) > 0 {
		locales = []string{""}
	}

	reports := make([]CatalogReport[K], 0, len(locales))
	for _, locale := range locales {
		catalog, ok := bundle.Catalogs[locale]
		report := CatalogReport[K]{Locale: locale}
		if !ok {
			report.MissingCatalog = true
			report.Missing = append([]MessageReference[K](nil), refs...)
			reports = append(reports, report)
			continue
		}
		for _, ref := range refs {
			if _, exists := catalog.Messages[ref.Key]; !exists {
				report.Missing = append(report.Missing, ref)
			}
		}
		for key := range catalog.Messages {
			if !requiredKeys[key] {
				report.Unused = append(report.Unused, key)
			}
		}
		sortKeys(report.Unused)
		reports = append(reports, report)
	}

	return BundleReport[K]{
		DefaultLocale: defaultLocale,
		Required:      refs,
		Catalogs:      reports,
	}
}

// Template returns a deterministic starter catalog for one locale. Existing
// values are used only when that exact locale is already present.
func (bundle Bundle[K]) Template(locale string, required []MessageReference[K]) CatalogTemplate[K] {
	locale = strings.TrimSpace(locale)
	if locale == "" {
		locale = strings.TrimSpace(bundle.DefaultLocale)
	}
	catalog, ok := bundle.Catalogs[locale]
	if !ok {
		catalog = Catalog[K]{Locale: locale}
	}
	return catalog.Template(required)
}

// OK reports whether the bundle has no missing catalogs, missing keys, or
// unused keys for the checked required references.
func (report BundleReport[K]) OK() bool {
	for _, catalog := range report.Catalogs {
		if catalog.MissingCatalog || len(catalog.Missing) > 0 || len(catalog.Unused) > 0 {
			return false
		}
	}
	return true
}

// Error renders the report as indented JSON for app-owned tests.
func (report BundleReport[K]) Error() string {
	payload, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Sprintf("%#v", report)
	}
	return string(payload)
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

// Cardinal returns the supported plural category for count. The root runtime
// intentionally keeps this bounded: East Asian locale families fall back to
// other, and the default family uses one for exactly +/-1.
func Cardinal(locale string, count int) PluralCategory {
	switch primaryLocale(locale) {
	case "ja", "ko", "th", "vi", "zh":
		return PluralOther
	}
	if count == 1 || count == -1 {
		return PluralOne
	}
	return PluralOther
}

// FormatPlural selects a plural form, injects {count}, and applies Format.
func FormatPlural(locale string, count int, forms PluralForms, vars map[string]string) string {
	pattern := forms.Other
	if count == 0 && forms.Zero != "" {
		pattern = forms.Zero
	} else if Cardinal(locale, count) == PluralOne && forms.One != "" {
		pattern = forms.One
	}
	if pattern == "" {
		pattern = forms.One
	}
	values := cloneStringMap(vars)
	values["count"] = strconv.Itoa(count)
	return Format(pattern, values)
}

// FormatNumber formats a number with deterministic locale separators. It is a
// bounded helper, not a CLDR replacement.
func FormatNumber(locale string, value float64, options NumberFormatOptions) string {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return strconv.FormatFloat(value, 'f', -1, 64)
	}
	negative := value < 0
	if negative {
		value = -value
	}
	text := formatNumberMagnitude(value, options)
	integer, fraction, hasFraction := strings.Cut(text, ".")
	decimal, group := numberSeparators(locale)
	out := groupInteger(integer, group)
	if hasFraction && fraction != "" {
		out += decimal + fraction
	}
	if negative && out != "0" {
		out = "-" + out
	}
	return out
}

// FormatDate formats a date using the supported deterministic style.
func FormatDate(locale string, value time.Time, style DateStyle) string {
	switch primaryLocale(locale) {
	case "pt":
		return value.Format("02/01/2006")
	default:
		if style == DateMedium {
			return value.Format("Jan 2, 2006")
		}
		return value.Format("01/02/2006")
	}
}

// FormatTime formats a time using the supported deterministic style.
func FormatTime(locale string, value time.Time, style TimeStyle) string {
	switch primaryLocale(locale) {
	case "pt":
		if style == TimeMedium {
			return value.Format("15:04:05")
		}
		return value.Format("15:04")
	default:
		if style == TimeMedium {
			return value.Format("3:04:05 PM")
		}
		return value.Format("3:04 PM")
	}
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

func cloneStringMap(values map[string]string) map[string]string {
	out := make(map[string]string, len(values)+1)
	for key, value := range values {
		out[key] = value
	}
	return out
}

func sortedReferences[K comparable](refs []MessageReference[K]) []MessageReference[K] {
	out := append([]MessageReference[K](nil), refs...)
	sort.SliceStable(out, func(i, j int) bool {
		return referenceSortKey(out[i]) < referenceSortKey(out[j])
	})
	return out
}

func uniqueReferences[K comparable](refs []MessageReference[K]) []MessageReference[K] {
	refs = sortedReferences(refs)
	seen := map[K]int{}
	unique := make([]MessageReference[K], 0, len(refs))
	for _, ref := range refs {
		if index, ok := seen[ref.Key]; ok {
			if unique[index].Source == "" && ref.Source != "" {
				unique[index] = ref
			}
			continue
		}
		seen[ref.Key] = len(unique)
		unique = append(unique, ref)
	}
	return unique
}

func referenceSortKey[K comparable](ref MessageReference[K]) string {
	return fmt.Sprintf("%v\x00%s\x00%09d\x00%09d", ref.Key, ref.Source, ref.Line, ref.Column)
}

func sortedCatalogLocales[K comparable](catalogs map[string]Catalog[K]) []string {
	locales := make([]string, 0, len(catalogs))
	for locale := range catalogs {
		if strings.TrimSpace(locale) != "" {
			locales = append(locales, locale)
		}
	}
	sort.Strings(locales)
	return locales
}

func containsString(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func sortKeys[K comparable](keys []K) {
	sort.SliceStable(keys, func(i, j int) bool {
		return fmt.Sprint(keys[i]) < fmt.Sprint(keys[j])
	})
}

func formatNumberMagnitude(value float64, options NumberFormatOptions) string {
	minDigits := clampFractionDigits(options.MinFractionDigits)
	maxDigits := clampFractionDigits(options.MaxFractionDigits)
	if options.MinFractionDigits == 0 && options.MaxFractionDigits == 0 {
		return strconv.FormatFloat(value, 'f', -1, 64)
	}
	if maxDigits < minDigits {
		maxDigits = minDigits
	}
	text := strconv.FormatFloat(value, 'f', maxDigits, 64)
	if maxDigits == minDigits {
		return text
	}
	integer, fraction, ok := strings.Cut(text, ".")
	if !ok {
		return text
	}
	for len(fraction) > minDigits && strings.HasSuffix(fraction, "0") {
		fraction = strings.TrimSuffix(fraction, "0")
	}
	if fraction == "" {
		return integer
	}
	return integer + "." + fraction
}

func clampFractionDigits(value int) int {
	if value < 0 {
		return 0
	}
	if value > 9 {
		return 9
	}
	return value
}

func numberSeparators(locale string) (decimal string, group string) {
	switch primaryLocale(locale) {
	case "pt":
		return ",", "."
	default:
		return ".", ","
	}
}

func groupInteger(value string, separator string) string {
	if len(value) <= 3 {
		return value
	}
	var groups []string
	for len(value) > 3 {
		groups = append(groups, value[len(value)-3:])
		value = value[:len(value)-3]
	}
	groups = append(groups, value)
	for left, right := 0, len(groups)-1; left < right; left, right = left+1, right-1 {
		groups[left], groups[right] = groups[right], groups[left]
	}
	return strings.Join(groups, separator)
}

func primaryLocale(locale string) string {
	locale = strings.ToLower(strings.TrimSpace(locale))
	if locale == "" {
		return ""
	}
	locale = strings.ReplaceAll(locale, "_", "-")
	primary, _, _ := strings.Cut(locale, "-")
	return primary
}
