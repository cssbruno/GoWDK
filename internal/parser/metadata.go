package parser

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

func applyMetadata(page *gwdkir.Page, name, rawValue string, lineNumber int, rawLine string) error {
	value := strings.TrimSpace(rawValue)
	span := sourceLineSpan(lineNumber, rawLine)
	switch name {
	case "page":
		if value == "" {
			return fmt.Errorf("page requires a value")
		}
		page.ID = value
		page.Spans.Page = span
	case "route":
		if value == "" {
			return fmt.Errorf("route requires a value")
		}
		route, params, spans, err := parseRouteDeclaration(trimQuotes(value), lineNumber, rawLine)
		if err != nil {
			return err
		}
		page.Route = route
		page.RouteParams = params
		page.Spans.Route = span
		page.Spans.RouteParams = spans
	case "layout":
		if value == "" {
			return fmt.Errorf("layout requires a value")
		}
		page.Layouts = splitList(value)
		page.Spans.Layouts = namedValueSpans(page.Layouts, lineNumber, rawLine)
	case "cache":
		policy, err := cachePolicyValue(value)
		if err != nil {
			return err
		}
		page.Cache = policy
		page.Spans.Cache = span
	case "revalidate":
		seconds, err := revalidateSecondsValue(value)
		if err != nil {
			return err
		}
		page.Revalidate = seconds
		page.Spans.Revalidate = span
	case "error":
		errorPage, err := source.ErrorPagePath(trimQuotes(value))
		if err != nil {
			return err
		}
		page.ErrorPage = errorPage
		page.Spans.ErrorPage = span
	case "title":
		title, err := metadataText(name, value)
		if err != nil {
			return err
		}
		page.Metadata.Title = title
		page.Spans.Title = span
	case "description":
		description, err := metadataText(name, value)
		if err != nil {
			return err
		}
		page.Metadata.Description = description
		page.Spans.Description = span
	case "canonical":
		canonical, err := metadataText(name, value)
		if err != nil {
			return err
		}
		page.Metadata.Canonical = canonical
		page.Spans.Canonical = span
	case "image":
		image, err := metadataText(name, value)
		if err != nil {
			return err
		}
		page.Metadata.Image = image
		page.Spans.Image = span
	case "robots":
		robots, err := metadataText(name, value)
		if err != nil {
			return err
		}
		page.Metadata.Robots = robots
		page.Spans.Robots = span
	case "noindex":
		noIndex, err := metadataBool(name, value)
		if err != nil {
			return err
		}
		page.Metadata.NoIndex = noIndex
		page.Spans.NoIndex = span
	case "preload":
		resource, err := metadataHeadResource(name, value)
		if err != nil {
			return err
		}
		page.Metadata.Preload = append(page.Metadata.Preload, resource)
		page.Spans.Preload = append(page.Spans.Preload, source.NamedSpan{Name: resource.Href, Span: span})
	case "prefetch":
		resource, err := metadataHeadResource(name, value)
		if err != nil {
			return err
		}
		page.Metadata.Prefetch = append(page.Metadata.Prefetch, resource)
		page.Spans.Prefetch = append(page.Spans.Prefetch, source.NamedSpan{Name: resource.Href, Span: span})
	case "jsonld":
		kind, err := metadataText(name, value)
		if err != nil {
			return err
		}
		page.Metadata.Structured = append(page.Metadata.Structured, gwdkir.StructuredData{Kind: kind})
		page.Spans.Structured = append(page.Spans.Structured, source.NamedSpan{Name: kind, Span: span})
	case "guard":
		if value == "" {
			return fmt.Errorf("guard requires a value")
		}
		page.Guards = splitList(value)
		page.Spans.Guard = namedValueSpans(page.Guards, lineNumber, rawLine)
	case "css":
		if value == "" {
			return fmt.Errorf("css requires a value")
		}
		page.CSS = splitCSSList(value)
		page.Spans.CSS = namedValueSpans(page.CSS, lineNumber, rawLine)
	default:
		return fmt.Errorf("unsupported metadata %s", name)
	}
	return nil
}

func metadataText(name, value string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("%s requires a value", name)
	}
	text := strings.TrimSpace(trimQuotes(value))
	if text == "" {
		return "", fmt.Errorf("%s requires a non-empty value", name)
	}
	return text, nil
}

func metadataBool(name, value string) (bool, error) {
	raw := strings.ToLower(strings.TrimSpace(trimQuotes(value)))
	if raw == "" {
		return true, nil
	}
	switch raw {
	case "true", "yes", "1":
		return true, nil
	case "false", "no", "0":
		return false, nil
	default:
		return false, fmt.Errorf("%s requires true or false", name)
	}
}

func metadataHeadResource(name, value string) (gwdkir.HeadResource, error) {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return gwdkir.HeadResource{}, fmt.Errorf("%s requires a resource URL", name)
	}
	if len(fields) != 1 && len(fields) != 2 && len(fields) != 3 {
		return gwdkir.HeadResource{}, fmt.Errorf("%s requires '<href>' or '<href> as <type>'", name)
	}
	href := strings.TrimSpace(trimQuotes(fields[0]))
	if err := validateHeadResourceHref(name, href); err != nil {
		return gwdkir.HeadResource{}, err
	}
	resource := gwdkir.HeadResource{Href: href}
	switch len(fields) {
	case 2:
		resource.As = strings.TrimSpace(trimQuotes(fields[1]))
	case 3:
		if fields[1] != "as" {
			return gwdkir.HeadResource{}, fmt.Errorf("%s requires '<href> as <type>'", name)
		}
		resource.As = strings.TrimSpace(trimQuotes(fields[2]))
	}
	if strings.ContainsAny(resource.As, "\r\n") {
		return gwdkir.HeadResource{}, fmt.Errorf("%s resource type must stay on one line", name)
	}
	return resource, nil
}

func validateHeadResourceHref(name, href string) error {
	if href == "" {
		return fmt.Errorf("%s requires a non-empty resource URL", name)
	}
	if strings.ContainsAny(href, "\r\n") {
		return fmt.Errorf("%s resource URL must stay on one line", name)
	}
	if strings.HasPrefix(href, "//") {
		return fmt.Errorf("%s resource URL must not be protocol-relative", name)
	}
	parsed, err := url.Parse(href)
	if err != nil {
		return fmt.Errorf("%s resource URL is invalid: %w", name, err)
	}
	if parsed.IsAbs() {
		scheme := strings.ToLower(parsed.Scheme)
		if scheme != "http" && scheme != "https" {
			return fmt.Errorf("%s resource URL must use http or https when absolute", name)
		}
	}
	return nil
}

func endpointErrorPage(match []string, lineNumber int) (string, error) {
	if len(match) < 5 || strings.TrimSpace(match[4]) == "" {
		return "", nil
	}
	errorPage, err := source.ErrorPagePath(match[4])
	if err != nil {
		return "", fmt.Errorf("line %d: %w", lineNumber, err)
	}
	return errorPage, nil
}

func endpointErrorPageSpan(match []string, fallback source.SourceSpan) source.SourceSpan {
	if len(match) < 5 || strings.TrimSpace(match[4]) == "" {
		return source.SourceSpan{}
	}
	return fallback
}

func cachePolicyValue(value string) (string, error) {
	policy := strings.TrimSpace(trimQuotes(value))
	if policy == "" {
		return "", fmt.Errorf("cache requires a value")
	}
	if strings.ContainsAny(policy, "\r\n") {
		return "", fmt.Errorf("cache must stay on one line")
	}
	return policy, nil
}

func revalidateSecondsValue(value string) (string, error) {
	raw := strings.TrimSpace(trimQuotes(value))
	if raw == "" {
		return "", fmt.Errorf("revalidate requires a value")
	}
	if strings.ContainsAny(raw, "\r\n") {
		return "", fmt.Errorf("revalidate must stay on one line")
	}
	if seconds, err := strconv.Atoi(raw); err == nil {
		if seconds <= 0 {
			return "", fmt.Errorf("revalidate requires a positive duration")
		}
		return strconv.Itoa(seconds), nil
	}
	duration, err := time.ParseDuration(raw)
	if err != nil || duration <= 0 {
		return "", fmt.Errorf("revalidate requires a positive duration such as 60s, 5m, or 1h")
	}
	if duration%time.Second != 0 {
		return "", fmt.Errorf("revalidate must resolve to whole seconds")
	}
	return strconv.FormatInt(int64(duration/time.Second), 10), nil
}

func applyLayoutMetadata(layout *gwdkir.Layout, name, rawValue string, lineNumber int, rawLine string) error {
	value := strings.TrimSpace(rawValue)
	switch name {
	case "layout":
		if value == "" {
			return fmt.Errorf("layout requires a value")
		}
		refs := splitList(value)
		layout.Layouts = append(layout.Layouts, refs...)
		layout.LayoutSpans = append(layout.LayoutSpans, namedValueSpans(refs, lineNumber, rawLine)...)
		if (layout.Span == source.SourceSpan{}) {
			layout.Span = sourceLineSpan(lineNumber, rawLine)
		}
	case "error":
		errorPage, err := source.ErrorPagePath(trimQuotes(value))
		if err != nil {
			return err
		}
		layout.ErrorPage = errorPage
		layout.ErrorPageSpan = sourceLineSpan(lineNumber, rawLine)
	default:
		return lineDiagnosticError(DiagnosticUnsupportedLayoutMetadata, lineNumber, rawLine, "unsupported metadata %s", name)
	}
	return nil
}

func applyComponentMetadata(component *gwdkir.Component, name, rawValue string, lineNumber int, rawLine string) error {
	value := strings.TrimSpace(rawValue)
	switch name {
	case "component":
		if value == "" {
			return fmt.Errorf("component requires a value")
		}
		component.Name = value
		component.Span = sourceLineSpan(lineNumber, rawLine)
	case "wasm":
		if value == "" {
			return fmt.Errorf("wasm requires a package path")
		}
		component.WASM = gwdkir.WASMContract{
			Package: trimQuotes(value),
			Span:    sourceLineSpan(lineNumber, rawLine),
		}
	case "css":
		if value == "" {
			return fmt.Errorf("css requires a value")
		}
		component.CSS = splitCSSList(value)
		component.Spans.CSS = namedValueSpans(component.CSS, lineNumber, rawLine)
	case "asset":
		if value == "" {
			return fmt.Errorf("asset requires a value")
		}
		component.Assets = splitCSSList(value)
		component.Spans.Assets = namedValueSpans(component.Assets, lineNumber, rawLine)
	default:
		return fmt.Errorf("unsupported metadata %s", name)
	}
	return nil
}
