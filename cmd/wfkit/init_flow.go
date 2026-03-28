package main

import (
	"fmt"
	"path/filepath"

	"wfkit/internal/globalconfig"
	"wfkit/internal/initialize"
	initconfig "wfkit/internal/initialize/config"
	"wfkit/internal/utils"

	"github.com/charmbracelet/huh"
	"github.com/urfave/cli/v2"
)

type initFlow struct {
	cliContext *cli.Context
	defaults   *globalconfig.Config

	projectDir     string
	packageManager string
	initGit        bool
	skipInstall    bool
	force          bool
	pagesDir       string
	globalEntry    string
	globalVar      string
	githubUser     string
	repositoryName string
}

func newInitFlow(c *cli.Context) *initFlow {
	return &initFlow{
		cliContext:     c,
		projectDir:     c.String("name"),
		packageManager: c.String("package-manager"),
		initGit:        c.Bool("init-git"),
		skipInstall:    c.Bool("skip-install"),
		force:          c.Bool("force"),
		pagesDir:       c.String("pages-dir"),
		globalEntry:    c.String("global-entry"),
		globalVar:      c.String("global-var"),
	}
}

func (f *initFlow) run() error {
	f.loadDefaults()

	if err := f.collectInput(); err != nil {
		return err
	}

	f.applyFallbacks()
	if err := f.saveDefaults(); err != nil {
		return err
	}

	opts := f.options()
	if err := initialize.InitProject(opts); err != nil {
		return fmt.Errorf("initialization failed: %w", err)
	}

	f.printSuccess(opts)
	return nil
}

func (f *initFlow) loadDefaults() {
	defaults, err := globalconfig.LoadConfig()
	if err != nil {
		defaults = &globalconfig.Config{PackageManager: "bun"}
	}

	f.defaults = defaults
	f.githubUser = defaults.GitHubUser
	f.repositoryName = defaults.RepositoryName
	if f.packageManager == "bun" && defaults.PackageManager != "" {
		f.packageManager = defaults.PackageManager
	}
}

func (f *initFlow) collectInput() error {
	if f.cliContext.NumFlags() != 0 {
		return nil
	}

	installDependencies := !f.skipInstall
	if err := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Project Name").
				Value(&f.projectDir),
			huh.NewInput().
				Title("GitHub Username").
				Description("Used for CDN links (e.g. yndmitry)").
				Value(&f.githubUser),
			huh.NewInput().
				Title("GitHub Repository").
				Description("If empty, we'll use the Project Name").
				Value(&f.repositoryName),
			huh.NewSelect[string]().
				Title("Package Manager").
				Options(packageManagerSelectOptions()...).
				Value(&f.packageManager),
			huh.NewConfirm().
				Title("Install dependencies now?").
				Description("Installs the generated project's local CLI and frontend tooling immediately.").
				Value(&installDependencies),
			huh.NewConfirm().
				Title("Initialize git repository?").
				Description("Runs `git init` in the new project directory.").
				Value(&f.initGit),
		),
	).Run(); err != nil {
		return err
	}

	f.skipInstall = !installDependencies
	return nil
}

func (f *initFlow) applyFallbacks() {
	if f.repositoryName == "" {
		f.repositoryName = filepath.Base(filepath.Clean(f.projectDir))
	}
	if f.pagesDir == "" {
		f.pagesDir = "src/pages"
	}
	if f.globalEntry == "" {
		f.globalEntry = "src/global/index.ts"
	}
	if f.globalVar == "" {
		f.globalVar = "WF"
	}
}

func (f *initFlow) saveDefaults() error {
	if f.defaults == nil {
		f.defaults = &globalconfig.Config{PackageManager: "bun"}
	}

	if err := globalconfig.SaveConfig(&globalconfig.Config{
		GitHubUser:     f.githubUser,
		RepositoryName: f.repositoryName,
		PackageManager: f.packageManager,
		Notify:         f.defaults.Notify,
	}); err != nil {
		return fmt.Errorf("failed to save global defaults: %w", err)
	}

	return nil
}

func (f *initFlow) options() initconfig.Options {
	projectName := filepath.Base(filepath.Clean(f.projectDir))
	return initconfig.Options{
		ProjectDir:     f.projectDir,
		Name:           projectName,
		PagesDir:       f.pagesDir,
		GlobalEntry:    f.globalEntry,
		GlobalVar:      f.globalVar,
		InitGit:        f.initGit,
		Force:          f.force,
		SkipInstall:    f.skipInstall,
		PackageManager: f.packageManager,
		CLIValue:       f.cliContext.App.Version,
		GitHubUser:     f.githubUser,
		RepositoryName: f.repositoryName,
	}
}

func (f *initFlow) printSuccess(opts initconfig.Options) {
	nextSteps := []string{fmt.Sprintf("cd %s", f.projectDir)}
	if f.skipInstall {
		nextSteps = append(nextSteps, packageManagerInstallCommand(f.packageManager))
	}
	nextSteps = append(nextSteps, packageManagerScriptCommand(f.packageManager, "dev"), "wfkit doctor")

	utils.PrintSuccessScreen(
		"Project initialized",
		"Your Webflow project scaffold is ready.",
		[]utils.SummaryMetric{
			{Label: "Project", Value: opts.Name, Tone: "success"},
			{Label: "Package manager", Value: f.packageManager, Tone: "info"},
			{Label: "Dependencies", Value: map[bool]string{true: "skipped", false: "installed"}[f.skipInstall], Tone: "info"},
		},
		nextSteps...,
	)
}

func packageManagerSelectOptions() []huh.Option[string] {
	return []huh.Option[string]{
		huh.NewOption("Bun (Recommended)", "bun"),
		huh.NewOption("NPM", "npm"),
		huh.NewOption("Yarn", "yarn"),
		huh.NewOption("PNPM", "pnpm"),
	}
}
