package gwdkanalysis

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/manifest"
)

func endpointSource(source manifest.EndpointSource) gwdkir.EndpointSource {
	if source == manifest.EndpointSourceGo {
		return gwdkir.EndpointSourceGo
	}
	return gwdkir.EndpointSourceGOWDK
}

func standaloneEndpointPageID(endpoint manifest.EndpointDeclaration) string {
	if endpoint.Package == "" {
		return endpoint.Name
	}
	return endpoint.Package + "." + endpoint.Name
}

func assetUse(uses []manifest.Use, path string) (name string, useAlias string, usePackage string) {
	alias, assetName, ok := strings.Cut(path, ".")
	if !ok {
		return path, "", ""
	}
	for _, use := range uses {
		if use.Alias == alias {
			return assetName, alias, use.Package
		}
	}
	return assetName, alias, ""
}

func splitCommaList(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(trimQuotes(part))
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func splitCSSList(value string) []string {
	value = strings.ReplaceAll(value, ",", " ")
	parts := strings.Fields(value)
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(trimQuotes(part))
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func cachePolicyValue(value string) (string, error) {
	policy := strings.TrimSpace(trimQuotes(value))
	if policy == "" {
		return "", fmt.Errorf("@cache requires a value")
	}
	if strings.ContainsAny(policy, "\r\n") {
		return "", fmt.Errorf("@cache must stay on one line")
	}
	return policy, nil
}

func revalidateSecondsValue(value string) (string, error) {
	raw := strings.TrimSpace(trimQuotes(value))
	if raw == "" {
		return "", fmt.Errorf("@revalidate requires a value")
	}
	if strings.ContainsAny(raw, "\r\n") {
		return "", fmt.Errorf("@revalidate must stay on one line")
	}
	if seconds, err := strconv.Atoi(raw); err == nil {
		if seconds <= 0 {
			return "", fmt.Errorf("@revalidate requires a positive duration")
		}
		return strconv.Itoa(seconds), nil
	}
	duration, err := time.ParseDuration(raw)
	if err != nil || duration <= 0 {
		return "", fmt.Errorf("@revalidate requires a positive duration such as 60s, 5m, or 1h")
	}
	if duration%time.Second != 0 {
		return "", fmt.Errorf("@revalidate must resolve to whole seconds")
	}
	return strconv.FormatInt(int64(duration/time.Second), 10), nil
}

func spanForName(spans []manifest.NamedSpan, name string, fallback manifest.SourceSpan) manifest.SourceSpan {
	for _, span := range spans {
		if span.Name == name {
			return span.Span
		}
	}
	return fallback
}

func contains(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

func hasImport(values []gwdkir.Import, value gwdkir.Import) bool {
	for _, item := range values {
		if item.Alias == value.Alias && item.Path == value.Path {
			return true
		}
	}
	return false
}

func hasUse(values []gwdkir.Use, value gwdkir.Use) bool {
	for _, item := range values {
		if item.Alias == value.Alias && item.Package == value.Package {
			return true
		}
	}
	return false
}

func trimQuotes(value string) string {
	return strings.Trim(strings.TrimSpace(value), `"`)
}
