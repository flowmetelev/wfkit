package publish

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"wfkit/internal/webflow"
)

func TestRenderDocsHubHTMLBuildsLayoutAndTitle(t *testing.T) {
	t.Helper()

	title, html, err := renderDocsHubHTML([]byte("# Docs\n\n## Install\n\nRun `wfkit docs`\n"))
	if err != nil {
		t.Fatalf("renderDocsHubHTML returned error: %v", err)
	}

	if title != "Docs" {
		t.Fatalf("expected title Docs, got %q", title)
	}
	if !strings.Contains(html, `class="wf-docs-shell"`) {
		t.Fatalf("expected docs shell layout, got: %s", html)
	}
	if !strings.Contains(html, `href="#install"`) {
		t.Fatalf("expected generated TOC link, got: %s", html)
	}
}

func TestPlanDocsHubSyncUsesManagedScriptBlock(t *testing.T) {
	t.Helper()

	tmpDir := t.TempDir()
	entryPath := filepath.Join(tmpDir, "docs", "index.md")
	if err := os.MkdirAll(filepath.Dir(entryPath), 0o755); err != nil {
		t.Fatalf("mkdir docs dir: %v", err)
	}
	if err := os.WriteFile(entryPath, []byte("# Product Docs\n\nHello\n"), 0o644); err != nil {
		t.Fatalf("write docs entry: %v", err)
	}

	plan, err := PlanDocsHubSync([]webflow.Page{
		{ID: "page-1", Title: "Docs", Slug: "docs", PostBody: `<script data-script-id="docs-hub">old()</script><script>keep()</script>`},
	}, DocsHubOptions{EntryPath: entryPath, PageSlug: "docs"})
	if err != nil {
		t.Fatalf("PlanDocsHubSync returned error: %v", err)
	}

	if plan.Action != "update" {
		t.Fatalf("expected update action, got %s", plan.Action)
	}
	if !strings.Contains(plan.NextPostBody, `data-script-id="docs-hub"`) {
		t.Fatalf("expected docs-hub managed block, got: %s", plan.NextPostBody)
	}
	if !strings.Contains(plan.NextPostBody, `<script>keep()</script>`) {
		t.Fatalf("expected unmanaged page code to be preserved, got: %s", plan.NextPostBody)
	}
}

func TestPlanDocsHubSyncCreatesMissingPagePlan(t *testing.T) {
	t.Helper()

	tmpDir := t.TempDir()
	entryPath := filepath.Join(tmpDir, "docs", "index.md")
	if err := os.MkdirAll(filepath.Dir(entryPath), 0o755); err != nil {
		t.Fatalf("mkdir docs dir: %v", err)
	}
	if err := os.WriteFile(entryPath, []byte("# WFKit Docs\n\nHello\n"), 0o644); err != nil {
		t.Fatalf("write docs entry: %v", err)
	}

	plan, err := PlanDocsHubSync(nil, DocsHubOptions{EntryPath: entryPath, PageSlug: "docs"})
	if err != nil {
		t.Fatalf("PlanDocsHubSync returned error: %v", err)
	}

	if plan.Action != "create" {
		t.Fatalf("expected create action, got %s", plan.Action)
	}
	if !plan.CreatePage {
		t.Fatalf("expected CreatePage to be true")
	}
	if plan.PageID != "" {
		t.Fatalf("expected empty page id before creation, got %q", plan.PageID)
	}
	if plan.PageTitle != "WFKit Docs" {
		t.Fatalf("expected page title from markdown heading, got %q", plan.PageTitle)
	}
	if plan.PageSlug != "docs" {
		t.Fatalf("expected docs slug, got %q", plan.PageSlug)
	}
	if !strings.Contains(plan.NextPostBody, `data-script-id="docs-hub"`) {
		t.Fatalf("expected docs-hub managed block for new page, got: %s", plan.NextPostBody)
	}
}
