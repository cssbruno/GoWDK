package main

import (
	"encoding/json"
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
	report, err := scanContractReport(root)
	if err != nil {
		return err
	}
	linkIRContractReferencesFromReport(ir, report)
	return nil
}

func scanContractReport(root string) (contractscan.Report, error) {
	if strings.TrimSpace(root) == "" {
		root = "."
	}
	report, err := contractscan.Scan(root)
	if err != nil {
		return contractscan.Report{}, err
	}
	if err := validateContractScanReport(report); err != nil {
		return contractscan.Report{}, err
	}
	return report, nil
}

func linkIRContractReferencesFromReport(ir *gwdkir.Program, report contractscan.Report) {
	if ir != nil && len(ir.ContractRefs) > 0 {
		ir.ContractRefs = contractscan.LinkReferences(ir.ContractRefs, report)
	}
	if ir != nil && len(ir.RealtimeSubscriptions) > 0 {
		ir.RealtimeSubscriptions = contractscan.LinkRealtimeSubscriptions(ir.RealtimeSubscriptions, report)
	}
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

func contractTrace(args []string) error {
	options, err := parseContractTraceOptions(args)
	if err != nil {
		return err
	}
	report, err := contractscan.Scan(options.Dir)
	if err != nil {
		return err
	}
	trace := buildContractTrace(report, options.Target)
	if len(trace.Matches) == 0 {
		return fmt.Errorf("contract %q was not found", options.Target)
	}
	if options.JSON {
		payload, err := json.MarshalIndent(trace, "", "  ")
		if err != nil {
			return err
		}
		_, err = os.Stdout.Write(append(payload, '\n'))
		return err
	}
	return printContractTrace(trace)
}

type contractReportOptions struct {
	Dir  string
	JSON bool
}

type contractTraceOptions struct {
	Target string
	Dir    string
	JSON   bool
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

func parseContractTraceOptions(args []string) (contractTraceOptions, error) {
	options := contractTraceOptions{Dir: "."}
	for _, arg := range args {
		switch {
		case arg == "--json":
			options.JSON = true
		case strings.HasPrefix(arg, "-"):
			return contractTraceOptions{}, fmt.Errorf("unknown trace flag %q", arg)
		case options.Target == "":
			options.Target = arg
		case options.Dir == ".":
			options.Dir = arg
		default:
			return contractTraceOptions{}, errors.New("usage: gowdk trace <contract> [--json] [dir]")
		}
	}
	if options.Target == "" {
		return contractTraceOptions{}, errors.New("usage: gowdk trace <contract> [--json] [dir]")
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

type contractTraceReport struct {
	Version     int                       `json:"version"`
	Target      string                    `json:"target"`
	Matches     []contractTraceMatch      `json:"matches"`
	Diagnostics []contractscan.Diagnostic `json:"diagnostics,omitempty"`
}

type contractTraceMatch struct {
	Contract    contractscan.Contract   `json:"contract"`
	Emits       []contractTraceEvent    `json:"emits,omitempty"`
	Subscribers []contractscan.Contract `json:"subscribers,omitempty"`
}

type contractTraceEvent struct {
	Category    runtimecontracts.EventCategory `json:"category"`
	Type        string                         `json:"type"`
	Subscribers []contractscan.Contract        `json:"subscribers,omitempty"`
}

func buildContractTrace(report contractscan.Report, target string) contractTraceReport {
	trace := contractTraceReport{Version: report.Version, Target: target, Diagnostics: report.Diagnostics}
	for _, contract := range report.Contracts {
		if !contractMatchesTarget(contract, target) {
			continue
		}
		match := contractTraceMatch{Contract: contract}
		switch contract.Kind {
		case runtimecontracts.Command:
			for _, emitted := range contract.Emits {
				match.Emits = append(match.Emits, contractTraceEvent{
					Category:    emitted.Category,
					Type:        emitted.Type,
					Subscribers: subscriberContracts(report.Contracts, emitted.Category, emitted.Type),
				})
			}
		case runtimecontracts.Event:
			match.Subscribers = subscriberContracts(report.Contracts, contract.EventCategory, contract.Type)
		}
		trace.Matches = append(trace.Matches, match)
	}
	return trace
}

func contractMatchesTarget(contract contractscan.Contract, target string) bool {
	if target == contract.Type || target == contract.Handler || target == contractIdentity(contract) {
		return true
	}
	if contract.Kind == runtimecontracts.Event {
		categoryTarget := string(contract.EventCategory) + ":" + contract.Type
		qualifiedCategoryTarget := string(contract.EventCategory) + ":" + contractIdentity(contract)
		return target == categoryTarget || target == qualifiedCategoryTarget
	}
	return false
}

func contractIdentity(contract contractscan.Contract) string {
	if contract.Package == "" || strings.Contains(contract.Type, ".") {
		return contract.Type
	}
	return contract.Package + "." + contract.Type
}

func subscriberContracts(contracts []contractscan.Contract, category runtimecontracts.EventCategory, typ string) []contractscan.Contract {
	var subscribers []contractscan.Contract
	for _, contract := range contracts {
		if contract.Kind == runtimecontracts.Event && contract.EventCategory == category && contract.Type == typ && contract.Handler != "" {
			subscribers = append(subscribers, contract)
		}
	}
	return subscribers
}

func printContractTrace(trace contractTraceReport) error {
	for matchIndex, match := range trace.Matches {
		if matchIndex > 0 {
			fmt.Println()
		}
		contract := match.Contract
		heading := contractHeading(contract)
		if suffix := contractIdentitySuffix(contract); suffix != "" {
			heading += " " + suffix
		}
		fmt.Println(heading)
		fmt.Printf("  handler: %s\n", emptyValue(contract.Handler))
		if contract.Result != "" {
			fmt.Printf("  result: %s\n", contract.Result)
		}
		if len(contract.Roles) > 0 {
			fmt.Printf("  roles: %s\n", strings.Join(contract.Roles, ", "))
		}
		fmt.Printf("  source: %s:%d:%d\n", contract.Source, contract.Line, contract.Column)
		switch contract.Kind {
		case runtimecontracts.Command:
			printTraceEmits(match.Emits)
		case runtimecontracts.Event:
			printTraceSubscribers(match.Subscribers)
		}
	}
	printContractDiagnostics(trace.Diagnostics)
	return nil
}

func contractIdentitySuffix(contract contractscan.Contract) string {
	identity := contractIdentity(contract)
	if identity == "" || identity == contract.Type {
		return ""
	}
	return "(" + identity + ")"
}

func printTraceEmits(events []contractTraceEvent) {
	if len(events) == 0 {
		fmt.Println("  emits: none detected")
		return
	}
	fmt.Println("  emits:")
	for _, event := range events {
		fmt.Printf("    - %s EVENT %s\n", strings.ToUpper(string(event.Category)), event.Type)
		if len(event.Subscribers) == 0 {
			fmt.Println("      subscribers: none")
			continue
		}
		fmt.Println("      subscribers:")
		for _, subscriber := range event.Subscribers {
			fmt.Printf("        - %s\n", emptyValue(subscriber.Handler))
		}
	}
}

func printTraceSubscribers(subscribers []contractscan.Contract) {
	if len(subscribers) == 0 {
		fmt.Println("  subscribers: none")
		return
	}
	fmt.Println("  subscribers:")
	for _, subscriber := range subscribers {
		fmt.Printf("    - %s\n", emptyValue(subscriber.Handler))
	}
}
