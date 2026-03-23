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

func migrateMode(c *cli.Context) error {
	cfg, err := config.ReadConfig()
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}
	if err := cfg.ValidatePublish(); err != nil {
		return err
	}

	pagesDir := resolveStringFlag(c, "pages-dir", "src/pages")
	args := map[string]interface{}{
		"env":           "prod",
		"branch":        resolveStringFlag(c, "branch", cfg.Branch),
		"build-dir":     resolveStringFlag(c, "build-dir", cfg.BuildDir),
		"custom-commit": c.String("custom-commit"),
		"notify":        resolveNotifyFlag(c),
	}

	baseURL := cfg.EffectiveDesignURL()
	utils.PrintSection("Migrate")
	utils.PrintKeyValue("Webflow", baseURL)
	fmt.Println()
	printMigrateTimeline(c.Bool("dry-run"), false, false, false, false, false, false, false)

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
	printMigrateTimeline(c.Bool("dry-run"), true, false, false, false, false, false, false)

	var pages []webflow.Page
	if err := utils.RunTask("Load pages from Webflow", func() error {
		var loadErr error
		pages, loadErr = webflow.GetPagesListFromDom(c.Context, cfg.AppName, pToken, cookies)
		if loadErr != nil {
			return fmt.Errorf("failed to fetch pages from Webflow: %w", loadErr)
		}
		return nil
	}); err != nil {
		return err
	}
	printMigrateTimeline(c.Bool("dry-run"), true, true, false, false, false, false, false)

	var globalCode webflow.GlobalCode
	if err := utils.RunTask("Load global custom code", func() error {
		var loadErr error
		globalCode, loadErr = webflow.GetGlobalCode(c.Context, cfg.AppName, pToken, cookies)
		if loadErr != nil {
			return fmt.Errorf("failed to fetch global code from Webflow: %w", loadErr)
		}
		return nil
	}); err != nil {
		return err
	}
	printMigrateTimeline(c.Bool("dry-run"), true, true, true, false, false, false, false)

	plan, err := publish.PlanMigration(globalCode, pages, pagesDir, cfg.GlobalEntry, c.Bool("force"))
	if err != nil {
		return fmt.Errorf("failed to plan migration: %w", err)
	}
	printMigrationPlan(plan)

	if !hasPendingMigrations(plan) {
		utils.CPrint("No page migrations are needed", "green")
		return nil
	}

	if c.Bool("dry-run") {
		utils.CPrint("Dry run mode: no files, git history, or Webflow pages were changed", "yellow")
		printMigrateTimeline(true, true, true, true, false, false, false, false)
		return nil
	}

	utils.CPrint("Writing migrated files...", "cyan")
	if err := publish.WriteMigrationFiles(plan); err != nil {
		return fmt.Errorf("failed to write migration files: %w", err)
	}
	printMigrateTimeline(false, true, true, true, true, false, false, false)

	utils.CPrint("Building migrated pages...", "cyan")
	scriptURL, err := build.DoBuild(args, cfg.GitHubUser, cfg.RepositoryName, cfg.PackageManager)
	if err != nil {
		return fmt.Errorf("build failed after migration: %w", err)
	}
	utils.CPrint(fmt.Sprintf("Build successful, global script URL: %s", scriptURL), "green")
	printMigrateTimeline(false, true, true, true, true, true, false, false)

	utils.CPrint("Pushing migrated files to GitHub...", "cyan")
	if err := ensureGitHubRepositoryReady(cfg.GitHubUser, cfg.RepositoryName); err != nil {
		return err
	}
	gitResult, err := build.DoPushToGithub(args["branch"].(string), args["custom-commit"].(string))
	if err != nil {
		return fmt.Errorf("GitHub push failed after migration: %w", err)
	}
	printGitPushSummary(gitResult)
	printMigrateTimeline(false, true, true, true, true, true, true, false)

	utils.CPrint("Publishing migrated page scripts to Webflow...", "cyan")
	publishCtx, cancel := context.WithCancel(c.Context)
	defer cancel()

	result, err := publish.PublishMigratedPages(publishCtx, cfg.AppName, baseURL, cookies, pToken, cfg.GitHubUser, cfg.RepositoryName, args, plan)
	if err != nil {
		return fmt.Errorf("migration publish failed: %w", err)
	}

	printMigrationPublishResult(result)
	printMigrateTimeline(false, true, true, true, true, true, true, result.Published)

	notifySuccess(args["notify"].(bool), "wfkit migrate completed", "Webflow code migration finished successfully.")

	utils.PrintSuccessScreen(
		"Migration completed",
		"Legacy Webflow code has been moved into local files and published via jsDelivr.",
		[]utils.SummaryMetric{
			{Label: "Pages updated", Value: fmt.Sprintf("%d", result.UpdatedPages), Tone: "success"},
			{Label: "Published", Value: map[bool]string{true: "yes", false: "no"}[result.Published], Tone: "info"},
		},
		"git status",
		"wfkit publish --env prod --dry-run",
	)

	return nil
}
