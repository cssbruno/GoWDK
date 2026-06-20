module github.com/cssbruno/gowdk-page

go 1.26.4

require github.com/cssbruno/gowdk v0.7.0

require github.com/yuin/goldmark v1.7.12

// The docs site lives inside the GOWDK monorepo and is built against the
// in-tree framework HEAD, not the published release, so the site always
// reflects the current sources it documents.
replace github.com/cssbruno/gowdk => ../
