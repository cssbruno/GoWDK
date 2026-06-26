package gowdkcmd

import (
	"debug/elf"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/cssbruno/gowdk/internal/buildgen"
)

const (
	dockerBaseDistroless = "distroless"
	dockerBaseScratch    = "scratch"
)

type dockerBinaryInfo struct {
	ELF    bool
	Static bool
}

type dockerArtifacts struct {
	DockerfilePath    string
	DockerignorePath  string
	Base              string
	RuntimeBinaryPath string
}

func normalizeDockerBase(value string) (string, error) {
	base := strings.TrimSpace(value)
	if base == "" {
		return dockerBaseDistroless, nil
	}
	switch base {
	case dockerBaseDistroless, dockerBaseScratch:
		return base, nil
	default:
		return "", fmt.Errorf("unsupported Docker base %q; expected distroless or scratch", value)
	}
}

func validateDockerBinary(base string, info dockerBinaryInfo) error {
	if !info.ELF {
		return fmt.Errorf("gowdk build --docker requires a Linux ELF binary; set GOOS=linux GOARCH=<arch> when building")
	}
	if base == dockerBaseScratch && !info.Static {
		return fmt.Errorf("gowdk build --docker-base scratch requires a statically linked Linux binary; set CGO_ENABLED=0 when building")
	}
	return nil
}

func writeDockerArtifacts(binaryPath string, baseValue string) (dockerArtifacts, error) {
	binaryPath = strings.TrimSpace(binaryPath)
	if binaryPath == "" {
		return dockerArtifacts{}, fmt.Errorf("gowdk build --docker requires --bin <file>")
	}
	base, err := normalizeDockerBase(baseValue)
	if err != nil {
		return dockerArtifacts{}, err
	}
	info, err := inspectDockerBinary(binaryPath)
	if err != nil {
		return dockerArtifacts{}, err
	}
	if err := validateDockerBinary(base, info); err != nil {
		return dockerArtifacts{}, err
	}

	dir := filepath.Dir(binaryPath)
	binaryName := filepath.Base(binaryPath)
	artifacts := dockerArtifacts{
		DockerfilePath:    filepath.Join(dir, "Dockerfile"),
		DockerignorePath:  filepath.Join(dir, ".dockerignore"),
		Base:              base,
		RuntimeBinaryPath: "/app/site",
	}
	if err := os.WriteFile(artifacts.DockerfilePath, []byte(dockerfilePayload(binaryName, base)), 0o644); err != nil {
		return dockerArtifacts{}, err
	}
	if err := os.WriteFile(artifacts.DockerignorePath, []byte(dockerignorePayload(binaryName)), 0o644); err != nil {
		return dockerArtifacts{}, err
	}
	return artifacts, nil
}

func inspectDockerBinary(path string) (dockerBinaryInfo, error) {
	file, err := elf.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return dockerBinaryInfo{}, fmt.Errorf("inspect Docker binary %q: %w", path, err)
		}
		return dockerBinaryInfo{}, fmt.Errorf("gowdk build --docker requires a Linux ELF binary; set GOOS=linux GOARCH=<arch> when building")
	}
	defer file.Close()

	info := dockerBinaryInfo{ELF: true, Static: true}
	for _, program := range file.Progs {
		if program.Type == elf.PT_INTERP || program.Type == elf.PT_DYNAMIC {
			info.Static = false
			break
		}
	}
	if libraries, err := file.ImportedLibraries(); err == nil && len(libraries) > 0 {
		info.Static = false
	}
	return info, nil
}

func dockerfilePayload(binaryName string, base string) string {
	from := "gcr.io/distroless/base-debian12"
	user := "nonroot:nonroot"
	if base == dockerBaseScratch {
		from = "scratch"
		user = "65532:65532"
	}
	return fmt.Sprintf(`FROM %s
WORKDIR /app
COPY [%s, "/app/site"]
ENV GOWDK_ADDR=0.0.0.0:8080
EXPOSE 8080
USER %s
ENTRYPOINT ["/app/site"]
`, from, strconv.Quote(binaryName), user)
}

func dockerignorePayload(binaryName string) string {
	return fmt.Sprintf(`*
!Dockerfile
!.dockerignore
!%s
`, binaryName)
}

func appendBuildReportEvents(reportPath string, events ...buildgen.BuildEvent) error {
	if strings.TrimSpace(reportPath) == "" || len(events) == 0 {
		return nil
	}
	payload, err := os.ReadFile(reportPath)
	if err != nil {
		return fmt.Errorf("append build report events: %w", err)
	}
	var report buildgen.BuildReport
	if err := json.Unmarshal(payload, &report); err != nil {
		return fmt.Errorf("append build report events: %w", err)
	}
	report.Events = append(report.Events, events...)
	payload, err = json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("append build report events: %w", err)
	}
	payload = append(payload, '\n')
	if err := os.WriteFile(reportPath, payload, 0o644); err != nil {
		return fmt.Errorf("append build report events: %w", err)
	}
	return nil
}
