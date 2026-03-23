package main

import (
	"fmt"

	"wfkit/internal/config"
	"wfkit/internal/publish"
	"wfkit/internal/utils"
	"wfkit/internal/webflow"

	"github.com/urfave/cli/v2"
)

func docsMode(c *cli.Context) error {
	cfg, err := config.ReadConfig()
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}
	if cfg.AppName == "" {
		return fmt.Errorf("missing appName configuration in wfkit.json")
	}

	opts := publish.DocsHubOptions{
		EntryPath: resolveStringFlag(c, "file", cfg.DocsEntry),
		PageSlug:  resolveStringFlag(c, "page-slug", cfg.DocsPageSlug),
		Publish:   c.Bool("publish"),
		Selector:  resolveStringFlag(c, "selector", `[data-wf-docs-root], main`),
	}

	baseURL := cfg.EffectiveDesignURL()
	utils.PrintSection("Docs Hub")
	utils.PrintKeyValue("Webflow", baseURL)
	utils.PrintKeyValue("Markdown", opts.EntryPath)
	utils.PrintKeyValue("Page slug", opts.PageSlug)
	fmt.Println()
	printDocsTimeline(false, false, false)

	var pToken string
	var cookies string
	if err := utils.RunTask("Authenticate with Webflow", func() error {
		var authErr error
		pToken, cookies, authErr = webflow.GetCsrfTokenAndCookies(c.Context, baseURL)
		if authErr != nil {
			return fmt.Errorf("failed to authenticate with Webflow: %w", authErr)
		}
		return nil
	}); err != nil {
		return err
	}

	var pages []webflow.Page
	if err := utils.RunTask("Load pages from Webflow", func() error {
		var loadErr error
		pages, loadErr = webflow.GetPagesListFromDom(c.Context, cfg.AppName, pToken, cookies)
		if loadErr != nil {
			return fmt.Errorf("failed to load pages: %w", loadErr)
		}
		return nil
	}); err != nil {
		return err
	}

	plan, err := publish.PlanDocsHubSync(pages, opts)
	if err != nil {
		return fmt.Errorf("failed to plan docs sync: %w", err)
	}
	printDocsPlan(plan)
	printDocsTimeline(true, true, false)

	if c.Bool("dry-run") {
		utils.CPrint("Dry run mode: the docs hub page was not changed", "yellow")
		return nil
	}

	var result publish.DocsHubResult
	if err := utils.RunTask("Publish docs hub", func() error {
		var applyErr error
		result, applyErr = publish.ApplyDocsHubSync(c.Context, cfg.AppName, baseURL, pToken, cookies, plan, opts.Publish)
		return applyErr
	}); err != nil {
		return err
	}

	printDocsTimeline(true, true, true)
	printDocsResult(result)

	if resolveNotifyFlag(c) {
		notifySuccess(true, "wfkit docs completed", "The docs hub page finished syncing.")
	}

	utils.PrintSuccessScreen(
		"Docs hub synced",
		"Markdown documentation has been published to the Webflow docs page.",
		[]utils.SummaryMetric{
			{Label: "Page", Value: opts.PageSlug, Tone: "success"},
			{Label: "Published", Value: map[bool]string{true: "yes", false: "no"}[result.Published], Tone: "info"},
		},
		"git status",
		"wfkit docs --dry-run",
	)

	return nil
}
