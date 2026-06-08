package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/cssbruno/gowdk/internal/gwdkanalysis"
	"github.com/cssbruno/gowdk/internal/lang"
)

const inspectIRUsage = "usage: gowdk inspect ir [--config <file>] [--module <name>] [--ssr] [files...]"

func inspect(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf(inspectIRUsage)
	}
	switch args[0] {
	case "ir":
		return inspectIR(args[1:])
	default:
		return fmt.Errorf("unknown inspect target %q", args[0])
	}
}

func inspectIR(args []string) error {
	options, paths, err := loadCommandInputs(args, "inspect ir", false)
	if err != nil {
		return err
	}

	app, diagnostics := lang.CheckFiles(options.Config, paths)
	for _, diagnostic := range diagnostics {
		fmt.Fprintln(os.Stderr, diagnostic.String())
	}
	if diagnostics.HasErrors() {
		return fmt.Errorf("inspect ir failed")
	}

	ir := gwdkanalysis.BuildIR(options.Config, app)
	if err := linkIRContractReferences(&ir, "."); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(ir, "", "  ")
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(append(payload, '\n'))
	return err
}
