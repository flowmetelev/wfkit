// internal/initialize/steps/directories.go
package steps

import (
	"fmt"
	"os"
	"path/filepath"
)

func CreateDirectories(pagesDir string) error {
	dirs := []string{
		"src",
		filepath.Join("src", "generated"),
		filepath.Join("src", "features"),
		filepath.Join("src", "global"),
		filepath.Join("src", "global", "modules"),
		pagesDir,
		filepath.Join("src", "utils"),
		"build",
		"docs",
		filepath.Join("dist", "assets"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %v", dir, err)
		}
	}
	return nil
}
