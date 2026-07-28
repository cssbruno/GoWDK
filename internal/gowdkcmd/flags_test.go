package gowdkcmd

import (
	"strings"
	"testing"
)

func TestConsumeValueFlag(t *testing.T) {
	tests := []struct {
		name             string
		args             []string
		index            int
		allowEmptyEquals bool
		wantValue        string
		wantNext         int
		wantOK           bool
		wantMissing      bool
	}{
		{
			name:        "space value",
			args:        []string{"--config", "gowdk.config.go"},
			wantValue:   "gowdk.config.go",
			wantNext:    1,
			wantOK:      true,
			wantMissing: false,
		},
		{
			name:        "equals value",
			args:        []string{"--config=gowdk.config.go"},
			wantValue:   "gowdk.config.go",
			wantNext:    0,
			wantOK:      true,
			wantMissing: false,
		},
		{
			name:        "missing space value",
			args:        []string{"--config"},
			wantNext:    0,
			wantOK:      true,
			wantMissing: true,
		},
		{
			name:             "empty equals accepted",
			args:             []string{"--config="},
			allowEmptyEquals: true,
			wantNext:         0,
			wantOK:           true,
		},
		{
			name:             "empty equals missing when disallowed",
			args:             []string{"--app="},
			allowEmptyEquals: false,
			wantNext:         0,
			wantOK:           true,
			wantMissing:      true,
		},
		{
			name:        "next flag is not a value",
			args:        []string{"--config", "--project-root", "."},
			wantNext:    0,
			wantOK:      true,
			wantMissing: true,
		},
		{
			name:        "equals dash value rejected by default",
			args:        []string{"--config=-local"},
			wantNext:    0,
			wantOK:      true,
			wantMissing: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, next, ok, missing := consumeValueFlag(tt.args, tt.index, "--config", tt.allowEmptyEquals)
			if tt.name == "empty equals missing when disallowed" {
				value, next, ok, missing = consumeValueFlag(tt.args, tt.index, "--app", tt.allowEmptyEquals)
			}
			if value != tt.wantValue || next != tt.wantNext || ok != tt.wantOK || missing != tt.wantMissing {
				t.Fatalf("consumeValueFlag = (%q, %d, %v, %v), want (%q, %d, %v, %v)", value, next, ok, missing, tt.wantValue, tt.wantNext, tt.wantOK, tt.wantMissing)
			}
		})
	}
}

func TestConsumeValueFlagAllowsExplicitDashValuePolicy(t *testing.T) {
	value, next, ok, missing := consumeValueFlagWithPolicy(
		[]string{"--run=-integration"},
		0,
		"--run",
		valueFlagPolicy{AllowDashValue: true},
	)
	if value != "-integration" || next != 0 || !ok || missing {
		t.Fatalf("consumeValueFlagWithPolicy = (%q, %d, %v, %v)", value, next, ok, missing)
	}
}

func TestCommandValueFlagsRejectFollowingFlag(t *testing.T) {
	tests := []struct {
		name     string
		flag     string
		parseErr func() error
	}{
		{
			name: "build",
			flag: "--out",
			parseErr: func() error {
				_, err := parseBuildOptions([]string{"--out", "--app", "app"})
				return err
			},
		},
		{
			name: "check",
			flag: "--config",
			parseErr: func() error {
				_, _, _, _, err := parseProjectOptions([]string{"--config", "--ssr"}, "check", true)
				return err
			},
		},
		{
			name: "serve",
			flag: "--dir",
			parseErr: func() error {
				_, _, err := parseServeOptions([]string{"--dir", "--addr", "127.0.0.1:8080"})
				return err
			},
		},
		{
			name: "audit",
			flag: "--diff",
			parseErr: func() error {
				_, _, err := parseAuditCommandOptions([]string{"--diff", "--json"})
				return err
			},
		},
		{
			name: "test",
			flag: "--stage",
			parseErr: func() error {
				_, err := parseTestOptions([]string{"--stage", "--json"})
				return err
			},
		},
		{
			name: "clean",
			flag: "--out",
			parseErr: func() error {
				return clean([]string{"--out", "--dry-run"})
			},
		},
		{
			name: "dev",
			flag: "--addr",
			parseErr: func() error {
				_, err := parseDevOptions([]string{"--addr", "--interval", "1s"})
				return err
			},
		},
		{
			name: "preview",
			flag: "--out",
			parseErr: func() error {
				_, err := parsePreviewOptions([]string{"--out", "--hot"})
				return err
			},
		},
		{
			name: "preview forwarded build flag",
			flag: "--config",
			parseErr: func() error {
				options, err := parsePreviewOptions([]string{"--config", "--ssr"})
				if err != nil {
					return err
				}
				_, err = parseBuildOptions(options.BuildArgs)
				return err
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.parseErr()
			if err == nil || !strings.Contains(err.Error(), test.flag+" requires a value") {
				t.Fatalf("expected missing %s diagnostic, got %v", test.flag, err)
			}
		})
	}
}
