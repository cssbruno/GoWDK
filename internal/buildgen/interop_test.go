package buildgen

import (
	"github.com/cssbruno/gowdk"
	fixture "github.com/cssbruno/gowdk/testfixture/interop"
)

func ssrTestConfig(loadPages ...string) gowdk.Config {
	config := gowdk.Config{
		Features: gowdk.FeatureConfig{Auth: gowdk.AuthFeatureConfig{Enabled: true}},
		Addons:   []gowdk.Addon{gowdk.NewAddon("ssr", gowdk.FeatureSSR)},
	}
	for _, page := range loadPages {
		config.Interop.Loads = append(config.Interop.Loads, gowdk.RegisterLoad(page, fixture.LoadDashboard))
	}
	return config
}
