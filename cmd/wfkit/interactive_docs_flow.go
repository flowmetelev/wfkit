package main

import (
	"flag"

	"wfkit/internal/config"

	"github.com/charmbracelet/huh"
	"github.com/urfave/cli/v2"
)

type interactiveDocsFlow struct {
	parent   *cli.Context
	file     string
	pageSlug string
	selector string
	dryRun   bool
	publish  bool
	notify   bool
}

func newInteractiveDocsFlow(parent *cli.Context) *interactiveDocsFlow {
	cfg, _ := config.ReadConfig()
	return &interactiveDocsFlow{
		parent:   parent,
		file:     cfg.DocsEntry,
		pageSlug: cfg.DocsPageSlug,
		selector: `[data-wf-docs-root], main`,
		publish:  true,
		notify:   resolveNotifyFlag(parent),
	}
}

func (f *interactiveDocsFlow) run() error {
	if err := f.collectInput(); err != nil {
		return err
	}
	return docsMode(f.newContext())
}

func (f *interactiveDocsFlow) collectInput() error {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Markdown file").
				Description("Entry file for the docs hub page.").
				Value(&f.file),
			huh.NewInput().
				Title("Webflow page slug").
				Description("wfkit will create the page if it does not exist.").
				Value(&f.pageSlug),
			huh.NewInput().
				Title("Mount selector").
				Description("Where the rendered docs content should be mounted.").
				Value(&f.selector),
			huh.NewConfirm().
				Title("Dry run only?").
				Description("Show the docs sync plan without updating Webflow.").
				Value(&f.dryRun),
			huh.NewConfirm().
				Title("Publish the site after syncing docs?").
				Value(&f.publish),
			huh.NewConfirm().
				Title("Desktop notification when finished?").
				Value(&f.notify),
		),
	).Run()
}

func (f *interactiveDocsFlow) newContext() *cli.Context {
	set := flag.NewFlagSet("wfkit docs", flag.ContinueOnError)
	_ = set.String("file", "", "")
	_ = set.String("page-slug", "", "")
	_ = set.String("selector", "", "")
	_ = set.Bool("dry-run", false, "")
	_ = set.Bool("publish", false, "")
	_ = set.Bool("notify", false, "")

	ctx := cli.NewContext(f.parent.App, set, f.parent)
	_ = ctx.Set("file", f.file)
	_ = ctx.Set("page-slug", f.pageSlug)
	_ = ctx.Set("selector", f.selector)
	_ = ctx.Set("dry-run", boolString(f.dryRun))
	_ = ctx.Set("publish", boolString(f.publish))
	_ = ctx.Set("notify", boolString(f.notify))
	return ctx
}
