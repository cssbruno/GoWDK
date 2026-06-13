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

func writeSecurityManifest(outputDir string, config gowdk.Config, ir gwdkir.Program) (string, error) {
	payload, err := securityManifestPayload(config, ir)
	if err != nil {
		return "", err
	}
	manifestPath := filepath.Join(outputDir, securityManifestFile)
	if err := writeFileIfChanged(manifestPath, payload); err != nil {
		return "", err
	}
	return manifestPath, nil
}
