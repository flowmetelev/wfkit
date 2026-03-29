package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"wfkit/internal/build"
	"wfkit/internal/config"
	"wfkit/internal/publish"
	"wfkit/internal/utils"
	"wfkit/internal/webflow"

	"github.com/urfave/cli/v2"
)

type publishRequest struct {
	cli       *cli.Context
	cfg       config.Config
	args      map[string]interface{}
	baseURL   string
	pToken    string
	cookies   string
	preflight *webflow.PublishPreflight
}

func newPublishRequest(c *cli.Context, cfg config.Config) *publishRequest {
	devHost := resolveStringFlag(c, "dev-host", cfg.DevHost)
	devPort := resolveIntFlag(c, "dev-port", cfg.DevPort)

	return &publishRequest{
		cli:     c,
		cfg:     cfg,
		baseURL: cfg.EffectiveDesignURL(),
		args: map[string]interface{}{
			"env":           resolveStringFlag(c, "env", "prod"),
			"by-page":       c.Bool("by-page"),
			"dry-run":       c.Bool("dry-run"),
			"script-url":    resolveStringFlag(c, "script-url", ""),
			"dev-port":      devPort,
			"dev-host":      devHost,
			"custom-commit": c.String("custom-commit"),
			"delivery":      resolveDeliveryModeFlag(c, cfg.DeliveryMode),
			"target":        resolvePublishTargetFlag(c),
			"asset-branch":  resolveAssetBranchFlag(c, cfg.AssetBranch),
			"build-dir":     resolveStringFlag(c, "build-dir", cfg.BuildDir),
			"notify":        resolveNotifyFlag(c),
		},
	}
}

func (r *publishRequest) run() error {
	if !r.isProd() && r.delivery() != "cdn" {
		return fmt.Errorf("delivery mode %q is only supported for production publishes", r.delivery())
	}
	if r.isProd() {
		return r.runProd()
	}
	return r.runDev()
}

func (r *publishRequest) authenticate() error {
	utils.PrintSection("Publish")
	utils.PrintKeyValue("Webflow", r.baseURL)
	fmt.Println()
	printPublishTimeline(r.env(), r.delivery(), r.byPage(), r.dryRun(), false, false, false, false)

	if err := utils.RunTask("Authenticate with Webflow", func() error {
		var authErr error
		r.pToken, r.cookies, authErr = webflow.GetCsrfTokenAndCookies(r.cli.Context, r.baseURL)
		if authErr != nil {
			return fmt.Errorf("failed to authenticate with Webflow: %w", authErr)
		}
		return nil
	}); err != nil {
		return err
	}

	printPublishTimeline(r.env(), r.delivery(), r.byPage(), r.dryRun(), true, false, false, false)

	if err := r.loadPreflight(); err != nil {
		return err
	}
	return nil
}

func (r *publishRequest) loadPreflight() error {
	preflight, err := webflow.GetPublishPreflight(r.cli.Context, r.cfg.AppName, r.pToken, r.cookies)
	if err != nil {
		return fmt.Errorf("failed to load Webflow publish readiness: %w", err)
	}

	r.preflight = &preflight
	r.args["publish-targets"] = resolvePublishTargets(preflight, r.target())
	printPublishReadinessForTarget(preflight, r.target())

	if err := validatePublishReadiness(preflight, r.target()); err != nil {
		return err
	}

	return nil
}

func (r *publishRequest) runProd() error {
	if r.delivery() == "cdn" && (r.cfg.GitHubUser == "" || r.cfg.RepositoryName == "") {
		return fmt.Errorf("missing ghUserName or repositoryName in wfkit.json")
	}

	utils.CPrint("Building for production...", "cyan")
	scriptURL, err := r.buildProdAssets()
	if err != nil {
		return err
	}

	printPublishTimeline("prod", r.delivery(), r.byPage(), r.dryRun(), true, true, false, false)

	if r.dryRun() {
		return r.runProdDryRun(scriptURL)
	}

	if r.delivery() == "cdn" {
		utils.CPrint("Publishing build artifacts to GitHub...", "cyan")
		if err := ensureGitHubRepositoryReady(r.cfg.GitHubUser, r.cfg.RepositoryName); err != nil {
			return err
		}

		gitResult, err := build.PublishBuildArtifacts(build.ArtifactPublishOptions{
			BuildDir:      r.buildDir(),
			AssetBranch:   r.assetBranch(),
			CommitMessage: r.customCommit(),
		})
		if err != nil {
			return fmt.Errorf("GitHub push failed: %w", err)
		}
		printGitPushSummary(gitResult)
		printPublishTimeline("prod", r.delivery(), r.byPage(), false, true, true, true, false)
	} else {
		printPublishTimeline("prod", r.delivery(), r.byPage(), false, true, true, false, false)
	}

	utils.CPrint("Publishing to Webflow...", "cyan")
	if r.byPage() {
		return r.publishProdByPage()
	}

	return r.publishProdGlobal(scriptURL)
}

func (r *publishRequest) runProdDryRun(scriptURL string) error {
	utils.CPrint("Dry run mode: no git push or Webflow update will be performed", "yellow")
	printPublishTimeline("prod", r.delivery(), r.byPage(), true, true, true, false, false)

	if r.byPage() {
		plan, err := publish.PlanByPagePublish(r.cli.Context, r.cfg.AppName, r.cookies, r.pToken, r.cfg.GitHubUser, r.cfg.RepositoryName, r.args)
		if err != nil {
			return fmt.Errorf("page-by-page dry run failed: %w", err)
		}
		printByPagePlan(plan)
		return nil
	}

	plan, err := publish.PreviewGlobalPublish(r.cli.Context, r.cfg.AppName, r.cookies, r.pToken, r.globalProdScript(scriptURL), "prod")
	if err != nil {
		return err
	}
	printGlobalPublishPlan(plan)
	return nil
}

func (r *publishRequest) publishProdGlobal(scriptURL string) error {
	plan, err := publish.PreviewGlobalPublish(r.cli.Context, r.cfg.AppName, r.cookies, r.pToken, r.globalProdScript(scriptURL), "prod")
	if err != nil {
		return err
	}
	printGlobalPublishPlan(plan)

	updated, oldCode, err := publish.PublishGlobalScript(r.cli.Context, r.cfg.AppName, r.cookies, r.pToken, r.globalProdScript(scriptURL), "prod", r.publishTargets())
	if err != nil {
		return fmt.Errorf("publishing failed: %w", err)
	}
	if updated {
		utils.CPrint("Successfully published global script to Webflow", "green")
		utils.CPrint(fmt.Sprintf("Previous code preserved for reference (%d bytes)", len(oldCode)), "green")
	} else {
		utils.CPrint("Global script is already up to date", "green")
	}

	printPublishTimeline("prod", r.delivery(), false, false, true, true, r.delivery() == "cdn", true)
	return nil
}

func (r *publishRequest) publishProdByPage() error {
	plan, err := publish.PlanByPagePublish(r.cli.Context, r.cfg.AppName, r.cookies, r.pToken, r.cfg.GitHubUser, r.cfg.RepositoryName, r.args)
	if err != nil {
		return fmt.Errorf("page-by-page planning failed: %w", err)
	}
	printByPagePlan(plan)

	result, err := publish.PublishByPage(r.cli.Context, r.cfg.AppName, r.baseURL, r.cookies, r.pToken, r.cfg.GitHubUser, r.cfg.RepositoryName, r.args)
	if err != nil {
		return fmt.Errorf("page-by-page publishing failed: %w", err)
	}

	utils.CPrint(fmt.Sprintf("Page publish summary: %d page(s) updated", result.UpdatedPages), "green")
	printPublishTimeline("prod", r.delivery(), true, false, true, true, r.delivery() == "cdn", result.Published)
	return nil
}

func (r *publishRequest) runDev() error {
	scriptURL := r.scriptURL()
	if scriptURL == "" {
		scriptURL = buildLocalScriptURL(r.devHost(), r.devPort(), r.cfg.GlobalEntry)
	}

	if r.dryRun() {
		return r.runDevDryRun(scriptURL)
	}

	ctx, cancel := context.WithCancel(r.cli.Context)
	defer cancel()

	devServer, err := ensureDevServerRunning(ctx, r.cfg.PackageManager, scriptURL, resolveListenHost(r.devHost()), r.devPort())
	if err != nil {
		return err
	}
	defer func() {
		_ = devServer.Stop(5 * time.Second)
	}()
	printPublishTimeline("dev", r.delivery(), r.byPage(), false, true, true, false, false)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	utils.CPrint("Publishing development script to Webflow...", "cyan")

	savedGlobalCode, shouldRevertGlobal, err := r.publishDev(ctx, scriptURL)
	if err != nil {
		cancel()
		return err
	}

	<-sigCh
	utils.CPrint("Shutting down development server...", "yellow")

	if shouldRevertGlobal {
		r.revertDevGlobal(savedGlobalCode)
	}

	cancel()

	if err := devServer.Stop(5 * time.Second); err != nil {
		return fmt.Errorf("development server error: %v", err)
	}

	return nil
}

func (r *publishRequest) runDevDryRun(scriptURL string) error {
	utils.CPrint("Dry run mode: no Webflow update will be performed", "yellow")
	printPublishTimeline("dev", r.delivery(), r.byPage(), true, true, false, false, false)

	if r.byPage() {
		plan, err := publish.PlanByPagePublish(r.cli.Context, r.cfg.AppName, r.cookies, r.pToken, r.cfg.GitHubUser, r.cfg.RepositoryName, r.args)
		if err != nil {
			return fmt.Errorf("page-by-page dry run failed: %w", err)
		}
		printByPagePlan(plan)
		return nil
	}

	plan, err := publish.PreviewGlobalPublish(r.cli.Context, r.cfg.AppName, r.cookies, r.pToken, publish.ManagedScript{Delivery: "cdn", URL: scriptURL}, "dev")
	if err != nil {
		return err
	}
	printGlobalPublishPlan(plan)
	return nil
}

func (r *publishRequest) publishDev(ctx context.Context, scriptURL string) ([2]string, bool, error) {
	if r.byPage() {
		plan, err := publish.PlanByPagePublish(ctx, r.cfg.AppName, r.cookies, r.pToken, r.cfg.GitHubUser, r.cfg.RepositoryName, r.args)
		if err != nil {
			return [2]string{}, false, fmt.Errorf("page-by-page planning failed: %w", err)
		}
		printByPagePlan(plan)

		result, err := publish.PublishByPage(ctx, r.cfg.AppName, r.baseURL, r.cookies, r.pToken, r.cfg.GitHubUser, r.cfg.RepositoryName, r.args)
		if err != nil {
			return [2]string{}, false, fmt.Errorf("page-by-page development publishing failed: %w", err)
		}

		utils.CPrint(fmt.Sprintf("Page publish summary: %d page(s) updated", result.UpdatedPages), "green")
		utils.CPrint("Press Ctrl+C to stop development mode", "yellow")
		printPublishTimeline("dev", r.delivery(), true, false, true, true, false, result.Published)
		return [2]string{}, false, nil
	}

	plan, err := publish.PreviewGlobalPublish(ctx, r.cfg.AppName, r.cookies, r.pToken, publish.ManagedScript{Delivery: "cdn", URL: scriptURL}, "dev")
	if err != nil {
		return [2]string{}, false, err
	}
	printGlobalPublishPlan(plan)

	updated, oldCode, err := publish.PublishGlobalScript(ctx, r.cfg.AppName, r.cookies, r.pToken, publish.ManagedScript{Delivery: "cdn", URL: scriptURL}, "dev", []string{r.cfg.AppName + ".webflow.io"})
	if err != nil {
		return [2]string{}, false, fmt.Errorf("publishing failed: %w", err)
	}

	if updated && len(oldCode) >= 2 {
		utils.CPrint("Successfully published global development script", "green")
		utils.CPrint("Press Ctrl+C to stop development and revert changes", "yellow")
		printPublishTimeline("dev", r.delivery(), false, false, true, true, false, true)
		return oldCode, true, nil
	}

	utils.CPrint("Global development script is already up to date", "green")
	utils.CPrint("Press Ctrl+C to stop development mode", "yellow")
	printPublishTimeline("dev", r.delivery(), false, false, true, true, false, true)
	return [2]string{}, false, nil
}

func (r *publishRequest) revertDevGlobal(savedGlobalCode [2]string) {
	utils.CPrint("Reverting Webflow code changes...", "yellow")
	revertCtx, revertCancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer revertCancel()

	if err := webflow.UpdateGlobalCode(revertCtx, r.cfg.AppName, r.pToken, r.cookies, savedGlobalCode[0], savedGlobalCode[1]); err != nil {
		utils.CPrint(fmt.Sprintf("Failed to restore Webflow code: %v", err), "red")
		return
	}
	if err := webflow.PublishSite(revertCtx, r.cfg.AppName, r.pToken, r.cookies); err != nil {
		utils.CPrint(fmt.Sprintf("Code restored, but failed to publish rollback: %v", err), "red")
		return
	}

	utils.CPrint("Webflow code restored and site republished", "green")
}

func (r *publishRequest) printSuccess() {
	notifySuccess(r.notify(), "wfkit publish completed", "Webflow production publish finished successfully.")

	if r.env() != "prod" || r.dryRun() {
		return
	}

	utils.PrintSuccessScreen(
		"Publish completed",
		"Production assets are built and Webflow has been updated.",
		[]utils.SummaryMetric{
			{Label: "Environment", Value: "prod", Tone: "success"},
			{Label: "Delivery", Value: r.delivery(), Tone: "info"},
			{Label: "Target", Value: r.target(), Tone: "info"},
			{Label: "Mode", Value: map[bool]string{true: "by-page", false: "global"}[r.byPage()], Tone: "info"},
		},
		"git status",
		"wfkit proxy",
	)
}

func (r *publishRequest) env() string      { return r.args["env"].(string) }
func (r *publishRequest) byPage() bool     { return r.args["by-page"].(bool) }
func (r *publishRequest) dryRun() bool     { return r.args["dry-run"].(bool) }
func (r *publishRequest) delivery() string { return r.args["delivery"].(string) }
func (r *publishRequest) target() string   { return r.args["target"].(string) }
func (r *publishRequest) publishTargets() []string {
	targets, _ := r.args["publish-targets"].([]string)
	return targets
}
func (r *publishRequest) scriptURL() string    { return r.args["script-url"].(string) }
func (r *publishRequest) devPort() int         { return r.args["dev-port"].(int) }
func (r *publishRequest) devHost() string      { return r.args["dev-host"].(string) }
func (r *publishRequest) assetBranch() string  { return r.args["asset-branch"].(string) }
func (r *publishRequest) buildDir() string     { return r.args["build-dir"].(string) }
func (r *publishRequest) customCommit() string { return r.args["custom-commit"].(string) }
func (r *publishRequest) notify() bool         { return r.args["notify"].(bool) }
func (r *publishRequest) isProd() bool         { return r.env() == "prod" }

func (r *publishRequest) buildProdAssets() (string, error) {
	if r.delivery() == "inline" {
		if err := build.RunProjectBuild(r.buildDir(), r.cfg.PackageManager); err != nil {
			return "", fmt.Errorf("build failed: %w", err)
		}
		inlineBundles, err := build.BuildInlineBundles(r.buildDir(), r.cfg.PackageManager)
		if err != nil {
			return "", fmt.Errorf("inline bundle build failed: %w", err)
		}
		if !r.byPage() && strings.TrimSpace(inlineBundles.Global) == "" {
			return "", fmt.Errorf("inline delivery requires a global bundle, but no global entry was built")
		}
		r.args["inline-global"] = inlineBundles.Global
		r.args["inline-pages"] = inlineBundles.Pages
		utils.CPrint("Build successful, inline bundles are ready for Webflow", "green")
		return fmt.Sprintf("inline module (%d page bundle(s))", len(inlineBundles.Pages)), nil
	}

	scriptURL, err := build.DoBuild(r.args, r.cfg.GitHubUser, r.cfg.RepositoryName, r.cfg.PackageManager)
	if err != nil {
		return "", fmt.Errorf("build failed: %w", err)
	}
	utils.CPrint(fmt.Sprintf("Build successful, script URL: %s", scriptURL), "green")
	return scriptURL, nil
}

func (r *publishRequest) globalProdScript(scriptURL string) publish.ManagedScript {
	if r.delivery() == "inline" {
		code, _ := r.args["inline-global"].(string)
		return publish.ManagedScript{Delivery: "inline", Code: code}
	}
	return publish.ManagedScript{Delivery: "cdn", URL: scriptURL}
}
