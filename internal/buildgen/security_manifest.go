package buildgen

import (
	"encoding/json"
	"path/filepath"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/securitymanifest"
)

// securityManifestPayload renders the IR-derived security posture
// (gowdk-security.json). It is pure data — declarative posture, no policy
// evaluation — so the artifact is stable and auditable on its own. Policy
// findings live in gowdk audit, which reads this same posture.
func securityManifestPayload(config gowdk.Config, ir gwdkir.Program) ([]byte, error) {
	manifest := securitymanifest.Build(config, ir)
	return json.MarshalIndent(manifest, "", "  ")
}

func securityManifestPath(outputDir string) (string, error) {
	absOutput, err := filepath.Abs(outputDir)
	if err != nil {
		return "", err
	}
	cleanOutput := filepath.Clean(absOutput)
	outputName := filepath.Base(cleanOutput)
	if outputName == "" || outputName == "." || outputName == string(filepath.Separator) {
		outputName = "root"
	}
	return filepath.Join(filepath.Dir(cleanOutput), ".gowdk", "reports", outputName, securityManifestFile), nil
}

func memorySecurityManifestPath(outputBase string, diskOutputPath bool) (string, error) {
	if diskOutputPath {
		return securityManifestPath(outputBase)
	}
	cleanOutput := filepath.Clean(outputBase)
	outputName := filepath.Base(cleanOutput)
	if outputName == "" || outputName == "." || outputName == string(filepath.Separator) {
		outputName = "root"
	}
	return filepath.Join(filepath.Dir(cleanOutput), ".gowdk", "reports", outputName, securityManifestFile), nil
}
