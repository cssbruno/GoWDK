package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cssbruno/gowdk/internal/diagnostics"
)

func explainDiagnostic(args []string) error {
	jsonOutput := false
	var positional []string
	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOutput = true
		default:
			if strings.HasPrefix(arg, "-") {
				return fmt.Errorf("unknown explain flag %q", arg)
			}
			positional = append(positional, arg)
		}
	}
	if len(positional) != 1 {
		return fmt.Errorf("usage: gowdk explain [--json] <diagnostic-code>")
	}
	code := positional[0]
	explanation, ok := diagnostics.Explain(code)
	if !ok {
		suggestions := diagnostics.Suggestions(code, 3)
		if len(suggestions) == 0 {
			return fmt.Errorf("unknown diagnostic code %q", code)
		}
		return fmt.Errorf("unknown diagnostic code %q. Did you mean: %s?", code, strings.Join(suggestions, ", "))
	}
	if jsonOutput {
		payload, err := json.MarshalIndent(explanation, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(payload))
		return nil
	}
	printDiagnosticExplanation(explanation)
	return nil
}

func printDiagnosticExplanation(explanation diagnostics.Explanation) {
	fmt.Println(explanation.Code)
	fmt.Printf("Area: %s\n", explanation.Area)
	fmt.Printf("Stability: %s\n", explanation.Stability)
	fmt.Printf("Summary: %s\n", explanation.Summary)
	if explanation.Details != "" {
		fmt.Println()
		fmt.Println(explanation.Details)
	}
	if len(explanation.NextSteps) > 0 {
		fmt.Println()
		fmt.Println("Next steps:")
		for _, step := range explanation.NextSteps {
			fmt.Printf("- %s\n", step)
		}
	}
	if explanation.Invalid != "" {
		fmt.Println()
		fmt.Println("Invalid:")
		fmt.Print(trimTrailingNewline(explanation.Invalid))
		fmt.Println()
	}
	if explanation.Fixed != "" {
		fmt.Println()
		fmt.Println("Fixed:")
		fmt.Print(trimTrailingNewline(explanation.Fixed))
		fmt.Println()
	}
}

func trimTrailingNewline(value string) string {
	return strings.TrimRight(value, "\n")
}
