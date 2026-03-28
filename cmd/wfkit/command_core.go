package main

import (
	"fmt"

	"wfkit/internal/updater"
	"wfkit/internal/utils"

	"github.com/urfave/cli/v2"
)

func configMode(c *cli.Context) error {
	return newConfigFlow().run()
}

func interactiveMode(c *cli.Context) error {
	return newInteractiveFlow(c).run()
}

func initMode(c *cli.Context) error {
	return newInitFlow(c).run()
}

func updateMode(c *cli.Context) error {
	updateManager := updater.NewUpdateManager(c.App.Version)
	result, err := updateManager.Check(updater.CheckOptions{Force: true, AllowStale: true})
	if err != nil {
		if err.Error() == "github api rate limit exceeded" {
			utils.CPrint("GitHub API rate limit exceeded. Please try again later.", "yellow")
			return nil
		}
		return fmt.Errorf("update check failed: %v", err)
	}

	if result.Available {
		utils.PrintUpdateBanner(c.App.Version, result.LatestVersion)
	} else {
		utils.CPrint(fmt.Sprintf("You are using the latest version (%s).", c.App.Version), "green")
	}

	return nil
}
