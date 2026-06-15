package buildgen

import (
	"bytes"
	"fmt"
	"path/filepath"

	"github.com/cssbruno/gowdk"
	"github.com/evanw/esbuild/pkg/api"
)

type assetObfuscationRecord struct {
	Path        string
	LogicalPath string
	BeforeHash  string
	AfterHash   string
	BeforeBytes int64
	AfterBytes  int64
	Changed     bool
}

func validateAssetObfuscationConfig(config gowdk.Config) error {
	if !config.Build.ObfuscateAssets {
		return nil
	}
	if config.Build.Mode != gowdk.Production {
		return fmt.Errorf("Build.ObfuscateAssets requires Build.Mode: gowdk.Production or the --obfuscate-assets CLI flag")
	}
	return nil
}

func applyAssetObfuscation(config gowdk.Config, outputDir string, artifacts []plannedAssetArtifact) ([]plannedAssetArtifact, []assetObfuscationRecord, error) {
	if err := validateAssetObfuscationConfig(config); err != nil {
		return nil, nil, err
	}
	if !config.Build.ObfuscateAssets {
		return artifacts, nil, nil
	}

	out := make([]plannedAssetArtifact, len(artifacts))
	copy(out, artifacts)
	var records []assetObfuscationRecord
	for index := range out {
		artifact := &out[index]
		if !artifact.obfuscationCandidate {
			continue
		}
		rel, err := relativeOutputPath(outputDir, artifact.Path)
		if err != nil {
			return nil, nil, err
		}
		transformed, err := obfuscateGeneratedJavaScript(rel, artifact.contents)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: obfuscate generated JavaScript: %w", rel, err)
		}
		beforeHash := contentHash(artifact.contents)
		afterHash := contentHash(transformed)
		artifact.contents = transformed
		artifact.Hash = afterHash
		artifact.Obfuscated = true
		records = append(records, assetObfuscationRecord{
			Path:        rel,
			LogicalPath: artifactLogicalPath(artifact.LogicalPath, rel),
			BeforeHash:  beforeHash,
			AfterHash:   afterHash,
			BeforeBytes: int64(len(artifacts[index].contents)),
			AfterBytes:  int64(len(transformed)),
			Changed:     !bytes.Equal(artifacts[index].contents, transformed),
		})
	}
	return out, records, nil
}

func obfuscateGeneratedJavaScript(sourcePath string, contents []byte) ([]byte, error) {
	result := api.Transform(string(contents), api.TransformOptions{
		Loader:            api.LoaderJS,
		Format:            api.FormatDefault,
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		MinifySyntax:      true,
		LegalComments:     api.LegalCommentsNone,
		Sourcefile:        filepath.ToSlash(sourcePath),
	})
	if len(result.Errors) > 0 {
		return nil, fmt.Errorf("%s", esbuildMessages(result.Errors))
	}
	return result.Code, nil
}

func reportAssetObfuscation(reporter *buildReporter, enabled bool, records []assetObfuscationRecord) {
	data := map[string]string{
		"enabled": fmt.Sprint(enabled),
		"assets":  fmt.Sprint(len(records)),
	}
	reporter.info("plan", "asset_obfuscation", "generated asset obfuscation summarized", BuildEvent{Data: data})
	for _, record := range records {
		eventData := map[string]string{
			"logicalPath": record.LogicalPath,
			"beforeHash":  record.BeforeHash,
			"afterHash":   record.AfterHash,
			"beforeBytes": fmt.Sprint(record.BeforeBytes),
			"afterBytes":  fmt.Sprint(record.AfterBytes),
			"changed":     fmt.Sprint(record.Changed),
		}
		reporter.info("plan", "asset_obfuscated", "generated asset obfuscated", BuildEvent{
			Path: record.Path,
			Data: eventData,
		})
	}
}
