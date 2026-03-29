package main

import (
	"flag"
	"fmt"

	"wfkit/internal/config"

	"github.com/charmbracelet/huh"
	"github.com/urfave/cli/v2"
)

type interactivePublishFlow struct {
	parent       *cli.Context
	byPage       bool
	dryRun       bool
	delivery     string
	target       string
	assetBranch  string
	buildDir     string
	customCommit string
	update       bool
	notify       bool
}

func newInteractivePublishFlow(parent *cli.Context) *interactivePublishFlow {
	cfg, _ := config.ReadConfig()
	return &interactivePublishFlow{
		parent:       parent,
		delivery:     cfg.DeliveryMode,
		target:       "staging",
		assetBranch:  cfg.AssetBranch,
		buildDir:     cfg.BuildDir,
		customCommit: "Auto publish from wfkit tool",
		notify:       resolveNotifyFlag(parent),
	}
}

func (f *interactivePublishFlow) run() error {
	configured := false
	for {
		if !configured {
			if err := f.collectInput(); err != nil {
				return err
			}
		}

		if err := publishMode(f.newContext()); err != nil {
			return err
		}

		next, err := f.postAction()
		if err != nil {
			return err
		}

		switch next {
		case "rerun":
			configured = true
		case "open":
			targetURL := siteURLFromParentContext(f.parent)
			if targetURL == "" {
				return fmt.Errorf("failed to derive published site URL from configuration")
			}
			if err := openURL(targetURL); err != nil {
				return fmt.Errorf("failed to open %s: %w", targetURL, err)
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

func (f *interactivePublishFlow) collectInput() error {
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
			huh.NewConfirm().
				Title("Publish each page separately?").
				Description("Enable page-by-page publishing instead of one global script.").
				Value(&f.byPage),
			huh.NewConfirm().
				Title("Dry run only?").
				Description("Build and preview the publish plan without updating Webflow.").
				Value(&f.dryRun),
			huh.NewConfirm().
				Title("Check for wfkit updates first?").
				Value(&f.update),
			huh.NewConfirm().
				Title("Desktop notification when finished?").
				Value(&f.notify),
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

func (f *interactivePublishFlow) newContext() *cli.Context {
	set := flag.NewFlagSet("wfkit publish", flag.ContinueOnError)
	_ = set.String("env", "", "")
	_ = set.Bool("by-page", false, "")
	_ = set.Bool("dry-run", false, "")
	_ = set.String("delivery", "", "")
	_ = set.String("target", "", "")
	_ = set.String("asset-branch", "", "")
	_ = set.String("build-dir", "", "")
	_ = set.String("custom-commit", "", "")
	_ = set.Bool("update", false, "")
	_ = set.Bool("notify", false, "")

	ctx := cli.NewContext(f.parent.App, set, f.parent)
	_ = ctx.Set("env", "prod")
	_ = ctx.Set("by-page", boolString(f.byPage))
	_ = ctx.Set("dry-run", boolString(f.dryRun))
	_ = ctx.Set("delivery", f.delivery)
	_ = ctx.Set("target", f.target)
	_ = ctx.Set("asset-branch", f.assetBranch)
	_ = ctx.Set("build-dir", f.buildDir)
	_ = ctx.Set("custom-commit", f.customCommit)
	_ = ctx.Set("update", boolString(f.update))
	_ = ctx.Set("notify", boolString(f.notify))
	return ctx
}

func (f *interactivePublishFlow) postAction() (string, error) {
	var action string
	options := []huh.Option[string]{
		huh.NewOption("Run again with same settings", "rerun"),
		huh.NewOption("Adjust settings", "adjust"),
		huh.NewOption("Open published site", "open"),
		huh.NewOption("Back to main menu", "back"),
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
