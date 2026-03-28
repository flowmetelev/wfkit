package main

import (
	"fmt"

	"wfkit/internal/build"
	"wfkit/internal/utils"

	"github.com/urfave/cli/v2"
)

func packageManagerScriptCommand(packageManager, script string) string {
	switch packageManager {
	case "bun":
		return fmt.Sprintf("bun run %s", script)
	case "yarn", "pnpm":
		return fmt.Sprintf("%s %s", packageManager, script)
	default:
		return fmt.Sprintf("npm run %s", script)
	}
}

func packageManagerInstallCommand(packageManager string) string {
	switch packageManager {
	case "bun":
		return "bun install"
	case "yarn":
		return "yarn"
	case "pnpm":
		return "pnpm install"
	default:
		return "npm install"
	}
}

func resolveStringFlag(c *cli.Context, name, fallback string) string {
	if c.IsSet(name) {
		return c.String(name)
	}
	if fallback != "" {
		return fallback
	}
	return c.String(name)
}

func resolveIntFlag(c *cli.Context, name string, fallback int) int {
	if c.IsSet(name) {
		return c.Int(name)
	}
	if fallback > 0 {
		return fallback
	}
	return c.Int(name)
}

func resolveBoolFlag(c *cli.Context, name string, fallback bool) bool {
	if c.IsSet(name) {
		return c.Bool(name)
	}
	return fallback
}

func printGitPushSummary(result build.GitPushResult) {
	switch {
	case !result.Committed:
		utils.PrintSection("Git Summary")
		utils.PrintStatus("WARN", "No new commit was needed", "")
	case result.Pushed:
		utils.PrintSection("Git Summary")
		utils.PrintStatus("PUSHED", fmt.Sprintf("Committed and pushed to %s", result.Branch), "")
	default:
		utils.PrintSection("Git Summary")
		utils.PrintStatus("WARN", "Commit created but push was skipped", "")
	}
	fmt.Println()
}
