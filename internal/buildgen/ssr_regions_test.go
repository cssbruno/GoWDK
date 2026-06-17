package buildgen

import "testing"

func TestExtractQueryRegionTemplatesBalancesNestedTags(t *testing.T) {
	html := `<body><section data-gowdk-query="patients.GetPatientPage" data-gowdk-query-type="example.com/app/patients.GetPatientPage">` +
		`<div><div>__GOWDK_SSR_LIST_board__</div></div></section><aside>after</aside></body>`
	regions := extractQueryRegionTemplates(html)
	if len(regions) != 1 {
		t.Fatalf("expected 1 region, got %d", len(regions))
	}
	if regions[0].queryType != "example.com/app/patients.GetPatientPage" {
		t.Fatalf("unexpected query type %q", regions[0].queryType)
	}
	want := `<section data-gowdk-query="patients.GetPatientPage" data-gowdk-query-type="example.com/app/patients.GetPatientPage"><div><div>__GOWDK_SSR_LIST_board__</div></div></section>`
	if regions[0].template != want {
		t.Fatalf("unexpected template:\n got: %q\nwant: %q", regions[0].template, want)
	}
}

func TestExtractQueryRegionTemplatesHandlesVoidSiblings(t *testing.T) {
	html := `<ul data-gowdk-query-type="example.com/app.List"><br><li>__GOWDK_SSR_LIST_x__</li></ul>`
	regions := extractQueryRegionTemplates(html)
	if len(regions) != 1 || regions[0].template != html {
		t.Fatalf("expected whole element, got %+v", regions)
	}
}

func TestSSRQueryRegionsPartitionsSpecsByContainment(t *testing.T) {
	html := `<main>` +
		`<section data-gowdk-query-type="example.com/app.Board"><ul>__GOWDK_SSR_LIST_board__</ul></section>` +
		`<section data-gowdk-query-type="example.com/app.Total"><span>__GOWDK_SSR_LOAD_total__</span></section>` +
		`</main>`
	lists := []SSRListSpec{{Placeholder: "__GOWDK_SSR_LIST_board__", SourcePath: "patients"}}
	loadReplacements := []SSRLoadReplacement{{Path: "total", Placeholder: "__GOWDK_SSR_LOAD_total__"}}

	regions := ssrQueryRegions(html, lists, nil, loadReplacements, nil, false)
	if len(regions) != 2 {
		t.Fatalf("expected 2 regions, got %d", len(regions))
	}
	byType := map[string]SSRQueryRegion{}
	for _, region := range regions {
		byType[region.QueryType] = region
	}
	board := byType["example.com/app.Board"]
	if len(board.ListSpecs) != 1 || len(board.LoadReplacements) != 0 {
		t.Fatalf("board region partition wrong: %+v", board)
	}
	total := byType["example.com/app.Total"]
	if len(total.ListSpecs) != 0 || len(total.LoadReplacements) != 1 {
		t.Fatalf("total region partition wrong: %+v", total)
	}
}

func TestSSRQueryRegionsSkipsParamAndStaticRegions(t *testing.T) {
	html := `<main>` +
		`<section data-gowdk-query-type="example.com/app.Static">no placeholders</section>` +
		`<section data-gowdk-query-type="example.com/app.Param"><span>__GOWDK_SSR_PARAM_page_id__</span><ul>__GOWDK_SSR_LIST_x__</ul></section>` +
		`</main>`
	lists := []SSRListSpec{{Placeholder: "__GOWDK_SSR_LIST_x__", SourcePath: "items"}}
	params := []SSRReplacement{{Param: "id", Placeholder: "__GOWDK_SSR_PARAM_page_id__"}}

	if regions := ssrQueryRegions(html, lists, nil, nil, params, false); len(regions) != 0 {
		t.Fatalf("expected static and param-bound regions to be skipped, got %+v", regions)
	}
}

func TestSSRQueryRegionsSkipsDynamicParamPages(t *testing.T) {
	html := `<section data-gowdk-query-type="example.com/app.Board"><ul>__GOWDK_SSR_LIST_x__</ul></section>`
	lists := []SSRListSpec{{Placeholder: "__GOWDK_SSR_LIST_x__", SourcePath: "items"}}
	if regions := ssrQueryRegions(html, lists, nil, nil, nil, true); regions != nil {
		t.Fatalf("expected nil for pages with dynamic route params, got %+v", regions)
	}
}
