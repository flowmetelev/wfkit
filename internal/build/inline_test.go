package build

import (
	"strings"
	"testing"
)

func TestInlineViteConfigSourceDoesNotImportViteFromTempDir(t *testing.T) {
	source, err := inlineViteConfigSource("/tmp/project/dist/assets/global/index-abc.js", "/tmp/out")
	if err != nil {
		t.Fatalf("inlineViteConfigSource: %v", err)
	}

	if strings.Contains(source, `from "vite"`) {
		t.Fatalf("expected temp inline config to avoid importing vite, got %s", source)
	}
	if !strings.Contains(source, "export default {") {
		t.Fatalf("expected plain exported config object, got %s", source)
	}
	if !strings.Contains(source, `inlineDynamicImports: true`) {
		t.Fatalf("expected inline dynamic imports config, got %s", source)
	}
}
