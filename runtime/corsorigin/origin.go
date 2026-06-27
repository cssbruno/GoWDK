// Package corsorigin validates and canonicalizes CORS origin strings.
package corsorigin

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// Origin is a canonical HTTP origin.
type Origin struct {
	Scheme string
	Host   string
	Port   string
}

// Parse validates a literal HTTP origin and returns its canonical form.
func Parse(raw string) (Origin, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return Origin{}, fmt.Errorf("origin is required")
	}
	if strings.ContainsAny(value, "\r\n") {
		return Origin{}, fmt.Errorf("origin %q contains a control character", raw)
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return Origin{}, fmt.Errorf("origin %q is invalid: %w", raw, err)
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return Origin{}, fmt.Errorf("origin %q must use http or https", raw)
	}
	if parsed.User != nil || parsed.Host == "" || parsed.Opaque != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return Origin{}, fmt.Errorf("origin %q must not include userinfo, query, or fragment", raw)
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return Origin{}, fmt.Errorf("origin %q must not include a path", raw)
	}
	host, port, err := splitHostPort(parsed.Host)
	if err != nil {
		return Origin{}, fmt.Errorf("origin %q has invalid host or port: %w", raw, err)
	}
	host, err = canonicalHost(host)
	if err != nil {
		return Origin{}, fmt.Errorf("origin %q has invalid host: %w", raw, err)
	}
	if port != "" {
		if err := validatePort(port); err != nil {
			return Origin{}, fmt.Errorf("origin %q has invalid port %q: %w", raw, port, err)
		}
		if defaultPort(scheme) == port {
			port = ""
		}
	}
	return Origin{Scheme: scheme, Host: host, Port: port}, nil
}

func (origin Origin) String() string {
	host := origin.Host
	if origin.Port != "" {
		host = net.JoinHostPort(origin.Host, origin.Port)
	} else if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	return origin.Scheme + "://" + host
}

func splitHostPort(value string) (host string, port string, err error) {
	if strings.HasPrefix(value, "[") {
		end := strings.LastIndex(value, "]")
		if end < 0 {
			return "", "", fmt.Errorf("missing closing IPv6 bracket")
		}
		host = value[1:end]
		rest := value[end+1:]
		if rest == "" {
			return host, "", nil
		}
		if !strings.HasPrefix(rest, ":") {
			return "", "", fmt.Errorf("unexpected data after IPv6 host")
		}
		port = rest[1:]
		if port == "" {
			return "", "", fmt.Errorf("port is required after colon")
		}
		return host, port, nil
	}
	if strings.Count(value, ":") == 0 {
		return value, "", nil
	}
	if strings.Count(value, ":") > 1 {
		return "", "", fmt.Errorf("IPv6 hosts must use brackets")
	}
	host, port, ok := strings.Cut(value, ":")
	if !ok || host == "" || port == "" {
		return "", "", fmt.Errorf("host and port are required")
	}
	return host, port, nil
}

func canonicalHost(value string) (string, error) {
	host := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(value)), ".")
	if host == "" {
		return "", fmt.Errorf("host is required")
	}
	for _, r := range host {
		if r > 127 {
			return "", fmt.Errorf("unicode hostnames are not accepted; use an ASCII punycode hostname")
		}
	}
	if ip := net.ParseIP(host); ip != nil {
		return strings.ToLower(ip.String()), nil
	}
	if len(host) > 253 {
		return "", fmt.Errorf("hostname is too long")
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" {
			return "", fmt.Errorf("hostname contains an empty label")
		}
		if len(label) > 63 {
			return "", fmt.Errorf("hostname label %q is too long", label)
		}
		if strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return "", fmt.Errorf("hostname label %q must not start or end with hyphen", label)
		}
		for _, r := range label {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
				continue
			}
			return "", fmt.Errorf("hostname label %q contains invalid character %q", label, r)
		}
	}
	return host, nil
}

func validatePort(value string) error {
	for _, r := range value {
		if r < '0' || r > '9' {
			return fmt.Errorf("port must be numeric")
		}
	}
	number, err := strconv.Atoi(value)
	if err != nil || number < 1 || number > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	return nil
}

func defaultPort(scheme string) string {
	switch scheme {
	case "http":
		return "80"
	case "https":
		return "443"
	default:
		return ""
	}
}
