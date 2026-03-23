package steps

import (
	"fmt"
	"os/exec"
)

func InitializeGitRepository() error {
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git is not installed")
	}

	if err := exec.Command("git", "init", "-b", "main").Run(); err == nil {
		return nil
	}

	if err := exec.Command("git", "init").Run(); err != nil {
		return fmt.Errorf("git init failed: %w", err)
	}

	if err := exec.Command("git", "branch", "-M", "main").Run(); err != nil {
		return fmt.Errorf("git branch -M main failed: %w", err)
	}

	return nil
}
