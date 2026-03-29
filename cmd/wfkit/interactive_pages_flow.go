package main

import (
	"flag"
	"fmt"
	"strings"

	"wfkit/internal/utils"
	"wfkit/internal/webflow"

	"github.com/charmbracelet/huh"
	"github.com/urfave/cli/v2"
)

type interactivePagesFlow struct {
	parent     *cli.Context
	action     string
	name       string
	slug       string
	output     string
	jsonOutput bool
	writeTypes bool
	confirmed  bool
}

func newInteractivePagesFlow(parent *cli.Context) *interactivePagesFlow {
	return &interactivePagesFlow{
		parent:     parent,
		output:     "src/generated/wfkit-pages.ts",
		writeTypes: true,
	}
}

func (f *interactivePagesFlow) run() error {
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
		return f.browsePages()
	case "create":
		if err := f.collectCreateInput(); err != nil {
			return err
		}
		return f.runCreateFlow()
	case "inspect":
		return f.inspectPageFromList()
	case "delete":
		return f.deletePageFromList()
	case "open":
		return f.openPageFromList()
	case "types":
		if err := f.collectTypesInput(); err != nil {
			return err
		}
		return pagesTypesMode(f.newContext(
			map[string]string{"output": f.output},
			nil,
		))
	default:
		return nil
	}
}

func (f *interactivePagesFlow) runCreateFlow() error {
	if err := pagesCreateMode(f.newContext(
		map[string]string{"name": f.name, "slug": f.slug, "output": f.output},
		map[string]bool{"types": f.writeTypes, "json": f.jsonOutput},
	)); err != nil {
		return err
	}

	targetSlug := strings.TrimSpace(f.slug)
	if targetSlug == "" {
		targetSlug = normalizePageSlug(f.name)
	}
	if targetSlug == "" {
		return nil
	}

	for {
		var action string
		if err := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title(fmt.Sprintf("Page %q created. What next?", targetSlug)).
					Options(
						huh.NewOption("Inspect created page", "inspect"),
						huh.NewOption("Open published page", "open"),
						huh.NewOption("Back to page management", "back"),
					).
					Value(&action),
			),
		).Run(); err != nil {
			return err
		}

		switch action {
		case "inspect":
			if err := pagesInspectMode(f.newContext(map[string]string{"slug": targetSlug}, nil)); err != nil {
				return err
			}
		case "open":
			if err := pagesOpenMode(f.newContext(map[string]string{"slug": targetSlug}, nil)); err != nil {
				return err
			}
		case "back":
			return nil
		default:
			return nil
		}
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

func (f *interactivePagesFlow) browsePages() error {
	for {
		page, ok, err := f.selectPage("Select a page")
		if err != nil || !ok {
			return err
		}

		var action string
		if err := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title(fmt.Sprintf("Page: %s", pageOptionLabel(page))).
					Options(
						huh.NewOption("Inspect page", "inspect"),
						huh.NewOption("Open published page", "open"),
						huh.NewOption("Delete page", "delete"),
						huh.NewOption("Back to page list", "back"),
					).
					Value(&action),
			),
		).Run(); err != nil {
			return err
		}

		switch action {
		case "inspect":
			if err := pagesInspectMode(f.newContext(map[string]string{"id": page.ID}, nil)); err != nil {
				return err
			}
		case "open":
			if err := pagesOpenMode(f.newContext(map[string]string{"id": page.ID}, nil)); err != nil {
				return err
			}
		case "delete":
			confirmed := false
			if err := huh.NewConfirm().
				Title(fmt.Sprintf("Delete %q from Webflow?", pageOptionLabel(page))).
				Description("This removes the page from the site.").
				Value(&confirmed).
				Run(); err != nil {
				return err
			}
			if !confirmed {
				continue
			}
			if err := pagesDeleteMode(f.newContext(map[string]string{"id": page.ID}, map[string]bool{"yes": true})); err != nil {
				return err
			}
		case "back":
			continue
		}
	}
}

func (f *interactivePagesFlow) inspectPageFromList() error {
	printJSON := false
	if err := huh.NewConfirm().
		Title("Print JSON output?").
		Value(&printJSON).
		Run(); err != nil {
		return err
	}

	page, ok, err := f.selectPage("Inspect page")
	if err != nil || !ok {
		return err
	}
	return pagesInspectMode(f.newContext(map[string]string{"id": page.ID}, map[string]bool{"json": printJSON}))
}

func (f *interactivePagesFlow) deletePageFromList() error {
	page, ok, err := f.selectPage("Delete page")
	if err != nil || !ok {
		return err
	}

	confirmed := false
	if err := huh.NewConfirm().
		Title(fmt.Sprintf("Delete %q from Webflow?", pageOptionLabel(page))).
		Description("This removes the page from the site.").
		Value(&confirmed).
		Run(); err != nil {
		return err
	}
	if !confirmed {
		return nil
	}

	return pagesDeleteMode(f.newContext(map[string]string{"id": page.ID}, map[string]bool{"yes": true}))
}

func (f *interactivePagesFlow) openPageFromList() error {
	page, ok, err := f.selectPage("Open page")
	if err != nil || !ok {
		return err
	}
	return pagesOpenMode(f.newContext(map[string]string{"id": page.ID}, nil))
}

func (f *interactivePagesFlow) selectPage(title string) (webflow.Page, bool, error) {
	pages, err := f.fetchPages()
	if err != nil {
		return webflow.Page{}, false, err
	}
	if len(pages) == 0 {
		utils.PrintStatus("WARN", "Pages", "No Webflow pages were found for this site.")
		fmt.Println()
		return webflow.Page{}, false, nil
	}

	selected := ""
	options := interactivePageOptions(pages)
	if err := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(title).
				Options(options...).
				Value(&selected),
		),
	).Run(); err != nil {
		return webflow.Page{}, false, err
	}
	if selected == "" {
		return webflow.Page{}, false, nil
	}

	for _, page := range pages {
		if page.ID == selected {
			return page, true, nil
		}
	}

	return webflow.Page{}, false, fmt.Errorf("failed to resolve selected page %q", selected)
}

func (f *interactivePagesFlow) fetchPages() ([]webflow.Page, error) {
	flow := newPagesFlow(f.parent)
	if err := flow.loadConfig(); err != nil {
		return nil, err
	}
	flow.printHeader("Pages")
	if err := flow.authenticate(); err != nil {
		return nil, err
	}
	if err := flow.loadPages(); err != nil {
		return nil, err
	}
	return sortPagesForOutput(flow.pages), nil
}

func interactivePageOptions(pages []webflow.Page) []huh.Option[string] {
	options := make([]huh.Option[string], 0, len(pages)+1)
	for _, page := range sortPagesForOutput(pages) {
		options = append(options, huh.NewOption(pageOptionLabel(page), page.ID))
	}
	options = append(options, huh.NewOption("Back", ""))
	return options
}

func pageOptionLabel(page webflow.Page) string {
	return fmt.Sprintf("%s  %s", displayValue(developerPageSlug(page)), displayValue(strings.TrimSpace(page.Title)))
}
