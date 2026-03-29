package main

import (
	"fmt"

	"wfkit/internal/updater"
	"wfkit/internal/utils"

	"github.com/charmbracelet/huh"
	"github.com/urfave/cli/v2"
)

type interactiveFlow struct {
	cliContext *cli.Context
	action     string
}

func newInteractiveFlow(c *cli.Context) *interactiveFlow {
	return &interactiveFlow{cliContext: c}
}

func (f *interactiveFlow) run() error {
	f.printHeader()

	if err := f.selectAction(); err != nil {
		return err
	}

	utils.ClearScreen()
	return f.dispatch()
}

func (f *interactiveFlow) printHeader() {
	version := f.cliContext.App.Version
	utils.PrintAppHeader(version, "Build Webflow scripts locally, proxy safely, and publish with confidence.")

	utils.PrintSection("Quick Start")
	for _, item := range interactiveQuickStartItems() {
		utils.PrintStatus("READY", item.title, item.description)
	}
	fmt.Println()

	if updateManager := updater.NewUpdateManager(version); updateManager != nil {
		if result, err := updateManager.Check(updater.CheckOptions{AllowStale: true}); err == nil && result.Available {
			utils.PrintUpdateBanner(version, result.LatestVersion)
		}
	}
}

func (f *interactiveFlow) selectAction() error {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("What would you like to do?").
				Options(interactiveActionOptions()...).
				Value(&f.action),
		),
	).Run()
}

func (f *interactiveFlow) dispatch() error {
	switch f.action {
	case "init":
		return initMode(f.cliContext)
	case "docs":
		return docsMode(f.cliContext)
	case "pages":
		return newInteractivePagesFlow(f.cliContext).run()
	case "migrate":
		return migrateMode(f.cliContext)
	case "publish_prod":
		f.cliContext.Set("env", "prod")
		return publishMode(f.cliContext)
	case "proxy_dev":
		return proxyMode(f.cliContext)
	case "doctor":
		return doctorMode(f.cliContext)
	case "config":
		return configMode(f.cliContext)
	case "update":
		return updateMode(f.cliContext)
	case "report_bug":
		return openBugReport(f.cliContext)
	case "request_feature":
		return openFeatureRequest(f.cliContext)
	case "exit":
		utils.CPrint("Goodbye!", "cyan")
		return nil
	default:
		return nil
	}
}

type interactiveQuickStartItem struct {
	title       string
	description string
}

func interactiveQuickStartItems() []interactiveQuickStartItem {
	return []interactiveQuickStartItem{
		{
			title:       "Initialize",
			description: "Scaffold a Webflow-ready Vite project with pages, globals, and config.",
		},
		{
			title:       "Develop",
			description: "Proxy the live site locally and inject your dev entry without touching production.",
		},
		{
			title:       "Docs",
			description: "Render markdown and publish a dedicated documentation page inside Webflow.",
		},
	}
}

func interactiveActionOptions() []huh.Option[string] {
	return []huh.Option[string]{
		huh.NewOption("Initialize a project", "init"),
		huh.NewOption("Publish docs", "docs"),
		huh.NewOption("Manage pages", "pages"),
		huh.NewOption("Migrate page code", "migrate"),
		huh.NewOption("Publish code to Webflow (prod)", "publish_prod"),
		huh.NewOption("Start dev proxy", "proxy_dev"),
		huh.NewOption("Run doctor", "doctor"),
		huh.NewOption("Configure CLI defaults", "config"),
		huh.NewOption("Check for updates", "update"),
		huh.NewOption("Report a bug", "report_bug"),
		huh.NewOption("Request a feature", "request_feature"),
		huh.NewOption("Exit", "exit"),
	}
}
