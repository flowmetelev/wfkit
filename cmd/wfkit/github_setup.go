package main

import (
	"fmt"
	"net/url"
	"os"

	"wfkit/internal/build"
	"wfkit/internal/utils"

	"github.com/charmbracelet/huh"
)

func ensureGitHubRepositoryReady(githubUser, repositoryName string) error {
	status, err := build.DetectGitHubRepositoryStatus()
	if err != nil {
		printGitInstallHint()
		return fmt.Errorf("git is not installed. Publishing build artifacts can't continue until git is available")
	}

	if !status.HasGitRepository {
		if err := initializeLocalRepository(githubUser, repositoryName); err != nil {
			return err
		}
		status, err = build.DetectGitHubRepositoryStatus()
		if err != nil {
			printGitInstallHint()
			return fmt.Errorf("git is not installed. Publishing build artifacts can't continue until git is available")
		}
	}

	switch {
	case !status.IsProjectRoot:
		printProjectRootHint(status.RepoRoot)
		return fmt.Errorf("current directory is inside another git repository (%s). Run wfkit from the project root after initializing its own git repo", status.RepoRoot)
	case !status.HasOriginRemote:
		openedBrowser := maybeOpenGitHubRepositorySetupPage(githubUser, repositoryName)
		printGitHubSetupHint(githubUser, repositoryName, openedBrowser)
		return fmt.Errorf("git remote `origin` is missing. Create the GitHub repository, run `git remote add origin <repo-url>`, then retry")
	case !status.IsGitHubOrigin:
		openedBrowser := maybeOpenGitHubRepositorySetupPage(githubUser, repositoryName)
		printGitHubSetupHint(githubUser, repositoryName, openedBrowser)
		return fmt.Errorf("git remote `origin` is not a GitHub repository (%s). Point origin to GitHub and retry", status.OriginURL)
	default:
		return nil
	}
}

func openGitHubRepositorySetupPage(githubUser, repositoryName string) {
	repoURL := githubRepositoryCreationURL(githubUser, repositoryName)
	_ = openURL(repoURL)
}

func maybeOpenGitHubRepositorySetupPage(githubUser, repositoryName string) bool {
	if !canPromptForBrowserChoice() {
		return false
	}

	openBrowser := false
	if err := huh.NewConfirm().
		Title("Open GitHub to create the repository now?").
		Description("wfkit can open the GitHub create-repository page in your browser.").
		Value(&openBrowser).
		Run(); err != nil {
		return false
	}
	if !openBrowser {
		return false
	}

	openGitHubRepositorySetupPage(githubUser, repositoryName)
	return true
}

func githubRepositoryCreationURL(githubUser, repositoryName string) string {
	values := url.Values{}
	if githubUser != "" {
		values.Set("owner", githubUser)
	}
	if repositoryName != "" {
		values.Set("name", repositoryName)
	}

	if encoded := values.Encode(); encoded != "" {
		return "https://github.com/new?" + encoded
	}
	return "https://github.com/new"
}

func printGitHubSetupHint(githubUser, repositoryName string, openedBrowser bool) {
	utils.PrintSection("GitHub Setup")
	if openedBrowser {
		utils.PrintStatus("WARN", "Repository setup", "Opened GitHub repository creation page in your browser")
	} else {
		utils.PrintStatus("WARN", "Repository setup", "Create the GitHub repository, then add it as `origin`")
	}
	if githubUser != "" && repositoryName != "" {
		utils.PrintStatus("READY", "Next", fmt.Sprintf("git remote add origin git@github.com:%s/%s.git", githubUser, repositoryName))
	}
	utils.PrintStatus("READY", "Then", "git branch -M main")
	fmt.Println()
}

func printGitInitHint(githubUser, repositoryName string) {
	utils.PrintStatus("READY", "Run", "these commands in the project directory")
	utils.PrintCommandHints("git init")
	if githubUser != "" && repositoryName != "" {
		utils.PrintCommandHints(fmt.Sprintf("git remote add origin git@github.com:%s/%s.git", githubUser, repositoryName))
	}
	utils.PrintCommandHints("git branch -M main")
}

func printProjectRootHint(repoRoot string) {
	utils.PrintSection("Git Repository Required")
	utils.PrintStatus("WARN", "Current directory", "Run wfkit from a project root with its own .git directory.")
	if repoRoot != "" {
		utils.PrintKeyValue("Detected", repoRoot)
	}
	utils.PrintStatus("READY", "Fix", "If this project should be standalone, create a local repo first")
	utils.PrintCommandHints("git init", "git branch -M main")
	fmt.Println()
}

func initializeLocalRepository(githubUser, repositoryName string) error {
	utils.PrintSection("Git Setup")
	utils.PrintStatus("READY", "Initializing", "No local git repository was found. wfkit will create one now.")
	if err := build.InitializeLocalRepository("main"); err != nil {
		printGitInitHint(githubUser, repositoryName)
		return fmt.Errorf("failed to initialize local git repository automatically: %w", err)
	}
	utils.PrintStatus("OK", "Initialized", "Local git repository created on branch main")
	fmt.Println()
	return nil
}

func printGitInstallHint() {
	utils.PrintSection("Git Required")
	utils.PrintStatus("WARN", "Git", "wfkit could not find `git` in your PATH.")
	utils.PrintStatus("READY", "Fix", "Install git first. Publishing build artifacts can't continue without it.")
	fmt.Println()
}

func canPromptForBrowserChoice() bool {
	return isTerminalDevice(os.Stdin) && isTerminalDevice(os.Stdout)
}

func isTerminalDevice(file *os.File) bool {
	if file == nil {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
