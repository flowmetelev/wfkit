package main

import (
	"strings"
	"testing"

	"wfkit/internal/webflow"
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
