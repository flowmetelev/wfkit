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

func resolveAssetBranchFlag(c *cli.Context, fallback string) string {
	if c.IsSet("asset-branch") {
		return c.String("asset-branch")
	}
	if c.IsSet("branch") {
		return c.String("branch")
	}
	if fallback != "" {
		return fallback
	}
	return c.String("asset-branch")
}

func resolveDeliveryModeFlag(c *cli.Context, fallback string) string {
	if c.IsSet("delivery") {
		return c.String("delivery")
	}
	if fallback != "" {
		return fallback
	}
	return "cdn"
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

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func printGitPushSummary(result build.GitPushResult) {
	switch {
	case !result.Committed:
		utils.PrintSection("Git Summary")
		utils.PrintStatus("WARN", "No new artifact commit was needed", "")
	case result.Pushed:
		utils.PrintSection("Git Summary")
		utils.PrintStatus("PUSHED", fmt.Sprintf("Artifacts committed and pushed to %s", result.Branch), "")
	default:
		utils.PrintSection("Git Summary")
		utils.PrintStatus("WARN", "Artifact commit created but push was skipped", "")
	}
	fmt.Println()
}
