package initialize

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"wfkit/internal/initialize/config"
)

func TestPrepareProjectDirCreatesAndEntersProjectFolder(t *testing.T) {
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

	restore, err := prepareProjectDir("demo-project", false)
	if err != nil {
		t.Fatalf("prepareProjectDir: %v", err)
	}
	defer func() {
		if err := restore(); err != nil {
			t.Fatalf("restore dir: %v", err)
		}
	}()

	gotDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd after prepare: %v", err)
	}

	gotDirEval, err := filepath.EvalSymlinks(gotDir)
	if err != nil {
		t.Fatalf("eval got dir: %v", err)
	}

	wantDir := filepath.Join(tmpDir, "demo-project")
	wantDirEval, err := filepath.EvalSymlinks(wantDir)
	if err != nil {
		t.Fatalf("eval want dir: %v", err)
	}

	if gotDirEval != wantDirEval {
		t.Fatalf("expected cwd %q, got %q", wantDirEval, gotDirEval)
	}

	info, err := os.Stat(wantDir)
	if err != nil {
		t.Fatalf("stat project dir: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("expected %q to be a directory", wantDir)
	}
}

func TestInitProjectCreatesScaffoldInsideProjectDirectory(t *testing.T) {
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

	installCalled := false
	gitInitCalled := false
	err = initProject(config.Options{
		Name:           "demo-project",
		PackageManager: "bun",
		GitHubUser:     "demo-user",
		RepositoryName: "demo-repo",
		CLIValue:       "1.2.3",
	}, func(pkgMgr string) error {
		installCalled = true
		if pkgMgr != "bun" {
			t.Fatalf("expected installer to receive bun, got %q", pkgMgr)
		}
		return nil
	}, func() error {
		gitInitCalled = true
		return nil
	})
	if err != nil {
		t.Fatalf("initProject: %v", err)
	}
	if !installCalled {
		t.Fatal("expected dependency installer to be called")
	}
	if gitInitCalled {
		t.Fatal("did not expect git initialization to be called")
	}

	projectDir := filepath.Join(tmpDir, "demo-project")
	if _, err := os.Stat(filepath.Join(projectDir, "package.json")); err != nil {
		t.Fatalf("expected package.json in project dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectDir, "wfkit.json")); err != nil {
		t.Fatalf("expected wfkit.json in project dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectDir, "README.md")); err != nil {
		t.Fatalf("expected README.md in project dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectDir, ".prettierignore")); err != nil {
		t.Fatalf("expected .prettierignore in project dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectDir, "src", "global", "index.ts")); err != nil {
		t.Fatalf("expected src/global/index.ts in project dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectDir, "src", "utils", "dom.ts")); err != nil {
		t.Fatalf("expected src/utils/dom.ts in project dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectDir, "src", "utils", "webflow.ts")); err != nil {
		t.Fatalf("expected src/utils/webflow.ts in project dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectDir, "src", "features", "site-status.ts")); err != nil {
		t.Fatalf("expected src/features/site-status.ts in project dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectDir, "src", "generated", "wfkit-pages.ts")); err != nil {
		t.Fatalf("expected src/generated/wfkit-pages.ts in project dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectDir, "build", "webflow-vite-plugin.js")); err != nil {
		t.Fatalf("expected build/webflow-vite-plugin.js in project dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectDir, "docs", "index.md")); err != nil {
		t.Fatalf("expected docs/index.md in project dir: %v", err)
	}

	cfgData, err := os.ReadFile(filepath.Join(projectDir, "wfkit.json"))
	if err != nil {
		t.Fatalf("read wfkit.json: %v", err)
	}
	cfgText := string(cfgData)
	if !strings.Contains(cfgText, `"appName": "demo-project"`) {
		t.Fatalf("expected appName in wfkit.json, got: %s", cfgText)
	}
	if !strings.Contains(cfgText, `"ghUserName": "demo-user"`) {
		t.Fatalf("expected ghUserName in wfkit.json, got: %s", cfgText)
	}
	if !strings.Contains(cfgText, `"globalEntry": "src/global/index.ts"`) {
		t.Fatalf("expected globalEntry in wfkit.json, got: %s", cfgText)
	}
	if !strings.Contains(cfgText, `"docsEntry": "docs/index.md"`) {
		t.Fatalf("expected docsEntry in wfkit.json, got: %s", cfgText)
	}
	if !strings.Contains(cfgText, `"docsPageSlug": "docs"`) {
		t.Fatalf("expected docsPageSlug in wfkit.json, got: %s", cfgText)
	}

	packageData, err := os.ReadFile(filepath.Join(projectDir, "package.json"))
	if err != nil {
		t.Fatalf("read package.json: %v", err)
	}
	if !strings.Contains(string(packageData), `"fast-glob":`) {
		t.Fatalf("expected fast-glob dependency in package.json, got: %s", string(packageData))
	}
	if !strings.Contains(string(packageData), `"@flowmetelev/wfkit": "latest"`) {
		t.Fatalf("expected wfkit local dependency in package.json, got: %s", string(packageData))
	}
	if !strings.Contains(string(packageData), `"private": true`) {
		t.Fatalf("expected private package in package.json, got: %s", string(packageData))
	}
	if !strings.Contains(string(packageData), `"dev": "wfkit proxy"`) {
		t.Fatalf("expected dev script to use wfkit proxy, got: %s", string(packageData))
	}
	if !strings.Contains(string(packageData), `"docs": "wfkit docs"`) {
		t.Fatalf("expected docs script in package.json, got: %s", string(packageData))
	}
	if !strings.Contains(string(packageData), `"pages:types": "wfkit pages types"`) {
		t.Fatalf("expected pages:types script in package.json, got: %s", string(packageData))
	}
	if !strings.Contains(string(packageData), `"typecheck": "tsc --noEmit"`) {
		t.Fatalf("expected typecheck script in package.json, got: %s", string(packageData))
	}

	generatedPagesData, err := os.ReadFile(filepath.Join(projectDir, "src", "generated", "wfkit-pages.ts"))
	if err != nil {
		t.Fatalf("read generated page types: %v", err)
	}
	generatedPagesText := string(generatedPagesData)
	if !strings.Contains(generatedPagesText, `export type WfPage = (typeof wfPages)[number]`) {
		t.Fatalf("expected WfPage type in generated pages file, got: %s", generatedPagesText)
	}
	if !strings.Contains(generatedPagesText, `'home'`) {
		t.Fatalf("expected home slug in generated pages file, got: %s", generatedPagesText)
	}

	readmeData, err := os.ReadFile(filepath.Join(projectDir, "README.md"))
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	readmeText := string(readmeData)
	if !strings.Contains(readmeText, "Generated by `wfkit init`.") {
		t.Fatalf("expected generated readme marker, got: %s", readmeText)
	}
	if !strings.Contains(readmeText, "bun run dev") {
		t.Fatalf("expected package-manager specific run command in README.md, got: %s", readmeText)
	}

	restoredDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd after initProject: %v", err)
	}
	restoredEval, err := filepath.EvalSymlinks(restoredDir)
	if err != nil {
		t.Fatalf("eval restored cwd: %v", err)
	}
	tmpEval, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		t.Fatalf("eval tmp dir: %v", err)
	}
	if restoredEval != tmpEval {
		t.Fatalf("expected cwd to be restored to %q, got %q", tmpEval, restoredEval)
	}
}

func TestInitProjectInitializesGitWhenRequested(t *testing.T) {
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

	installCalled := false
	gitInitCalled := false
	err = initProject(config.Options{
		Name:           "demo-project",
		PackageManager: "bun",
		InitGit:        true,
	}, func(pkgMgr string) error {
		installCalled = true
		return nil
	}, func() error {
		gitInitCalled = true
		return nil
	})
	if err != nil {
		t.Fatalf("initProject: %v", err)
	}
	if !installCalled {
		t.Fatal("expected dependency installer to be called")
	}
	if !gitInitCalled {
		t.Fatal("expected git initialization to be called")
	}
}

func TestInitProjectSkipsDependencyInstallWhenRequested(t *testing.T) {
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

	installCalled := false
	err = initProject(config.Options{
		Name:           "demo-project",
		PackageManager: "npm",
		SkipInstall:    true,
	}, func(pkgMgr string) error {
		installCalled = true
		return nil
	}, func() error {
		return nil
	})
	if err != nil {
		t.Fatalf("initProject: %v", err)
	}
	if installCalled {
		t.Fatal("did not expect dependency installation when SkipInstall is enabled")
	}
}

func TestPrepareProjectDirFailsForNonEmptyDirectoryWithoutForce(t *testing.T) {
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

	projectDir := filepath.Join(tmpDir, "demo-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "existing.txt"), []byte("keep"), 0644); err != nil {
		t.Fatalf("write existing file: %v", err)
	}

	_, err = prepareProjectDir(projectDir, false)
	if err == nil {
		t.Fatal("expected prepareProjectDir to fail for a non-empty directory without force")
	}
	if !strings.Contains(err.Error(), "is not empty") {
		t.Fatalf("expected non-empty directory error, got: %v", err)
	}
}

func TestInitProjectUsesProjectDirForFolderAndBaseNameForMetadata(t *testing.T) {
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

	projectDir := filepath.Join("nested", "demo-project")
	err = initProject(config.Options{
		ProjectDir:     projectDir,
		PackageManager: "npm",
	}, func(pkgMgr string) error {
		if pkgMgr != "npm" {
			t.Fatalf("expected installer to receive npm, got %q", pkgMgr)
		}
		return nil
	}, func() error {
		return nil
	})
	if err != nil {
		t.Fatalf("initProject: %v", err)
	}

	packageData, err := os.ReadFile(filepath.Join(tmpDir, projectDir, "package.json"))
	if err != nil {
		t.Fatalf("read package.json: %v", err)
	}
	if !strings.Contains(string(packageData), `"name": "demo-project"`) {
		t.Fatalf("expected package name to use the base folder name, got: %s", string(packageData))
	}

	readmeData, err := os.ReadFile(filepath.Join(tmpDir, projectDir, "README.md"))
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	if !strings.Contains(string(readmeData), "# demo-project") {
		t.Fatalf("expected README to use the base folder name, got: %s", string(readmeData))
	}
}
