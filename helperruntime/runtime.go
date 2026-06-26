// Package helperruntime runs project-scoped gowdk commands inside a generated
// helper that imports the user's config package.
package helperruntime

import (
	"fmt"
	"os"
	"strconv"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gowdkcmd"
)

const (
	ProtocolMin = 1
	ProtocolMax = 1

	GOWDKVersion = "0.12.0" // x-release-please-version
)

type Options struct {
	Config      *gowdk.Config
	ProjectRoot string
}

func Main(options Options) {
	if options.Config == nil {
		fatal(2, "missing gowdk.Config")
	}
	handshake()
	if len(os.Args) < 2 {
		fatal(2, "missing helper command")
	}
	if err := gowdkcmd.RunWithConfig(os.Args[1:], options.Config, options.ProjectRoot); err != nil {
		if _, silent := err.(interface{ SilentCLIError() }); !silent {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(gowdkcmd.ExitCodeFor(err))
	}
}

func handshake() int {
	cliMin := envInt("GOWDK_HELPER_PROTOCOL_MIN", -1)
	cliMax := envInt("GOWDK_HELPER_PROTOCOL_MAX", -1)
	cliVersion := os.Getenv("GOWDK_CLI_VERSION")
	if cliMin < 0 || cliMax < 0 {
		fatal(2, "missing GOWDK helper protocol environment")
	}

	selected := min(cliMax, ProtocolMax)
	if selected < max(cliMin, ProtocolMin) {
		fatal(
			2,
			"GOWDK helper protocol mismatch\n\nCLI supports protocol v%d..v%d.\nProject helper supports protocol v%d..v%d.\nCLI GOWDK version: %s\nProject GOWDK version: %s",
			cliMin,
			cliMax,
			ProtocolMin,
			ProtocolMax,
			cliVersion,
			GOWDKVersion,
		)
	}
	return selected
}

func fatal(code int, format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(code)
}

func envInt(name string, fallback int) int {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return n
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
