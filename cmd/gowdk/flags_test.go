package main

import "testing"

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
			name:             "empty equals ignored when disallowed",
			args:             []string{"--app="},
			allowEmptyEquals: false,
			wantNext:         0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, next, ok, missing := consumeValueFlag(tt.args, tt.index, "--config", tt.allowEmptyEquals)
			if tt.name == "empty equals ignored when disallowed" {
				value, next, ok, missing = consumeValueFlag(tt.args, tt.index, "--app", tt.allowEmptyEquals)
			}
			if value != tt.wantValue || next != tt.wantNext || ok != tt.wantOK || missing != tt.wantMissing {
				t.Fatalf("consumeValueFlag = (%q, %d, %v, %v), want (%q, %d, %v, %v)", value, next, ok, missing, tt.wantValue, tt.wantNext, tt.wantOK, tt.wantMissing)
			}
		})
	}
}
