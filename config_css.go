package gowdk

// CSSConfig controls discovered CSS inputs and page CSS output.
type CSSConfig struct {
	Include []string
	Exclude []string
	Default []string
	Output  CSSOutputConfig
}

type CSSOutputConfig struct {
	Dir        string
	HrefPrefix string
}
