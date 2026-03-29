package main

import (
	"flag"
	"strings"
	"testing"

	"wfkit/internal/webflow"

	"github.com/urfave/cli/v2"
)

func TestNormalizePageSlug(t *testing.T) {
	if got := normalizePageSlug("Docs Hub / Getting Started"); got != "docs-hub-getting-started" {
		t.Fatalf("unexpected slug: %q", got)
	}
}

func TestDeveloperPageSlugFallsBackToNormalizedTitle(t *testing.T) {
	page := webflow.Page{Title: "Home", Slug: ""}
	if got := developerPageSlug(page); got != "home" {
		t.Fatalf("expected fallback slug home, got %q", got)
	}
}

func TestRenderPagesTypesModuleIncludesUnionAndMetadata(t *testing.T) {
	module := renderPagesTypesModule([]webflow.Page{
		{ID: "page-2", Title: "Docs", Slug: "docs"},
		{ID: "page-1", Title: "Home", Slug: ""},
	})

	if !strings.Contains(module, `export type WfPage = (typeof wfPages)[number]`) {
		t.Fatalf("expected WfPage type, got:\n%s", module)
	}
	if !strings.Contains(module, `"docs"`) || !strings.Contains(module, `"home"`) {
		t.Fatalf("expected page slugs in generated module, got:\n%s", module)
	}
	if !strings.Contains(module, `"docs": { slug: "docs", title: "Docs", id: "page-2" }`) {
		t.Fatalf("expected docs metadata entry, got:\n%s", module)
	}
	if !strings.Contains(module, `export const wfPageSelectors: Record<WfPage, string>`) {
		t.Fatalf("expected page selectors contract, got:\n%s", module)
	}
	if !strings.Contains(module, `"docs": "[data-page=\"docs\"]"`) {
		t.Fatalf("expected typed page selector for docs, got:\n%s", module)
	}
	if !strings.Contains(module, `"docs": "[data-wf-docs-root]"`) {
		t.Fatalf("expected typed page root selector for docs, got:\n%s", module)
	}
	if !strings.Contains(module, `"home": { slug: "home", title: "Home", id: "page-1" }`) {
		t.Fatalf("expected home metadata entry via title fallback, got:\n%s", module)
	}
	if !strings.Contains(module, `export const wfGlobalSelectors =`) {
		t.Fatalf("expected global selector contracts, got:\n%s", module)
	}
}

func TestRenderPagesTypesModuleFallsBackToStringWhenEmpty(t *testing.T) {
	module := renderPagesTypesModule(nil)
	if !strings.Contains(module, `export type WfPage = string`) {
		t.Fatalf("expected string fallback, got:\n%s", module)
	}
	if !strings.Contains(module, `siteRoot: '[data-wf-site-root]'`) {
		t.Fatalf("expected default global selector contracts, got:\n%s", module)
	}
}

func TestManagedScriptIDsDeduplicatesAndSorts(t *testing.T) {
	got := managedScriptIDs(`
		<script data-script-id="docs-hub"></script>
		<script data-script-id="global-script"></script>
		<script data-script-id="docs-hub"></script>
	`)

	if len(got) != 2 || got[0] != "docs-hub" || got[1] != "global-script" {
		t.Fatalf("unexpected managed script ids: %#v", got)
	}
}

func TestResolveTargetPageSupportsSlugFallbackAndID(t *testing.T) {
	app := &cli.App{}
	set := flag.NewFlagSet("wfkit pages inspect", flag.ContinueOnError)
	_ = set.String("slug", "", "")
	_ = set.String("id", "", "")
	if err := set.Parse([]string{"--slug", "home"}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	flow := newPagesFlow(cli.NewContext(app, set, nil))
	flow.pages = []webflow.Page{
		{ID: "page-home", Title: "Home", Slug: ""},
		{ID: "page-docs", Title: "Docs", Slug: "docs"},
	}

	page, err := flow.resolveTargetPage()
	if err != nil {
		t.Fatalf("resolveTargetPage by slug: %v", err)
	}
	if page.ID != "page-home" {
		t.Fatalf("expected home page by normalized title fallback, got %q", page.ID)
	}

	set = flag.NewFlagSet("wfkit pages inspect", flag.ContinueOnError)
	_ = set.String("slug", "", "")
	_ = set.String("id", "", "")
	if err := set.Parse([]string{"--id", "page-docs"}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	flow = newPagesFlow(cli.NewContext(app, set, nil))
	flow.pages = []webflow.Page{
		{ID: "page-home", Title: "Home", Slug: ""},
		{ID: "page-docs", Title: "Docs", Slug: "docs"},
	}

	page, err = flow.resolveTargetPage()
	if err != nil {
		t.Fatalf("resolveTargetPage by id: %v", err)
	}
	if page.ID != "page-docs" {
		t.Fatalf("expected docs page by id, got %q", page.ID)
	}
}

func TestResolveTargetPageUpdateUsesPageSlugSelector(t *testing.T) {
	app := &cli.App{}
	set := flag.NewFlagSet("wfkit pages update", flag.ContinueOnError)
	_ = set.String("page-slug", "", "")
	_ = set.String("id", "", "")
	_ = set.String("slug", "", "")
	if err := set.Parse([]string{"--page-slug", "docs", "--slug", "docs-updated"}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	flow := newPagesFlow(cli.NewContext(app, set, nil))
	flow.pages = []webflow.Page{
		{ID: "page-home", Title: "Home", Slug: ""},
		{ID: "page-docs", Title: "Docs", Slug: "docs"},
	}

	page, err := flow.resolveTargetPageUpdate()
	if err != nil {
		t.Fatalf("resolveTargetPageUpdate: %v", err)
	}
	if page.ID != "page-docs" {
		t.Fatalf("expected docs page, got %q", page.ID)
	}
}

func TestBuildUpdatedPageAppliesMetadataFlags(t *testing.T) {
	app := &cli.App{}
	set := flag.NewFlagSet("wfkit pages update", flag.ContinueOnError)
	_ = set.String("title", "", "")
	_ = set.String("slug", "", "")
	_ = set.String("seo-title", "", "")
	_ = set.String("seo-description", "", "")
	_ = set.String("search-title", "", "")
	_ = set.String("search-description", "", "")
	_ = set.String("canonical-url", "", "")
	_ = set.Bool("include-in-sitemap", false, "")
	_ = set.Bool("exclude-from-sitemap", false, "")
	_ = set.Bool("exclude-from-search", false, "")
	if err := set.Parse([]string{
		"--title", "Docs Hub",
		"--slug", "docs-hub",
		"--seo-title", "SEO Docs",
		"--seo-description", "SEO Description",
		"--search-title", "Search Docs",
		"--search-description", "Search Description",
		"--canonical-url", "https://example.com/docs",
		"--exclude-from-sitemap",
		"--exclude-from-search",
	}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	flow := newPagesFlow(cli.NewContext(app, set, nil))
	updated := flow.buildUpdatedPage(webflow.Page{
		Title:             "Docs",
		Slug:              "docs",
		IncludeInSitemap:  true,
		SearchExclude:     false,
		SEOTitle:          "Old SEO",
		SEODescription:    "Old Desc",
		SearchTitle:       "Old Search",
		SearchDescription: "Old Search Desc",
	})

	if updated.Title != "Docs Hub" || updated.Slug != "docs-hub" {
		t.Fatalf("unexpected title/slug update: %#v", updated)
	}
	if updated.SEOTitle != "SEO Docs" || updated.SEODescription != "SEO Description" {
		t.Fatalf("unexpected SEO fields: %#v", updated)
	}
	if updated.SearchTitle != "Search Docs" || updated.SearchDescription != "Search Description" {
		t.Fatalf("unexpected search fields: %#v", updated)
	}
	if updated.CanonicalURL == nil || *updated.CanonicalURL != "https://example.com/docs" {
		t.Fatalf("unexpected canonical URL: %#v", updated.CanonicalURL)
	}
	if updated.IncludeInSitemap {
		t.Fatalf("expected sitemap exclusion")
	}
	if !updated.SearchExclude {
		t.Fatalf("expected search exclusion")
	}
}

func TestBuildUpdatedPagePayloadPreservesRawFieldsAndAppliesMetadata(t *testing.T) {
	flow := &pagesFlow{}
	canonicalURL := "https://example.com/docs"
	payload := flow.buildUpdatedPagePayload(webflow.Page{
		ID:                "page-123",
		Title:             "Docs Hub",
		Slug:              "docs-hub",
		SEOTitle:          "SEO Docs",
		SEODescription:    "SEO Description",
		SearchTitle:       "Search Docs",
		SearchDescription: "Search Description",
		CanonicalURL:      &canonicalURL,
		IncludeInSitemap:  false,
		SearchExclude:     true,
	}, map[string]interface{}{
		"_id":              "page-123",
		"title":            "Docs",
		"slug":             "docs",
		"type":             "Static",
		"site":             "site-1",
		"includeInSitemap": true,
		"searchExclude":    false,
		"nested": map[string]interface{}{
			"foo": "bar",
		},
	})

	if payload["title"] != "Docs Hub" || payload["slug"] != "docs-hub" {
		t.Fatalf("unexpected updated title/slug payload: %#v", payload)
	}
	if payload["seoTitle"] != "SEO Docs" || payload["seoDesc"] != "SEO Description" {
		t.Fatalf("unexpected SEO payload: %#v", payload)
	}
	if payload["type"] != "Static" || payload["site"] != "site-1" {
		t.Fatalf("expected raw metadata to be preserved: %#v", payload)
	}
	nested, ok := payload["nested"].(map[string]interface{})
	if !ok || nested["foo"] != "bar" {
		t.Fatalf("expected nested raw metadata to be preserved: %#v", payload["nested"])
	}
}

func TestRawPageByIDReturnsClonedPayload(t *testing.T) {
	flow := &pagesFlow{
		rawPages: []map[string]interface{}{
			{
				"_id":   "page-123",
				"title": "Docs",
				"nested": map[string]interface{}{
					"foo": "bar",
				},
			},
		},
	}

	page, err := flow.rawPageByID("page-123")
	if err != nil {
		t.Fatalf("rawPageByID returned error: %v", err)
	}
	nested := page["nested"].(map[string]interface{})
	nested["foo"] = "baz"

	originalNested := flow.rawPages[0]["nested"].(map[string]interface{})
	if originalNested["foo"] != "bar" {
		t.Fatalf("expected raw page payload to be cloned, got %#v", flow.rawPages[0])
	}
}

func TestPublishedPageURLUsesSiteRootForHomeAndSlugForOtherPages(t *testing.T) {
	siteURL := "https://demo.webflow.io/"

	if got := publishedPageURL(siteURL, webflow.Page{Title: "Home"}); got != "https://demo.webflow.io" {
		t.Fatalf("expected site root for home page, got %q", got)
	}

	if got := publishedPageURL(siteURL, webflow.Page{Title: "Docs", Slug: "docs"}); got != "https://demo.webflow.io/docs" {
		t.Fatalf("expected docs page URL, got %q", got)
	}
}
