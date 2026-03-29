package main

import (
	"flag"

	"github.com/charmbracelet/huh"
	"github.com/urfave/cli/v2"
)

type interactiveDoctorFlow struct {
	parent   *cli.Context
	skipAuth bool
}

func newInteractiveDoctorFlow(parent *cli.Context) *interactiveDoctorFlow {
	return &interactiveDoctorFlow{
		parent:   parent,
		skipAuth: resolveBoolFlag(parent, "skip-auth", false),
	}
}

func (f *interactiveDoctorFlow) run() error {
	configured := false
	for {
		if !configured {
			if err := f.collectInput(); err != nil {
				return err
			}
		}

		if err := doctorMode(f.newContext()); err != nil {
			return err
		}

		next, err := f.postAction()
		if err != nil {
			return err
		}

		switch next {
		case "rerun":
			configured = true
		case "adjust":
			configured = false
		case "config":
			if err := configMode(f.parent); err != nil {
				return err
			}
			configured = true
		case "back":
			return nil
		default:
			return nil
		}
	}
}

func (f *interactiveDoctorFlow) collectInput() error {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Skip Webflow auth checks?").
				Description("Use this if you only want local project and tooling diagnostics.").
				Value(&f.skipAuth),
		),
	).Run()
}

func (f *interactiveDoctorFlow) newContext() *cli.Context {
	set := flag.NewFlagSet("wfkit doctor", flag.ContinueOnError)
	_ = set.Bool("skip-auth", false, "")

	ctx := cli.NewContext(f.parent.App, set, f.parent)
	_ = ctx.Set("skip-auth", boolString(f.skipAuth))
	return ctx
}

func (f *interactiveDoctorFlow) postAction() (string, error) {
	var action string
	options := []huh.Option[string]{
		huh.NewOption("Run again with same settings", "rerun"),
		huh.NewOption("Adjust settings", "adjust"),
		huh.NewOption("Open config flow", "config"),
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
