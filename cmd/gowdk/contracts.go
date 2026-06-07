package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/cssbruno/gowdk/internal/contractscan"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	runtimecontracts "github.com/cssbruno/gowdk/runtime/contracts"
)

func contractsReport(args []string) error {
	options, err := parseContractReportOptions(args)
	if err != nil {
		return err
	}
	report, err := contractscan.Scan(options.Dir)
	if err != nil {
		return err
	}
	return printContractReport(report, "", options.JSON)
}

func linkIRContractReferences(ir *gwdkir.Program, root string) error {
	report, err := contractscan.Scan(root)
	if err != nil {
		return err
	}
	if err := validateContractScanReport(report); err != nil {
		return err
	}
	if ir != nil && len(ir.ContractRefs) > 0 {
		ir.ContractRefs = contractscan.LinkReferences(ir.ContractRefs, report)
	}
	return nil
}

func validateContractScanReport(report contractscan.Report) error {
	if len(report.Diagnostics) == 0 {
		return nil
	}
	messages := make([]string, 0, len(report.Diagnostics))
	for _, diagnostic := range report.Diagnostics {
		messages = append(messages, fmt.Sprintf("%s:%d:%d: %s", diagnostic.Source, diagnostic.Line, diagnostic.Column, diagnostic.Message))
	}
	return errors.New(strings.Join(messages, "\n"))
}

func listContracts(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: gowdk list commands|queries|events|jobs [--json] [dir]")
	}
	kind, err := contractListKind(args[0])
	if err != nil {
		return err
	}
	options, err := parseContractReportOptions(args[1:])
	if err != nil {
		return err
	}
	report, err := contractscan.Scan(options.Dir)
	if err != nil {
		return err
	}
	return printContractReport(report, kind, options.JSON)
}

func contractGraph(args []string) error {
	options, err := parseContractReportOptions(args)
	if err != nil {
		return err
	}
	report, err := contractscan.Scan(options.Dir)
	if err != nil {
		return err
	}
	if options.JSON {
		return printContractReport(report, "", true)
	}
	return printContractGraph(report)
}

type contractReportOptions struct {
	Dir  string
	JSON bool
}

func parseContractReportOptions(args []string) (contractReportOptions, error) {
	options := contractReportOptions{Dir: "."}
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch arg {
		case "--json":
			options.JSON = true
		default:
			if strings.HasPrefix(arg, "-") {
				return contractReportOptions{}, fmt.Errorf("unknown contracts flag %q", arg)
			}
			if options.Dir != "." {
				return contractReportOptions{}, errors.New("usage: gowdk contracts [--json] [dir]")
			}
			options.Dir = arg
		}
	}
	return options, nil
}

func contractListKind(value string) (runtimecontracts.Kind, error) {
	switch value {
	case "commands":
		return runtimecontracts.Command, nil
	case "queries":
		return runtimecontracts.Query, nil
	case "events":
		return runtimecontracts.Event, nil
	case "jobs":
		return runtimecontracts.Job, nil
	default:
		return "", fmt.Errorf("unknown contract list %q", value)
	}
}

func printContractReport(report contractscan.Report, kind runtimecontracts.Kind, jsonOutput bool) error {
	if jsonOutput {
		payload, err := report.JSON(kind)
		if err != nil {
			return err
		}
		_, err = os.Stdout.Write(append(payload, '\n'))
		return err
	}
	contracts := report.Filter(kind)
	if len(contracts) == 0 {
		fmt.Println("No contracts found.")
		return nil
	}
	for _, contract := range contracts {
		fmt.Println(contractHeading(contract))
		fmt.Printf("  handler: %s\n", emptyValue(contract.Handler))
		if contract.Result != "" {
			fmt.Printf("  result: %s\n", contract.Result)
		}
		if len(contract.Roles) > 0 {
			fmt.Printf("  roles: %s\n", strings.Join(contract.Roles, ", "))
		}
		fmt.Printf("  source: %s:%d:%d\n", contract.Source, contract.Line, contract.Column)
	}
	printContractDiagnostics(report.Diagnostics)
	return nil
}

func printContractDiagnostics(diagnostics []contractscan.Diagnostic) {
	if len(diagnostics) == 0 {
		return
	}
	fmt.Println("Diagnostics:")
	for _, diagnostic := range diagnostics {
		fmt.Printf("  %s: %s:%d:%d: %s\n", strings.ToUpper(diagnostic.Severity), diagnostic.Source, diagnostic.Line, diagnostic.Column, diagnostic.Message)
	}
}

func contractHeading(contract contractscan.Contract) string {
	switch contract.Kind {
	case runtimecontracts.Event:
		category := strings.ToUpper(string(contract.EventCategory))
		if category == "" {
			category = "EVENT"
		} else {
			category += " EVENT"
		}
		return category + " " + contract.Type
	default:
		return strings.ToUpper(string(contract.Kind)) + " " + contract.Type
	}
}

func emptyValue(value string) string {
	if value == "" {
		return "(unknown)"
	}
	return value
}

func printContractGraph(report contractscan.Report) error {
	commands := report.Filter(runtimecontracts.Command)
	events := report.Filter(runtimecontracts.Event)
	if len(commands) == 0 && len(events) == 0 {
		fmt.Println("No contract graph found.")
		return nil
	}
	for _, command := range commands {
		fmt.Println(contractHeading(command))
		if len(command.Emits) == 0 {
			fmt.Println("  emits: none detected")
			continue
		}
		fmt.Println("  emits:")
		for _, event := range command.Emits {
			fmt.Printf("    - %s EVENT %s\n", strings.ToUpper(string(event.Category)), event.Type)
		}
	}
	seenEvents := map[string]bool{}
	for _, event := range events {
		key := string(event.EventCategory) + ":" + event.Type
		if seenEvents[key] {
			continue
		}
		seenEvents[key] = true
		fmt.Println(contractHeading(event))
		subscribers := eventSubscribers(events, event)
		if len(subscribers) == 0 {
			fmt.Println("  subscribers: none")
			continue
		}
		fmt.Println("  subscribers:")
		for _, subscriber := range subscribers {
			fmt.Printf("    - %s\n", subscriber)
		}
	}
	return nil
}

func eventSubscribers(events []contractscan.Contract, event contractscan.Contract) []string {
	var subscribers []string
	for _, candidate := range events {
		if candidate.Type == event.Type && candidate.EventCategory == event.EventCategory && candidate.Handler != "" {
			subscribers = append(subscribers, candidate.Handler)
		}
	}
	return subscribers
}
