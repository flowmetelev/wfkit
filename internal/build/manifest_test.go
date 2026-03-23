package build

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadManifestAndResolveEntries(t *testing.T) {
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, manifestFileName)
	content := `{
  "global": "global/index-abc123.js",
  "pages": {
    "home": "pages/home/index-def456.js",
    "pricing/enterprise": "pages/pricing/enterprise/index-ghi789.js"
  }
}`

	if err := os.WriteFile(manifestPath, []byte(content), 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	globalEntry, err := ResolveGlobalEntry(tmpDir)
	if err != nil {
		t.Fatalf("ResolveGlobalEntry: %v", err)
	}
	if globalEntry != "global/index-abc123.js" {
		t.Fatalf("unexpected global entry: %q", globalEntry)
	}

	pages, err := ResolvePageEntries(tmpDir)
	if err != nil {
		t.Fatalf("ResolvePageEntries: %v", err)
	}
	if pages["home"] != "pages/home/index-def456.js" {
		t.Fatalf("unexpected home page entry: %q", pages["home"])
	}
	if pages["pricing/enterprise"] != "pages/pricing/enterprise/index-ghi789.js" {
		t.Fatalf("unexpected nested page entry: %q", pages["pricing/enterprise"])
	}
}
