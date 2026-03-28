package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"wfkit/internal/build"
	"wfkit/internal/config"
	"wfkit/internal/publish"
	"wfkit/internal/updater"
	"wfkit/internal/utils"
	"wfkit/internal/webflow"

	"github.com/urfave/cli/v2"
)

func publishMode(c *cli.Context) error {
	if c.Bool("update") {
		updateManager := updater.NewUpdateManager(c.App.Version)
		result, err := updateManager.Check(updater.CheckOptions{Force: true, AllowStale: true})
		if err != nil {
			if err.Error() == "github api rate limit exceeded" {
				utils.CPrint("Warning: GitHub API rate limit exceeded. Skipping update check.", "yellow")
			} else {
				utils.CPrint(fmt.Sprintf("Warning: couldn't check for updates: %v", err), "yellow")
			}
		} else if result.Available {
			utils.PrintUpdateBanner(c.App.Version, result.LatestVersion)
		} else {
			utils.CPrint("You are using the latest version", "green")
		}
	}

	cfg, err := config.ReadConfig()
	if err != nil {
		return fmt.Errorf("failed to read config: %v", err)
	}
	if cfg.AppName == "" {
		return fmt.Errorf("missing appName configuration in wfkit.json")
	}

	devHost := resolveStringFlag(c, "dev-host", cfg.DevHost)
	devPort := resolveIntFlag(c, "dev-port", cfg.DevPort)
	args := map[string]interface{}{
		"env":           c.String("env"),
		"by-page":       c.Bool("by-page"),
		"dry-run":       c.Bool("dry-run"),
		"script-url":    resolveStringFlag(c, "script-url", ""),
		"dev-port":      devPort,
		"dev-host":      devHost,
		"custom-commit": c.String("custom-commit"),
		"branch":        resolveStringFlag(c, "branch", cfg.Branch),
		"build-dir":     resolveStringFlag(c, "build-dir", cfg.BuildDir),
		"notify":        resolveNotifyFlag(c),
	}

	baseURL := cfg.EffectiveDesignURL()
	utils.PrintSection("Publish")
	utils.PrintKeyValue("Webflow", baseURL)
	fmt.Println()
	printPublishTimeline(c.String("env"), args["by-page"].(bool), args["dry-run"].(bool), false, false, false, false)

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
	printPublishTimeline(c.String("env"), args["by-page"].(bool), args["dry-run"].(bool), true, false, false, false)

	if c.String("env") == "prod" {
		if cfg.GitHubUser == "" || cfg.RepositoryName == "" {
			return fmt.Errorf("missing ghUserName or repositoryName in wfkit.json")
		}
		utils.CPrint("Building for production...", "cyan")
		scriptURL, err := build.DoBuild(args, cfg.GitHubUser, cfg.RepositoryName, cfg.PackageManager)
		if err != nil {
			return fmt.Errorf("build failed: %w", err)
		}
		utils.CPrint(fmt.Sprintf("Build successful, script URL: %s", scriptURL), "green")
		printPublishTimeline("prod", args["by-page"].(bool), args["dry-run"].(bool), true, true, false, false)

		if args["dry-run"].(bool) {
			utils.CPrint("Dry run mode: no git push or Webflow update will be performed", "yellow")
			printPublishTimeline("prod", args["by-page"].(bool), true, true, true, false, false)
			if !args["by-page"].(bool) {
				plan, err := publish.PreviewGlobalPublish(c.Context, cfg.AppName, cookies, pToken, scriptURL, "prod")
				if err != nil {
					return err
				}
				printGlobalPublishPlan(plan)
			} else {
				plan, err := publish.PlanByPagePublish(c.Context, cfg.AppName, cookies, pToken, cfg.GitHubUser, cfg.RepositoryName, args)
				if err != nil {
					return fmt.Errorf("page-by-page dry run failed: %w", err)
				}
				printByPagePlan(plan)
			}
			return nil
		}

		utils.CPrint("Pushing to GitHub...", "cyan")
		if err := ensureGitHubRepositoryReady(cfg.GitHubUser, cfg.RepositoryName); err != nil {
			return err
		}
		gitResult, err := build.DoPushToGithub(args["branch"].(string), args["custom-commit"].(string))
		if err != nil {
			return fmt.Errorf("GitHub push failed: %w", err)
		}
		printGitPushSummary(gitResult)
		printPublishTimeline("prod", args["by-page"].(bool), false, true, true, true, false)

		utils.CPrint("Publishing to Webflow...", "cyan")
		if !args["by-page"].(bool) {
			plan, err := publish.PreviewGlobalPublish(c.Context, cfg.AppName, cookies, pToken, scriptURL, "prod")
			if err != nil {
				return err
			}
			printGlobalPublishPlan(plan)

			updated, oldCode, err := publish.PublishGlobalScript(c.Context, cfg.AppName, cookies, pToken, scriptURL, "prod")
			if err != nil {
				return fmt.Errorf("publishing failed: %w", err)
			}
			if updated {
				utils.CPrint("Successfully published global script to Webflow", "green")
				utils.CPrint(fmt.Sprintf("Previous code preserved for reference (%d bytes)", len(oldCode)), "green")
			} else {
				utils.CPrint("Global script is already up to date", "green")
			}
			printPublishTimeline("prod", false, false, true, true, true, true)
		} else {
			plan, err := publish.PlanByPagePublish(c.Context, cfg.AppName, cookies, pToken, cfg.GitHubUser, cfg.RepositoryName, args)
			if err != nil {
				return fmt.Errorf("page-by-page planning failed: %w", err)
			}
			printByPagePlan(plan)

			result, err := publish.PublishByPage(c.Context, cfg.AppName, baseURL, cookies, pToken, cfg.GitHubUser, cfg.RepositoryName, args)
			if err != nil {
				return fmt.Errorf("page-by-page publishing failed: %w", err)
			}
			utils.CPrint(fmt.Sprintf("Page publish summary: %d page(s) updated", result.UpdatedPages), "green")
			printPublishTimeline("prod", true, false, true, true, true, result.Published)
		}
	} else {
		scriptURL := args["script-url"].(string)
		if scriptURL == "" {
			scriptURL = buildLocalScriptURL(devHost, devPort, cfg.GlobalEntry)
		}

		if args["dry-run"].(bool) {
			utils.CPrint("Dry run mode: no Webflow update will be performed", "yellow")
			printPublishTimeline("dev", args["by-page"].(bool), true, true, false, false, false)
			if !args["by-page"].(bool) {
				plan, err := publish.PreviewGlobalPublish(c.Context, cfg.AppName, cookies, pToken, scriptURL, "dev")
				if err != nil {
					return err
				}
				printGlobalPublishPlan(plan)
			} else {
				plan, err := publish.PlanByPagePublish(c.Context, cfg.AppName, cookies, pToken, cfg.GitHubUser, cfg.RepositoryName, args)
				if err != nil {
					return fmt.Errorf("page-by-page dry run failed: %w", err)
				}
				printByPagePlan(plan)
			}
			return nil
		}

		ctx, cancel := context.WithCancel(c.Context)
		defer cancel()

		devServer, err := ensureDevServerRunning(ctx, cfg.PackageManager, scriptURL, resolveListenHost(devHost), devPort)
		if err != nil {
			return err
		}
		printPublishTimeline("dev", args["by-page"].(bool), false, true, true, false, false)
		defer func() {
			_ = devServer.Stop(5 * time.Second)
		}()

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(sigCh)

		utils.CPrint("Publishing development script to Webflow...", "cyan")

		var (
			savedGlobalCode    [2]string
			shouldRevertGlobal bool
		)

		if !args["by-page"].(bool) {
			plan, err := publish.PreviewGlobalPublish(ctx, cfg.AppName, cookies, pToken, scriptURL, "dev")
			if err != nil {
				cancel()
				return err
			}
			printGlobalPublishPlan(plan)

			updated, oldCode, err := publish.PublishGlobalScript(ctx, cfg.AppName, cookies, pToken, scriptURL, "dev")
			if err != nil {
				cancel()
				return fmt.Errorf("publishing failed: %w", err)
			}

			if updated && len(oldCode) >= 2 {
				savedGlobalCode = oldCode
				shouldRevertGlobal = true
				utils.CPrint("Successfully published global development script", "green")
				utils.CPrint("Press Ctrl+C to stop development and revert changes", "yellow")
			} else {
				utils.CPrint("Global development script is already up to date", "green")
				utils.CPrint("Press Ctrl+C to stop development mode", "yellow")
			}
			printPublishTimeline("dev", false, false, true, true, false, true)
		} else {
			plan, err := publish.PlanByPagePublish(ctx, cfg.AppName, cookies, pToken, cfg.GitHubUser, cfg.RepositoryName, args)
			if err != nil {
				cancel()
				return fmt.Errorf("page-by-page planning failed: %w", err)
			}
			printByPagePlan(plan)

			result, err := publish.PublishByPage(ctx, cfg.AppName, baseURL, cookies, pToken, cfg.GitHubUser, cfg.RepositoryName, args)
			if err != nil {
				cancel()
				return fmt.Errorf("page-by-page development publishing failed: %w", err)
			}
			utils.CPrint(fmt.Sprintf("Page publish summary: %d page(s) updated", result.UpdatedPages), "green")
			utils.CPrint("Press Ctrl+C to stop development mode", "yellow")
			printPublishTimeline("dev", true, false, true, true, false, result.Published)
		}

		<-sigCh
		utils.CPrint("Shutting down development server...", "yellow")

		if shouldRevertGlobal {
			utils.CPrint("Reverting Webflow code changes...", "yellow")
			revertCtx, revertCancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer revertCancel()

			if err := webflow.UpdateGlobalCode(revertCtx, cfg.AppName, pToken, cookies, savedGlobalCode[0], savedGlobalCode[1]); err != nil {
				utils.CPrint(fmt.Sprintf("Failed to restore Webflow code: %v", err), "red")
			} else if err := webflow.PublishSite(revertCtx, cfg.AppName, pToken, cookies); err != nil {
				utils.CPrint(fmt.Sprintf("Code restored, but failed to publish rollback: %v", err), "red")
			} else {
				utils.CPrint("Webflow code restored and site republished", "green")
			}
		}

		cancel()

		if err := devServer.Stop(5 * time.Second); err != nil {
			return fmt.Errorf("development server error: %v", err)
		}
	}

	notifySuccess(args["notify"].(bool), "wfkit publish completed", "Webflow production publish finished successfully.")

	if c.String("env") == "prod" && !args["dry-run"].(bool) {
		utils.PrintSuccessScreen(
			"Publish completed",
			"Production assets are built and Webflow has been updated.",
			[]utils.SummaryMetric{
				{Label: "Environment", Value: "prod", Tone: "success"},
				{Label: "Branch", Value: args["branch"].(string), Tone: "info"},
				{Label: "Mode", Value: map[bool]string{true: "by-page", false: "global"}[args["by-page"].(bool)], Tone: "info"},
			},
			"git status",
			"wfkit proxy",
		)
	}

	return nil
}
