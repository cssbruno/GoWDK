package addonregistry

import (
	"strconv"
	"strings"
)

// VersionSupport is the result of checking a registry entry's declared
// GOWDK-version bounds against a concrete CLI version.
type VersionSupport int

const (
	// VersionUnknown means the bounds or the queried version could not be
	// compared (an unset or unparseable bound), so compatibility cannot be
	// proven either way. Callers should warn, not hard-fail.
	VersionUnknown VersionSupport = iota
	// VersionSupported means the queried version is within the declared bounds.
	VersionSupported
	// VersionUnsupported means the queried version is below MinGOWDK or above
	// MaxGOWDK.
	VersionUnsupported
)

// SupportsVersion reports whether the queried GOWDK version satisfies the
// entry's declared [MinGOWDK, MaxGOWDK] bounds. This is the version handshake:
// an addon declares the versions it targets and tooling can check a concrete
// CLI version against it before wiring or trusting the addon.
//
// Bounds are inclusive. An unset bound is open on that side. If either relevant
// bound or the queried version cannot be parsed as a dotted numeric version,
// the result is VersionUnknown so a caller warns rather than wrongly blocking.
func (e Entry) SupportsVersion(version string) VersionSupport {
	target, ok := parseVersion(version)
	if !ok {
		return VersionUnknown
	}

	if strings.TrimSpace(e.MinGOWDK) != "" {
		minimum, ok := parseVersion(e.MinGOWDK)
		if !ok {
			return VersionUnknown
		}
		if compareVersion(target, minimum) < 0 {
			return VersionUnsupported
		}
	}
	if strings.TrimSpace(e.MaxGOWDK) != "" {
		maximum, ok := parseVersion(e.MaxGOWDK)
		if !ok {
			return VersionUnknown
		}
		if compareVersion(target, maximum) > 0 {
			return VersionUnsupported
		}
	}
	return VersionSupported
}

// UnsupportedFor returns the entries whose declared version bounds exclude the
// queried version. Entries with unknown (unparseable or unset) bounds are not
// reported, because their incompatibility cannot be proven.
func (r Registry) UnsupportedFor(version string) []Entry {
	var unsupported []Entry
	for _, entry := range r.Addons {
		if entry.SupportsVersion(version) == VersionUnsupported {
			unsupported = append(unsupported, entry)
		}
	}
	return unsupported
}

// parseVersion parses a "major.minor.patch" string, tolerating a leading "v"
// and a trailing pre-release/build suffix (for example "1.2.0-rc1"). Missing
// minor/patch components default to zero.
func parseVersion(version string) ([3]int, bool) {
	trimmed := strings.TrimSpace(version)
	trimmed = strings.TrimPrefix(trimmed, "v")
	if trimmed == "" {
		return [3]int{}, false
	}
	// Drop any pre-release/build metadata so "0.5.0-rc1" compares as 0.5.0.
	if cut := strings.IndexAny(trimmed, "-+"); cut >= 0 {
		trimmed = trimmed[:cut]
	}
	parts := strings.Split(trimmed, ".")
	if len(parts) > 3 {
		return [3]int{}, false
	}
	var out [3]int
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return [3]int{}, false
		}
		out[i] = n
	}
	return out, true
}

// compareVersion returns -1, 0, or 1 as a is less than, equal to, or greater
// than b.
func compareVersion(a, b [3]int) int {
	for i := 0; i < 3; i++ {
		switch {
		case a[i] < b[i]:
			return -1
		case a[i] > b[i]:
			return 1
		}
	}
	return 0
}
