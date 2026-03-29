package main

import (
	"flag"
	"fmt"

	"wfkit/internal/utils"

	"github.com/charmbracelet/huh"
	"github.com/urfave/cli/v2"
)

type interactiveCMSFlow struct {
	parent        *cli.Context
	action        string
	output        string
	target        string
	jsonMode      bool
	filter        string
	deleteMissing bool
}

func newInteractiveCMSFlow(parent *cli.Context) *interactiveCMSFlow {
	return &interactiveCMSFlow{
		parent: parent,
		output: "webflow/cms",
		target: "staging",
	}
}

func (f *interactiveCMSFlow) run() error {
	for {
		if err := f.selectAction(); err != nil {
			return err
		}
		utils.ClearScreen()
		if f.action == "back" {
			return nil
		}
		if err := f.dispatch(); err != nil {
			return err
		}
	}
}

func (f *interactiveCMSFlow) selectAction() error {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("CMS management").
				Options(
					huh.NewOption("List collections", "collections"),
					huh.NewOption("Pull collections to local JSON", "pull"),
					huh.NewOption("Diff local JSON vs Webflow", "diff"),
					huh.NewOption("Push local JSON to Webflow", "push"),
					huh.NewOption("Back", "back"),
				).
				Value(&f.action),
		),
	).Run()
}

func (f *interactiveCMSFlow) dispatch() error {
	switch f.action {
	case "collections":
		return cmsCollectionsMode(f.newContext(nil, nil))
	case "pull":
		if err := f.collectPullInput(); err != nil {
			return err
		}
		return cmsPullMode(f.newContext(
			map[string]string{
				"dir":        f.output,
				"target":     f.target,
				"collection": f.filter,
			},
			map[string]bool{"json": f.jsonMode},
		))
	case "diff":
		if err := f.collectSyncInput("Diff"); err != nil {
			return err
		}
		return cmsDiffMode(f.newContext(
			map[string]string{
				"dir":        f.output,
				"target":     f.target,
				"collection": f.filter,
			},
			map[string]bool{"json": f.jsonMode, "delete-missing": f.deleteMissing},
		))
	case "push":
		if err := f.collectSyncInput("Push"); err != nil {
			return err
		}
		return cmsPushMode(f.newContext(
			map[string]string{
				"dir":        f.output,
				"target":     f.target,
				"collection": f.filter,
			},
			map[string]bool{"json": f.jsonMode, "delete-missing": f.deleteMissing},
		))
	default:
		return nil
	}
}

func (f *interactiveCMSFlow) collectPullInput() error {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Output directory").
				Description("Where to write pulled CMS JSON files.").
				Value(&f.output),
			huh.NewInput().
				Title("Collection filter").
				Description("Optional collection slug or id. Leave empty to pull all collections.").
				Value(&f.filter),
			huh.NewSelect[string]().
				Title("Target").
				Description("Which Webflow content target to read items from.").
				Options(
					huh.NewOption("Staging", "staging"),
					huh.NewOption("Production", "production"),
				).
				Value(&f.target),
			huh.NewConfirm().
				Title("Print JSON summary?").
				Value(&f.jsonMode),
		),
	).Run()
}

func (f *interactiveCMSFlow) collectSyncInput(mode string) error {
	title := mode + " CMS JSON"
	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("CMS directory").
				Description("Directory containing the pulled CMS JSON files.").
				Value(&f.output),
			huh.NewInput().
				Title("Collection filter").
				Description("Optional collection slug or id. Leave empty to sync all collections.").
				Value(&f.filter),
			huh.NewSelect[string]().
				Title("Target").
				Description("Which Webflow content target to diff against before syncing.").
				Options(
					huh.NewOption("Staging", "staging"),
					huh.NewOption("Production", "production"),
				).
				Value(&f.target),
			huh.NewConfirm().
				Title("Delete remote items missing from local JSON?").
				Description("Leave this off for a safe create/update-only sync.").
				Value(&f.deleteMissing),
			huh.NewConfirm().
				Title("Print JSON summary?").
				Value(&f.jsonMode),
		).Title(title),
	).Run()
}

func (f *interactiveCMSFlow) newContext(stringFlags map[string]string, boolFlags map[string]bool) *cli.Context {
	set := flag.NewFlagSet("wfkit cms", flag.ContinueOnError)
	_ = set.String("dir", "", "")
	_ = set.String("collection", "", "")
	_ = set.String("target", "", "")
	_ = set.Bool("json", false, "")
	_ = set.Bool("delete-missing", false, "")

	ctx := cli.NewContext(f.parent.App, set, f.parent)
	for name, value := range stringFlags {
		_ = ctx.Set(name, value)
	}
	for name, value := range boolFlags {
		_ = ctx.Set(name, fmt.Sprintf("%t", value))
	}
	return ctx
}
