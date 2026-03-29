package build

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type InlineBundles struct {
	Global string
	Pages  map[string]string
}

func BuildInlineBundles(buildDir, packageManager string) (InlineBundles, error) {
	result := InlineBundles{Pages: map[string]string{}}

	manifest, err := ReadManifest(buildDir)
	if err != nil {
		return result, fmt.Errorf("read build manifest: %w", err)
	}

	if manifest.Global != "" {
		code, err := bundleBuiltEntryInline(buildDir, manifest.Global, packageManager)
		if err != nil {
			return result, fmt.Errorf("bundle inline global entry: %w", err)
		}
		result.Global = code
	}

	pageKeys := make([]string, 0, len(manifest.Pages))
	for key := range manifest.Pages {
		pageKeys = append(pageKeys, key)
	}
	sort.Strings(pageKeys)

	for _, key := range pageKeys {
		code, err := bundleBuiltEntryInline(buildDir, manifest.Pages[key], packageManager)
		if err != nil {
			return result, fmt.Errorf("bundle inline page %s: %w", key, err)
		}
		result.Pages[key] = code
	}

	return result, nil
}

func bundleBuiltEntryInline(buildDir, manifestPath, packageManager string) (string, error) {
	absBuildDir, err := filepath.Abs(buildDir)
	if err != nil {
		return "", fmt.Errorf("resolve build dir: %w", err)
	}
	entryPath := filepath.Join(absBuildDir, filepath.FromSlash(manifestPath))

	tempDir, err := os.MkdirTemp("", "wfkit-inline-build-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	outDir := filepath.Join(tempDir, "out")
	configPath := filepath.Join(tempDir, "vite.inline.config.mjs")
	configSource, err := inlineViteConfigSource(entryPath, outDir)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(configPath, []byte(configSource), 0o644); err != nil {
		return "", fmt.Errorf("write temp vite config: %w", err)
	}

	cmd := inlineBuildCommand(packageManager, configPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("vite inline build failed: %v\nOutput:\n%s", err, strings.TrimSpace(string(output)))
	}

	bundlePath := filepath.Join(outDir, "bundle.js")
	data, err := os.ReadFile(bundlePath)
	if err != nil {
		return "", fmt.Errorf("read inline bundle %s: %w", bundlePath, err)
	}

	return strings.TrimSpace(string(data)), nil
}

func inlineBuildCommand(packageManager, configPath string) *exec.Cmd {
	switch packageManager {
	case "bun":
		return exec.Command("bun", "x", "vite", "build", "--config", configPath)
	case "pnpm":
		return exec.Command("pnpm", "exec", "vite", "build", "--config", configPath)
	case "yarn":
		return exec.Command("yarn", "vite", "build", "--config", configPath)
	default:
		return exec.Command("npm", "exec", "vite", "build", "--", "--config", configPath)
	}
}

func inlineViteConfigSource(entryPath, outDir string) (string, error) {
	entryJSON, err := json.Marshal(entryPath)
	if err != nil {
		return "", fmt.Errorf("marshal entry path: %w", err)
	}
	outDirJSON, err := json.Marshal(outDir)
	if err != nil {
		return "", fmt.Errorf("marshal out dir: %w", err)
	}

	return fmt.Sprintf(`import { defineConfig } from "vite"

export default defineConfig({
  build: {
    outDir: %s,
    emptyOutDir: true,
    minify: false,
    sourcemap: false,
    rollupOptions: {
      input: %s,
      output: {
        format: "es",
        inlineDynamicImports: true,
        entryFileNames: "bundle.js"
      }
    }
  }
})
`, outDirJSON, entryJSON), nil
}
