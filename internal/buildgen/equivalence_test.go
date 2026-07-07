package buildgen

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkanalysis"
	"github.com/cssbruno/gowdk/internal/gwdkir"
)

type incrementalEquivalenceScenario struct {
	Name    string
	Initial gwdkanalysis.Sources
	Steps   []incrementalEquivalenceStep
}

type incrementalEquivalenceStep struct {
	Name                string
	ChangedPageSources  []string
	Mutate              func(*gwdkanalysis.Sources)
	PrepareCleanFixture func(t *testing.T)
}

func (scenario incrementalEquivalenceScenario) Assert(t *testing.T) {
	t.Helper()
	t.Run(scenario.Name, func(t *testing.T) {
		t.Helper()
		root := t.TempDir()
		incrementalDir := filepath.Join(root, "incremental", "site")
		state := cloneSources(t, scenario.Initial)
		if _, err := Build(gowdk.Config{}, state, incrementalDir); err != nil {
			t.Fatalf("initial clean build failed: %v", err)
		}

		for index, step := range scenario.Steps {
			if step.Mutate == nil {
				t.Fatalf("step %d %q is missing a mutation", index+1, step.Name)
			}
			step.Mutate(&state)
			if step.PrepareCleanFixture != nil {
				step.PrepareCleanFixture(t)
			}
			if _, err := BuildIncremental(gowdk.Config{}, state, incrementalDir, step.ChangedPageSources); err != nil {
				t.Fatalf("incremental build after step %d %q failed: %v", index+1, step.Name, err)
			}
			cleanDir := filepath.Join(root, fmt.Sprintf("clean-%02d", index+1), "site")
			if _, err := Build(gowdk.Config{}, state, cleanDir); err != nil {
				t.Fatalf("clean build after step %d %q failed: %v", index+1, step.Name, err)
			}
			assertEquivalentBuildTrees(t, incrementalDir, cleanDir, fmt.Sprintf("after step %d %q", index+1, step.Name))
		}
	})
}

func TestIncrementalBuildMatchesCleanBuildEquivalence(t *testing.T) {
	projectRoot := t.TempDir()
	homeSource := filepath.Join(projectRoot, "pages", "home.page.gwdk")
	docsSource := filepath.Join(projectRoot, "pages", "docs.page.gwdk")
	heroSource := filepath.Join(projectRoot, "components", "hero.cmp.gwdk")
	layoutSource := filepath.Join(projectRoot, "layouts", "shell.layout.gwdk")
	heroAsset := filepath.Join(projectRoot, "components", "hero.txt")
	if err := os.MkdirAll(filepath.Dir(heroAsset), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, heroAsset, "hero asset v1\n")

	base := gwdkanalysis.Sources{
		Pages: []gwdkir.Page{
			{
				Source:  homeSource,
				Package: "app",
				ID:      "home",
				Route:   "/old-home",
				Layouts: []string{"shell"},
				Blocks: gwdkir.Blocks{
					View:     true,
					ViewBody: `<main><Hero /></main>`,
				},
			},
			{
				Source:  docsSource,
				Package: "app",
				ID:      "docs",
				Route:   "/docs",
				Blocks: gwdkir.Blocks{
					View:     true,
					ViewBody: `<main>Docs stable</main>`,
				},
			},
		},
		Components: []gwdkir.Component{{
			Source:  heroSource,
			Package: "app",
			Name:    "Hero",
			Assets:  []string{"./hero.txt"},
			Blocks: gwdkir.Blocks{
				View:     true,
				ViewBody: `<section>Hero v1</section>`,
			},
		}},
		Layouts: []gwdkir.Layout{{
			Source:  layoutSource,
			Package: "app",
			ID:      "shell",
			Blocks: gwdkir.Blocks{
				View:     true,
				ViewBody: `<body><slot /></body>`,
			},
		}},
	}

	incrementalEquivalenceScenario{
		Name:    "route component asset and layout edits converge",
		Initial: base,
		Steps: []incrementalEquivalenceStep{
			{
				Name:               "rename route and rewrite page body",
				ChangedPageSources: []string{homeSource},
				Mutate: func(sources *gwdkanalysis.Sources) {
					sources.Pages[0].Route = "/home"
					sources.Pages[0].Blocks.ViewBody = `<main><Hero />Home v2</main>`
				},
			},
			{
				Name:               "change component asset and transitive page output",
				ChangedPageSources: []string{homeSource},
				PrepareCleanFixture: func(t *testing.T) {
					t.Helper()
					writeFile(t, heroAsset, "hero asset v2\n")
				},
				Mutate: func(sources *gwdkanalysis.Sources) {
					sources.Components[0].Blocks.ViewBody = `<section>Hero v2</section>`
				},
			},
			{
				Name:               "remove layout from page",
				ChangedPageSources: []string{homeSource},
				Mutate: func(sources *gwdkanalysis.Sources) {
					sources.Pages[0].Layouts = nil
					sources.Pages[0].Blocks.ViewBody = `<main><Hero />Home v3</main>`
				},
			},
		},
	}.Assert(t)
}

type buildTreeEntry struct {
	contents []byte
	mode     fs.FileMode
}

func assertEquivalentBuildTrees(t *testing.T, incrementalDir string, cleanDir string, context string) {
	t.Helper()
	incremental := snapshotBuildTree(t, incrementalDir)
	clean := snapshotBuildTree(t, cleanDir)
	for rel := range incremental {
		if _, ok := clean[rel]; !ok {
			t.Fatalf("%s: file only in incremental output: %s", context, rel)
		}
	}
	for rel := range clean {
		if _, ok := incremental[rel]; !ok {
			t.Fatalf("%s: file only in clean output: %s", context, rel)
		}
	}
	var rels []string
	for rel := range incremental {
		rels = append(rels, rel)
	}
	sort.Strings(rels)
	for _, rel := range rels {
		left := incremental[rel]
		right := clean[rel]
		if left.mode != right.mode {
			t.Fatalf("%s: mode mismatch for %s: incremental=%s clean=%s", context, rel, left.mode, right.mode)
		}
		if !bytes.Equal(left.contents, right.contents) {
			t.Fatalf("%s: byte mismatch for %s: %s", context, rel, firstByteDifference(left.contents, right.contents))
		}
	}
}

func snapshotBuildTree(t *testing.T, outputDir string) map[string]buildTreeEntry {
	t.Helper()
	out := map[string]buildTreeEntry{}
	collectBuildTree(t, out, outputDir, outputDir, "")
	reportPath, err := securityManifestPath(outputDir)
	if err != nil {
		t.Fatal(err)
	}
	reportRoot := filepath.Dir(reportPath)
	collectBuildTree(t, out, reportRoot, reportRoot, filepath.ToSlash(filepath.Join(".gowdk", "reports", filepath.Base(outputDir))))
	return out
}

func collectBuildTree(t *testing.T, out map[string]buildTreeEntry, root string, outputDir string, prefix string) {
	t.Helper()
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return
	} else if err != nil {
		t.Fatal(err)
	}
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if prefix != "" {
			rel = filepath.ToSlash(filepath.Join(prefix, rel))
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if filepath.Base(path) == buildReportFile {
			contents, err = canonicalBuildReport(contents)
			if err != nil {
				return err
			}
		}
		out[rel] = buildTreeEntry{contents: contents, mode: info.Mode().Perm()}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func canonicalBuildReport(payload []byte) ([]byte, error) {
	var report BuildReport
	if err := json.Unmarshal(payload, &report); err != nil {
		return nil, fmt.Errorf("canonicalize build report: %w", err)
	}
	report.Mode = "equivalence"
	report.OutputDir = "@OUTPUT_DIR@"
	events := make([]BuildEvent, 0, len(report.Events))
	for _, event := range report.Events {
		if !buildReportEquivalenceEvent(event) {
			continue
		}
		event.Path = canonicalReportPath(event.Path)
		events = append(events, event)
	}
	report.Events = events
	out, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

func buildReportEquivalenceEvent(event BuildEvent) bool {
	if event.Level == BuildEventDebug {
		return false
	}
	switch event.Stage {
	case "bind", "seo":
		return true
	case "report":
		return event.Kind == "cache_policy" || event.Kind == "asset_size"
	default:
		return false
	}
}

func canonicalReportPath(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	path = filepath.ToSlash(path)
	if index := strings.Index(path, "/site/"); index >= 0 {
		return "@OUTPUT_DIR@/" + strings.TrimPrefix(path[index+len("/site/"):], "/")
	}
	if index := strings.Index(path, "/.gowdk/reports/site/"); index >= 0 {
		return "@REPORT_DIR@/" + strings.TrimPrefix(path[index+len("/.gowdk/reports/site/"):], "/")
	}
	return path
}

func firstByteDifference(left []byte, right []byte) string {
	limit := len(left)
	if len(right) < limit {
		limit = len(right)
	}
	for i := 0; i < limit; i++ {
		if left[i] != right[i] {
			return fmt.Sprintf("first difference at byte %d: incremental=%q clean=%q", i, excerptAt(left, i), excerptAt(right, i))
		}
	}
	return fmt.Sprintf("length differs: incremental=%d clean=%d", len(left), len(right))
}

func excerptAt(payload []byte, index int) string {
	start := index - 24
	if start < 0 {
		start = 0
	}
	end := index + 24
	if end > len(payload) {
		end = len(payload)
	}
	return string(payload[start:end])
}

func cloneSources(t *testing.T, sources gwdkanalysis.Sources) gwdkanalysis.Sources {
	t.Helper()
	payload, err := json.Marshal(sources)
	if err != nil {
		t.Fatal(err)
	}
	var cloned gwdkanalysis.Sources
	if err := json.Unmarshal(payload, &cloned); err != nil {
		t.Fatal(err)
	}
	return cloned
}
