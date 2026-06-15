package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/cssbruno/gowdk/internal/buildgen"
)

const (
	deployRecipeCaddy  = "caddy"
	deployRecipeNginx  = "nginx"
	deployRecipeSplit  = "split"
	deployRecipeStatic = "static"
	deployRecipeSystem = "systemd"
)

type deploymentRecipeRequest struct {
	OutputDir         string
	BinaryPath        string
	BackendBinaryPath string
	Recipes           []string
}

type deploymentRecipeArtifact struct {
	Kind string
	Path string
}

func normalizeDeploymentRecipes(values []string) ([]string, error) {
	cleaned := cleanNames(values)
	if len(cleaned) == 0 {
		return nil, nil
	}
	seen := map[string]bool{}
	var recipes []string
	for _, recipe := range cleaned {
		switch recipe {
		case deployRecipeCaddy, deployRecipeNginx, deployRecipeSplit, deployRecipeStatic, deployRecipeSystem:
		default:
			return nil, fmt.Errorf("unsupported deploy recipe %q; expected caddy, nginx, split, static, or systemd", recipe)
		}
		if seen[recipe] {
			continue
		}
		seen[recipe] = true
		recipes = append(recipes, recipe)
	}
	sort.Strings(recipes)
	return recipes, nil
}

func writeDeploymentRecipes(request deploymentRecipeRequest) ([]deploymentRecipeArtifact, error) {
	recipes, err := normalizeDeploymentRecipes(request.Recipes)
	if err != nil {
		return nil, err
	}
	var artifacts []deploymentRecipeArtifact
	for _, recipe := range recipes {
		var artifact deploymentRecipeArtifact
		switch recipe {
		case deployRecipeStatic:
			artifact, err = writeStaticRecipe(request.OutputDir)
		case deployRecipeSystem:
			artifact, err = writeSystemdRecipe(deploymentRecipeBinaryPath(request))
		case deployRecipeCaddy:
			artifact, err = writeCaddyRecipe(deploymentRecipeBinaryPath(request))
		case deployRecipeNginx:
			artifact, err = writeNginxRecipe(deploymentRecipeBinaryPath(request))
		case deployRecipeSplit:
			artifact, err = writeSplitRecipe(request.OutputDir, request.BackendBinaryPath)
		default:
			err = fmt.Errorf("unsupported deploy recipe %q; expected caddy, nginx, split, static, or systemd", recipe)
		}
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, artifact)
	}
	return artifacts, nil
}

func deploymentRecipeBuildEvents(artifacts []deploymentRecipeArtifact) []buildgen.BuildEvent {
	events := make([]buildgen.BuildEvent, 0, len(artifacts))
	for _, artifact := range artifacts {
		events = append(events, buildgen.BuildEvent{
			Level:   buildgen.BuildEventInfo,
			Stage:   "package",
			Kind:    "deploy_recipe_written",
			Message: "wrote optional deployment recipe",
			Path:    filepath.ToSlash(artifact.Path),
			Data: map[string]string{
				"recipe": artifact.Kind,
			},
		})
	}
	return events
}

func deploymentRecipeBinaryPath(request deploymentRecipeRequest) string {
	if strings.TrimSpace(request.BinaryPath) != "" {
		return request.BinaryPath
	}
	return request.BackendBinaryPath
}

func writeStaticRecipe(outputDir string) (deploymentRecipeArtifact, error) {
	if strings.TrimSpace(outputDir) == "" {
		return deploymentRecipeArtifact{}, fmt.Errorf("deploy recipe static requires --out <dir>")
	}
	path := filepath.Join(outputDir, "deploy", "static-host.md")
	payload := fmt.Sprintf(`# GOWDK Static Host Recipe

This file is a starting point, not a production guarantee.

Serve the generated files from:

%s

Required host behavior:

- Serve directory indexes such as /docs/ from index.html files.
- Keep /_gowdk/ paths reserved for generated runtime endpoints when a backend is also deployed.
- Do not serve gowdk-security.json if a copy exists outside the generated output.
- Own domains, TLS, CDN policy, cache invalidation, rollbacks, backups, and secrets in the deployment platform.

Local smoke test:

%s
`, outputDir, "gowdk serve --dir "+shellQuote(outputDir)+" --addr 127.0.0.1:8080")
	if err := writeDeploymentRecipeFile(path, payload); err != nil {
		return deploymentRecipeArtifact{}, err
	}
	return deploymentRecipeArtifact{Kind: deployRecipeStatic, Path: path}, nil
}

func writeSystemdRecipe(binaryPath string) (deploymentRecipeArtifact, error) {
	if strings.TrimSpace(binaryPath) == "" {
		return deploymentRecipeArtifact{}, fmt.Errorf("deploy recipe systemd requires --bin <file> or --backend-bin <file>")
	}
	unit := deploymentServiceName(binaryPath)
	path := filepath.Join(filepath.Dir(binaryPath), unit+".service")
	payload := fmt.Sprintf(`[Unit]
Description=GOWDK %s
After=network.target

[Service]
WorkingDirectory=%s
ExecStart=%s
Environment=GOWDK_ADDR=127.0.0.1:8080
Restart=on-failure
RestartSec=2s
User=gowdk
Group=gowdk
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=full

[Install]
WantedBy=multi-user.target

# Starting point only. Keep secrets in app-owned drop-ins, an environment file
# with correct filesystem permissions, or the host secret manager.
`, unit, filepath.ToSlash(filepath.Dir(binaryPath)), filepath.ToSlash(binaryPath))
	if err := writeDeploymentRecipeFile(path, payload); err != nil {
		return deploymentRecipeArtifact{}, err
	}
	return deploymentRecipeArtifact{Kind: deployRecipeSystem, Path: path}, nil
}

func writeCaddyRecipe(binaryPath string) (deploymentRecipeArtifact, error) {
	if strings.TrimSpace(binaryPath) == "" {
		return deploymentRecipeArtifact{}, fmt.Errorf("deploy recipe caddy requires --bin <file> or --backend-bin <file>")
	}
	path := filepath.Join(filepath.Dir(binaryPath), "Caddyfile")
	payload := `# GOWDK Caddy reverse-proxy recipe.
# Starting point only: replace the host, own TLS policy, and keep secrets out of this file.
example.com {
	reverse_proxy 127.0.0.1:8080
}
`
	if err := writeDeploymentRecipeFile(path, payload); err != nil {
		return deploymentRecipeArtifact{}, err
	}
	return deploymentRecipeArtifact{Kind: deployRecipeCaddy, Path: path}, nil
}

func writeNginxRecipe(binaryPath string) (deploymentRecipeArtifact, error) {
	if strings.TrimSpace(binaryPath) == "" {
		return deploymentRecipeArtifact{}, fmt.Errorf("deploy recipe nginx requires --bin <file> or --backend-bin <file>")
	}
	path := filepath.Join(filepath.Dir(binaryPath), "nginx.gowdk.conf")
	payload := `# GOWDK Nginx reverse-proxy recipe.
# Starting point only: replace server_name, own TLS policy, and keep secrets out of this file.
server {
	listen 80;
	server_name example.com;

	location / {
		proxy_set_header Host $host;
		proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
		proxy_set_header X-Forwarded-Proto $scheme;
		proxy_pass http://127.0.0.1:8080;
	}
}
`
	if err := writeDeploymentRecipeFile(path, payload); err != nil {
		return deploymentRecipeArtifact{}, err
	}
	return deploymentRecipeArtifact{Kind: deployRecipeNginx, Path: path}, nil
}

func writeSplitRecipe(outputDir string, backendBinaryPath string) (deploymentRecipeArtifact, error) {
	if strings.TrimSpace(outputDir) == "" || strings.TrimSpace(backendBinaryPath) == "" {
		return deploymentRecipeArtifact{}, fmt.Errorf("deploy recipe split requires --out <dir> and --backend-bin <file>")
	}
	path := filepath.Join(outputDir, "deploy", "split-frontend-backend.md")
	payload := fmt.Sprintf(`# GOWDK Split Frontend/Backend Recipe

This file is a starting point, not a production guarantee.

Frontend files:

%s

Backend binary:

%s

Deployment notes:

- Deploy the frontend files to a static host that can serve generated directory indexes.
- Run the backend binary with GOWDK_ADDR set to the private listener address.
- Point frontend proxy behavior at the backend with GOWDK_BACKEND_ORIGIN where the generated frontend binary is used.
- Keep domains, TLS, CDN policy, cache invalidation, secrets, storage, backups, health checks, and rollout strategy app-owned.

Smoke checks:

%s
%s
`, outputDir, backendBinaryPath, "gowdk serve --dir "+shellQuote(outputDir)+" --addr 127.0.0.1:8080", "GOWDK_ADDR=127.0.0.1:18086 "+shellQuote(backendBinaryPath))
	if err := writeDeploymentRecipeFile(path, payload); err != nil {
		return deploymentRecipeArtifact{}, err
	}
	return deploymentRecipeArtifact{Kind: deployRecipeSplit, Path: path}, nil
}

func writeDeploymentRecipeFile(path string, payload string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(payload), 0o644)
}

func deploymentServiceName(binaryPath string) string {
	name := strings.TrimSpace(filepath.Base(binaryPath))
	if name == "" || name == "." || name == string(filepath.Separator) {
		return "gowdk-app"
	}
	name = strings.TrimSuffix(name, filepath.Ext(name))
	if name == "" {
		return "gowdk-app"
	}
	return "gowdk-" + safeDeploymentName(name)
}

func safeDeploymentName(value string) string {
	var builder strings.Builder
	for _, char := range value {
		switch {
		case char >= 'A' && char <= 'Z':
			builder.WriteRune(char + ('a' - 'A'))
		case char >= 'a' && char <= 'z':
			builder.WriteRune(char)
		case char >= '0' && char <= '9':
			builder.WriteRune(char)
		case char == '-' || char == '_' || char == '.':
			builder.WriteRune('-')
		default:
			builder.WriteRune('-')
		}
	}
	name := strings.Trim(builder.String(), "-")
	if name == "" {
		return "app"
	}
	return name
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return strconv.Quote(value)
}
