package main

import (
	"fmt"
	"strings"

	"wfkit/internal/config"
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
	for {
		f.printHeader()

		if err := f.selectAction(); err != nil {
			return err
		}

		if f.action == "exit" {
			utils.CPrint("Goodbye!", "cyan")
			return nil
		}

		utils.ClearScreen()
		if err := f.dispatch(); err != nil {
			return err
		}
	}
}

func (f *interactiveFlow) printHeader() {
	version := f.cliContext.App.Version
	utils.PrintAppHeader(version, "Build Webflow scripts locally, proxy safely, and publish with confidence.")
	f.printProjectSummary()

	if updateManager := updater.NewUpdateManager(version); updateManager != nil {
		if result, err := updateManager.Check(updater.CheckOptions{AllowStale: true}); err == nil && result.Available {
			f.printUpdateNotice(version, result.LatestVersion)
		}
	}
}

func (f *interactiveFlow) printProjectSummary() {
	cfg, err := config.ReadConfig()
	if err != nil {
		utils.PrintSection("Project")
		utils.PrintStatus("WARN", "Config", err.Error())
		fmt.Println()
		return
	}

	utils.PrintSection("Project")
	utils.PrintStatus("INFO", displayValue(cfg.AppName), displayValue(cfg.EffectiveSiteURL()))
	utils.PrintSummary(
		utils.SummaryMetric{Label: "pkg", Value: displayValue(cfg.PackageManager), Tone: "info"},
		utils.SummaryMetric{Label: "delivery", Value: displayValue(cfg.DeliveryMode), Tone: "info"},
		utils.SummaryMetric{Label: "assets", Value: displayValue(cfg.AssetBranch), Tone: "info"},
		utils.SummaryMetric{Label: "docs", Value: displayValue(cfg.DocsPageSlug), Tone: "info"},
	)
	utils.PrintStatus("INFO", "Build", displayValue(cfg.BuildDir))
	fmt.Println()
}

func (f *interactiveFlow) printUpdateNotice(currentVersion, latestVersion string) {
	utils.PrintStatus("WARN", fmt.Sprintf("Update available: v%s", latestVersion), compactUpdateMessage(currentVersion, latestVersion))
	fmt.Println()
}

func (f *interactiveFlow) selectAction() error {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Actions").
				Description("Type to filter. Enter selects. Esc or Ctrl+C exits.").
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
		return newInteractiveDocsFlow(f.cliContext).run()
	case "pages":
		return newInteractivePagesFlow(f.cliContext).run()
	case "cms":
		return newInteractiveCMSFlow(f.cliContext).run()
	case "migrate":
		return newInteractiveMigrateFlow(f.cliContext).run()
	case "publish":
		return newInteractivePublishFlow(f.cliContext).run()
	case "proxy_dev":
		return proxyMode(f.cliContext)
	case "doctor":
		return newInteractiveDoctorFlow(f.cliContext).run()
	case "config":
		return configMode(f.cliContext)
	case "update":
		return updateMode(f.cliContext)
	case "report_bug":
		return openBugReport(f.cliContext)
	case "request_feature":
		return openFeatureRequest(f.cliContext)
	default:
		return nil
	}
}

func interactiveActionOptions() []huh.Option[string] {
	return []huh.Option[string]{
		huh.NewOption("Proxy local site", "proxy_dev"),
		huh.NewOption("Publish code", "publish"),
		huh.NewOption("Publish docs", "docs"),
		huh.NewOption("Migrate code", "migrate"),
		huh.NewOption("Manage pages", "pages"),
		huh.NewOption("Manage CMS", "cms"),
		huh.NewOption("Run doctor", "doctor"),
		huh.NewOption("Initialize project", "init"),
		huh.NewOption("Configure CLI defaults", "config"),
		huh.NewOption("Check for updates", "update"),
		huh.NewOption("Report a bug", "report_bug"),
		huh.NewOption("Request a feature", "request_feature"),
		huh.NewOption("Exit", "exit"),
	}
}

func compactUpdateMessage(currentVersion, latestVersion string) string {
	parts := make([]string, 0, 2)
	if strings.TrimSpace(currentVersion) != "" {
		parts = append(parts, "current v"+currentVersion)
	}
	if strings.TrimSpace(latestVersion) != "" {
		parts = append(parts, "latest v"+latestVersion)
	}
	if len(parts) == 0 {
		return "Run `wfkit update` when ready."
	}
	return strings.Join(parts, "  ") + "  Run `wfkit update` when ready."
}
