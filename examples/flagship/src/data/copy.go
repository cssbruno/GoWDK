package data

type HomeCopy struct {
	Eyebrow   string `json:"eyebrow"`
	Title     string `json:"title"`
	Tagline   string `json:"tagline"`
	BuildNote string `json:"buildNote"`
}

func HomeCopyForBuild() HomeCopy {
	return HomeCopy{
		Eyebrow:   "Native GOWDK",
		Title:     "Full-stack example",
		Tagline:   "One source tree demonstrates static output, request-time Go, fragments, contracts, SSR, and islands.",
		BuildNote: "This copy was returned by Go during the compiler build.",
	}
}

type HybridCopy struct {
	Eyebrow    string `json:"eyebrow"`
	Title      string `json:"title"`
	Tagline    string `json:"tagline"`
	StaticNote string `json:"staticNote"`
}

func HybridCopyForBuild() HybridCopy {
	return HybridCopy{
		Eyebrow:    "Hybrid",
		Title:      "Static shell on a request-time route",
		Tagline:    "The route uses the SSR addon while keeping its copy build-time owned.",
		StaticNote: "No page load function is required for this route.",
	}
}
