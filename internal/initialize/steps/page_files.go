package steps

import (
	"fmt"
	"os"
	"path/filepath"

	"wfkit/internal/initialize/utils"
)

func CreatePageFiles(pagesDir string) error {
	pagePath := filepath.Join(pagesDir, "home", "index.ts")
	if err := os.MkdirAll(filepath.Dir(pagePath), 0755); err != nil {
		return fmt.Errorf("failed to create page directory %s: %w", filepath.Dir(pagePath), err)
	}
	if err := utils.RenderTemplateToFile("page-entry.ts.tmpl", nil, pagePath); err != nil {
		return fmt.Errorf("failed to create %s: %w", pagePath, err)
	}
	return nil
}
