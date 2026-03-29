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

func TestPublishedPageURLUsesSiteRootForHomeAndSlugForOtherPages(t *testing.T) {
	siteURL := "https://demo.webflow.io/"

	if got := publishedPageURL(siteURL, webflow.Page{Title: "Home"}); got != "https://demo.webflow.io" {
		t.Fatalf("expected site root for home page, got %q", got)
	}

	if got := publishedPageURL(siteURL, webflow.Page{Title: "Docs", Slug: "docs"}); got != "https://demo.webflow.io/docs" {
		t.Fatalf("expected docs page URL, got %q", got)
	}
}
