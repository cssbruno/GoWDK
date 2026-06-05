// Package staticgen emits static HTML artifacts for build-time pages.
package staticgen

import (
	"regexp"

	"github.com/cssbruno/gowdk/internal/clientrt"
)

const routeManifestFile = "gowdk-routes.json"

const assetManifestFile = "gowdk-assets.json"

const buildReportFile = "gowdk-build-report.json"

const defaultPageCSSDir = "assets/gowdk"

const clientRuntimeAssetPath = "assets/gowdk/" + clientrt.Filename

const clientRuntimeHref = "/" + clientRuntimeAssetPath

const DisableCSSDiscovery = "__gowdk_disable_css_discovery__"

const islandRuntimeDir = "assets/gowdk/islands"

var (
	literalDeclarationPattern = regexp.MustCompile(`^=>\s*\{(.*)\}$`)
	buildCallPattern          = regexp.MustCompile(`^=>\s*([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z_][A-Za-z0-9_]*)\(\)$`)
	literalNamePattern        = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	cssInputNamePattern       = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.-]*$`)
	layoutSlotPattern         = regexp.MustCompile(`<slot\s*/>`)
)

var (
	defaultCSSIncludes = []string{"**/*.css"}
	defaultCSSExcludes = []string{".git/**", "vendor/**", "node_modules/**"}
)
