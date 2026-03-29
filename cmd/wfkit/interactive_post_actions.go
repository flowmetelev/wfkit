package main

import (
	"os"
	"os/exec"

	"wfkit/internal/webflow"
)

func runTerminalCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func showGitStatus() error {
	return runTerminalCommand("git", "status", "--short")
}

func showGitDiffSummary() error {
	return runTerminalCommand("git", "diff", "--stat")
}

func publishedDocsURL(siteURL, slug string) string {
	return publishedPageURL(siteURL, webflow.Page{
		Title: slug,
		Slug:  slug,
	})
}
