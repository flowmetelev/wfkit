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
	rawPages   []map[string]interface{}
	created    webflow.Page
	target     webflow.Page
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

type pageInspection struct {
	ID                string   `json:"id"`
	Slug              string   `json:"slug"`
	Title             string   `json:"title"`
	SEOTitle          string   `json:"seoTitle,omitempty"`
	SEODescription    string   `json:"seoDescription,omitempty"`
	SearchTitle       string   `json:"searchTitle,omitempty"`
	SearchDescription string   `json:"searchDescription,omitempty"`
	CanonicalURL      string   `json:"canonicalUrl,omitempty"`
	IncludeInSitemap  bool     `json:"includeInSitemap"`
	SearchExclude     bool     `json:"searchExclude"`
	PostBodyBytes     int      `json:"postBodyBytes"`
	HasCustomCode     bool     `json:"hasCustomCode"`
	ManagedScriptIDs  []string `json:"managedScriptIds,omitempty"`
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

func (f *pagesFlow) runUpdate() error {
	if err := f.loadConfig(); err != nil {
		return err
	}
	f.printHeader("Update Page")
	if err := f.authenticate(); err != nil {
		return err
	}
	if err := f.loadPages(); err != nil {
		return err
	}

	page, err := f.resolveTargetPageUpdate()
	if err != nil {
		return err
	}
	f.target = page

	if !f.hasPageUpdateInput() {
		return fmt.Errorf("no page metadata changes provided")
	}

	rawPage, err := f.rawPageByID(page.ID)
	if err != nil {
		return err
	}
	updatedPage := f.buildUpdatedPage(page)
	updatedPayload := f.buildUpdatedPagePayload(updatedPage, rawPage)
	if err := utils.RunTask("Update page metadata in Webflow", func() error {
		return webflow.PutRawPageObject(f.cliContext.Context, f.baseURL, f.token, f.cookies, updatedPage.ID, updatedPage.Title, updatedPayload)
	}); err != nil {
		return err
	}

	if f.cliContext.Bool("json") {
		return printJSON(pageInspectionFromPage(updatedPage))
	}

	utils.PrintSection("Updated Page")
	f.printPageDetails(pageInspectionFromPage(updatedPage))
	utils.PrintSuccessScreen(
		"Page updated",
		"The Webflow page metadata was updated successfully.",
		[]utils.SummaryMetric{
			{Label: "Slug", Value: developerPageSlug(updatedPage), Tone: "success"},
			{Label: "Title", Value: updatedPage.Title, Tone: "info"},
		},
		"wfkit pages inspect --slug "+developerPageSlug(updatedPage),
		"wfkit pages types",
	)
	return nil
}

func (f *pagesFlow) runInspect() error {
	if err := f.loadConfig(); err != nil {
		return err
	}
	f.printHeader("Inspect Page")
	if err := f.authenticate(); err != nil {
		return err
	}
	if err := f.loadPages(); err != nil {
		return err
	}

	page, err := f.resolveTargetPage()
	if err != nil {
		return err
	}
	f.target = page

	inspection := inspectPage(page)
	if f.cliContext.Bool("json") {
		return printJSON(inspection)
	}

	utils.PrintSection("Page")
	f.printPageDetails(inspection)
	return nil
}

func (f *pagesFlow) runDelete() error {
	if err := f.loadConfig(); err != nil {
		return err
	}
	f.printHeader("Delete Page")
	if err := f.authenticate(); err != nil {
		return err
	}
	if err := f.loadPages(); err != nil {
		return err
	}

	page, err := f.resolveTargetPage()
	if err != nil {
		return err
	}
	f.target = page

	if !f.cliContext.Bool("yes") {
		return fmt.Errorf("refusing to delete page %q without --yes", developerPageSlug(page))
	}

	if err := utils.RunTask("Delete page in Webflow", func() error {
		return webflow.DeletePage(f.cliContext.Context, f.baseURL, f.token, f.cookies, page.ID)
	}); err != nil {
		return err
	}

	utils.PrintSuccessScreen(
		"Page deleted",
		"The Webflow page was deleted successfully.",
		[]utils.SummaryMetric{
			{Label: "Slug", Value: developerPageSlug(page), Tone: "success"},
			{Label: "Page ID", Value: page.ID, Tone: "info"},
		},
		"wfkit pages list",
		"wfkit pages types",
	)
	return nil
}

func (f *pagesFlow) runOpen() error {
	if err := f.loadConfig(); err != nil {
		return err
	}
	if f.config.EffectiveSiteURL() == "" {
		return fmt.Errorf("missing site configuration: set appName or siteUrl in wfkit.json")
	}
	f.printHeader("Open Page")
	if err := f.authenticate(); err != nil {
		return err
	}
	if err := f.loadPages(); err != nil {
		return err
	}

	page, err := f.resolveTargetPage()
	if err != nil {
		return err
	}
	f.target = page

	targetURL := publishedPageURL(f.config.EffectiveSiteURL(), page)
	if targetURL == "" {
		return fmt.Errorf("failed to derive published URL for page %q", developerPageSlug(page))
	}

	if err := utils.RunTask("Open page in browser", func() error {
		return openURL(targetURL)
	}); err != nil {
		return fmt.Errorf("failed to open %s: %w", targetURL, err)
	}

	utils.PrintSection("Opened Page")
	utils.PrintStatus("OK", targetURL, displayValue(strings.TrimSpace(page.Title)))
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
		rawPages, err := webflow.GetRawPagesListFromDom(f.cliContext.Context, f.config.AppName, f.token, f.cookies)
		if err != nil {
			return fmt.Errorf("failed to load raw pages: %w", err)
		}
		f.pages = pages
		f.rawPages = rawPages
		return nil
	})
}

func (f *pagesFlow) rawPageByID(id string) (map[string]interface{}, error) {
	id = strings.TrimSpace(id)
	for _, page := range f.rawPages {
		if strings.TrimSpace(rawStringValue(page["_id"])) == id {
			return cloneRawMap(page), nil
		}
	}
	return nil, fmt.Errorf("failed to find raw Webflow page payload for id %q", id)
}

func (f *pagesFlow) resolveTargetPage() (webflow.Page, error) {
	id := strings.TrimSpace(f.cliContext.String("id"))
	slug := strings.TrimSpace(f.cliContext.String("page-slug"))
	if slug == "" {
		slug = strings.TrimSpace(f.cliContext.String("slug"))
	}
	switch {
	case id != "" && slug != "":
		return webflow.Page{}, fmt.Errorf("pass either --id or --slug, not both")
	case id == "" && slug == "":
		return webflow.Page{}, fmt.Errorf("missing page selector: pass --id or --slug")
	case id != "":
		for _, page := range f.pages {
			if strings.TrimSpace(page.ID) == id {
				return page, nil
			}
		}
		return webflow.Page{}, fmt.Errorf("no Webflow page found with id %q", id)
	default:
		slug = normalizePageSlug(slug)
		for _, page := range f.pages {
			if developerPageSlug(page) == slug {
				return page, nil
			}
		}
		return webflow.Page{}, fmt.Errorf("no Webflow page found with slug %q", slug)
	}
}

func (f *pagesFlow) resolveTargetPageUpdate() (webflow.Page, error) {
	id := strings.TrimSpace(f.cliContext.String("id"))
	slug := strings.TrimSpace(f.cliContext.String("page-slug"))
	switch {
	case id != "" && slug != "":
		return webflow.Page{}, fmt.Errorf("pass either --id or --page-slug, not both")
	case id == "" && slug == "":
		return webflow.Page{}, fmt.Errorf("missing page selector: pass --id or --page-slug")
	case id != "":
		for _, page := range f.pages {
			if strings.TrimSpace(page.ID) == id {
				return page, nil
			}
		}
		return webflow.Page{}, fmt.Errorf("no Webflow page found with id %q", id)
	default:
		slug = normalizePageSlug(slug)
		for _, page := range f.pages {
			if developerPageSlug(page) == slug {
				return page, nil
			}
		}
		return webflow.Page{}, fmt.Errorf("no Webflow page found with slug %q", slug)
	}
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

func (f *pagesFlow) buildUpdatedPagePayload(page webflow.Page, raw map[string]interface{}) map[string]interface{} {
	payload := cloneRawMap(raw)
	payload["_id"] = page.ID
	payload["title"] = page.Title
	payload["slug"] = page.Slug
	payload["seoTitle"] = page.SEOTitle
	payload["seoDesc"] = page.SEODescription
	payload["searchTitle"] = page.SearchTitle
	payload["searchDescription"] = page.SearchDescription
	payload["includeInSitemap"] = page.IncludeInSitemap
	payload["searchExclude"] = page.SearchExclude
	if page.CanonicalURL == nil || strings.TrimSpace(*page.CanonicalURL) == "" {
		payload["canonicalUrl"] = nil
	} else {
		payload["canonicalUrl"] = strings.TrimSpace(*page.CanonicalURL)
	}
	return payload
}

func cloneRawMap(input map[string]interface{}) map[string]interface{} {
	if input == nil {
		return map[string]interface{}{}
	}
	cloned := make(map[string]interface{}, len(input))
	for key, value := range input {
		cloned[key] = cloneRawValue(value)
	}
	return cloned
}

func cloneRawValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		return cloneRawMap(typed)
	case []interface{}:
		cloned := make([]interface{}, len(typed))
		for i, item := range typed {
			cloned[i] = cloneRawValue(item)
		}
		return cloned
	default:
		return typed
	}
}

func rawStringValue(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return ""
	}
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

func inspectPage(page webflow.Page) pageInspection {
	return pageInspectionFromPage(page)
}

func pageInspectionFromPage(page webflow.Page) pageInspection {
	postBody := strings.TrimSpace(page.PostBody)
	return pageInspection{
		ID:                page.ID,
		Slug:              developerPageSlug(page),
		Title:             page.Title,
		SEOTitle:          strings.TrimSpace(page.SEOTitle),
		SEODescription:    strings.TrimSpace(page.SEODescription),
		SearchTitle:       strings.TrimSpace(page.SearchTitle),
		SearchDescription: strings.TrimSpace(page.SearchDescription),
		CanonicalURL:      strings.TrimSpace(derefString(page.CanonicalURL)),
		IncludeInSitemap:  page.IncludeInSitemap,
		SearchExclude:     page.SearchExclude,
		PostBodyBytes:     len(page.PostBody),
		HasCustomCode:     postBody != "",
		ManagedScriptIDs:  managedScriptIDs(page.PostBody),
	}
}

func (f *pagesFlow) hasPageUpdateInput() bool {
	for _, name := range []string{"title", "slug", "seo-title", "seo-description", "search-title", "search-description", "canonical-url"} {
		if f.cliContext.IsSet(name) {
			return true
		}
	}
	for _, name := range []string{"include-in-sitemap", "exclude-from-sitemap", "exclude-from-search"} {
		if f.cliContext.IsSet(name) {
			return true
		}
	}
	return false
}

func (f *pagesFlow) buildUpdatedPage(page webflow.Page) webflow.Page {
	updated := page
	if f.cliContext.IsSet("title") {
		updated.Title = strings.TrimSpace(f.cliContext.String("title"))
	}
	if f.cliContext.IsSet("slug") {
		updated.Slug = normalizePageSlug(f.cliContext.String("slug"))
	}
	if f.cliContext.IsSet("seo-title") {
		updated.SEOTitle = strings.TrimSpace(f.cliContext.String("seo-title"))
	}
	if f.cliContext.IsSet("seo-description") {
		updated.SEODescription = strings.TrimSpace(f.cliContext.String("seo-description"))
	}
	if f.cliContext.IsSet("search-title") {
		updated.SearchTitle = strings.TrimSpace(f.cliContext.String("search-title"))
	}
	if f.cliContext.IsSet("search-description") {
		updated.SearchDescription = strings.TrimSpace(f.cliContext.String("search-description"))
	}
	if f.cliContext.IsSet("canonical-url") {
		value := strings.TrimSpace(f.cliContext.String("canonical-url"))
		if value == "" {
			updated.CanonicalURL = nil
		} else {
			updated.CanonicalURL = &value
		}
	}
	if f.cliContext.IsSet("include-in-sitemap") {
		updated.IncludeInSitemap = true
	}
	if f.cliContext.IsSet("exclude-from-sitemap") {
		updated.IncludeInSitemap = false
	}
	if f.cliContext.IsSet("exclude-from-search") {
		updated.SearchExclude = true
	}
	return updated
}

func (f *pagesFlow) printPageDetails(inspection pageInspection) {
	utils.PrintStatus("INFO", displayValue(inspection.Slug), displayValue(inspection.Title))
	utils.PrintKeyValue("Page ID", inspection.ID)
	utils.PrintKeyValue("SEO title", displayValue(inspection.SEOTitle))
	utils.PrintKeyValue("SEO description", displayValue(inspection.SEODescription))
	utils.PrintKeyValue("Search title", displayValue(inspection.SearchTitle))
	utils.PrintKeyValue("Search description", displayValue(inspection.SearchDescription))
	utils.PrintKeyValue("Canonical URL", displayValue(inspection.CanonicalURL))
	utils.PrintKeyValue("Include in sitemap", boolLabel(inspection.IncludeInSitemap))
	utils.PrintKeyValue("Exclude from search", boolLabel(inspection.SearchExclude))
	utils.PrintKeyValue("Custom code", boolLabel(inspection.HasCustomCode))
	utils.PrintKeyValue("postBody bytes", fmt.Sprintf("%d", inspection.PostBodyBytes))
	if len(inspection.ManagedScriptIDs) > 0 {
		utils.PrintKeyValue("Managed scripts", strings.Join(inspection.ManagedScriptIDs, ", "))
	}
	fmt.Println()
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func boolLabel(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func publishedPageURL(siteURL string, page webflow.Page) string {
	base := strings.TrimRight(strings.TrimSpace(siteURL), "/")
	if base == "" {
		return ""
	}

	slug := developerPageSlug(page)
	if slug == "" || slug == "home" {
		return base
	}
	return base + "/" + slug
}

var managedScriptIDPattern = regexp.MustCompile(`data-script-id=["']([^"']+)["']`)

func managedScriptIDs(postBody string) []string {
	matches := managedScriptIDPattern.FindAllStringSubmatch(postBody, -1)
	if len(matches) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(matches))
	var ids []string
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		id := strings.TrimSpace(match[1])
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
