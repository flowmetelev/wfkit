package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"wfkit/internal/webflow"
)

func TestCMSCollectionSlugFallsBackToName(t *testing.T) {
	collection := webflow.CMSCollection{Name: "Blog Posts"}
	if got := cmsCollectionSlug(collection); got != "blog-posts" {
		t.Fatalf("expected blog-posts, got %q", got)
	}
}

func TestCMSItemFileStemPrefersFieldDataSlug(t *testing.T) {
	item := map[string]interface{}{
		"_id": "item-1",
		"fieldData": map[string]interface{}{
			"slug": "hello-world",
		},
	}
	if got := cmsItemFileStem(item); got != "hello-world" {
		t.Fatalf("expected hello-world, got %q", got)
	}
}

func TestWriteJSONFileFormatsWithTrailingNewline(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "cms", "schema.json")

	if err := writeJSONFile(path, map[string]interface{}{"slug": "docs"}); err != nil {
		t.Fatalf("writeJSONFile returned error: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if !strings.HasSuffix(string(content), "\n") {
		t.Fatalf("expected trailing newline, got %q", string(content))
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(content, &decoded); err != nil {
		t.Fatalf("decode file: %v", err)
	}
	if decoded["slug"] != "docs" {
		t.Fatalf("expected docs slug, got %#v", decoded)
	}
}

func TestBuildCMSCollectionPlanMatchesBySlugAndDetectsChanges(t *testing.T) {
	collection := webflow.CMSCollection{
		ID:   "coll-1",
		Slug: "articles",
		Fields: []webflow.CMSField{
			{Slug: "name"},
			{Slug: "slug"},
			{Slug: "summary"},
		},
	}

	localItems := []cmsLocalItem{
		{
			Path:   "articles/items/hello.json",
			ID:     "item-1",
			Slug:   "hello",
			Name:   "Hello",
			Fields: map[string]interface{}{"name": "Hello", "slug": "hello", "summary": "New copy"},
		},
		{
			Path:   "articles/items/new-item.json",
			Slug:   "new-item",
			Name:   "New item",
			Fields: map[string]interface{}{"name": "New item", "slug": "new-item"},
		},
	}

	remoteItems := []map[string]interface{}{
		{"_id": "item-1", "name": "Hello", "slug": "hello", "summary": "Old copy"},
		{"_id": "item-2", "name": "Delete me", "slug": "delete-me"},
	}

	plan := buildCMSCollectionPlan(collection, localItems, remoteItems, true)
	if got := len(plan.Create); got != 1 {
		t.Fatalf("expected 1 create, got %d", got)
	}
	if got := len(plan.Update); got != 1 {
		t.Fatalf("expected 1 update, got %d", got)
	}
	if got := len(plan.Delete); got != 1 {
		t.Fatalf("expected 1 delete, got %d", got)
	}
	if got := len(plan.Unchanged); got != 0 {
		t.Fatalf("expected 0 unchanged, got %d", got)
	}
}

func TestCMSMutationFieldsPrefersFieldDataAndWritableSystemFields(t *testing.T) {
	collection := webflow.CMSCollection{
		Fields: []webflow.CMSField{
			{Slug: "name", Editable: true},
			{Slug: "slug", Editable: true},
			{Slug: "summary", Editable: true},
			{Slug: "_archived", Editable: true},
			{Slug: "created-on", Editable: false},
		},
	}

	fields := cmsMutationFields(map[string]interface{}{
		"_id":        "item-1",
		"_archived":  false,
		"created-on": "ignore-me",
		"fieldData": map[string]interface{}{
			"name":    "Hello",
			"slug":    "hello",
			"summary": "World",
		},
	}, collection)

	if fields["name"] != "Hello" || fields["slug"] != "hello" || fields["summary"] != "World" {
		t.Fatalf("unexpected writable fields: %#v", fields)
	}
	if _, ok := fields["created-on"]; ok {
		t.Fatalf("did not expect readonly metadata in writable fields: %#v", fields)
	}
	if _, ok := fields["_archived"]; !ok {
		t.Fatalf("expected writable system field to be preserved: %#v", fields)
	}
}
