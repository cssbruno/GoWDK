package buildgen

import (
	"encoding/xml"
	"fmt"
	"go/ast"
	"go/token"
	"net/url"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	gowdkauth "github.com/cssbruno/gowdk/runtime/auth"
	runtimeseo "github.com/cssbruno/gowdk/runtime/seo"
)

type seoPlan struct {
	Enabled    bool
	Sitemap    []byte
	Robots     []byte
	URLs       []sitemapURLEntry
	Exclusions []seoExclusion
}

type sitemapURLEntry struct {
	Loc        string `xml:"loc"`
	LastMod    string `xml:"lastmod,omitempty"`
	ChangeFreq string `xml:"changefreq,omitempty"`
	Priority   string `xml:"priority,omitempty"`
}

type sitemapURLSet struct {
	XMLName xml.Name          `xml:"urlset"`
	XMLNS   string            `xml:"xmlns,attr"`
	URLs    []sitemapURLEntry `xml:"url"`
}

type seoExclusion struct {
	PageID string
	Route  string
	Reason string
	Mode   string
}

type RuntimeSitemapPlan struct {
	Enabled    bool
	BaseURL    string
	StaticURLs []runtimeseo.URL
	Dynamic    gowdk.SEODynamicSitemap
}

func planSEOArtifacts(config gowdk.Config, ir gwdkir.Program, artifacts []Artifact) (seoPlan, error) {
	options, enabled, err := seoOptionsFromConfig(config)
	if err != nil || !enabled {
		return seoPlan{}, err
	}
	base, err := parseSEOBaseURL(options.BaseURL)
	if err != nil {
		return seoPlan{}, err
	}

	publicPages := publicSEOPageIDs(ir)
	urls, err := sitemapURLs(base, artifacts, options.ExtraURLs, publicPages)
	if err != nil {
		return seoPlan{}, err
	}
	sitemap, err := sitemapPayload(urls)
	if err != nil {
		return seoPlan{}, err
	}
	robots := robotsPayload(base, options.Disallow)

	return seoPlan{
		Enabled:    true,
		Sitemap:    sitemap,
		Robots:     robots,
		URLs:       urls,
		Exclusions: seoExclusions(config, ir, artifacts, publicPages),
	}, nil
}

func RuntimeSitemapPlanFromIR(config gowdk.Config, ir gwdkir.Program) (RuntimeSitemapPlan, error) {
	options, enabled, err := seoOptionsFromConfig(config)
	if err != nil || !enabled {
		return RuntimeSitemapPlan{}, err
	}
	if _, err := parseSEOBaseURL(options.BaseURL); err != nil {
		return RuntimeSitemapPlan{}, err
	}
	dynamic := options.DynamicSitemap
	if err := validateDynamicSitemap(dynamic); err != nil {
		return RuntimeSitemapPlan{}, err
	}
	publicPages := publicSEOPageIDs(ir)
	var urls []runtimeseo.URL
	for _, page := range ir.Pages {
		if !publicPages[page.ID] || isRequestTimePage(config, page) {
			continue
		}
		outputs, err := pageOutputs(config, page)
		if err != nil {
			return RuntimeSitemapPlan{}, fmt.Errorf("%s: %w", page.ID, err)
		}
		for _, output := range outputs {
			urls = append(urls, runtimeseo.URL{Loc: output.route})
		}
	}
	for _, extra := range options.ExtraURLs {
		urls = append(urls, runtimeseo.URL(extra))
	}
	return RuntimeSitemapPlan{
		Enabled:    true,
		BaseURL:    options.BaseURL,
		StaticURLs: urls,
		Dynamic:    dynamic,
	}, nil
}

func validateDynamicSitemap(options gowdk.SEODynamicSitemap) error {
	importPath := strings.TrimSpace(options.ImportPath)
	function := strings.TrimSpace(options.Function)
	if importPath == "" && function == "" {
		return nil
	}
	if importPath == "" {
		return fmt.Errorf("seo DynamicSitemap.ImportPath is required when Function is set")
	}
	if function == "" {
		return fmt.Errorf("seo DynamicSitemap.Function is required when ImportPath is set")
	}
	if strings.ContainsAny(importPath, "\r\n") {
		return fmt.Errorf("seo DynamicSitemap.ImportPath must stay on one line")
	}
	if strings.ContainsAny(function, "\r\n") || !token.IsIdentifier(function) {
		return fmt.Errorf("seo DynamicSitemap.Function must be an exported function name")
	}
	if !ast.IsExported(function) {
		return fmt.Errorf("seo DynamicSitemap.Function must be exported")
	}
	if options.MaxURLs < 0 {
		return fmt.Errorf("seo DynamicSitemap.MaxURLs must be non-negative")
	}
	if options.CacheSeconds < 0 {
		return fmt.Errorf("seo DynamicSitemap.CacheSeconds must be non-negative")
	}
	return nil
}

func seoOptionsFromConfig(config gowdk.Config) (gowdk.SEOOptions, bool, error) {
	var found bool
	var options gowdk.SEOOptions
	for _, addon := range config.Addons {
		if !addonHasFeature(addon, gowdk.FeatureSEO) {
			continue
		}
		found = true
		provider, ok := addon.(gowdk.SEOProvider)
		if !ok {
			return gowdk.SEOOptions{}, true, fmt.Errorf("seo feature requires an addon that implements gowdk.SEOProvider")
		}
		options = provider.SEOOptions()
		break
	}
	return options, found, nil
}

func addonHasFeature(addon gowdk.Addon, feature gowdk.Feature) bool {
	for _, candidate := range addon.Features() {
		if candidate == feature {
			return true
		}
	}
	return false
}

func parseSEOBaseURL(value string) (*url.URL, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return nil, fmt.Errorf("seo BaseURL is required")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("seo BaseURL is invalid: %w", err)
	}
	if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, fmt.Errorf("seo BaseURL must be an absolute http or https URL")
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed, nil
}

func sitemapURLs(base *url.URL, artifacts []Artifact, extra []gowdk.SEOURL, publicPages map[string]bool) ([]sitemapURLEntry, error) {
	seen := map[string]bool{}
	var urls []sitemapURLEntry
	add := func(entry sitemapURLEntry) {
		if seen[entry.Loc] {
			return
		}
		seen[entry.Loc] = true
		urls = append(urls, entry)
	}

	for _, artifact := range artifacts {
		if !publicPages[artifact.PageID] {
			continue
		}
		add(sitemapURLEntry{Loc: absoluteSEOURL(base, artifact.Route)})
	}
	for _, candidate := range extra {
		loc := strings.TrimSpace(candidate.Loc)
		if loc == "" {
			return nil, fmt.Errorf("seo extra URL loc is required")
		}
		absolute, err := normalizeExtraSEOURL(base, loc)
		if err != nil {
			return nil, err
		}
		add(sitemapURLEntry{
			Loc:        absolute,
			LastMod:    strings.TrimSpace(candidate.LastMod),
			ChangeFreq: strings.TrimSpace(candidate.ChangeFreq),
			Priority:   strings.TrimSpace(candidate.Priority),
		})
	}

	sort.Slice(urls, func(i, j int) bool {
		return urls[i].Loc < urls[j].Loc
	})
	return urls, nil
}

func normalizeExtraSEOURL(base *url.URL, loc string) (string, error) {
	parsed, err := url.Parse(loc)
	if err != nil {
		return "", fmt.Errorf("seo extra URL %q is invalid: %w", loc, err)
	}
	if parsed.IsAbs() {
		if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return "", fmt.Errorf("seo extra URL %q must be an absolute http(s) URL or a root-relative path", loc)
		}
		parsed.RawQuery = ""
		parsed.Fragment = ""
		return parsed.String(), nil
	}
	if !strings.HasPrefix(loc, "/") {
		return "", fmt.Errorf("seo extra URL %q must be absolute or root-relative", loc)
	}
	return absoluteSEOURL(base, loc), nil
}

func absoluteSEOURL(base *url.URL, route string) string {
	resolved := *base
	resolved.RawQuery = ""
	resolved.Fragment = ""
	resolved.RawPath = ""

	basePath := strings.TrimRight(resolved.Path, "/")
	routePath := "/" + strings.TrimLeft(rootRelativeSEOPath(route), "/")
	if routePath == "/" {
		if basePath == "" {
			resolved.Path = "/"
		} else {
			resolved.Path = basePath + "/"
		}
		return resolved.String()
	}
	resolved.Path = basePath + routePath
	return resolved.String()
}

func rootRelativeSEOPath(route string) string {
	parsed, err := url.Parse(route)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || !strings.HasPrefix(route, "/") {
		return route
	}
	if parsed.Path == "" {
		return "/"
	}
	return parsed.Path
}

func sitemapPayload(urls []sitemapURLEntry) ([]byte, error) {
	payload, err := xml.MarshalIndent(sitemapURLSet{
		XMLNS: "http://www.sitemaps.org/schemas/sitemap/0.9",
		URLs:  urls,
	}, "", "  ")
	if err != nil {
		return nil, err
	}
	out := append([]byte(xml.Header), payload...)
	out = append(out, '\n')
	return out, nil
}

func robotsPayload(base *url.URL, disallow []string) []byte {
	lines := []string{"User-agent: *"}
	entries := cleanRobotDisallow(disallow)
	if len(entries) == 0 {
		lines = append(lines, "Disallow:")
	} else {
		for _, entry := range entries {
			lines = append(lines, "Disallow: "+entry)
		}
	}
	lines = append(lines, "Sitemap: "+absoluteSEOURL(base, "/"+sitemapFile), "")
	return []byte(strings.Join(lines, "\n"))
}

func cleanRobotDisallow(disallow []string) []string {
	seen := map[string]bool{}
	var values []string
	for _, value := range disallow {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		values = append(values, value)
	}
	return values
}

func publicSEOPageIDs(ir gwdkir.Program) map[string]bool {
	publicPages := map[string]bool{}
	for _, page := range ir.Pages {
		if pageHasPublicGuard(page) && !pageNoIndex(page) {
			publicPages[page.ID] = true
		}
	}
	return publicPages
}

func pageHasPublicGuard(page gwdkir.Page) bool {
	for _, guard := range page.Guards {
		if gowdkauth.IsPublicGuard(guard) {
			return true
		}
	}
	return false
}

func pageNoIndex(page gwdkir.Page) bool {
	for _, token := range strings.Split(robotsContent(page.Metadata), ",") {
		if strings.EqualFold(strings.TrimSpace(token), "noindex") {
			return true
		}
	}
	return false
}

func seoExclusions(config gowdk.Config, ir gwdkir.Program, artifacts []Artifact, publicPages map[string]bool) []seoExclusion {
	included := map[string]bool{}
	for _, artifact := range artifacts {
		if publicPages[artifact.PageID] {
			included[artifact.PageID] = true
		}
	}

	var excluded []seoExclusion
	for _, page := range ir.Pages {
		if included[page.ID] {
			continue
		}
		mode := page.RenderMode(config.Render.DefaultMode())
		switch {
		case pageNoIndex(page):
			excluded = append(excluded, seoExclusion{
				PageID: page.ID,
				Route:  page.Route,
				Reason: "noindex",
				Mode:   string(mode),
			})
		case isRequestTimePage(config, page):
			excluded = append(excluded, seoExclusion{
				PageID: page.ID,
				Route:  page.Route,
				Reason: "request_time_rendering",
				Mode:   string(mode),
			})
		case len(page.Guards) == 0:
			excluded = append(excluded, seoExclusion{
				PageID: page.ID,
				Route:  page.Route,
				Reason: "guardless_route_denied",
				Mode:   string(mode),
			})
		case !pageHasPublicGuard(page):
			excluded = append(excluded, seoExclusion{
				PageID: page.ID,
				Route:  page.Route,
				Reason: "non_public_route",
				Mode:   string(mode),
			})
		case len(page.DynamicParams()) > 0 && !page.Blocks.Paths:
			excluded = append(excluded, seoExclusion{
				PageID: page.ID,
				Route:  page.Route,
				Reason: "dynamic_route_missing_paths",
				Mode:   string(mode),
			})
		}
	}
	sort.Slice(excluded, func(i, j int) bool {
		if excluded[i].Route == excluded[j].Route {
			return excluded[i].PageID < excluded[j].PageID
		}
		return excluded[i].Route < excluded[j].Route
	})
	return excluded
}

func writeSEOArtifacts(outputDir string, plan seoPlan) (string, string, bool, bool, error) {
	if !plan.Enabled {
		return "", "", false, false, nil
	}
	sitemapPath := filepath.Join(outputDir, sitemapFile)
	sitemapWrote, err := writeFileIfChangedStatus(sitemapPath, plan.Sitemap)
	if err != nil {
		return "", "", false, false, err
	}
	robotsPath := filepath.Join(outputDir, robotsFile)
	robotsWrote, err := writeFileIfChangedStatus(robotsPath, plan.Robots)
	if err != nil {
		return "", "", false, false, err
	}
	return sitemapPath, robotsPath, sitemapWrote, robotsWrote, nil
}

func reportSEOExclusions(reporter *buildReporter, exclusions []seoExclusion) {
	for _, exclusion := range exclusions {
		data := map[string]string{"reason": exclusion.Reason}
		if exclusion.Mode != "" {
			data["mode"] = exclusion.Mode
		}
		reporter.info("seo", "seo_route_excluded", "route excluded from sitemap", BuildEvent{
			PageID: exclusion.PageID,
			Route:  exclusion.Route,
			Data:   data,
		})
	}
}
