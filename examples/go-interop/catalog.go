package gointerop

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
