package playground

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	DefaultMaxFiles      = 128
	DefaultMaxFileBytes  = 256 * 1024
	DefaultMaxTotalBytes = 2 * 1024 * 1024
)

type Policy struct {
	HostedExecutionEnabled bool     `json:"hostedExecutionEnabled"`
	Workspace              string   `json:"workspace"`
	Network                string   `json:"network"`
	Filesystem             string   `json:"filesystem"`
	Persistence            string   `json:"persistence"`
	VersionPinning         string   `json:"versionPinning"`
	Export                 string   `json:"export"`
	Limits                 Limits   `json:"limits"`
	Environment            []string `json:"environment"`
	RequiredAbuseControls  []string `json:"requiredAbuseControls"`
}

type Limits struct {
	WallClockSeconds int   `json:"wallClockSeconds"`
	MaxFiles         int   `json:"maxFiles"`
	MaxFileBytes     int64 `json:"maxFileBytes"`
	MaxTotalBytes    int64 `json:"maxTotalBytes"`
	MaxOutputBytes   int64 `json:"maxOutputBytes"`
}

type Options struct {
	MaxFiles      int
	MaxFileBytes  int64
	MaxTotalBytes int64
}

type File struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

type ExportResult struct {
	Archive string `json:"archive"`
	Files   []File `json:"files"`
}

type Workspace struct {
	Root  string
	Files []File
}

func DefaultPolicy() Policy {
	return Policy{
		HostedExecutionEnabled: false,
		Workspace:              "isolated disposable workspace copied from allowed project files",
		Network:                "disabled by default; hosted runners must keep GOPROXY=off and block outbound access",
		Filesystem:             "empty workspace only; repository secrets, host credentials, generated output, and private files are not mounted",
		Persistence:            "none unless the user explicitly exports a project archive",
		VersionPinning:         "hosted runners must pin the GOWDK binary version for the session",
		Export:                 "ordinary GOWDK source project archive without .gowdk, dist, bin, env files, secrets, or generated reports",
		Limits: Limits{
			WallClockSeconds: 20,
			MaxFiles:         DefaultMaxFiles,
			MaxFileBytes:     DefaultMaxFileBytes,
			MaxTotalBytes:    DefaultMaxTotalBytes,
			MaxOutputBytes:   2 * 1024 * 1024,
		},
		Environment: []string{
			"only PATH and isolated Go cache variables are inherited or synthesized",
			"secret-looking environment variables are rejected",
			"GOPROXY=off",
			"GOSUMDB=off",
			"GOWORK=off",
		},
		RequiredAbuseControls: []string{
			"per-session rate limits",
			"audit logs without submitted secrets",
			"workspace cleanup on success and failure",
			"bounded stdout/stderr capture with redaction",
		},
	}
}

func PolicyJSON(policy Policy) ([]byte, error) {
	return json.MarshalIndent(policy, "", "  ")
}

func ExportArchive(sourceDir string, archivePath string, options Options) (ExportResult, error) {
	files, err := CollectFiles(sourceDir, options)
	if err != nil {
		return ExportResult{}, err
	}
	if strings.TrimSpace(archivePath) == "" {
		return ExportResult{}, fmt.Errorf("playground export requires --out <file.zip>")
	}
	if archiveDir := filepath.Dir(archivePath); archiveDir != "." {
		if err := os.MkdirAll(archiveDir, 0o755); err != nil {
			return ExportResult{}, err
		}
	}
	file, err := os.Create(archivePath)
	if err != nil {
		return ExportResult{}, err
	}
	defer file.Close()
	writer := zip.NewWriter(file)
	for _, item := range files {
		if err := writeZipFile(writer, sourceDir, item.Path); err != nil {
			writer.Close()
			return ExportResult{}, err
		}
	}
	if err := writer.Close(); err != nil {
		return ExportResult{}, err
	}
	return ExportResult{Archive: archivePath, Files: files}, nil
}

func StageWorkspace(sourceDir string, options Options) (Workspace, func() error, error) {
	files, err := CollectFiles(sourceDir, options)
	if err != nil {
		return Workspace{}, nil, err
	}
	root, err := os.MkdirTemp("", "gowdk-playground-*")
	if err != nil {
		return Workspace{}, nil, err
	}
	cleanup := func() error { return os.RemoveAll(root) }
	for _, item := range files {
		sourcePath := filepath.Join(sourceDir, filepath.FromSlash(item.Path))
		targetPath := filepath.Join(root, filepath.FromSlash(item.Path))
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			cleanup()
			return Workspace{}, nil, err
		}
		if err := copyFile(sourcePath, targetPath); err != nil {
			cleanup()
			return Workspace{}, nil, err
		}
	}
	return Workspace{Root: root, Files: files}, cleanup, nil
}

func CollectFiles(sourceDir string, options Options) ([]File, error) {
	root, err := filepath.Abs(sourceDir)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("playground source must be a directory")
	}
	options = normalizeOptions(options)
	var files []File
	var total int64
	if err := filepath.WalkDir(root, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, current)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			if UnsafeExportDirectory(rel) {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeType != 0 {
			return nil
		}
		if UnsafeExportFile(rel) {
			return nil
		}
		if info.Size() > options.MaxFileBytes {
			return fmt.Errorf("playground file %s is %d bytes; max is %d", rel, info.Size(), options.MaxFileBytes)
		}
		total += info.Size()
		if total > options.MaxTotalBytes {
			return fmt.Errorf("playground source is %d bytes; max is %d", total, options.MaxTotalBytes)
		}
		files = append(files, File{Path: rel, Size: info.Size()})
		if len(files) > options.MaxFiles {
			return fmt.Errorf("playground source has more than %d files", options.MaxFiles)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	if !hasFile(files, "gowdk.config.go") {
		return nil, fmt.Errorf("playground export requires gowdk.config.go")
	}
	return files, nil
}

func UnsafeExportDirectory(rel string) bool {
	base := strings.ToLower(path.Base(filepath.ToSlash(rel)))
	switch base {
	case ".git", ".hg", ".svn", ".gowdk", "gowdk_cache", "dist", "bin", "node_modules", "vendor", "tmp", "temp", ".tmp", "private", ".private", "secrets", ".secrets":
		return true
	default:
		return false
	}
}

func UnsafeExportFile(rel string) bool {
	rel = filepath.ToSlash(rel)
	base := strings.ToLower(path.Base(rel))
	ext := path.Ext(base)
	switch {
	case base == ".env" || strings.HasPrefix(base, ".env."):
		return true
	case base == ".npmrc" || base == ".netrc":
		return true
	case base == "gowdk-security.json" || base == "gowdk-build-report.json" || base == "gowdk-build-timings.json":
		return true
	case privateKeyFile(base):
		return true
	case ext == ".key" || ext == ".pem" || ext == ".p12" || ext == ".pfx":
		return true
	case ext == ".tmp" || ext == ".temp" || strings.HasSuffix(base, "~"):
		return true
	case strings.HasSuffix(base, ".swp") || strings.HasSuffix(base, ".swo"):
		return true
	default:
		return false
	}
}

func SanitizedEnvironment(cacheRoot string) []string {
	pathValue := os.Getenv("PATH")
	env := []string{
		"PATH=" + pathValue,
		"HOME=" + cacheRoot,
		"GOCACHE=" + filepath.Join(cacheRoot, "go-build"),
		"GOMODCACHE=" + filepath.Join(cacheRoot, "mod"),
		"GOPROXY=off",
		"GOSUMDB=off",
		"GOWORK=off",
	}
	return env
}

func SecretLikeEnvName(name string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(name, "-", "_"))
	for _, needle := range []string{"secret", "token", "password", "passwd", "private_key", "apikey", "api_key", "credential", "session"} {
		if strings.Contains(normalized, needle) {
			return true
		}
	}
	return false
}

func RejectSecretEnvironment(env []string) error {
	for _, item := range env {
		name, _, ok := strings.Cut(item, "=")
		if !ok {
			name = item
		}
		if SecretLikeEnvName(name) {
			return fmt.Errorf("playground environment variable %s looks like a secret and is not allowed", name)
		}
	}
	return nil
}

func ExecutionTimeout() time.Duration {
	return time.Duration(DefaultPolicy().Limits.WallClockSeconds) * time.Second
}

func normalizeOptions(options Options) Options {
	if options.MaxFiles <= 0 {
		options.MaxFiles = DefaultMaxFiles
	}
	if options.MaxFileBytes <= 0 {
		options.MaxFileBytes = DefaultMaxFileBytes
	}
	if options.MaxTotalBytes <= 0 {
		options.MaxTotalBytes = DefaultMaxTotalBytes
	}
	return options
}

func hasFile(files []File, name string) bool {
	for _, file := range files {
		if file.Path == name {
			return true
		}
	}
	return false
}

func writeZipFile(writer *zip.Writer, sourceDir string, rel string) error {
	sourcePath := filepath.Join(sourceDir, filepath.FromSlash(rel))
	info, err := os.Stat(sourcePath)
	if err != nil {
		return err
	}
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = filepath.ToSlash(rel)
	header.Method = zip.Deflate
	target, err := writer.CreateHeader(header)
	if err != nil {
		return err
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()
	_, err = io.Copy(target, source)
	return err
}

func copyFile(sourcePath string, targetPath string) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()
	target, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer target.Close()
	_, err = io.Copy(target, source)
	return err
}

func privateKeyFile(base string) bool {
	switch strings.ToLower(base) {
	case "id_rsa", "id_dsa", "id_ecdsa", "id_ed25519":
		return true
	default:
		return false
	}
}
