package main

import (
	"fmt"
	"os"

	"github.com/cssbruno/gowdk/internal/lsp"
)

func languageServer(args []string) error {
	options, paths := parseOptions(args)
	if len(paths) > 0 {
		return fmt.Errorf("usage: gowdk lsp [--ssr]")
	}
	return lsp.NewServer(options.Config).Serve(os.Stdin, os.Stdout)
}
