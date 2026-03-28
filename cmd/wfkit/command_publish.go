package main

import (
	"fmt"

	"wfkit/internal/config"
	"wfkit/internal/updater"
	"wfkit/internal/utils"

	"github.com/urfave/cli/v2"
)

func publishMode(c *cli.Context) error {
	if err := maybeCheckForPublishUpdates(c); err != nil {
		return err
	}

	cfg, err := config.ReadConfig()
	if err != nil {
		return fmt.Errorf("failed to read config: %v", err)
	}
	if cfg.AppName == "" {
		return fmt.Errorf("missing appName configuration in wfkit.json")
	}

	request := newPublishRequest(c, cfg)
	if err := request.authenticate(); err != nil {
		return err
	}
	if err := request.run(); err != nil {
		return err
	}

	request.printSuccess()
	return nil
}

func maybeCheckForPublishUpdates(c *cli.Context) error {
	if !c.Bool("update") {
		return nil
	}

	updateManager := updater.NewUpdateManager(c.App.Version)
	result, err := updateManager.Check(updater.CheckOptions{Force: true, AllowStale: true})
	if err != nil {
		if err.Error() == "github api rate limit exceeded" {
			utils.CPrint("Warning: GitHub API rate limit exceeded. Skipping update check.", "yellow")
			return nil
		}

		utils.CPrint(fmt.Sprintf("Warning: couldn't check for updates: %v", err), "yellow")
		return nil
	}

	if result.Available {
		utils.PrintUpdateBanner(c.App.Version, result.LatestVersion)
	} else {
		utils.CPrint("You are using the latest version", "green")
	}

	return nil
}
