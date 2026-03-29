package main

import (
	"fmt"

	"wfkit/internal/config"
	"wfkit/internal/publish"
	"wfkit/internal/utils"
	"wfkit/internal/webflow"

	"github.com/urfave/cli/v2"
)

type docsFlow struct {
	cliContext *cli.Context
	config     config.Config
	options    publish.DocsHubOptions
	baseURL    string
	token      string
	cookies    string
	pages      []webflow.Page
	plan       publish.DocsHubPlan
	result     publish.DocsHubResult
}

func newDocsFlow(c *cli.Context) *docsFlow {
	return &docsFlow{cliContext: c}
}

func (f *docsFlow) run() error {
	if err := f.loadConfig(); err != nil {
		return err
	}

	f.printHeader()
	printDocsTimeline(false, false, false)

	if err := f.authenticate(); err != nil {
		return err
	}
	if err := f.loadPages(); err != nil {
		return err
	}
	if err := f.planSync(); err != nil {
		return err
	}

	printDocsPlan(f.plan)
	printDocsTimeline(true, true, false)

	if f.cliContext.Bool("dry-run") {
		utils.CPrint("Dry run mode: the docs hub page was not changed", "yellow")
		return nil
	}

	if err := f.apply(); err != nil {
		return err
	}

	f.printSuccess()
	return nil
}

func (f *docsFlow) loadConfig() error {
	cfg, err := config.ReadConfig()
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}
	if cfg.AppName == "" {
		return fmt.Errorf("missing appName configuration in wfkit.json")
	}

	f.config = cfg
	f.options = publish.DocsHubOptions{
		EntryPath: resolveStringFlag(f.cliContext, "file", cfg.DocsEntry),
		PageSlug:  resolveStringFlag(f.cliContext, "page-slug", cfg.DocsPageSlug),
		Publish:   f.cliContext.Bool("publish"),
		Selector:  resolveStringFlag(f.cliContext, "selector", `[data-wf-docs-root], main`),
	}
	f.baseURL = cfg.EffectiveDesignURL()
	return nil
}

func (f *docsFlow) printHeader() {
	utils.PrintSection("Docs Hub")
	utils.PrintKeyValue("Site", f.baseURL)
	utils.PrintKeyValue("Entry", f.options.EntryPath)
	utils.PrintKeyValue("Page", f.options.PageSlug)
	fmt.Println()
}

func (f *docsFlow) authenticate() error {
	return utils.RunTask("Authenticate with Webflow", func() error {
		token, cookies, err := webflow.GetCsrfTokenAndCookies(f.cliContext.Context, f.baseURL)
		if err != nil {
			return fmt.Errorf("failed to authenticate with Webflow: %w", err)
		}

		f.token = token
		f.cookies = cookies
		return nil
	})
}

func (f *docsFlow) loadPages() error {
	return utils.RunTask("Load pages from Webflow", func() error {
		pages, err := webflow.GetPagesListFromDom(f.cliContext.Context, f.config.AppName, f.token, f.cookies)
		if err != nil {
			return fmt.Errorf("failed to load pages: %w", err)
		}

		f.pages = pages
		return nil
	})
}

func (f *docsFlow) planSync() error {
	plan, err := publish.PlanDocsHubSync(f.pages, f.options)
	if err != nil {
		return fmt.Errorf("failed to plan docs sync: %w", err)
	}

	f.plan = plan
	return nil
}

func (f *docsFlow) apply() error {
	return utils.RunTask("Publish docs hub", func() error {
		result, err := publish.ApplyDocsHubSync(
			f.cliContext.Context,
			f.config.AppName,
			f.baseURL,
			f.token,
			f.cookies,
			f.plan,
			f.options.Publish,
		)
		if err != nil {
			return err
		}

		f.result = result
		return nil
	})
}

func (f *docsFlow) printSuccess() {
	printDocsTimeline(true, true, true)
	printDocsResult(f.result)

	if resolveNotifyFlag(f.cliContext) {
		notifySuccess(true, "wfkit docs completed", "The docs hub page finished syncing.")
	}

	utils.PrintSuccessScreen(
		"Docs hub synced",
		"Markdown documentation has been published to the Webflow docs page.",
		[]utils.SummaryMetric{
			{Label: "Page", Value: f.options.PageSlug, Tone: "success"},
			{Label: "Created", Value: map[bool]string{true: "yes", false: "no"}[f.result.Created], Tone: "info"},
			{Label: "Published", Value: map[bool]string{true: "yes", false: "no"}[f.result.Published], Tone: "info"},
		},
		"git status",
		"wfkit docs --dry-run",
	)
}
