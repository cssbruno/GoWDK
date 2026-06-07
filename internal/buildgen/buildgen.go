// Package buildgen emits app-shell HTML artifacts for build-time pages.
package buildgen

import (
	"regexp"

	"github.com/cssbruno/gowdk/internal/clientrt"
)

const routeManifestFile = "gowdk-routes.json"

const assetManifestFile = "gowdk-assets.json"

const buildReportFile = "gowdk-build-report.json"

const immutableAssetCachePolicy = "public, max-age=31536000, immutable"

const defaultPageCSSDir = "assets/gowdk"

const inlineStyleAssetPath = "<inline-style>"

const clientRuntimeAssetPath = "assets/gowdk/" + clientrt.Filename

const clientRuntimeHref = "/" + clientRuntimeAssetPath

const DisableCSSDiscovery = "__gowdk_disable_css_discovery__"

const islandRuntimeDir = "assets/gowdk/islands"

const storeRuntimeAssetPath = islandRuntimeDir + "/stores.js"

const storeRuntimeHref = "/" + storeRuntimeAssetPath

var (
	literalDeclarationPattern = regexp.MustCompile(`^=>\s*\{(.*)\}$`)
	literalNamePattern        = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	cssInputNamePattern       = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.-]*$`)
	layoutSlotPattern         = regexp.MustCompile(`<slot\s*/>`)
)

var (
	defaultCSSIncludes = []string{"**/*.css"}
	defaultCSSExcludes = []string{".git/**", "**/.git/**", "vendor/**", "**/vendor/**", "node_modules/**", "**/node_modules/**", ".gowdk/**", "**/.gowdk/**", "dist/**", "**/dist/**"}
)
