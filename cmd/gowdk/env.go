package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/project"
)

const envUsage = "usage: gowdk env check [--config <file>] [--env-file <file>] [--json]"

type envCheckReport struct {
	Version     int                  `json:"version"`
	Status      string               `json:"status"`
	Variables   int                  `json:"variables"`
	Secrets     int                  `json:"secrets"`
	EnvFilePath string               `json:"envFilePath,omitempty"`
	Errors      []envCheckDiagnostic `json:"errors,omitempty"`
}

type envCheckDiagnostic struct {
	Code    string `json:"code,omitempty"`
	Name    string `json:"name,omitempty"`
	Message string `json:"message"`
}

func envCommand(args []string) error {
	if len(args) == 0 || args[0] != "check" {
		return errors.New(envUsage)
	}
	var configPath string
	var envFilePath string
	jsonOutput := false
	for index := 1; index < len(args); index++ {
		arg := args[index]
		if value, next, ok, missing := consumeValueFlag(args, index, "--config", true); ok {
			if missing {
				return errors.New(envUsage)
			}
			configPath = value
			index = next
			continue
		}
		if value, next, ok, missing := consumeValueFlag(args, index, "--env-file", true); ok {
			if missing {
				return errors.New(envUsage)
			}
			envFilePath = value
			index = next
			continue
		}
		switch {
		case arg == "--json":
			jsonOutput = true
		case arg == "-h" || arg == "--help":
			return errors.New(envUsage)
		case strings.HasPrefix(arg, "-"):
			return fmt.Errorf("unknown env check flag %q", arg)
		default:
			return fmt.Errorf("unexpected env check argument %q", arg)
		}
	}

	options := cliOptions{EnvFilePath: envFilePath}
	if err := loadProjectConfig(&options, configPath); err != nil {
		return err
	}
	validationErr := project.ValidateRuntimeEnvironment(options.Config, os.LookupEnv)
	report := newEnvCheckReport(options, validationErr)
	if jsonOutput {
		payload, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(payload))
		if validationErr != nil {
			return silentEnvCheckError{err: validationErr}
		}
		return nil
	}
	if validationErr != nil {
		return fmt.Errorf("environment validation failed: %w", validationErr)
	}
	fmt.Printf("environment ok (%d variables, %d secrets)\n", report.Variables, report.Secrets)
	return nil
}

func newEnvCheckReport(options cliOptions, validationErr error) envCheckReport {
	report := envCheckReport{
		Version:     1,
		Status:      "ok",
		Variables:   len(options.Config.Env.Vars),
		Secrets:     len(options.Config.Env.Secrets),
		EnvFilePath: options.EnvFilePath,
	}
	if validationErr == nil {
		return report
	}
	report.Status = "error"
	var validationErrors gowdk.EnvValidationErrors
	if errors.As(validationErr, &validationErrors) {
		for _, item := range validationErrors {
			report.Errors = append(report.Errors, envCheckDiagnostic{
				Code:    item.Code,
				Name:    item.Name,
				Message: item.Message,
			})
		}
		return report
	}
	report.Errors = append(report.Errors, envCheckDiagnostic{Message: validationErr.Error()})
	return report
}

type silentEnvCheckError struct {
	err error
}

func (err silentEnvCheckError) Error() string {
	return err.err.Error()
}

func (silentEnvCheckError) SilentCLIError() {}
