package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/inspectreport"
	"github.com/cssbruno/gowdk/internal/lang"
)

const inspectUsage = "usage: gowdk inspect ir|tree|endpoint-graph [--config <file>] [--module <name>] [--json] [--ssr] [files...]"

func inspect(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf(inspectUsage)
	}
	switch args[0] {
	case "ir":
		return inspectIR(args[1:])
	case "tree":
		return inspectTree(args[1:])
	case "endpoint-graph":
		return inspectEndpointGraph(args[1:])
	default:
		return fmt.Errorf("unknown inspect target %q", args[0])
	}
}

func inspectIR(args []string) error {
	_, ir, err := inspectProgram(args, "inspect ir")
	if err != nil {
		return err
	}
	payload, err := json.MarshalIndent(ir, "", "  ")
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(append(payload, '\n'))
	return err
}

func inspectTree(args []string) error {
	_, ir, err := inspectProgram(args, "inspect tree")
	if err != nil {
		return err
	}
	return writeInspectJSON(inspectreport.BuildTree(ir))
}

func inspectEndpointGraph(args []string) error {
	options, ir, err := inspectProgram(args, "inspect endpoint-graph")
	if err != nil {
		return err
	}
	return writeInspectJSON(inspectreport.BuildEndpointGraph(options.Config, ir))
}

func inspectProgram(args []string, command string) (cliOptions, gwdkir.Program, error) {
	options, paths, err := loadCommandInputs(args, command, true)
	if err != nil {
		return cliOptions{}, gwdkir.Program{}, err
	}

	checked, diagnostics := lang.CheckFilesWithOptions(options.Config, paths, lang.CheckOptions{ProjectRoot: options.ProjectRoot})
	for _, diagnostic := range diagnostics {
		fmt.Fprintln(os.Stderr, diagnostic.String())
	}
	if diagnostics.HasErrors() {
		return cliOptions{}, gwdkir.Program{}, fmt.Errorf("%s failed", command)
	}

	ir := checked.IR
	if err := linkIRContractReferences(&ir, options.ProjectRoot); err != nil {
		return cliOptions{}, gwdkir.Program{}, err
	}
	return options, ir, nil
}

func writeInspectJSON(report any) error {
	payload, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(append(payload, '\n'))
	return err
}
