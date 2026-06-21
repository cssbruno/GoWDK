package trace

import (
	"path"
	"strings"
	"sync/atomic"
)

// SourcePathMode controls how SourceRef.File paths are exposed outside the
// process (trace viewer, JSON/SSE, console, and OTLP export).
type SourcePathMode int

const (
	// SourcePathRelative reduces absolute filesystem paths to project-relative
	// logical paths so local filesystem structure is not revealed to trace
	// consumers. It is the default and the only production-safe mode.
	SourcePathRelative SourcePathMode = iota
	// SourcePathAbsolute exposes source paths verbatim, including absolute local
	// filesystem paths. It is a debugging aid for local development and must not
	// be enabled when traces leave the machine.
	SourcePathAbsolute
)

// SourcePolicy controls source-path normalization applied to a span snapshot's
// SourceRef.File before the snapshot is exposed outside the process.
type SourcePolicy struct {
	// Mode selects relative (default) or absolute path exposure.
	Mode SourcePathMode
	// ProjectRoot, when set, is the directory that absolute source paths under
	// it are made relative to in SourcePathRelative mode.
	ProjectRoot string
}

var sourcePolicy atomic.Pointer[SourcePolicy]

// SetSourcePolicy installs the process-wide source-path normalization policy.
// It is applied to every span snapshot before it is exposed through the trace
// viewer, JSON/SSE, console, or OTLP surfaces. The zero SourcePolicy (relative
// mode, no project root) is the default and never exposes absolute paths.
func SetSourcePolicy(policy SourcePolicy) {
	sourcePolicy.Store(&policy)
}

// CurrentSourcePolicy returns the active source-path normalization policy.
func CurrentSourcePolicy() SourcePolicy {
	if policy := sourcePolicy.Load(); policy != nil {
		return *policy
	}
	return SourcePolicy{}
}

// NormalizeSourceFile applies policy to a single SourceRef.File value.
//
// In the default relative mode it never returns an absolute path: Windows drive
// letters and UNC prefixes are stripped, the path is made relative to
// policy.ProjectRoot when it lies beneath it, leading separators are removed,
// and leading parent-directory ("..") segments are collapsed so the logical
// path cannot escape the project root. In absolute mode the path is returned
// cleaned but otherwise unchanged.
func NormalizeSourceFile(file string, policy SourcePolicy) string {
	if file == "" {
		return ""
	}
	// Normalize separators ourselves rather than with filepath.ToSlash so a
	// Windows-style path is reduced the same way on every host OS.
	slashed := strings.ReplaceAll(file, `\`, "/")
	if policy.Mode == SourcePathAbsolute {
		return path.Clean(slashed)
	}
	return reduceToProjectRelative(slashed, policy.ProjectRoot)
}

func reduceToProjectRelative(slashed, root string) string {
	// Make the path relative to the project root when it lies beneath it.
	if root != "" {
		cleanRoot := path.Clean(strings.ReplaceAll(root, `\`, "/"))
		clean := path.Clean(slashed)
		if clean == cleanRoot {
			return "."
		}
		if rel := strings.TrimPrefix(clean, cleanRoot+"/"); rel != clean {
			return rel
		}
	}
	trimmed := slashed
	// Drop a Windows drive prefix such as "C:" so "C:/Users/..." → "/Users/...".
	if len(trimmed) >= 2 && trimmed[1] == ':' && isASCIILetter(trimmed[0]) {
		trimmed = trimmed[2:]
	}
	// Force a relative path: strip leading separators (covers absolute POSIX
	// paths and UNC "//host/share" forms).
	trimmed = strings.TrimLeft(trimmed, "/")
	trimmed = path.Clean(trimmed)
	// Collapse any leading traversal so the logical path stays inside the root.
	for strings.HasPrefix(trimmed, "../") {
		trimmed = trimmed[len("../"):]
	}
	switch trimmed {
	case "", ".", "..":
		return ""
	}
	return trimmed
}

func isASCIILetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// normalizeSnapshotSource returns snapshot with its SourceRef.File normalized
// per the active policy. It is applied at the points where a snapshot is born
// (span completion and external ingest) so the viewer, JSON/SSE, console, and
// OTLP surfaces all observe the same normalized value.
func normalizeSnapshotSource(snapshot Snapshot) Snapshot {
	snapshot.Source.File = NormalizeSourceFile(snapshot.Source.File, CurrentSourcePolicy())
	return snapshot
}
