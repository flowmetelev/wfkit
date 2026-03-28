package build

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestPublishBuildArtifactsPushesOnlyBuildDirectory(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(originalDir)
	}()

	remoteDir := filepath.Join(t.TempDir(), "remote.git")
	runGitInTestDir(t, "", "init", "--bare", remoteDir)

	sourceDir := t.TempDir()
	runGitInTestDir(t, sourceDir, "init")
	runGitInTestDir(t, sourceDir, "remote", "add", "origin", remoteDir)

	writeTestArtifactFile(t, filepath.Join(sourceDir, "dist/assets/index.js"), "console.log('asset')\n")
	writeTestArtifactFile(t, filepath.Join(sourceDir, "README.md"), "# source repo\n")

	if err := os.Chdir(sourceDir); err != nil {
		t.Fatalf("chdir source dir: %v", err)
	}

	result, err := PublishBuildArtifacts(ArtifactPublishOptions{
		BuildDir:      "dist/assets",
		AssetBranch:   "wfkit-dist",
		CommitMessage: "Publish assets",
	})
	if err != nil {
		t.Fatalf("PublishBuildArtifacts: %v", err)
	}
	if !result.Pushed {
		t.Fatalf("expected artifacts to be pushed, got %+v", result)
	}

	cloneDir := filepath.Join(t.TempDir(), "clone")
	runGitInTestDir(t, "", "clone", "--branch", "wfkit-dist", "--single-branch", remoteDir, cloneDir)

	if _, err := os.Stat(filepath.Join(cloneDir, "dist/assets/index.js")); err != nil {
		t.Fatalf("expected published asset file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cloneDir, "README.md")); !os.IsNotExist(err) {
		t.Fatalf("expected source repo files to stay out of artifact branch, got err=%v", err)
	}
}

func TestPublishBuildArtifactsSkipsUnchangedAssets(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(originalDir)
	}()

	remoteDir := filepath.Join(t.TempDir(), "remote.git")
	runGitInTestDir(t, "", "init", "--bare", remoteDir)

	sourceDir := t.TempDir()
	runGitInTestDir(t, sourceDir, "init")
	runGitInTestDir(t, sourceDir, "remote", "add", "origin", remoteDir)
	writeTestArtifactFile(t, filepath.Join(sourceDir, "dist/assets/index.js"), "console.log('asset')\n")

	if err := os.Chdir(sourceDir); err != nil {
		t.Fatalf("chdir source dir: %v", err)
	}

	if _, err := PublishBuildArtifacts(ArtifactPublishOptions{
		BuildDir:      "dist/assets",
		AssetBranch:   "wfkit-dist",
		CommitMessage: "Publish assets",
	}); err != nil {
		t.Fatalf("first PublishBuildArtifacts: %v", err)
	}

	result, err := PublishBuildArtifacts(ArtifactPublishOptions{
		BuildDir:      "dist/assets",
		AssetBranch:   "wfkit-dist",
		CommitMessage: "Publish assets",
	})
	if err != nil {
		t.Fatalf("second PublishBuildArtifacts: %v", err)
	}
	if result.Committed || result.Pushed {
		t.Fatalf("expected unchanged artifacts to skip commit/push, got %+v", result)
	}
}

func TestPublishBuildArtifactsAcceptsAbsoluteBuildDirectory(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(originalDir)
	}()

	remoteDir := filepath.Join(t.TempDir(), "remote.git")
	runGitInTestDir(t, "", "init", "--bare", remoteDir)

	sourceDir := t.TempDir()
	runGitInTestDir(t, sourceDir, "init")
	runGitInTestDir(t, sourceDir, "remote", "add", "origin", remoteDir)

	buildDir := filepath.Join(sourceDir, "dist/assets")
	writeTestArtifactFile(t, filepath.Join(buildDir, "index.js"), "console.log('asset')\n")

	if err := os.Chdir(sourceDir); err != nil {
		t.Fatalf("chdir source dir: %v", err)
	}

	result, err := PublishBuildArtifacts(ArtifactPublishOptions{
		BuildDir:      buildDir,
		AssetBranch:   "wfkit-dist",
		CommitMessage: "Publish assets",
	})
	if err != nil {
		t.Fatalf("PublishBuildArtifacts with absolute build dir: %v", err)
	}
	if !result.Pushed {
		t.Fatalf("expected absolute build dir publish to push, got %+v", result)
	}
}

func runGitInTestDir(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
}

func writeTestArtifactFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
