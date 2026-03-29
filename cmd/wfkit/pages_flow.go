package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"wfkit/internal/config"
	"wfkit/internal/utils"
	"wfkit/internal/webflow"

	"github.com/urfave/cli/v2"
)

type pagesFlow struct {
	cliContext *cli.Context
	config     config.Config
	baseURL    string
	token      string
	cookies    string
	pages      []webflow.Page
	created    webflow.Page
}

type generatedPageInfo struct {
	Slug  string
	Title string
	ID    string
}

type pageSummary struct {
	ID    string `json:"id"`
	Slug  string `json:"slug"`
	Title string `json:"title"`
}

func newPagesFlow(c *cli.Context) *pagesFlow {
	return &pagesFlow{cliContext: c}
}

func (f *pagesFlow) runList() error {
	if err := f.loadConfig(); err != nil {
		return err
	}
	f.printHeader("Pages")
	if err := f.authenticate(); err != nil {
		return err
	}
	if err := f.loadPages(); err != nil {
		return err
	}

	if f.cliContext.Bool("json") {
		return printJSON(pageSummaries(f.pages))
	}

	utils.PrintSection("Pages")
	for _, page := range sortPagesForOutput(f.pages) {
		utils.PrintStatus("INFO", displayValue(developerPageSlug(page)), displayValue(page.Title))
	}
	utils.PrintSummary(utils.SummaryMetric{Label: "Pages", Value: fmt.Sprintf("%d", len(f.pages)), Tone: "info"})
	return nil
}

func (f *pagesFlow) runCreate() error {
	if err := f.loadConfig(); err != nil {
		return err
	}
	f.printHeader("Create Page")
	if err := f.authenticate(); err != nil {
		return err
	}

	name := strings.TrimSpace(f.cliContext.String("name"))
	if name == "" {
		return fmt.Errorf("missing page name: pass --name")
	}
	slug := strings.TrimSpace(f.cliContext.String("slug"))
	if slug == "" {
		slug = normalizePageSlug(name)
	} else {
		slug = normalizePageSlug(slug)
	}
	if slug == "" {
		return fmt.Errorf("failed to derive a valid slug from %q", name)
	}

	if err := utils.RunTask("Create page in Webflow", func() error {
		page, err := webflow.CreateStaticPage(f.cliContext.Context, f.config.AppName, f.baseURL, f.token, f.cookies, name, slug)
		if err != nil {
			return err
		}
		f.created = page
		return nil
	}); err != nil {
		return err
	}

	if f.cliContext.Bool("types") {
		if err := f.loadPages(); err != nil {
			return err
		}
		if err := writePagesTypesFile(f.cliContext.String("output"), f.pages); err != nil {
			return fmt.Errorf("created page %s, but failed to update page types: %w", displayValue(f.created.Slug), err)
		}
	}

	if f.cliContext.Bool("json") {
		return printJSON(pageSummary{ID: f.created.ID, Slug: f.created.Slug, Title: f.created.Title})
	}

	utils.PrintSection("Created Page")
	utils.PrintStatus("OK", displayValue(f.created.Slug), displayValue(f.created.Title))
	utils.PrintKeyValue("Page ID", f.created.ID)
	if f.cliContext.Bool("types") {
		utils.PrintKeyValue("Types", f.cliContext.String("output"))
	}
	fmt.Println()
	utils.PrintSuccessScreen(
		"Page created",
		"The Webflow page was created successfully.",
		[]utils.SummaryMetric{
			{Label: "Slug", Value: f.created.Slug, Tone: "success"},
			{Label: "Types", Value: map[bool]string{true: "updated", false: "skipped"}[f.cliContext.Bool("types")], Tone: "info"},
		},
		"wfkit pages list",
		"wfkit pages types",
	)
	return nil
}

func (f *pagesFlow) runTypes() error {
	if err := f.loadConfig(); err != nil {
		return err
	}
	f.printHeader("Page Types")
	if err := f.authenticate(); err != nil {
		return err
	}
	if err := f.loadPages(); err != nil {
		return err
	}

	output := f.cliContext.String("output")
	if err := writePagesTypesFile(output, f.pages); err != nil {
		return err
	}

	utils.PrintSection("Generated Types")
	utils.PrintStatus("OK", output, fmt.Sprintf("%d page slug(s) synced from Webflow", len(sortPagesForOutput(f.pages))))
	fmt.Println()
	return nil
}

func (f *pagesFlow) loadConfig() error {
	cfg, err := config.ReadConfig()
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}
	if cfg.AppName == "" {
		return fmt.Errorf("missing appName configuration in wfkit.json")
	}
	f.config = cfg
	f.baseURL = cfg.EffectiveDesignURL()
	return nil
}

func (f *pagesFlow) printHeader(title string) {
	utils.PrintSection(title)
	utils.PrintKeyValue("Webflow", f.baseURL)
	fmt.Println()
}

func (f *pagesFlow) authenticate() error {
	return utils.RunTask("Authenticate with Webflow", func() error {
		token, cookies, err := webflow.GetCsrfTokenAndCookies(f.cliContext.Context, f.baseURL)
		if err != nil {
			return fmt.Errorf("failed to authenticate with Webflow: %w", err)
		}
		f.token = token
		f.cookies = cookies
		return nil
	})
}

func (f *pagesFlow) loadPages() error {
	return utils.RunTask("Load pages from Webflow", func() error {
		pages, err := webflow.GetPagesListFromDom(f.cliContext.Context, f.config.AppName, f.token, f.cookies)
		if err != nil {
			return fmt.Errorf("failed to load pages: %w", err)
		}
		f.pages = pages
		return nil
	})
}

func writePagesTypesFile(output string, pages []webflow.Page) error {
	output = strings.TrimSpace(output)
	if output == "" {
		output = "src/generated/wfkit-pages.ts"
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return fmt.Errorf("failed to create %s: %w", filepath.Dir(output), err)
	}
	if err := os.WriteFile(output, []byte(renderPagesTypesModule(pages)), 0o644); err != nil {
		return fmt.Errorf("failed to write %s: %w", output, err)
	}
	return nil
}

func renderPagesTypesModule(pages []webflow.Page) string {
	infos := pagesForTypes(pages)

	var builder strings.Builder
	builder.WriteString("// Generated by `wfkit pages types`. Do not edit by hand.\n\n")

	if len(infos) == 0 {
		builder.WriteString("export const wfPages = [] as const\n")
		builder.WriteString("export type WfPage = string\n\n")
		builder.WriteString("export type WfGlobalSelector = keyof typeof wfGlobalSelectors\n\n")
		builder.WriteString("export interface WfPageInfo {\n  slug: string\n  title: string\n  id?: string\n}\n\n")
		builder.WriteString("export const wfPageInfo: Record<string, WfPageInfo> = {}\n\n")
		builder.WriteString("export const wfPageSelectors: Record<string, string> = {}\n\n")
		builder.WriteString("export const wfPageRootSelectors: Record<string, string> = {}\n\n")
		builder.WriteString("export const wfGlobalSelectors = {\n  siteRoot: '[data-wf-site-root]',\n  docsRoot: '[data-wf-docs-root]'\n} as const\n")
		return builder.String()
	}

	builder.WriteString("export const wfPages = [\n")
	for _, page := range infos {
		builder.WriteString(fmt.Sprintf("  %q,\n", page.Slug))
	}
	builder.WriteString("] as const\n\n")
	builder.WriteString("export type WfPage = (typeof wfPages)[number]\n\n")
	builder.WriteString("export type WfGlobalSelector = keyof typeof wfGlobalSelectors\n\n")
	builder.WriteString("export interface WfPageInfo {\n  slug: WfPage\n  title: string\n  id?: string\n}\n\n")
	builder.WriteString("export const wfPageInfo: Record<WfPage, WfPageInfo> = {\n")
	for _, page := range infos {
		builder.WriteString(fmt.Sprintf("  %q: { slug: %q, title: %q", page.Slug, page.Slug, page.Title))
		if page.ID != "" {
			builder.WriteString(fmt.Sprintf(", id: %q", page.ID))
		}
		builder.WriteString(" },\n")
	}
	builder.WriteString("}\n\n")
	builder.WriteString("export const wfPageSelectors: Record<WfPage, string> = {\n")
	for _, page := range infos {
		builder.WriteString(fmt.Sprintf("  %q: %q,\n", page.Slug, fmt.Sprintf(`[data-page="%s"]`, page.Slug)))
	}
	builder.WriteString("}\n\n")
	builder.WriteString("export const wfPageRootSelectors: Record<WfPage, string> = {\n")
	for _, page := range infos {
		builder.WriteString(fmt.Sprintf("  %q: %q,\n", page.Slug, fmt.Sprintf("[data-wf-%s-root]", selectorToken(page.Slug))))
	}
	builder.WriteString("}\n\n")
	builder.WriteString("export const wfGlobalSelectors = {\n  siteRoot: '[data-wf-site-root]',\n  docsRoot: '[data-wf-docs-root]'\n} as const\n")
	return builder.String()
}

func pagesForTypes(pages []webflow.Page) []generatedPageInfo {
	var infos []generatedPageInfo
	for _, page := range pages {
		slug := developerPageSlug(page)
		if slug == "" {
			continue
		}
		title := strings.TrimSpace(page.Title)
		if title == "" {
			title = slug
		}
		infos = append(infos, generatedPageInfo{
			Slug:  slug,
			Title: title,
			ID:    strings.TrimSpace(page.ID),
		})
	}
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Slug < infos[j].Slug
	})
	return infos
}

func sortPagesForOutput(pages []webflow.Page) []webflow.Page {
	sorted := append([]webflow.Page(nil), pages...)
	sort.Slice(sorted, func(i, j int) bool {
		left := developerPageSlug(sorted[i])
		right := developerPageSlug(sorted[j])
		if left == right {
			return strings.TrimSpace(sorted[i].Title) < strings.TrimSpace(sorted[j].Title)
		}
		return left < right
	})
	return sorted
}

func normalizePageSlug(value string) string {
	slug := strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer(" ", "-", "_", "-", "/", "-", "\\", "-", ".", "-")
	slug = replacer.Replace(slug)
	re := regexp.MustCompile(`[^a-z0-9-]+`)
	slug = re.ReplaceAllString(slug, "")
	slug = regexp.MustCompile(`-+`).ReplaceAllString(slug, "-")
	return strings.Trim(slug, "-")
}

func selectorToken(value string) string {
	return normalizePageSlug(value)
}

func developerPageSlug(page webflow.Page) string {
	slug := strings.TrimSpace(page.Slug)
	if slug != "" {
		return slug
	}
	return normalizePageSlug(page.Title)
}

func printJSON(value interface{}) error {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode JSON output: %w", err)
	}
	fmt.Println(string(encoded))
	return nil
}

func pageSummaries(pages []webflow.Page) []pageSummary {
	sorted := sortPagesForOutput(pages)
	summaries := make([]pageSummary, 0, len(sorted))
	for _, page := range sorted {
		summaries = append(summaries, pageSummary{
			ID:    page.ID,
			Slug:  developerPageSlug(page),
			Title: page.Title,
		})
	}
	return summaries
}
