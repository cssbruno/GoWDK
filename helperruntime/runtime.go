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
		fatal("missing gowdk.Config")
	}
	handshake()
	if len(os.Args) < 2 {
		fatal("missing helper command")
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
		fatal("missing GOWDK helper protocol environment")
	}

	selected := minInt(cliMax, ProtocolMax)
	if selected < maxInt(cliMin, ProtocolMin) {
		fatal(
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

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(2)
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

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
