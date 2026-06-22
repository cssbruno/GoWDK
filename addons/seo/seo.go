package seo

import "github.com/cssbruno/gowdk"

// ImportPath is the canonical Go import path for the SEO addon.
const ImportPath = "github.com/cssbruno/gowdk/addons/seo"

// Options configures sitemap.xml and robots.txt build output.
type Options = gowdk.SEOOptions

// URL describes one additional sitemap URL.
type URL = gowdk.SEOURL

// DynamicSitemap describes an optional request-time sitemap provider.
type DynamicSitemap = gowdk.SEODynamicSitemap

// Addon enables build-time SEO output. BaseURL is required when building.
func Addon(options ...Options) gowdk.Addon {
	var selected Options
	if len(options) > 0 {
		selected = options[0]
	}
	return addon{options: selected}
}

type addon struct {
	options Options
}

func (addon) Name() string {
	return "seo"
}

func (addon) Features() []gowdk.Feature {
	return []gowdk.Feature{gowdk.FeatureSEO}
}

func (a addon) SEOOptions() gowdk.SEOOptions {
	options := cloneOptions(a.options)
	if options.ExtraURLProvider != nil {
		options.ExtraURLs = append(options.ExtraURLs, options.ExtraURLProvider()...)
		options.ExtraURLProvider = nil
	}
	return options
}

func cloneOptions(options Options) Options {
	options.Disallow = append([]string(nil), options.Disallow...)
	options.ExtraURLs = append([]gowdk.SEOURL(nil), options.ExtraURLs...)
	return options
}
