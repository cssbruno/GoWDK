package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func preview(args []string) error {
	options, err := parsePreviewOptions(args)
	if err != nil {
		return err
	}
	outputDir := options.OutputDir
	if strings.TrimSpace(outputDir) == "" {
		outputDir, err = os.MkdirTemp("", "gowdk-preview-*")
		if err != nil {
			return err
		}
	}
	buildArgs := previewBuildArgs(options.BuildArgs, outputDir)
	if options.Hot {
		return dev(append([]string{"--addr", options.Addr}, buildArgs...))
	}
	if err := build(buildArgs); err != nil {
		return err
	}
	absDir, err := filepath.Abs(outputDir)
	if err != nil {
		return err
	}
	server := &http.Server{
		Addr:              options.Addr,
		Handler:           outputFileHandler(absDir),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	fmt.Printf("Preview serving %s at http://%s\n", absDir, options.Addr)
	return server.ListenAndServe()
}

type previewOptions struct {
	Addr      string
	Hot       bool
	OutputDir string
	BuildArgs []string
}

func parsePreviewOptions(args []string) (previewOptions, error) {
	options := previewOptions{Addr: "127.0.0.1:8080"}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if value, next, ok, missing := consumeValueFlag(args, i, "--addr", true); ok {
			if missing {
				return previewOptions{}, errors.New(previewUsage())
			}
			options.Addr = value
			i = next
			continue
		}
		if value, next, ok, missing := consumeValueFlag(args, i, "--out", true); ok {
			if missing {
				return previewOptions{}, errors.New(previewUsage())
			}
			options.OutputDir = value
			options.BuildArgs = append(options.BuildArgs, "--out", value)
			i = next
			continue
		}
		switch {
		case arg == "--hot":
			options.Hot = true
		default:
			options.BuildArgs = append(options.BuildArgs, arg)
		}
	}
	if strings.TrimSpace(options.Addr) == "" {
		return previewOptions{}, fmt.Errorf("preview address is required")
	}
	return options, nil
}

func previewBuildArgs(args []string, outputDir string) []string {
	if devArgsHaveOutput(args) {
		return append([]string(nil), args...)
	}
	next := append([]string(nil), args...)
	next = append(next, "--out", outputDir)
	return next
}

func previewUsage() string {
	return "usage: gowdk preview [--addr <addr>] [--hot] [build flags...]"
}
