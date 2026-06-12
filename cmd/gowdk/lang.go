package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/cssbruno/gowdk/internal/compiler"
	"github.com/cssbruno/gowdk/internal/lang"
)

func tokens(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: gowdk tokens <file.gwdk>")
	}
	source, err := os.ReadFile(args[0])
	if err != nil {
		return err
	}
	tokens, diagnostics := lang.Lex(string(source))
	for _, diagnostic := range diagnostics {
		diagnostic.File = args[0]
		fmt.Fprintln(os.Stderr, diagnostic.String())
	}
	for _, token := range tokens {
		fmt.Printf("%d:%d\t%s\t%q\n", token.Pos.Line, token.Pos.Column, token.Kind, token.Lexeme)
	}
	if diagnostics.HasErrors() {
		return fmt.Errorf("tokenization failed")
	}
	return nil
}

func format(args []string) error {
	write := false
	var paths []string
	for _, arg := range args {
		if arg == "--write" {
			write = true
			continue
		}
		paths = append(paths, arg)
	}
	if len(paths) == 0 {
		return fmt.Errorf("usage: gowdk fmt [--write] <files>")
	}

	for _, path := range paths {
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		formatted := lang.Format(source)
		if write {
			if err := os.WriteFile(path, formatted, 0o644); err != nil {
				return err
			}
			continue
		}
		if len(paths) > 1 {
			fmt.Printf("==> %s <==\n", path)
		}
		fmt.Print(string(formatted))
	}
	return nil
}

func check(args []string) error {
	options, paths, err := loadCommandInputs(args, "check", true)
	if err != nil {
		return err
	}

	if options.JSON {
		payload, diagnostics := lang.CheckJSONWithOptions(options.Config, paths, lang.CheckOptions{ProjectRoot: options.ProjectRoot})
		if len(payload) > 0 {
			fmt.Print(string(payload))
		}
		if checkShouldFail(options, diagnostics) {
			return fmt.Errorf("check failed")
		}
		return nil
	}

	_, diagnostics := lang.CheckFilesWithOptions(options.Config, paths, lang.CheckOptions{ProjectRoot: options.ProjectRoot})
	if len(diagnostics) == 0 {
		fmt.Println("ok")
		return nil
	}
	for _, diagnostic := range diagnostics {
		fmt.Fprintln(os.Stderr, diagnostic.String())
	}
	if checkShouldFail(options, diagnostics) {
		return fmt.Errorf("check failed")
	}
	return nil
}

func checkShouldFail(options cliOptions, diagnostics lang.Diagnostics) bool {
	if diagnostics.HasErrors() {
		return true
	}
	return options.WarningsAsErrors && len(diagnostics) > 0
}

func manifestJSON(args []string) error {
	options, paths, err := loadCommandInputs(args, "manifest", false)
	if err != nil {
		return err
	}

	payload, diagnostics := lang.ManifestJSONWithOptions(options.Config, paths, lang.CheckOptions{ProjectRoot: options.ProjectRoot})
	for _, diagnostic := range diagnostics {
		fmt.Fprintln(os.Stderr, diagnostic.String())
	}
	if diagnostics.HasErrors() {
		return fmt.Errorf("manifest failed")
	}
	fmt.Print(string(payload))
	return nil
}

func siteMapJSON(args []string) error {
	options, paths, err := loadCommandInputs(args, "sitemap", false)
	if err != nil {
		return err
	}

	payload, diagnostics := lang.SiteMapJSONWithOptions(options.Config, paths, lang.CheckOptions{ProjectRoot: options.ProjectRoot})
	for _, diagnostic := range diagnostics {
		fmt.Fprintln(os.Stderr, diagnostic.String())
	}
	if diagnostics.HasErrors() {
		return fmt.Errorf("sitemap failed")
	}
	fmt.Print(string(payload))
	return nil
}

func routesJSON(args []string) error {
	metadata, err := routeMetadataForCommand(args, "routes")
	if err != nil {
		return err
	}
	printRouteInfos(metadata.Info)
	payload, err := json.MarshalIndent(routeMetadataJSON(metadata), "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(payload))
	return nil
}

func endpointsJSONCommand(args []string) error {
	metadata, err := routeMetadataForCommand(args, "endpoints")
	if err != nil {
		return err
	}
	payload, err := json.MarshalIndent(endpointMetadataJSON(metadata), "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(payload))
	return nil
}

func routeMetadataForCommand(args []string, command string) (compiler.RouteMetadata, error) {
	options, paths, err := loadCommandInputs(args, command, false)
	if err != nil {
		return compiler.RouteMetadata{}, err
	}
	checked, diagnostics := lang.CheckFilesWithOptions(options.Config, paths, lang.CheckOptions{ProjectRoot: options.ProjectRoot})
	for _, diagnostic := range diagnostics {
		fmt.Fprintln(os.Stderr, diagnostic.String())
	}
	if diagnostics.HasErrors() {
		return compiler.RouteMetadata{}, fmt.Errorf("%s failed", command)
	}

	ir := checked.IR
	if err := linkIRContractReferences(&ir, options.ProjectRoot); err != nil {
		return compiler.RouteMetadata{}, err
	}
	metadata := compiler.BuildRouteMetadataFromIR(options.Config, ir)
	return metadata, nil
}

func printRouteInfos(infos []compiler.RouteInfo) {
	for _, info := range infos {
		fmt.Fprintf(os.Stderr, "info: %s: %s\n", info.Code, info.Message)
	}
}
