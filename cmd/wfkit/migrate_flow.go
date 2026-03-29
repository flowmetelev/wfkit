package main

import (
	"context"
	"fmt"

	"wfkit/internal/build"
	"wfkit/internal/config"
	"wfkit/internal/publish"
	"wfkit/internal/utils"
	"wfkit/internal/webflow"

	"github.com/urfave/cli/v2"
)

type migrateFlow struct {
	cliContext *cli.Context
	config     config.Config
	args       map[string]interface{}
	pagesDir   string
	baseURL    string
	token      string
	cookies    string
	pages      []webflow.Page
	globalCode webflow.GlobalCode
	plan       publish.MigrationPlan
	result     publish.MigrationPublishResult
}

func newMigrateFlow(c *cli.Context) *migrateFlow {
	return &migrateFlow{cliContext: c}
}

func (f *migrateFlow) run() error {
	if err := f.loadConfig(); err != nil {
		return err
	}

	f.printHeader()
	printMigrateTimeline(f.dryRun(), f.shouldPublish(), f.shouldPushAssets(), f.shouldPublish(), false, false, false, false, false, false, false)

	if err := f.authenticate(); err != nil {
		return err
	}
	printMigrateTimeline(f.dryRun(), f.shouldPublish(), f.shouldPushAssets(), f.shouldPublish(), true, false, false, false, false, false, false)

	if err := f.loadPages(); err != nil {
		return err
	}
	printMigrateTimeline(f.dryRun(), f.shouldPublish(), f.shouldPushAssets(), f.shouldPublish(), true, true, false, false, false, false, false)

	if err := f.loadGlobalCode(); err != nil {
		return err
	}
	printMigrateTimeline(f.dryRun(), f.shouldPublish(), f.shouldPushAssets(), f.shouldPublish(), true, true, true, false, false, false, false)

	if err := f.planMigration(); err != nil {
		return err
	}
	printMigrationPlan(f.plan)

	if !hasPendingMigrations(f.plan) {
		utils.CPrint("No page migrations are needed", "green")
		return nil
	}

	if f.dryRun() {
		utils.CPrint("Dry run mode: no files or Webflow code were changed", "yellow")
		printMigrateTimeline(true, f.shouldPublish(), f.shouldPushAssets(), f.shouldPublish(), true, true, true, false, false, false, false)
		return nil
	}

	if err := f.writeFiles(); err != nil {
		return err
	}
	printMigrateTimeline(false, f.shouldPublish(), f.shouldPushAssets(), f.shouldPublish(), true, true, true, true, false, false, false)

	if !f.shouldPublish() {
		f.printSuccess()
		return nil
	}

	if err := f.buildAssets(); err != nil {
		return err
	}
	printMigrateTimeline(false, f.shouldPublish(), f.shouldPushAssets(), f.shouldPublish(), true, true, true, true, true, false, false)

	if f.shouldPushAssets() {
		if err := f.pushGit(); err != nil {
			return err
		}
	}
	printMigrateTimeline(false, f.shouldPublish(), f.shouldPushAssets(), f.shouldPublish(), true, true, true, true, true, f.shouldPushAssets(), false)

	if err := f.publish(); err != nil {
		return err
	}

	f.printSuccess()
	return nil
}

func (f *migrateFlow) loadConfig() error {
	cfg, err := config.ReadConfig()
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}
	deliveryMode := resolveDeliveryModeFlag(f.cliContext, cfg.DeliveryMode)
	if cfg.AppName == "" {
		return fmt.Errorf("missing appName configuration in wfkit.json")
	}
	if f.shouldPublish() && deliveryMode == "cdn" {
		if err := cfg.ValidatePublish(); err != nil {
			return err
		}
	}

	f.config = cfg
	f.pagesDir = resolveStringFlag(f.cliContext, "pages-dir", "src/pages")
	f.args = map[string]interface{}{
		"env":           "prod",
		"delivery":      deliveryMode,
		"asset-branch":  resolveAssetBranchFlag(f.cliContext, cfg.AssetBranch),
		"build-dir":     resolveStringFlag(f.cliContext, "build-dir", cfg.BuildDir),
		"custom-commit": f.cliContext.String("custom-commit"),
		"notify":        resolveNotifyFlag(f.cliContext),
	}
	f.baseURL = cfg.EffectiveDesignURL()
	return nil
}

func (f *migrateFlow) printHeader() {
	utils.PrintSection("Migrate")
	utils.PrintKeyValue("Webflow", f.baseURL)
	fmt.Println()
}

func (f *migrateFlow) authenticate() error {
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

func (f *migrateFlow) loadPages() error {
	return utils.RunTask("Load pages from Webflow", func() error {
		pages, err := webflow.GetPagesListFromDom(f.cliContext.Context, f.config.AppName, f.token, f.cookies)
		if err != nil {
			return fmt.Errorf("failed to fetch pages from Webflow: %w", err)
		}

		f.pages = pages
		return nil
	})
}

func (f *migrateFlow) loadGlobalCode() error {
	return utils.RunTask("Load global custom code", func() error {
		globalCode, err := webflow.GetGlobalCode(f.cliContext.Context, f.config.AppName, f.token, f.cookies)
		if err != nil {
			return fmt.Errorf("failed to fetch global code from Webflow: %w", err)
		}

		f.globalCode = globalCode
		return nil
	})
}

func (f *migrateFlow) planMigration() error {
	plan, err := publish.PlanMigration(f.globalCode, f.pages, f.pagesDir, f.config.GlobalEntry, f.cliContext.Bool("force"))
	if err != nil {
		return fmt.Errorf("failed to plan migration: %w", err)
	}

	f.plan = plan
	return nil
}

func (f *migrateFlow) writeFiles() error {
	utils.CPrint("Writing migrated files...", "cyan")
	if err := publish.WriteMigrationFiles(f.plan); err != nil {
		return fmt.Errorf("failed to write migration files: %w", err)
	}

	return nil
}

func (f *migrateFlow) buildAssets() error {
	utils.CPrint("Building migrated pages...", "cyan")
	if f.delivery() == "inline" {
		if err := build.RunProjectBuild(f.args["build-dir"].(string), f.config.PackageManager); err != nil {
			return fmt.Errorf("build failed after migration: %w", err)
		}
		inlineBundles, err := build.BuildInlineBundles(f.args["build-dir"].(string), f.config.PackageManager)
		if err != nil {
			return fmt.Errorf("inline bundle build failed after migration: %w", err)
		}
		f.args["inline-global"] = inlineBundles.Global
		f.args["inline-pages"] = inlineBundles.Pages
		utils.CPrint("Build successful, inline bundles are ready for Webflow", "green")
		return nil
	}

	scriptURL, err := build.DoBuild(f.args, f.config.GitHubUser, f.config.RepositoryName, f.config.PackageManager)
	if err != nil {
		return fmt.Errorf("build failed after migration: %w", err)
	}
	utils.CPrint(fmt.Sprintf("Build successful, global script URL: %s", scriptURL), "green")

	return nil
}

func (f *migrateFlow) pushGit() error {
	utils.CPrint("Publishing migrated build artifacts to GitHub...", "cyan")
	if err := ensureGitHubRepositoryReady(f.config.GitHubUser, f.config.RepositoryName); err != nil {
		return err
	}
	gitResult, err := build.PublishBuildArtifacts(build.ArtifactPublishOptions{
		BuildDir:      f.args["build-dir"].(string),
		AssetBranch:   f.args["asset-branch"].(string),
		CommitMessage: f.args["custom-commit"].(string),
	})
	if err != nil {
		return fmt.Errorf("GitHub push failed after migration: %w", err)
	}
	printGitPushSummary(gitResult)

	return nil
}

func (f *migrateFlow) publish() error {
	utils.CPrint("Publishing migrated page scripts to Webflow...", "cyan")
	publishCtx, cancel := context.WithCancel(f.cliContext.Context)
	defer cancel()

	result, err := publish.PublishMigratedPages(
		publishCtx,
		f.config.AppName,
		f.baseURL,
		f.cookies,
		f.token,
		f.config.GitHubUser,
		f.config.RepositoryName,
		f.args,
		f.plan,
	)
	if err != nil {
		return fmt.Errorf("migration publish failed: %w", err)
	}

	f.result = result
	return nil
}

func (f *migrateFlow) printSuccess() {
	if f.shouldPublish() {
		printMigrationPublishResult(f.result)
		printMigrateTimeline(false, f.shouldPublish(), f.shouldPushAssets(), f.shouldPublish(), true, true, true, true, true, f.shouldPushAssets(), f.result.Published)

		notifySuccess(f.args["notify"].(bool), "wfkit migrate completed", "Webflow code migration finished successfully.")

		utils.PrintSuccessScreen(
			"Migration completed",
			"Legacy Webflow code has been moved into local files and published back to Webflow.",
			[]utils.SummaryMetric{
				{Label: "Pages updated", Value: fmt.Sprintf("%d", f.result.UpdatedPages), Tone: "success"},
				{Label: "Delivery", Value: f.delivery(), Tone: "info"},
				{Label: "Published", Value: map[bool]string{true: "yes", false: "no"}[f.result.Published], Tone: "info"},
			},
			"git status",
			"wfkit publish --env prod --dry-run",
		)
		return
	}

	printMigrateTimeline(false, false, false, false, true, true, true, true, false, false, false)
	notifySuccess(f.args["notify"].(bool), "wfkit migrate completed", "Webflow code migration files were written locally.")
	utils.PrintSuccessScreen(
		"Migration completed",
		"Legacy Webflow code has been moved into local files. Review the diff, then publish when you are ready.",
		[]utils.SummaryMetric{
			{Label: "Pages migrated", Value: fmt.Sprintf("%d", countPageMigrations(f.plan)), Tone: "success"},
			{Label: "Global migrated", Value: map[bool]string{true: "yes", false: "no"}[f.plan.Global.Action == "write" || f.plan.Global.Action == "overwrite"], Tone: "info"},
		},
		"git status",
		"wfkit publish --env prod --dry-run",
	)
}

func (f *migrateFlow) dryRun() bool {
	return f.cliContext.Bool("dry-run")
}

func (f *migrateFlow) shouldPublish() bool {
	return f.cliContext.Bool("publish")
}

func (f *migrateFlow) delivery() string {
	return f.args["delivery"].(string)
}

func (f *migrateFlow) shouldPushAssets() bool {
	return f.shouldPublish() && f.delivery() == "cdn"
}
