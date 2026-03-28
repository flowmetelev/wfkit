package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"wfkit/internal/build"
	"wfkit/internal/config"
	"wfkit/internal/globalconfig"
	"wfkit/internal/initialize"
	initconfig "wfkit/internal/initialize/config"
	devproxy "wfkit/internal/proxy"
	"wfkit/internal/publish"
	"wfkit/internal/updater"
	"wfkit/internal/utils"
	"wfkit/internal/version"
	"wfkit/internal/webflow"

	"github.com/charmbracelet/huh"
	"github.com/urfave/cli/v2"
)

// configMode handles global configuration settings
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
				Options(
					huh.NewOption("Bun (Recommended)", "bun"),
					huh.NewOption("NPM", "npm"),
					huh.NewOption("Yarn", "yarn"),
					huh.NewOption("PNPM", "pnpm"),
				).
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

// interactiveMode launches the main interactive menu
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

	err := form.Run()
	if err != nil {
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

// initMode handles project initialization
func initMode(c *cli.Context) error {
	name := c.String("name")
	packageManager := c.String("package-manager")
	initGit := c.Bool("init-git")
	skipInstall := c.Bool("skip-install")
	force := c.Bool("force")

	// Попытка загрузить глобальный конфиг для подстановки значений по умолчанию
	importConfig, _ := globalconfig.LoadConfig()
	githubUser := ""
	repositoryName := ""

	if importConfig != nil {
		githubUser = importConfig.GitHubUser
		repositoryName = importConfig.RepositoryName
		if packageManager == "bun" && importConfig.PackageManager != "" {
			packageManager = importConfig.PackageManager
		}
	}

	// If the user didn't provide specific flags (or just ran 'wfkit init'), ask them interactively
	if c.NumFlags() == 0 {
		installDependencies := true
		err := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Project Name").
					Value(&name),
				huh.NewInput().
					Title("GitHub Username").
					Description("Used for CDN links (e.g. yndmitry)").
					Value(&githubUser),
				huh.NewInput().
					Title("GitHub Repository").
					Description("If empty, we'll use the Project Name").
					Value(&repositoryName),
				huh.NewSelect[string]().
					Title("Package Manager").
					Options(
						huh.NewOption("Bun (Recommended)", "bun"),
						huh.NewOption("NPM", "npm"),
						huh.NewOption("Yarn", "yarn"),
						huh.NewOption("PNPM", "pnpm"),
					).
					Value(&packageManager),
				huh.NewConfirm().
					Title("Install dependencies now?").
					Description("Installs the generated project's local CLI and frontend tooling immediately.").
					Value(&installDependencies),
				huh.NewConfirm().
					Title("Initialize git repository?").
					Description("Runs `git init` in the new project directory.").
					Value(&initGit),
			),
		).Run()

		if err != nil {
			return err
		}

		skipInstall = !installDependencies
	}

	if repositoryName == "" {
		repositoryName = filepath.Base(filepath.Clean(name))
	}

	// Сохраняем введенные данные в глобальный конфиг, чтобы использовать их в следующий раз
	globalconfig.SaveConfig(&globalconfig.Config{
		GitHubUser:     githubUser,
		RepositoryName: repositoryName,
		PackageManager: packageManager,
		Notify:         importConfig != nil && importConfig.Notify,
	})

	// Get default values for flags if they were not set (especially in interactive mode from root command)
	pagesDir := c.String("pages-dir")
	if pagesDir == "" {
		pagesDir = "src/pages"
	}

	globalEntry := c.String("global-entry")
	if globalEntry == "" {
		globalEntry = "src/global/index.ts"
	}

	globalVar := c.String("global-var")
	if globalVar == "" {
		globalVar = "WF"
	}

	opts := initconfig.Options{
		ProjectDir:     name,
		Name:           filepath.Base(filepath.Clean(name)),
		PagesDir:       pagesDir,
		GlobalEntry:    globalEntry,
		GlobalVar:      globalVar,
		InitGit:        initGit,
		Force:          force,
		SkipInstall:    skipInstall,
		PackageManager: packageManager,
		CLIValue:       c.App.Version,
		GitHubUser:     githubUser,
		RepositoryName: repositoryName,
	}

	if err := initialize.InitProject(opts); err != nil {
		return fmt.Errorf("initialization failed: %w", err)
	}

	nextSteps := []string{fmt.Sprintf("cd %s", name)}
	if skipInstall {
		nextSteps = append(nextSteps, packageManagerInstallCommand(packageManager))
	}
	nextSteps = append(nextSteps, packageManagerScriptCommand(packageManager, "dev"), "wfkit doctor")

	utils.PrintSuccessScreen(
		"Project initialized",
		"Your Webflow project scaffold is ready.",
		[]utils.SummaryMetric{
			{Label: "Project", Value: opts.Name, Tone: "success"},
			{Label: "Package manager", Value: packageManager, Tone: "info"},
			{Label: "Dependencies", Value: map[bool]string{true: "skipped", false: "installed"}[skipInstall], Tone: "info"},
		},
		nextSteps...,
	)
	return nil
}

// updateMode checks for and processes updates
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

func proxyMode(c *cli.Context) error {
	cfg, err := config.ReadConfig()
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}
	if err := cfg.ValidateProxy(); err != nil {
		return err
	}

	devHost := resolveStringFlag(c, "dev-host", cfg.DevHost)
	proxyHost := resolveStringFlag(c, "proxy-host", cfg.ProxyHost)
	if sharedHost := resolveStringFlag(c, "host", ""); sharedHost != "" {
		devHost = sharedHost
		proxyHost = sharedHost
	}
	devPort := resolveIntFlag(c, "dev-port", cfg.DevPort)
	proxyPort := resolveIntFlag(c, "proxy-port", cfg.ProxyPort)
	openBrowserFlag := resolveBoolFlag(c, "open", cfg.OpenBrowser)
	scriptURL := resolveStringFlag(c, "script-url", "")
	if scriptURL == "" {
		scriptURL = buildLocalScriptURL(devHost, devPort, cfg.GlobalEntry)
	}

	targetURL := resolveStringFlag(c, "site-url", cfg.EffectiveSiteURL())
	if targetURL == "" {
		return fmt.Errorf("missing site URL: set --site-url or configure siteUrl/appName in wfkit.json")
	}
	if _, err := url.ParseRequestURI(targetURL); err != nil {
		return fmt.Errorf("invalid site URL: %w", err)
	}

	proxyListenHost := resolveListenHost(proxyHost)
	devListenHost := resolveListenHost(devHost)

	resolvedProxyPort, relocated, err := findAvailablePort(proxyListenHost, proxyPort)
	if err != nil {
		return fmt.Errorf("failed to reserve proxy port: %w", err)
	}
	if relocated {
		utils.CPrint(fmt.Sprintf("Proxy port %d is busy, using %d instead", proxyPort, resolvedProxyPort), "yellow")
	}
	proxyPort = resolvedProxyPort

	ctx, cancel := context.WithCancel(c.Context)
	defer cancel()

	devServer, err := ensureDevServerRunning(ctx, cfg.PackageManager, scriptURL, devListenHost, devPort)
	if err != nil {
		return err
	}
	defer func() {
		_ = devServer.Stop(5 * time.Second)
	}()

	proxyURL := fmt.Sprintf("http://%s:%d", displayHost(proxyHost), proxyPort)
	proxyErrCh := make(chan error, 1)
	go func() {
		proxyErrCh <- devproxy.Serve(ctx, devproxy.Options{
			ListenHost: proxyListenHost,
			ListenPort: proxyPort,
			ScriptURL:  scriptURL,
			TargetURL:  targetURL,
		})
	}()

	if err := waitForURL(ctx, proxyURL, 10*time.Second); err != nil {
		cancel()
		return fmt.Errorf("proxy did not become ready at %s: %w", proxyURL, err)
	}

	printProxyStatus(targetURL, scriptURL, proxyURL)

	if openBrowserFlag {
		if err := openURL(proxyURL); err != nil {
			utils.CPrint(fmt.Sprintf("Failed to open browser: %v", err), "yellow")
		}
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	var retErr error
	select {
	case <-sigCh:
		utils.CPrint("Shutting down dev proxy...", "yellow")
	case err := <-proxyErrCh:
		if err != nil {
			retErr = fmt.Errorf("proxy server failed: %w", err)
		}
	}

	cancel()

	if err := devServer.Stop(5 * time.Second); err != nil {
		return fmt.Errorf("development server error: %v", err)
	}

	return retErr
}

// publishMode handles the publishing process
func publishMode(c *cli.Context) error {
	// Check for updates if requested
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

	// Read configuration
	cfg, err := config.ReadConfig()
	if err != nil {
		return fmt.Errorf("failed to read config: %v", err)
	}
	if cfg.AppName == "" {
		return fmt.Errorf("missing appName configuration in wfkit.json")
	}

	// Extract all CLI arguments
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

	// Get webflow authentication
	baseUrl := cfg.EffectiveDesignURL()
	utils.PrintSection("Publish")
	utils.PrintKeyValue("Webflow", baseUrl)
	fmt.Println()
	printPublishTimeline(c.String("env"), args["by-page"].(bool), args["dry-run"].(bool), false, false, false, false)

	var pToken string
	var cookies string
	if err := utils.RunTask("Authenticate with Webflow", func() error {
		var authErr error
		pToken, cookies, authErr = webflow.GetCsrfTokenAndCookies(c.Context, baseUrl)
		if authErr != nil {
			return fmt.Errorf("failed to authenticate with Webflow: %w", authErr)
		}
		return nil
	}); err != nil {
		return err
	}
	printPublishTimeline(c.String("env"), args["by-page"].(bool), args["dry-run"].(bool), true, false, false, false)

	// Handle production mode
	if c.String("env") == "prod" {
		if cfg.GitHubUser == "" || cfg.RepositoryName == "" {
			return fmt.Errorf("missing ghUserName or repositoryName in wfkit.json")
		}
		utils.CPrint("Building for production...", "cyan")
		scriptUrl, err := build.DoBuild(args, cfg.GitHubUser, cfg.RepositoryName, cfg.PackageManager)
		if err != nil {
			return fmt.Errorf("build failed: %w", err)
		}
		utils.CPrint(fmt.Sprintf("Build successful, script URL: %s", scriptUrl), "green")
		printPublishTimeline("prod", args["by-page"].(bool), args["dry-run"].(bool), true, true, false, false)

		if args["dry-run"].(bool) {
			utils.CPrint("Dry run mode: no git push or Webflow update will be performed", "yellow")
			printPublishTimeline("prod", args["by-page"].(bool), true, true, true, false, false)
			if !args["by-page"].(bool) {
				plan, err := publish.PreviewGlobalPublish(c.Context, cfg.AppName, cookies, pToken, scriptUrl, "prod")
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
			plan, err := publish.PreviewGlobalPublish(c.Context, cfg.AppName, cookies, pToken, scriptUrl, "prod")
			if err != nil {
				return err
			}
			printGlobalPublishPlan(plan)

			updated, oldCode, err := publish.PublishGlobalScript(c.Context, cfg.AppName, cookies, pToken, scriptUrl, "prod")
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

			result, err := publish.PublishByPage(c.Context, cfg.AppName, baseUrl, cookies, pToken, cfg.GitHubUser, cfg.RepositoryName, args)
			if err != nil {
				return fmt.Errorf("page-by-page publishing failed: %w", err)
			}
			utils.CPrint(fmt.Sprintf("Page publish summary: %d page(s) updated", result.UpdatedPages), "green")
			printPublishTimeline("prod", true, false, true, true, true, result.Published)
		}
	} else {
		// Development mode
		scriptUrl := args["script-url"].(string)
		if scriptUrl == "" {
			scriptUrl = buildLocalScriptURL(devHost, devPort, cfg.GlobalEntry)
		}

		if args["dry-run"].(bool) {
			utils.CPrint("Dry run mode: no Webflow update will be performed", "yellow")
			printPublishTimeline("dev", args["by-page"].(bool), true, true, false, false, false)
			if !args["by-page"].(bool) {
				plan, err := publish.PreviewGlobalPublish(c.Context, cfg.AppName, cookies, pToken, scriptUrl, "dev")
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

		// Set up context with cancellation for proper command handling
		ctx, cancel := context.WithCancel(c.Context)
		defer cancel()

		devServer, err := ensureDevServerRunning(ctx, cfg.PackageManager, scriptUrl, resolveListenHost(devHost), devPort)
		if err != nil {
			return err
		}
		printPublishTimeline("dev", args["by-page"].(bool), false, true, true, false, false)
		defer func() {
			_ = devServer.Stop(5 * time.Second)
		}()

		// Set up signal handling
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(sigCh)

		utils.CPrint("Publishing development script to Webflow...", "cyan")

		// Save the original global code so it can be restored on Ctrl+C.
		var (
			savedGlobalCode    [2]string
			shouldRevertGlobal bool
		)

		// Handle publishing based on mode
		if !args["by-page"].(bool) {
			plan, err := publish.PreviewGlobalPublish(ctx, cfg.AppName, cookies, pToken, scriptUrl, "dev")
			if err != nil {
				cancel()
				return err
			}
			printGlobalPublishPlan(plan)

			updated, oldCode, err := publish.PublishGlobalScript(ctx, cfg.AppName, cookies, pToken, scriptUrl, "dev")
			if err != nil {
				cancel() // Stop the dev server on error
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

			result, err := publish.PublishByPage(ctx, cfg.AppName, baseUrl, cookies, pToken, cfg.GitHubUser, cfg.RepositoryName, args)
			if err != nil {
				cancel() // Stop the dev server on error
				return fmt.Errorf("page-by-page development publishing failed: %w", err)
			}
			utils.CPrint(fmt.Sprintf("Page publish summary: %d page(s) updated", result.UpdatedPages), "green")
			utils.CPrint("Press Ctrl+C to stop development mode", "yellow")
			printPublishTimeline("dev", true, false, true, true, false, result.Published)
		}

		// Wait for termination signal
		<-sigCh
		utils.CPrint("Shutting down development server...", "yellow")

		// Revert changes if needed
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

		cancel() // Ensure the command is canceled

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

func main() {
	app := &cli.App{
		Name:    "wfkit",
		Usage:   "Webflow Publishing Tool",
		Version: version.Version,
		Action:  interactiveMode,
		Authors: []*cli.Author{
			{
				Name:  "Dmitry Metelev",
				Email: "mailmetelev@gmail.com",
			},
		},
		Commands: []*cli.Command{
			{
				Name:  "init",
				Usage: "Initialize a new Webflow project",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "name", Value: "my-project", Usage: "Project name"},
					&cli.StringFlag{Name: "pages-dir", Value: "src/pages", Usage: "Directory for pages"},
					&cli.StringFlag{Name: "global-entry", Value: "src/global/index.ts", Usage: "Global entry file"},
					&cli.StringFlag{Name: "global-var", Value: "WF", Usage: "Global variable name"},
					&cli.BoolFlag{Name: "init-git", Usage: "Initialize a local git repository inside the project"},
					&cli.BoolFlag{Name: "skip-install", Usage: "Skip dependency installation after generating the scaffold"},
					&cli.BoolFlag{Name: "force", Usage: "Allow writing scaffold files into an existing non-empty directory"},
					&cli.StringFlag{Name: "package-manager", Value: "bun", Usage: "Package manager (npm, yarn, pnpm, bun)"},
				},
				Action: initMode,
			},
			{
				Name:  "docs",
				Usage: "Render markdown and publish it to a dedicated Webflow docs page",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "file", Value: "docs/index.md", Usage: "Markdown entry file for the docs hub"},
					&cli.StringFlag{Name: "page-slug", Value: "docs", Usage: "Target Webflow page slug"},
					&cli.StringFlag{Name: "selector", Value: "[data-wf-docs-root], main", Usage: "Selector used to mount the rendered docs content"},
					&cli.BoolFlag{Name: "dry-run", Usage: "Show what would be changed without updating Webflow"},
					&cli.BoolFlag{Name: "publish", Value: true, Usage: "Publish the site after updating the docs page"},
					&cli.BoolFlag{Name: "notify", Usage: "Show desktop notification and play a sound when finished"},
				},
				Action: docsMode,
			},
			{
				Name:  "migrate",
				Usage: "Migrate inline Webflow page scripts into local page files and publish them via jsDelivr",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "pages-dir", Value: "src/pages", Usage: "Directory for generated page entry files"},
					&cli.BoolFlag{Name: "force", Usage: "Overwrite existing page entry files when they already exist"},
					&cli.BoolFlag{Name: "dry-run", Usage: "Show what would be migrated without writing files, pushing git, or updating Webflow"},
					&cli.StringFlag{Name: "custom-commit", Value: "Migrate Webflow page code via wfkit", Usage: "Custom commit message"},
					&cli.StringFlag{Name: "branch", Value: "main", Usage: "Git branch for CDN URLs"},
					&cli.StringFlag{Name: "build-dir", Value: "dist/assets", Usage: "Build directory"},
					&cli.BoolFlag{Name: "notify", Usage: "Show desktop notification and play a sound when finished"},
				},
				Action: migrateMode,
			},
			{
				Name:  "publish",
				Usage: "Publish code to Webflow",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "env", Value: "prod", Usage: "Environment mode: dev or prod"},
					&cli.BoolFlag{Name: "by-page", Usage: "Publish code for each page individually"},
					&cli.BoolFlag{Name: "dry-run", Usage: "Build and show what would change without pushing or updating Webflow"},
					&cli.StringFlag{Name: "script-url", Usage: "Custom script URL (overrides auto-generation)"},
					&cli.IntFlag{Name: "dev-port", Value: 5173, Usage: "Local dev server port (dev mode)"},
					&cli.StringFlag{Name: "dev-host", Value: "localhost", Usage: "Local dev server host (dev mode)"},
					&cli.StringFlag{Name: "custom-commit", Value: "Auto publish from wfkit tool", Usage: "Custom commit message"},
					&cli.StringFlag{Name: "branch", Value: "main", Usage: "Git branch for CDN URLs"},
					&cli.StringFlag{Name: "build-dir", Value: "dist/assets", Usage: "Build directory"},
					&cli.BoolFlag{Name: "notify", Usage: "Show desktop notification and play a sound when finished"},
					&cli.BoolFlag{Name: "update", Usage: "Check for updates before publishing"},
				},
				Action: publishMode,
			},
			{
				Name:  "proxy",
				Usage: "Proxy the published .webflow.io site and inject local dev scripts",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "site-url", Usage: "Published .webflow.io site URL (defaults to https://<name>.webflow.io)"},
					&cli.StringFlag{Name: "host", Usage: "Shared public host for proxy and injected dev URLs"},
					&cli.StringFlag{Name: "script-url", Usage: "Custom local script URL (overrides auto-generation)"},
					&cli.IntFlag{Name: "dev-port", Value: 5173, Usage: "Local Vite dev server port"},
					&cli.StringFlag{Name: "dev-host", Value: "localhost", Usage: "Local Vite dev server host"},
					&cli.IntFlag{Name: "proxy-port", Value: 3000, Usage: "Local proxy port"},
					&cli.StringFlag{Name: "proxy-host", Value: "localhost", Usage: "Local proxy host"},
					&cli.BoolFlag{Name: "open", Value: true, Usage: "Open the proxied site in your browser automatically"},
				},
				Action: proxyMode,
			},
			{
				Name:   "report-bug",
				Usage:  "Open the GitHub bug report form",
				Action: openBugReport,
			},
			{
				Name:   "request-feature",
				Usage:  "Open the GitHub feature request form",
				Action: openFeatureRequest,
			},
			{
				Name:  "doctor",
				Usage: "Check local configuration, tools, auth, and proxy readiness",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "skip-auth", Usage: "Skip the Webflow authentication check"},
				},
				Action: doctorMode,
			},
			{
				Name:  "update",
				Usage: "Check for and install updates",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "auto-update", Usage: "Automatically install updates if available"},
				},
				Action: updateMode,
			},
		},
		EnableBashCompletion: true,
		HideHelpCommand:      true,
		Before: func(c *cli.Context) error {
			// Global setup code can go here
			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		notifyFailure(notificationsEnabled(os.Args[1:]), "wfkit failed", notificationBody(err, "The command finished with an error."))
		hints := errorHints(err, app.Version, os.Args[1:])
		utils.PrintErrorScreen(
			"wfkit failed",
			err,
			hints...,
		)
		os.Exit(1)
	}
}
