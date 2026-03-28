package build

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"wfkit/internal/utils"
)

type ArtifactPublishOptions struct {
	BuildDir      string
	AssetBranch   string
	CommitMessage string
}

func PublishBuildArtifacts(opts ArtifactPublishOptions) (GitPushResult, error) {
	utils.CPrint("Preparing build artifacts for GitHub...", "cyan")

	buildDir := strings.TrimSpace(opts.BuildDir)
	if buildDir == "" {
		return GitPushResult{}, fmt.Errorf("missing build directory for artifact publish")
	}

	assetBranch := strings.TrimSpace(opts.AssetBranch)
	if assetBranch == "" {
		assetBranch = defaultAssetBranch
	}

	buildDirAbs, err := filepath.Abs(buildDir)
	if err != nil {
		return GitPushResult{Branch: assetBranch}, fmt.Errorf("resolve build directory: %w", err)
	}
	if err := validateDir(buildDirAbs); err != nil {
		return GitPushResult{Branch: assetBranch}, err
	}

	artifactDir, err := normalizeArtifactDir(buildDir)
	if err != nil {
		return GitPushResult{Branch: assetBranch}, err
	}

	originURL, err := originRemoteURL()
	if err != nil {
		return GitPushResult{Branch: assetBranch}, err
	}

	tempDir, err := os.MkdirTemp("", "wfkit-artifacts-*")
	if err != nil {
		return GitPushResult{Branch: assetBranch}, fmt.Errorf("create temp artifact repository: %w", err)
	}
	defer os.RemoveAll(tempDir)

	if err := prepareArtifactRepository(tempDir, originURL, assetBranch); err != nil {
		return GitPushResult{Branch: assetBranch}, err
	}

	if err := copyDir(buildDirAbs, filepath.Join(tempDir, artifactDir)); err != nil {
		return GitPushResult{Branch: assetBranch}, fmt.Errorf("copy build artifacts: %w", err)
	}

	result := GitPushResult{Branch: assetBranch}
	if _, err := runGitInDir(tempDir, "add", "--all", "."); err != nil {
		return result, fmt.Errorf("git add failed: %w", err)
	}
	result.Staged = true

	hasChanges, err := hasStagedChangesInDir(tempDir)
	if err != nil {
		return result, err
	}
	if !hasChanges {
		utils.CPrint("No artifact changes to publish, skipping push", "yellow")
		return result, nil
	}

	commitMessage := strings.TrimSpace(opts.CommitMessage)
	if commitMessage == "" {
		commitMessage = "Publish build artifacts via wfkit"
	}
	if _, err := runGitInDir(tempDir, "-c", "user.name=wfkit", "-c", "user.email=wfkit@local", "commit", "-m", commitMessage); err != nil {
		return result, fmt.Errorf("git commit failed: %w", err)
	}
	result.Committed = true

	if _, err := runGitInDir(tempDir, "push", "-u", "origin", assetBranch); err != nil {
		return result, fmt.Errorf("git push failed: %w", err)
	}
	result.Pushed = true
	utils.CPrint(fmt.Sprintf("Build artifacts pushed to %s", assetBranch), "green")
	return result, nil
}

func originRemoteURL() (string, error) {
	output, err := runGitInDir("", "remote", "get-url", "origin")
	if err != nil {
		return "", fmt.Errorf("git remote get-url origin failed: %w", err)
	}
	value := strings.TrimSpace(output)
	if value == "" {
		return "", fmt.Errorf("git remote `origin` is missing")
	}
	return value, nil
}

func prepareArtifactRepository(dir, originURL, assetBranch string) error {
	if _, err := runGitInDir(dir, "init"); err != nil {
		return fmt.Errorf("git init failed: %w", err)
	}
	if _, err := runGitInDir(dir, "remote", "add", "origin", originURL); err != nil {
		return fmt.Errorf("git remote add origin failed: %w", err)
	}

	if _, err := runGitInDir(dir, "fetch", "--depth", "1", "origin", assetBranch); err == nil {
		if _, err := runGitInDir(dir, "checkout", "-B", assetBranch, "FETCH_HEAD"); err != nil {
			return fmt.Errorf("git checkout %s failed: %w", assetBranch, err)
		}
	} else {
		if _, err := runGitInDir(dir, "checkout", "--orphan", assetBranch); err != nil {
			return fmt.Errorf("git checkout --orphan %s failed: %w", assetBranch, err)
		}
	}

	if err := removeDirContents(dir); err != nil {
		return fmt.Errorf("clean artifact repository: %w", err)
	}

	return nil
}

func runGitInDir(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("command failed: %v\nOutput:\n%s", err, output)
	}
	return string(output), nil
}

func hasStagedChangesInDir(dir string) (bool, error) {
	cmd := exec.Command("git", "diff", "--cached", "--quiet")
	cmd.Dir = dir
	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return true, nil
		}
		return false, fmt.Errorf("git diff --cached --quiet failed: %v", err)
	}
	return false, nil
}

func normalizeArtifactDir(value string) (string, error) {
	cleaned := filepath.Clean(value)
	if filepath.IsAbs(cleaned) {
		workingDir, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("resolve working directory for build artifacts: %w", err)
		}
		workingDir, err = normalizeRealPath(workingDir)
		if err != nil {
			return "", fmt.Errorf("normalize project root for build artifacts: %w", err)
		}
		cleaned, err = normalizeRealPath(cleaned)
		if err != nil {
			return "", fmt.Errorf("normalize build directory path: %w", err)
		}
		relativePath, err := filepath.Rel(workingDir, cleaned)
		if err != nil {
			return "", fmt.Errorf("resolve artifact path relative to project root: %w", err)
		}
		cleaned = filepath.Clean(relativePath)
	}
	if cleaned == "." || cleaned == "" {
		return "", fmt.Errorf("build directory must not point at the repository root")
	}
	if strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) || cleaned == ".." {
		return "", fmt.Errorf("build directory must stay within the project root")
	}
	return cleaned, nil
}

func normalizeRealPath(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return resolved, nil
	}
	if os.IsNotExist(err) {
		return path, nil
	}
	return "", err
}

func removeDirContents(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() == ".git" {
			continue
		}
		if err := os.RemoveAll(filepath.Join(dir, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(dst, relPath)

		if d.IsDir() {
			return os.MkdirAll(targetPath, 0o755)
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}

		sourceFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer sourceFile.Close()

		targetFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
		if err != nil {
			return err
		}
		defer targetFile.Close()

		if _, err := io.Copy(targetFile, sourceFile); err != nil {
			return err
		}

		return nil
	})
}
