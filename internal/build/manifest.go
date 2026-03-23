package build

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const manifestFileName = "wfkit-manifest.json"

type Manifest struct {
	Global string            `json:"global"`
	Pages  map[string]string `json:"pages"`
}

func ReadManifest(buildDir string) (Manifest, error) {
	path := filepath.Join(buildDir, manifestFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("failed to parse %s: %w", path, err)
	}
	if manifest.Pages == nil {
		manifest.Pages = map[string]string{}
	}
	return manifest, nil
}

func ResolveGlobalEntry(buildDir string) (string, error) {
	manifest, err := ReadManifest(buildDir)
	if err == nil && manifest.Global != "" {
		return normalizeManifestPath(manifest.Global), nil
	}

	latestJS, findErr := findLatestJSFile(buildDir)
	if findErr != nil {
		if err != nil {
			return "", err
		}
		return "", findErr
	}

	return normalizeManifestPath(latestJS), nil
}

func ResolvePageEntries(buildDir string) (map[string]string, error) {
	manifest, err := ReadManifest(buildDir)
	if err != nil {
		return nil, err
	}

	result := make(map[string]string, len(manifest.Pages))
	for key, value := range manifest.Pages {
		result[key] = normalizeManifestPath(value)
	}

	return result, nil
}

func normalizeManifestPath(value string) string {
	return strings.TrimPrefix(filepath.ToSlash(value), "/")
}
