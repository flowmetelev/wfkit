package main

import (
	"testing"

	"wfkit/internal/webflow"
)

func TestInteractivePageOptionsIncludeSortedPagesAndBack(t *testing.T) {
	options := interactivePageOptions([]webflow.Page{
		{ID: "page-docs", Title: "Docs", Slug: "docs"},
		{ID: "page-home", Title: "Home", Slug: ""},
	})

	if len(options) != 3 {
		t.Fatalf("expected 3 options, got %d", len(options))
	}
	if options[0].Key != "docs  Docs" || options[0].Value != "page-docs" {
		t.Fatalf("unexpected first option: %#v", options[0])
	}
	if options[1].Key != "home  Home" || options[1].Value != "page-home" {
		t.Fatalf("unexpected second option: %#v", options[1])
	}
	if options[2].Key != "Back" || options[2].Value != "" {
		t.Fatalf("unexpected back option: %#v", options[2])
	}
}

func TestPageOptionLabelUsesDeveloperSlugFallback(t *testing.T) {
	label := pageOptionLabel(webflow.Page{Title: "Home"})
	if label != "home  Home" {
		t.Fatalf("unexpected page label: %q", label)
	}
}
