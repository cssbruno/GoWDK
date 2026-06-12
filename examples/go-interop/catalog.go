package gointerop

import (
	"fmt"
	"os"
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
