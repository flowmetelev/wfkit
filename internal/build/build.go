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
	buildDirKey        = "build-dir"
	assetBranchKey     = "asset-branch"
	branchKey          = "branch"
	scriptUrlKey       = "script-url"
	defaultAssetBranch = "wfkit-dist"
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
		scriptUrl = buildCDNUrl(ghUser, repo, resolveAssetBranchArg(args), buildDir, globalEntry)
	}
	utils.CPrint(fmt.Sprintf("Build completed successfully. Script URL: %s", scriptUrl), "green")
	return scriptUrl, nil
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

func resolveAssetBranchArg(args map[string]interface{}) string {
	if branch, ok := args[assetBranchKey].(string); ok && strings.TrimSpace(branch) != "" {
		return branch
	}
	if branch, ok := args[branchKey].(string); ok && strings.TrimSpace(branch) != "" {
		return branch
	}
	return defaultAssetBranch
}
