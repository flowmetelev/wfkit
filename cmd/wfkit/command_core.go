package main

import (
	"fmt"

	"wfkit/internal/globalconfig"
	"wfkit/internal/updater"
	"wfkit/internal/utils"

	"github.com/charmbracelet/huh"
	"github.com/urfave/cli/v2"
)

func configMode(c *cli.Context) error {
	conf, err := globalconfig.LoadConfig()
	if err != nil {
		conf = &globalconfig.Config{
			PackageManager: "bun",
		}
	}

	err = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Default GitHub Username").
				Description("Used for CDN links (e.g. yndmitry)").
				Value(&conf.GitHubUser),
			huh.NewSelect[string]().
				Title("Default Package Manager").
				Options(packageManagerSelectOptions()...).
				Value(&conf.PackageManager),
			huh.NewConfirm().
				Title("Desktop notifications by default?").
				Description("Used by `publish --notify` and `migrate --notify` when you don't pass the flag explicitly.").
				Value(&conf.Notify),
		),
	).Run()

	if err != nil {
		return err
	}

	if err := globalconfig.SaveConfig(conf); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	utils.CPrint("Global configuration saved successfully!", "green")
	return nil
}

func interactiveMode(c *cli.Context) error {
	var action string

	utils.PrintAppHeader(c.App.Version, "Build Webflow scripts locally, proxy safely, and publish with confidence.")
	if updateManager := updater.NewUpdateManager(c.App.Version); updateManager != nil {
		if result, err := updateManager.Check(updater.CheckOptions{AllowStale: true}); err == nil && result.Available {
			utils.PrintUpdateBanner(c.App.Version, result.LatestVersion)
		}
	}
	utils.PrintActionCards(
		utils.ActionCard{
			Title:       "Initialize",
			Description: "Scaffold a new Webflow-ready Vite project with pages, globals, and config.",
			Command:     "wfkit init",
		},
		utils.ActionCard{
			Title:       "Develop",
			Description: "Proxy the live site locally and inject your dev entry without touching production.",
			Command:     "wfkit proxy",
		},
		utils.ActionCard{
			Title:       "Docs Hub",
			Description: "Render markdown and publish a dedicated documentation page inside Webflow.",
			Command:     "wfkit docs",
		},
	)

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("What would you like to do?").
				Options(
					huh.NewOption("🚀 Initialize a new project", "init"),
					huh.NewOption("📚 Publish docs hub", "docs"),
					huh.NewOption("🧬 Migrate page code from Webflow", "migrate"),
					huh.NewOption("📡 Publish code to Webflow (Prod)", "publish_prod"),
					huh.NewOption("🛠️ Start Dev Proxy", "proxy_dev"),
					huh.NewOption("🩺 Run Doctor", "doctor"),
					huh.NewOption("⚙️  Configure CLI defaults", "config"),
					huh.NewOption("🔄 Check for updates", "update"),
					huh.NewOption("🐛 Report a bug", "report_bug"),
					huh.NewOption("💡 Request a feature", "request_feature"),
					huh.NewOption("❌ Exit", "exit"),
				).
				Value(&action),
		),
	)

	if err := form.Run(); err != nil {
		return err
	}

	utils.ClearScreen()

	switch action {
	case "init":
		return initMode(c)
	case "docs":
		return docsMode(c)
	case "migrate":
		return migrateMode(c)
	case "publish_prod":
		c.Set("env", "prod")
		return publishMode(c)
	case "proxy_dev":
		return proxyMode(c)
	case "doctor":
		return doctorMode(c)
	case "config":
		return configMode(c)
	case "update":
		return updateMode(c)
	case "report_bug":
		return openBugReport(c)
	case "request_feature":
		return openFeatureRequest(c)
	case "exit":
		utils.CPrint("Goodbye!", "cyan")
		return nil
	}

	return nil
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
