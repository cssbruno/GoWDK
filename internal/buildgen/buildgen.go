// Package buildgen emits app-shell HTML artifacts for build-time pages.
package buildgen

import (
	"strings"

	"github.com/cssbruno/gowdk/internal/clientrt"
)

const routeManifestFile = "gowdk-routes.json"

const assetManifestFile = "gowdk-assets.json"

const buildReportFile = "gowdk-build-report.json"

const securityManifestFile = "gowdk-security.json"

const immutableAssetCachePolicy = "public, max-age=31536000, immutable"

// noCacheAssetCachePolicy forces revalidation (via ETag/ModTime) for assets
// served at stable, unhashed paths whose contents change between deploys.
const noCacheAssetCachePolicy = "no-cache"

const defaultPageCSSDir = "assets/gowdk"

const inlineStyleAssetPath = "<inline-style>"

const clientRuntimeAssetPath = "assets/gowdk/" + clientrt.Filename

const clientRuntimeHref = "/" + clientRuntimeAssetPath

const DisableCSSDiscovery = "__gowdk_disable_css_discovery__"

const islandRuntimeDir = "assets/gowdk/islands"

const storeRuntimeAssetPath = islandRuntimeDir + "/stores.js"

const storeRuntimeHref = "/" + storeRuntimeAssetPath

var (
	defaultCSSIncludes = []string{"**/*.css"}
	defaultCSSExcludes = []string{".git/**", "**/.git/**", "vendor/**", "**/vendor/**", "node_modules/**", "**/node_modules/**", ".gowdk/**", "**/.gowdk/**", "dist/**", "**/dist/**"}
)

func parseLiteralDeclaration(line string) (string, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "=>") {
		return "", false
	}
	body := strings.TrimSpace(strings.TrimPrefix(line, "=>"))
	if !strings.HasPrefix(body, "{") || !strings.HasSuffix(body, "}") {
		return "", false
	}
	return strings.TrimSpace(body[1 : len(body)-1]), true
}

func isLiteralName(value string) bool {
	if value == "" {
		return false
	}
	for index, r := range value {
		if index == 0 {
			if r != '_' && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') {
				return false
			}
			continue
		}
		if r != '_' && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}

func isCSSInputName(value string) bool {
	if value == "" {
		return false
	}
	for index, r := range value {
		if index == 0 {
			if r != '_' && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') {
				return false
			}
			continue
		}
		if r != '_' && r != '.' && r != '-' && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}

func layoutSlotIndexes(source string) [][2]int {
	var matches [][2]int
	for index := 0; index < len(source); index++ {
		if source[index] != '<' || !strings.HasPrefix(source[index:], "<slot") {
			continue
		}
		cursor := index + len("<slot")
		for cursor < len(source) && (source[cursor] == ' ' || source[cursor] == '\t' || source[cursor] == '\n' || source[cursor] == '\r') {
			cursor++
		}
		if cursor >= len(source) || source[cursor] != '/' {
			continue
		}
		cursor++
		for cursor < len(source) && (source[cursor] == ' ' || source[cursor] == '\t' || source[cursor] == '\n' || source[cursor] == '\r') {
			cursor++
		}
		if cursor >= len(source) || source[cursor] != '>' {
			continue
		}
		matches = append(matches, [2]int{index, cursor + 1})
		index = cursor
	}
	return matches
}
