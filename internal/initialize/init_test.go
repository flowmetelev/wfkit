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

	restore, err := prepareProjectDir("demo-project")
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
	if _, err := os.Stat(filepath.Join(projectDir, ".prettierignore")); err != nil {
		t.Fatalf("expected .prettierignore in project dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectDir, "src", "global", "index.ts")); err != nil {
		t.Fatalf("expected src/global/index.ts in project dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectDir, "src", "utils", "dom.ts")); err != nil {
		t.Fatalf("expected src/utils/dom.ts in project dir: %v", err)
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
