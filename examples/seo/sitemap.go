package seoexample

import (
	"context"

	"github.com/cssbruno/gowdk/runtime/seo"
)

func DynamicSitemapURLs(ctx context.Context) ([]seo.URL, error) {
	return []seo.URL{
		{Loc: "/seo/runtime-only", LastMod: "2026-06-22"},
	}, nil
}
