package main

import (
	"flag"
	"testing"

	"github.com/urfave/cli/v2"
)

func TestBoolString(t *testing.T) {
	if got := boolString(true); got != "true" {
		t.Fatalf("expected true, got %q", got)
	}
	if got := boolString(false); got != "false" {
		t.Fatalf("expected false, got %q", got)
	}
}

func TestInteractivePublishFlowBuildsProdContext(t *testing.T) {
	parent := cli.NewContext(&cli.App{}, flag.NewFlagSet("wfkit", flag.ContinueOnError), nil)
	flow := &interactivePublishFlow{
		parent:       parent,
		byPage:       true,
		dryRun:       true,
		delivery:     "inline",
		target:       "production",
		assetBranch:  "wfkit-dist",
		buildDir:     "dist/assets",
		customCommit: "Ship build",
		update:       true,
		notify:       true,
	}

	ctx := flow.newContext()
	if ctx.String("env") != "prod" {
		t.Fatalf("expected prod env, got %q", ctx.String("env"))
	}
	if !ctx.Bool("by-page") || !ctx.Bool("dry-run") || !ctx.Bool("update") || !ctx.Bool("notify") {
		t.Fatal("expected publish context bool flags to be set")
	}
	if ctx.String("delivery") != "inline" || ctx.String("target") != "production" {
		t.Fatalf("unexpected publish context: delivery=%q target=%q", ctx.String("delivery"), ctx.String("target"))
	}
}

func TestInteractiveMigrateFlowBuildsPublishContext(t *testing.T) {
	parent := cli.NewContext(&cli.App{}, flag.NewFlagSet("wfkit", flag.ContinueOnError), nil)
	flow := &interactiveMigrateFlow{
		parent:       parent,
		pagesDir:     "src/pages",
		force:        true,
		dryRun:       false,
		publish:      true,
		delivery:     "cdn",
		target:       "all",
		assetBranch:  "wfkit-dist",
		buildDir:     "dist/assets",
		customCommit: "Migrate code",
		notify:       true,
	}

	ctx := flow.newContext()
	if ctx.String("pages-dir") != "src/pages" {
		t.Fatalf("unexpected pages-dir: %q", ctx.String("pages-dir"))
	}
	if !ctx.Bool("force") || !ctx.Bool("publish") || !ctx.Bool("notify") {
		t.Fatal("expected migrate context bool flags to be set")
	}
	if ctx.String("target") != "all" {
		t.Fatalf("unexpected target: %q", ctx.String("target"))
	}
}

func TestInteractiveDocsFlowBuildsContext(t *testing.T) {
	parent := cli.NewContext(&cli.App{}, flag.NewFlagSet("wfkit", flag.ContinueOnError), nil)
	flow := &interactiveDocsFlow{
		parent:   parent,
		file:     "docs/index.md",
		pageSlug: "docs",
		selector: "[data-wf-docs-root]",
		dryRun:   true,
		publish:  false,
		notify:   true,
	}

	ctx := flow.newContext()
	if ctx.String("file") != "docs/index.md" || ctx.String("page-slug") != "docs" {
		t.Fatalf("unexpected docs context values: file=%q slug=%q", ctx.String("file"), ctx.String("page-slug"))
	}
	if !ctx.Bool("dry-run") || ctx.Bool("publish") || !ctx.Bool("notify") {
		t.Fatal("unexpected docs context flags")
	}
}

func TestPublishedDocsURLUsesPublishedPageRules(t *testing.T) {
	if got := publishedDocsURL("https://demo.webflow.io", "docs"); got != "https://demo.webflow.io/docs" {
		t.Fatalf("unexpected docs URL: %q", got)
	}
	if got := publishedDocsURL("https://demo.webflow.io", "home"); got != "https://demo.webflow.io" {
		t.Fatalf("unexpected home docs URL: %q", got)
	}
}
