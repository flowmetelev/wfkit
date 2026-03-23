package main

import (
	"fmt"
	"net/url"

	"wfkit/internal/build"
	"wfkit/internal/utils"
)

func ensureGitHubRepositoryReady(githubUser, repositoryName string) error {
	status, err := build.DetectGitHubRepositoryStatus()
	if err != nil {
		return err
	}

	switch {
	case !status.HasGitRepository:
		printGitInitHint(githubUser, repositoryName)
		return fmt.Errorf("git repository is not initialized in this project. Run `git init` here, then retry")
	case !status.IsProjectRoot:
		printProjectRootHint(status.RepoRoot)
		return fmt.Errorf("current directory is inside another git repository (%s). Run wfkit from the project root after initializing its own git repo", status.RepoRoot)
	case !status.HasOriginRemote:
		openGitHubRepositorySetupPage(githubUser, repositoryName)
		printGitHubSetupHint(githubUser, repositoryName)
		return fmt.Errorf("git remote `origin` is missing. Create the GitHub repository, run `git remote add origin <repo-url>`, then retry")
	case !status.IsGitHubOrigin:
		openGitHubRepositorySetupPage(githubUser, repositoryName)
		printGitHubSetupHint(githubUser, repositoryName)
		return fmt.Errorf("git remote `origin` is not a GitHub repository (%s). Point origin to GitHub and retry", status.OriginURL)
	default:
		return nil
	}
}

func openGitHubRepositorySetupPage(githubUser, repositoryName string) {
	repoURL := githubRepositoryCreationURL(githubUser, repositoryName)
	_ = openURL(repoURL)
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

func printGitHubSetupHint(githubUser, repositoryName string) {
	utils.PrintSection("GitHub Setup")
	utils.PrintStatus("WARN", "Repository setup", "Opened GitHub repository creation page in your browser")
	printGitInitHint(githubUser, repositoryName)
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
