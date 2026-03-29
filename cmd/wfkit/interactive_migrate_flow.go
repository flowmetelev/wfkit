package main

import (
	"flag"
	"fmt"

	"wfkit/internal/config"
	"wfkit/internal/utils"

	"github.com/charmbracelet/huh"
	"github.com/urfave/cli/v2"
)

type interactiveMigrateFlow struct {
	parent       *cli.Context
	pagesDir     string
	force        bool
	dryRun       bool
	publish      bool
	delivery     string
	target       string
	assetBranch  string
	buildDir     string
	customCommit string
	notify       bool
}

func newInteractiveMigrateFlow(parent *cli.Context) *interactiveMigrateFlow {
	cfg, _ := config.ReadConfig()
	return &interactiveMigrateFlow{
		parent:       parent,
		pagesDir:     "src/pages",
		delivery:     cfg.DeliveryMode,
		target:       "staging",
		assetBranch:  cfg.AssetBranch,
		buildDir:     cfg.BuildDir,
		customCommit: "Migrate Webflow page code via wfkit",
		notify:       resolveNotifyFlag(parent),
	}
}

func (f *interactiveMigrateFlow) run() error {
	configured := false
	for {
		if !configured {
			if err := f.collectInput(); err != nil {
				return err
			}
		}

		if err := migrateMode(f.newContext()); err != nil {
			return err
		}

		next, err := f.postAction()
		if err != nil {
			return err
		}

		switch next {
		case "rerun":
			configured = true
		case "status":
			if err := showGitStatus(); err != nil {
				return err
			}
			configured = true
		case "diff":
			if err := showGitDiffSummary(); err != nil {
				return err
			}
			configured = true
		case "adjust":
			configured = false
		case "back":
			return nil
		default:
			return nil
		}
	}
}

func (f *interactiveMigrateFlow) collectInput() error {
	if err := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Pages directory").
				Description("Where migrated page files should be written.").
				Value(&f.pagesDir),
			huh.NewConfirm().
				Title("Overwrite existing files if needed?").
				Value(&f.force),
			huh.NewConfirm().
				Title("Dry run only?").
				Description("Show the migration plan without writing files.").
				Value(&f.dryRun),
			huh.NewConfirm().
				Title("Publish after writing local files?").
				Description("If disabled, wfkit will only migrate code into the project.").
				Value(&f.publish),
			huh.NewConfirm().
				Title("Desktop notification when finished?").
				Value(&f.notify),
		),
	).Run(); err != nil {
		return err
	}

	if !f.publish {
		return nil
	}

	return huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Delivery mode").
				Options(
					huh.NewOption("CDN via jsDelivr", "cdn"),
					huh.NewOption("Inline Webflow script", "inline"),
				).
				Value(&f.delivery),
			huh.NewSelect[string]().
				Title("Publish target").
				Options(
					huh.NewOption("Staging", "staging"),
					huh.NewOption("Production", "production"),
					huh.NewOption("All destinations", "all"),
				).
				Value(&f.target),
			huh.NewInput().
				Title("Artifact branch").
				Description("Only used for CDN delivery.").
				Value(&f.assetBranch),
			huh.NewInput().
				Title("Build directory").
				Value(&f.buildDir),
			huh.NewInput().
				Title("Artifact commit message").
				Value(&f.customCommit),
		),
	).Run()
}

func (f *interactiveMigrateFlow) newContext() *cli.Context {
	set := flag.NewFlagSet("wfkit migrate", flag.ContinueOnError)
	_ = set.String("pages-dir", "", "")
	_ = set.Bool("force", false, "")
	_ = set.Bool("dry-run", false, "")
	_ = set.Bool("publish", false, "")
	_ = set.String("delivery", "", "")
	_ = set.String("target", "", "")
	_ = set.String("asset-branch", "", "")
	_ = set.String("build-dir", "", "")
	_ = set.String("custom-commit", "", "")
	_ = set.Bool("notify", false, "")

	ctx := cli.NewContext(f.parent.App, set, f.parent)
	_ = ctx.Set("pages-dir", f.pagesDir)
	_ = ctx.Set("force", boolString(f.force))
	_ = ctx.Set("dry-run", boolString(f.dryRun))
	_ = ctx.Set("publish", boolString(f.publish))
	_ = ctx.Set("delivery", f.delivery)
	_ = ctx.Set("target", f.target)
	_ = ctx.Set("asset-branch", f.assetBranch)
	_ = ctx.Set("build-dir", f.buildDir)
	_ = ctx.Set("custom-commit", f.customCommit)
	_ = ctx.Set("notify", boolString(f.notify))
	return ctx
}

func (f *interactiveMigrateFlow) postAction() (string, error) {
	var action string
	options := []huh.Option[string]{
		huh.NewOption("Run again with same settings", "rerun"),
		huh.NewOption("Show git status", "status"),
		huh.NewOption("Show diff summary", "diff"),
		huh.NewOption("Adjust settings", "adjust"),
		huh.NewOption("Back to main menu", "back"),
	}

	if !f.dryRun {
		utils.PrintStatus("READY", "Next step", "Review migrated files before running a separate publish.")
		fmt.Println()
	}

	return action, huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("What next?").
				Options(options...).
				Value(&action),
		),
	).Run()
}
