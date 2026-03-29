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

func TestRenderPagesTypesModuleIncludesUnionAndMetadata(t *testing.T) {
	module := renderPagesTypesModule([]webflow.Page{
		{ID: "page-2", Title: "Docs", Slug: "docs"},
		{ID: "page-1", Title: "Home", Slug: "home"},
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
}

func TestRenderPagesTypesModuleFallsBackToStringWhenEmpty(t *testing.T) {
	module := renderPagesTypesModule(nil)
	if !strings.Contains(module, `export type WfPage = string`) {
		t.Fatalf("expected string fallback, got:\n%s", module)
	}
}
