package gointerop

import (
	"fmt"
	"os"

	"github.com/cssbruno/gowdk"
)

type FeaturedCopy struct {
	Title   string `json:"title"`
	Tagline string `json:"tagline"`
}

func FeaturedCopyForBuild() FeaturedCopy {
	return FeaturedCopy{
		Title:   "Imported Go data",
		Tagline: "This page rendered data from a Go package imported directly in .gwdk.",
	}
}

func FeaturedCopyWithStderrForBuild() FeaturedCopy {
	fmt.Fprintln(os.Stderr, "gowdk example build data log")
	return FeaturedCopy{
		Title:   "Logged Go data",
		Tagline: "Build helper stderr does not corrupt JSON build data.",
	}
}

func FeaturedCopyWithErrorForBuild() (FeaturedCopy, error) {
	return FeaturedCopy{
		Title:   "Checked Go data",
		Tagline: "Build helpers can return a value and error.",
	}, nil
}

func StaticCopyWithParamsForBuild(params gowdk.BuildParams) FeaturedCopy {
	if len(params.Route) != 0 {
		return FeaturedCopy{
			Title:   "Unexpected route params",
			Tagline: fmt.Sprintf("got %d params", len(params.Route)),
		}
	}
	return FeaturedCopy{
		Title:   "Static Go params",
		Tagline: "Static pages receive empty BuildParams.",
	}
}

type PostCopy struct {
	Title     string `json:"title"`
	Canonical string `json:"canonical"`
}

func PostCopyForBuild(params gowdk.BuildParams) PostCopy {
	slug, _ := params.Param("slug")
	return PostCopy{
		Title:     "Post " + slug,
		Canonical: "/go-post/" + slug,
	}
}
