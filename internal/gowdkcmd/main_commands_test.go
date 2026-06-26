package gowdkcmd

import (
	"strings"
	"testing"
)

func TestTopLevelCommandDescriptorsDriveHelp(t *testing.T) {
	seen := map[string]bool{}
	for _, command := range topLevelCommands {
		if strings.TrimSpace(command.Name) == "" {
			t.Fatal("top-level command has empty name")
		}
		if seen[command.Name] {
			t.Fatalf("top-level command %q is registered more than once", command.Name)
		}
		seen[command.Name] = true
		if command.Handler == nil {
			t.Fatalf("top-level command %q has nil handler", command.Name)
		}
		if strings.TrimSpace(command.Usage()) == "" {
			t.Fatalf("top-level command %q has empty usage", command.Name)
		}
		if help, ok := commandUsage(command.Name); !ok || help != command.Usage() {
			t.Fatalf("commandUsage(%q) = %q, %v; want descriptor usage %q", command.Name, help, ok, command.Usage())
		}
	}
	if help, ok := commandUsage("missing"); ok || help != "" {
		t.Fatalf("commandUsage(missing) = %q, %v; want no help-only command", help, ok)
	}
}

func TestUsageListsRegisteredTopLevelCommands(t *testing.T) {
	stdout, stderr, err := captureCLIOutput(t, func() error {
		usage()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if stderr != "" {
		t.Fatalf("usage stderr = %q, want empty", stderr)
	}
	for _, command := range topLevelCommands {
		line := "  " + command.Name + command.ListSuffix
		if !strings.Contains(stdout, line) {
			t.Fatalf("usage output missing %q\n%s", line, stdout)
		}
	}
}
