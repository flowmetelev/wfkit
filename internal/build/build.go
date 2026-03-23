package build

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"wfkit/internal/utils"
)

const (
	buildDirKey   = "build-dir"
	branchKey     = "branch"
	scriptUrlKey  = "script-url"
	defaultBranch = "main"
)

type GitPushResult struct {
	Staged    bool
	Committed bool
	Pushed    bool
	Branch    string
}

// DoBuild выполняет сборку и возвращает URL последнего JS-файла
func DoBuild(args map[string]interface{}, ghUser, repo, pkgMgr string) (string, error) {
	buildDir, ok := args[buildDirKey].(string)
	if !ok || buildDir == "" {
		return "", fmt.Errorf("missing or empty 'build-dir' argument")
	}
	branch, ok := args[branchKey].(string)
	if !ok || branch == "" {
		branch = defaultBranch
	}
	scriptUrl, ok := args[scriptUrlKey].(string)

	utils.CPrint("Starting build process...", "cyan")
	if err := validateDir(buildDir); err != nil {
		return "", err
	}

	if _, err := runCmd(pkgMgr, "run", "build"); err != nil {
		return "", err
	}

	globalEntry, err := ResolveGlobalEntry(buildDir)
	if err != nil {
		return "", err
	}

	if scriptUrl == "" {
		scriptUrl = buildCDNUrl(ghUser, repo, branch, buildDir, globalEntry)
	}
	utils.CPrint(fmt.Sprintf("Build completed successfully. Script URL: %s", scriptUrl), "green")
	return scriptUrl, nil
}

// DoPushToGithub выполняет коммит и пуш в GitHub
func DoPushToGithub(branch, commitMsg string) (GitPushResult, error) {
	utils.CPrint("Preparing to push to GitHub...", "cyan")
	result := GitPushResult{Branch: branch}

	if branch == "" {
		branch = defaultBranch
		result.Branch = branch
	}

	if _, err := runCmd("git", "add", "."); err != nil {
		return result, fmt.Errorf("git add failed: %w", err)
	}
	result.Staged = true

	hasChanges, err := hasStagedChanges()
	if err != nil {
		return result, err
	}
	if !hasChanges {
		utils.CPrint("No git changes to commit after staging, skipping push", "yellow")
		return result, nil
	}

	if _, err := runCmd("git", "commit", "-m", commitMsg); err != nil {
		return result, fmt.Errorf("git commit failed: %w", err)
	}
	result.Committed = true

	if _, err := runCmd("git", "push", "-u", "origin", branch); err != nil {
		return result, fmt.Errorf("git push failed: %w", err)
	}
	result.Pushed = true
	utils.CPrint("Changes pushed successfully", "green")
	return result, nil
}

// Вспомогательные функции

func runCmd(name string, args ...string) (string, error) {
	utils.CPrint(fmt.Sprintf("Running command: %s %s", name, strings.Join(args, " ")), "yellow")
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("command failed: %v\nOutput:\n%s", err, output)
	}
	trimmed := strings.TrimSpace(string(output))
	if trimmed != "" {
		utils.CPrint(trimmed, "blue")
	}
	return string(output), nil
}

func validateDir(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("invalid build directory: %v", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", dir)
	}
	return nil
}

func findLatestJSFile(dir string) (string, error) {
	type fileInfo struct {
		Name  string
		MTime time.Time
	}

	var files []fileInfo

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(strings.ToLower(info.Name()), ".js") {
			return nil
		}
		files = append(files, fileInfo{
			Name:  info.Name(),
			MTime: info.ModTime(),
		})
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("error walking directory: %v", err)
	}

	if len(files) == 0 {
		return "", fmt.Errorf("no JS files found in %s", dir)
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].MTime.After(files[j].MTime)
	})

	return files[0].Name, nil
}

func buildCDNUrl(user, repo, branch, baseDir, fileName string) string {
	return fmt.Sprintf(
		"https://cdn.jsdelivr.net/gh/%s/%s@%s/%s/%s",
		user, repo, branch, baseDir, fileName,
	)
}

func hasStagedChanges() (bool, error) {
	cmd := exec.Command("git", "diff", "--cached", "--quiet")
	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return true, nil
		}
		return false, fmt.Errorf("git diff --cached --quiet failed: %v", err)
	}
	return false, nil
}
