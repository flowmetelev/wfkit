package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadConfigPrefersProjectConfigFile(t *testing.T) {
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}
	defer func() {
		_ = os.Chdir(originalDir)
	}()

	writeTestFile(t, filepath.Join(tmpDir, "package.json"), `{
		"name": "package-name",
		"packageManager": "pnpm@10.0.0",
		"config": {
			"ghUserName": "legacy-user",
			"repositoryName": "legacy-repo",
			"devPort": 4444
		}
	}`)
	writeTestFile(t, filepath.Join(tmpDir, "wfkit.json"), `{
		"appName": "custom-site",
		"siteUrl": "https://custom-site.webflow.io",
		"ghUserName": "project-user",
		"repositoryName": "project-repo",
		"packageManager": "bun",
		"devPort": 5174,
		"proxyPort": 3001,
		"branch": "release",
		"buildDir": "public/assets",
		"globalEntry": "src/entry.ts",
		"docsEntry": "docs/handbook.md",
		"docsPageSlug": "handbook"
	}`)

	cfg, err := ReadConfig()
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}

	if cfg.AppName != "custom-site" {
		t.Fatalf("expected appName from project config, got %q", cfg.AppName)
	}
	if cfg.GitHubUser != "project-user" {
		t.Fatalf("expected ghUserName from project config, got %q", cfg.GitHubUser)
	}
	if cfg.RepositoryName != "project-repo" {
		t.Fatalf("expected repositoryName from project config, got %q", cfg.RepositoryName)
	}
	if cfg.PackageManager != "bun" {
		t.Fatalf("expected package manager bun, got %q", cfg.PackageManager)
	}
	if cfg.DevPort != 5174 {
		t.Fatalf("expected dev port 5174, got %d", cfg.DevPort)
	}
	if cfg.ProxyPort != 3001 {
		t.Fatalf("expected proxy port 3001, got %d", cfg.ProxyPort)
	}
	if cfg.Branch != "release" {
		t.Fatalf("expected branch release, got %q", cfg.Branch)
	}
	if cfg.BuildDir != "public/assets" {
		t.Fatalf("expected build dir public/assets, got %q", cfg.BuildDir)
	}
	if cfg.GlobalEntry != "src/entry.ts" {
		t.Fatalf("expected global entry src/entry.ts, got %q", cfg.GlobalEntry)
	}
	if cfg.DocsEntry != "docs/handbook.md" {
		t.Fatalf("expected docs entry docs/handbook.md, got %q", cfg.DocsEntry)
	}
	if cfg.DocsPageSlug != "handbook" {
		t.Fatalf("expected docs page slug handbook, got %q", cfg.DocsPageSlug)
	}
}

func TestReadConfigFallsBackToPackageJSONAndDefaults(t *testing.T) {
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}
	defer func() {
		_ = os.Chdir(originalDir)
	}()

	writeTestFile(t, filepath.Join(tmpDir, "package.json"), `{
		"name": "demo-site",
		"packageManager": "bun@1.2.0",
		"config": {
			"ghUserName": "demo-user",
			"repositoryName": "demo-repo"
		}
	}`)

	cfg, err := ReadConfig()
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}

	if cfg.AppName != "demo-site" {
		t.Fatalf("expected appName demo-site, got %q", cfg.AppName)
	}
	if cfg.PackageManager != "bun" {
		t.Fatalf("expected normalized package manager bun, got %q", cfg.PackageManager)
	}
	if cfg.EffectiveSiteURL() != "https://demo-site.webflow.io" {
		t.Fatalf("unexpected site url: %q", cfg.EffectiveSiteURL())
	}
	if cfg.BuildDir != defaultBuildDir {
		t.Fatalf("expected default build dir %q, got %q", defaultBuildDir, cfg.BuildDir)
	}
	if cfg.Branch != defaultBranch {
		t.Fatalf("expected default branch %q, got %q", defaultBranch, cfg.Branch)
	}
	if cfg.DevPort != defaultDevPort {
		t.Fatalf("expected default dev port %d, got %d", defaultDevPort, cfg.DevPort)
	}
	if cfg.DocsEntry != defaultDocsEntry {
		t.Fatalf("expected default docs entry %q, got %q", defaultDocsEntry, cfg.DocsEntry)
	}
	if cfg.DocsPageSlug != defaultDocsSlug {
		t.Fatalf("expected default docs page slug %q, got %q", defaultDocsSlug, cfg.DocsPageSlug)
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
