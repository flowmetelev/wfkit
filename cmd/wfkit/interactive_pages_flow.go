package main

import (
	"flag"
	"fmt"

	"wfkit/internal/utils"

	"github.com/charmbracelet/huh"
	"github.com/urfave/cli/v2"
)

type interactivePagesFlow struct {
	parent     *cli.Context
	action     string
	lookupMode string
	name       string
	slug       string
	pageID     string
	output     string
	jsonOutput bool
	writeTypes bool
	confirmed  bool
}

func newInteractivePagesFlow(parent *cli.Context) *interactivePagesFlow {
	return &interactivePagesFlow{
		parent:     parent,
		lookupMode: "slug",
		output:     "src/generated/wfkit-pages.ts",
		writeTypes: true,
	}
}

func (f *interactivePagesFlow) run() error {
	if err := f.selectAction(); err != nil {
		return err
	}
	utils.ClearScreen()
	return f.dispatch()
}

func (f *interactivePagesFlow) selectAction() error {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Page management").
				Options(
					huh.NewOption("List pages", "list"),
					huh.NewOption("Create page", "create"),
					huh.NewOption("Inspect page", "inspect"),
					huh.NewOption("Delete page", "delete"),
					huh.NewOption("Open page", "open"),
					huh.NewOption("Generate page types", "types"),
					huh.NewOption("Back", "back"),
				).
				Value(&f.action),
		),
	).Run()
}

func (f *interactivePagesFlow) dispatch() error {
	switch f.action {
	case "list":
		return pagesListMode(f.newContext(nil, nil))
	case "create":
		if err := f.collectCreateInput(); err != nil {
			return err
		}
		return pagesCreateMode(f.newContext(
			map[string]string{"name": f.name, "slug": f.slug, "output": f.output},
			map[string]bool{"types": f.writeTypes, "json": f.jsonOutput},
		))
	case "inspect":
		if err := f.collectInspectInput(); err != nil {
			return err
		}
		return pagesInspectMode(f.newContext(
			f.pageSelectorFlags(),
			map[string]bool{"json": f.jsonOutput},
		))
	case "delete":
		if err := f.collectDeleteInput(); err != nil {
			return err
		}
		return pagesDeleteMode(f.newContext(
			f.pageSelectorFlags(),
			map[string]bool{"yes": f.confirmed},
		))
	case "open":
		if err := f.collectOpenInput(); err != nil {
			return err
		}
		return pagesOpenMode(f.newContext(
			f.pageSelectorFlags(),
			nil,
		))
	case "types":
		if err := f.collectTypesInput(); err != nil {
			return err
		}
		return pagesTypesMode(f.newContext(
			map[string]string{"output": f.output},
			nil,
		))
	case "back":
		return interactiveMode(f.parent)
	default:
		return nil
	}
}

func (f *interactivePagesFlow) collectCreateInput() error {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Page name").
				Description("Used as the Webflow page title.").
				Value(&f.name),
			huh.NewInput().
				Title("Slug").
				Description("Optional. Leave empty to derive it from the page name.").
				Value(&f.slug),
			huh.NewConfirm().
				Title("Regenerate local page types after creating the page?").
				Value(&f.writeTypes),
		),
	).Run()
}

func (f *interactivePagesFlow) collectInspectInput() error {
	if err := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Find page by").
				Options(
					huh.NewOption("Slug", "slug"),
					huh.NewOption("Page ID", "id"),
				).
				Value(&f.lookupMode),
			huh.NewConfirm().
				Title("Print JSON output?").
				Value(&f.jsonOutput),
		),
	).Run(); err != nil {
		return err
	}

	if f.lookupMode == "id" {
		return huh.NewInput().
			Title("Page ID").
			Description("For example: 69c924814b191f0b01fc6156").
			Value(&f.pageID).
			Run()
	}

	return huh.NewInput().
		Title("Slug").
		Description("For example: docs").
		Value(&f.slug).
		Run()
}

func (f *interactivePagesFlow) collectDeleteInput() error {
	if err := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Delete page by").
				Options(
					huh.NewOption("Slug", "slug"),
					huh.NewOption("Page ID", "id"),
				).
				Value(&f.lookupMode),
		),
	).Run(); err != nil {
		return err
	}

	if f.lookupMode == "id" {
		if err := huh.NewInput().
			Title("Page ID").
			Description("For example: 69c924814b191f0b01fc6156").
			Value(&f.pageID).
			Run(); err != nil {
			return err
		}
	} else {
		if err := huh.NewInput().
			Title("Slug").
			Description("For example: docs").
			Value(&f.slug).
			Run(); err != nil {
			return err
		}
	}

	target := f.slug
	if f.lookupMode == "id" {
		target = f.pageID
	}
	return huh.NewConfirm().
		Title(fmt.Sprintf("Delete %q from Webflow?", target)).
		Description("This removes the page from the site.").
		Value(&f.confirmed).
		Run()
}

func (f *interactivePagesFlow) collectTypesInput() error {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Output file").
				Description("Where to write the generated page types.").
				Value(&f.output),
		),
	).Run()
}

func (f *interactivePagesFlow) collectOpenInput() error {
	return f.collectPageLookupInput("Open page by")
}

func (f *interactivePagesFlow) collectPageLookupInput(title string) error {
	if err := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(title).
				Options(
					huh.NewOption("Slug", "slug"),
					huh.NewOption("Page ID", "id"),
				).
				Value(&f.lookupMode),
		),
	).Run(); err != nil {
		return err
	}

	if f.lookupMode == "id" {
		return huh.NewInput().
			Title("Page ID").
			Description("For example: 69c924814b191f0b01fc6156").
			Value(&f.pageID).
			Run()
	}

	return huh.NewInput().
		Title("Slug").
		Description("For example: docs").
		Value(&f.slug).
		Run()
}

func (f *interactivePagesFlow) pageSelectorFlags() map[string]string {
	if f.lookupMode == "id" {
		return map[string]string{"id": f.pageID}
	}
	return map[string]string{"slug": f.slug}
}

func (f *interactivePagesFlow) newContext(stringFlags map[string]string, boolFlags map[string]bool) *cli.Context {
	set := flag.NewFlagSet("wfkit pages", flag.ContinueOnError)
	_ = set.Bool("json", false, "")
	_ = set.Bool("types", false, "")
	_ = set.Bool("yes", false, "")
	_ = set.String("output", "", "")
	_ = set.String("name", "", "")
	_ = set.String("slug", "", "")
	_ = set.String("id", "", "")

	ctx := cli.NewContext(f.parent.App, set, f.parent)
	for name, value := range stringFlags {
		_ = ctx.Set(name, value)
	}
	for name, value := range boolFlags {
		_ = ctx.Set(name, fmt.Sprintf("%t", value))
	}
	return ctx
}
