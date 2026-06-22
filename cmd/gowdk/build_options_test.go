package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
)

func TestBuildRequestHasAdHocArgs(t *testing.T) {
	tests := []struct {
		name    string
		request buildRequest
		want    bool
	}{
		{name: "empty"},
		{name: "output", request: buildRequest{OutputDir: "dist"}, want: true},
		{name: "app", request: buildRequest{AppDir: "app"}, want: true},
		{name: "binary", request: buildRequest{BinaryPath: "bin/site"}, want: true},
		{name: "wasm", request: buildRequest{WASMPath: "bin/site.wasm"}, want: true},
		{name: "backend app", request: buildRequest{BackendAppDir: "backend"}, want: true},
		{name: "backend binary", request: buildRequest{BackendBinaryPath: "bin/backend"}, want: true},
		{name: "docker", request: buildRequest{Docker: true}, want: true},
		{name: "docker base", request: buildRequest{DockerBase: "scratch"}, want: true},
		{name: "deploy recipe", request: buildRequest{DeployRecipes: []string{"caddy"}}, want: true},
		{name: "modules", request: buildRequest{Modules: []string{"site"}}, want: true},
		{name: "paths", request: buildRequest{Paths: []string{"home.page.gwdk"}}, want: true},
		{name: "timings path is not build input", request: buildRequest{TimingsPath: "timings.json"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.request.hasAdHocArgs(); got != tt.want {
				t.Fatalf("hasAdHocArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildOptionsShouldBuildConfiguredTargets(t *testing.T) {
	targetConfig := gowdk.Config{Build: gowdk.BuildConfig{Targets: []gowdk.BuildTargetConfig{{Name: "site"}}}}
	tests := []struct {
		name string
		plan buildOptions
		want bool
	}{
		{name: "configured targets without ad hoc args", plan: buildOptions{Options: cliOptions{Config: targetConfig}}, want: true},
		{name: "explicit target", plan: buildOptions{TargetNames: []string{"site"}}, want: true},
		{name: "no configured targets", plan: buildOptions{}, want: false},
		{name: "ad hoc output", plan: buildOptions{Options: cliOptions{Config: targetConfig}, OutputDir: "dist"}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.plan.shouldBuildConfiguredTargets(); got != tt.want {
				t.Fatalf("shouldBuildConfiguredTargets() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResolveConfiguredBuildTargets(t *testing.T) {
	targets := []gowdk.BuildTargetConfig{
		{Name: " site ", Output: "out/site"},
		{Name: "admin", Output: "out/admin"},
	}
	selected, err := resolveConfiguredBuildTargets(targets, []string{" admin ", "site"})
	if err != nil {
		t.Fatal(err)
	}
	if got := targetNames(selected); !reflect.DeepEqual(got, []string{"admin", "site"}) {
		t.Fatalf("selected names = %#v", got)
	}
	if selected[0].Output != "out/admin" || selected[1].Output != "out/site" {
		t.Fatalf("selected targets = %#v, want requested order with original target fields", selected)
	}

	all, err := resolveConfiguredBuildTargets(targets, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := targetNames(all); !reflect.DeepEqual(got, []string{"site", "admin"}) {
		t.Fatalf("all names = %#v, want configured order", got)
	}
}

func TestResolveConfiguredBuildTargetsRejectsDuplicateAndUnknownNames(t *testing.T) {
	if _, err := resolveConfiguredBuildTargets([]gowdk.BuildTargetConfig{{Name: "site"}, {Name: " site "}}, nil); err == nil || !strings.Contains(err.Error(), `build target "site" is configured more than once`) {
		t.Fatalf("duplicate target error = %v", err)
	}
	if _, err := resolveConfiguredBuildTargets([]gowdk.BuildTargetConfig{{Name: "site"}}, []string{"missing"}); err == nil || !strings.Contains(err.Error(), `build target "missing" is not configured`) {
		t.Fatalf("unknown target error = %v", err)
	}
}

func TestSelectBuildTargetsAppliesBuildOnlyDefaultsAndValidation(t *testing.T) {
	targets, err := selectBuildTargets([]gowdk.BuildTargetConfig{{
		Name:    " site ",
		Modules: []string{" app ", ""},
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 1 {
		t.Fatalf("targets = %#v, want one", targets)
	}
	target := targets[0]
	if target.Name != "site" || target.Output != ".gowdk/output/site" || !reflect.DeepEqual(target.Modules, []string{"app"}) {
		t.Fatalf("normalized build target = %#v", target)
	}
	if _, err := selectBuildTargets([]gowdk.BuildTargetConfig{{Name: "site", Binary: "bin/site"}}, nil); err == nil || !strings.Contains(err.Error(), "binary requires app") {
		t.Fatalf("binary without app error = %v", err)
	}
}

func targetNames(targets []gowdk.BuildTargetConfig) []string {
	names := make([]string, 0, len(targets))
	for _, target := range targets {
		names = append(names, target.Name)
	}
	return names
}
