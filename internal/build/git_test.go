package build

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestDetectGitHubRepositoryStatus(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}

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

	status, err := DetectGitHubRepositoryStatus()
	if err != nil {
		t.Fatalf("DetectGitHubRepositoryStatus: %v", err)
	}
	if status.HasGitRepository {
		t.Fatalf("expected no git repository in temp dir, got %+v", status)
	}

	runGit(t, "init")
	status, err = DetectGitHubRepositoryStatus()
	if err != nil {
		t.Fatalf("DetectGitHubRepositoryStatus after git init: %v", err)
	}
	if !status.HasGitRepository {
		t.Fatalf("expected git repository after init, got %+v", status)
	}
	if !status.IsProjectRoot {
		t.Fatalf("expected current directory to be repo root, got %+v", status)
	}
	if status.HasOriginRemote {
		t.Fatalf("did not expect origin remote yet, got %+v", status)
	}

	runGit(t, "remote", "add", "origin", "git@github.com:test-owner/test-repo.git")
	status, err = DetectGitHubRepositoryStatus()
	if err != nil {
		t.Fatalf("DetectGitHubRepositoryStatus after remote add: %v", err)
	}
	if !status.HasOriginRemote || !status.IsGitHubOrigin {
		t.Fatalf("expected GitHub origin remote, got %+v", status)
	}

	nestedDir := filepath.Join(tmpDir, "nested")
	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatalf("mkdir nested dir: %v", err)
	}
	if err := os.Chdir(nestedDir); err != nil {
		t.Fatalf("chdir nested dir: %v", err)
	}

	status, err = DetectGitHubRepositoryStatus()
	if err != nil {
		t.Fatalf("DetectGitHubRepositoryStatus inside nested dir: %v", err)
	}
	if status.IsProjectRoot {
		t.Fatalf("expected nested directory to not be treated as repo root, got %+v", status)
	}
}

func TestDetectGitHubRepositoryStatusTreatsSymlinkedProjectRootAsRoot(t *testing.T) {
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

	baseDir := t.TempDir()
	repoDir := filepath.Join(baseDir, "repo")
	linkDir := filepath.Join(baseDir, "repo-link")

	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("mkdir repo dir: %v", err)
	}
	if err := os.Symlink(repoDir, linkDir); err != nil {
		t.Skipf("symlink is not supported: %v", err)
	}

	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("chdir repo dir: %v", err)
	}
	runGit(t, "init")

	if err := os.Chdir(linkDir); err != nil {
		t.Fatalf("chdir symlink dir: %v", err)
	}

	status, err := DetectGitHubRepositoryStatus()
	if err != nil {
		t.Fatalf("DetectGitHubRepositoryStatus via symlink: %v", err)
	}
	if !status.IsProjectRoot {
		t.Fatalf("expected symlinked project root to be treated as repo root, got %+v", status)
	}
}

func TestInitializeLocalRepository(t *testing.T) {
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

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}

	if err := InitializeLocalRepository("main"); err != nil {
		t.Fatalf("InitializeLocalRepository: %v", err)
	}

	status, err := DetectGitHubRepositoryStatus()
	if err != nil {
		t.Fatalf("DetectGitHubRepositoryStatus: %v", err)
	}
	if !status.HasGitRepository || !status.IsProjectRoot {
		t.Fatalf("expected initialized git repository at project root, got %+v", status)
	}
}

func runGit(t *testing.T, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir, _ = filepath.Abs(".")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
}
