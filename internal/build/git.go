package build

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type GitHubRepositoryStatus struct {
	HasGitRepository bool
	HasOriginRemote  bool
	OriginURL        string
	IsGitHubOrigin   bool
	RepoRoot         string
	CurrentDir       string
	IsProjectRoot    bool
}

func DetectGitHubRepositoryStatus() (GitHubRepositoryStatus, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return GitHubRepositoryStatus{}, fmt.Errorf("git is not installed")
	}

	status := GitHubRepositoryStatus{}
	currentDir, err := os.Getwd()
	if err == nil {
		if absPath, absErr := filepath.Abs(currentDir); absErr == nil {
			currentDir = absPath
		}
		if realPath, realErr := normalizeRealPath(currentDir); realErr == nil {
			currentDir = realPath
		}
		status.CurrentDir = filepath.Clean(currentDir)
	}

	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return status, nil
	}
	if strings.TrimSpace(string(output)) != "true" {
		return status, nil
	}
	status.HasGitRepository = true

	cmd = exec.Command("git", "rev-parse", "--show-toplevel")
	output, err = cmd.CombinedOutput()
	if err == nil {
		repoRoot := filepath.Clean(strings.TrimSpace(string(output)))
		if realPath, realErr := normalizeRealPath(repoRoot); realErr == nil {
			repoRoot = realPath
		}
		status.RepoRoot = repoRoot
		status.IsProjectRoot = status.CurrentDir != "" && status.RepoRoot == status.CurrentDir
	}

	cmd = exec.Command("git", "remote", "get-url", "origin")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return status, nil
	}

	status.OriginURL = strings.TrimSpace(string(output))
	status.HasOriginRemote = status.OriginURL != ""
	status.IsGitHubOrigin = isGitHubRemoteURL(status.OriginURL)

	return status, nil
}

func isGitHubRemoteURL(rawURL string) bool {
	value := strings.ToLower(strings.TrimSpace(rawURL))
	if value == "" {
		return false
	}
	return strings.Contains(value, "github.com") || strings.HasPrefix(value, "git@github:")
}
