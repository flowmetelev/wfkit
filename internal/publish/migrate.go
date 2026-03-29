package publish

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"wfkit/internal/build"
	"wfkit/internal/webflow"
)

var (
	inlineScriptPattern = regexp.MustCompile(`(?is)<script\b([^>]*)>(.*?)</script>`)
	scriptTypePattern   = regexp.MustCompile(`(?i)\btype\s*=\s*(?:"([^"]+)"|'([^']+)'|([^\s>]+))`)
	excessNewlines      = regexp.MustCompile(`\n{3,}`)
)

type MigrationPlan struct {
	Global GlobalMigration
	Pages  []PageMigration
}

type GlobalMigration struct {
	CurrentHead        string
	CurrentPostBody    string
	CleanedHead        string
	CleanedPostBody    string
	CurrentSrc         string
	ModulePath         string
	ModuleRelativePath string
	EntryPath          string
	ImportPath         string
	Action             string
	Message            string
	HeadScripts        []string
	PostBodyScripts    []string
}

type PageMigration struct {
	PageID          string
	Title           string
	Slug            string
	FolderKey       string
	RelativePath    string
	FilePath        string
	EntryPath       string
	EntryRelative   string
	ImportPath      string
	CurrentPostBody string
	CleanedPostBody string
	CurrentSrc      string
	NextSrc         string
	Action          string
	Message         string
	Scripts         []string
}

type MigrationPublishResult struct {
	GlobalPlan    GlobalPublishPlan
	GlobalUpdated bool
	UpdatedPages  int
	Published     bool
}

func PlanPageMigration(pages []webflow.Page, pagesDir string, force bool) (MigrationPlan, error) {
	return PlanMigration(webflow.GlobalCode{}, pages, pagesDir, "", force)
}

func PlanMigration(globalData webflow.GlobalCode, pages []webflow.Page, pagesDir, globalEntry string, force bool) (MigrationPlan, error) {
	if pagesDir == "" {
		pagesDir = filepath.Join("src", "pages")
	}

	plan := MigrationPlan{
		Global: planGlobalMigration(globalData, globalEntry, force),
	}
	for _, page := range pages {
		item := PageMigration{
			PageID:          page.ID,
			Title:           page.Title,
			Slug:            page.Slug,
			CurrentPostBody: page.PostBody,
			CurrentSrc:      extractScriptSrc(page.PostBody, pageScriptID),
		}

		item.FolderKey = pageFolderKey(page)
		if item.FolderKey == "" {
			item.Action = "missing_slug"
			item.Message = fmt.Sprintf("Skipping %s: page slug/title is missing", pageLabel(page))
			plan.Pages = append(plan.Pages, item)
			continue
		}

		relativePath, filePath, hasExistingFile, entryPath, entryRelative, importPath := resolveMigrationTargetPath(pagesDir, item.FolderKey)
		item.RelativePath = filepath.ToSlash(relativePath)
		item.FilePath = filePath
		item.EntryPath = entryPath
		item.EntryRelative = filepath.ToSlash(entryRelative)
		item.ImportPath = importPath

		scripts, cleanedPostBody := extractMigratableInlineScripts(page.PostBody)
		item.Scripts = scripts
		item.CleanedPostBody = cleanedPostBody

		if len(item.Scripts) == 0 {
			item.Action = "no_inline_scripts"
			item.Message = fmt.Sprintf("Skipping %s: no migratable inline page scripts found", pageLabel(page))
			plan.Pages = append(plan.Pages, item)
			continue
		}

		if hasExistingFile && !force {
			item.Action = "existing_file"
			item.Message = fmt.Sprintf("Skipping %s: %s already exists (use --force to overwrite)", pageLabel(page), item.RelativePath)
			plan.Pages = append(plan.Pages, item)
			continue
		}

		if hasExistingFile {
			item.Action = "overwrite"
			item.Message = fmt.Sprintf("Will overwrite %s and wire it into %s from %s", item.RelativePath, item.EntryRelative, pageLabel(page))
		} else {
			item.Action = "write"
			item.Message = fmt.Sprintf("Will create %s and wire it into %s from %s", item.RelativePath, item.EntryRelative, pageLabel(page))
		}

		plan.Pages = append(plan.Pages, item)
	}

	return plan, nil
}

func WriteMigrationFiles(plan MigrationPlan) error {
	if shouldWriteGlobalMigration(plan.Global) {
		if err := os.MkdirAll(filepath.Dir(plan.Global.ModulePath), 0o755); err != nil {
			return fmt.Errorf("failed to create %s: %w", filepath.Dir(plan.Global.ModulePath), err)
		}
		content := buildMigratedGlobalModule(plan.Global)
		if err := os.WriteFile(plan.Global.ModulePath, []byte(content), 0o644); err != nil {
			return fmt.Errorf("failed to write %s: %w", plan.Global.ModuleRelativePath, err)
		}
		if err := ensureModuleImport(plan.Global.EntryPath, plan.Global.ImportPath); err != nil {
			return fmt.Errorf("failed to wire migrated global module into %s: %w", plan.Global.EntryPath, err)
		}
	}

	for _, page := range plan.Pages {
		if !shouldWriteMigration(page) {
			continue
		}

		if err := os.MkdirAll(filepath.Dir(page.FilePath), 0o755); err != nil {
			return fmt.Errorf("failed to create %s: %w", filepath.Dir(page.FilePath), err)
		}

		content := buildMigratedPageModule(page)
		if err := os.WriteFile(page.FilePath, []byte(content), 0o644); err != nil {
			return fmt.Errorf("failed to write %s: %w", page.RelativePath, err)
		}
		if err := ensurePageEntry(page); err != nil {
			return fmt.Errorf("failed to ensure %s: %w", page.EntryRelative, err)
		}
		if err := ensureModuleImport(page.EntryPath, page.ImportPath); err != nil {
			return fmt.Errorf("failed to wire migrated page module into %s: %w", page.EntryRelative, err)
		}
	}

	return nil
}

func WritePageMigrationFiles(plan MigrationPlan) error {
	return WriteMigrationFiles(plan)
}

func PublishMigratedPages(ctx context.Context, siteName, baseURL, cookies, pToken, ghUser, repo string, args map[string]interface{}, plan MigrationPlan) (MigrationPublishResult, error) {
	buildDir, ok := args["build-dir"].(string)
	if !ok || buildDir == "" {
		return MigrationPublishResult{}, fmt.Errorf("missing or invalid 'build-dir' argument")
	}

	env, ok := args["env"].(string)
	if !ok || env == "" {
		env = "prod"
	}

	result := MigrationPublishResult{}
	pageEntries, err := build.ResolvePageEntries(buildDir)
	if err != nil {
		return result, fmt.Errorf("failed to resolve page entries from build manifest: %w", err)
	}

	if globalPath, err := build.ResolveGlobalEntry(buildDir); err == nil && globalPath != "" {
		globalScript, err := resolveGlobalManagedScript(ghUser, repo, args)
		if err != nil {
			return result, err
		}
		if shouldWriteGlobalMigration(plan.Global) {
			newPostBody := updateScript(plan.Global.CleanedPostBody, globalScript, globalScriptID, env)
			result.GlobalPlan = GlobalPublishPlan{
				CurrentSrc: currentManagedScriptLabel(plan.Global.CurrentPostBody, globalScriptID),
				NextSrc:    managedScriptLabel(globalScript),
				Action:     "update",
			}
			if result.GlobalPlan.CurrentSrc != result.GlobalPlan.NextSrc || plan.Global.CleanedHead != plan.Global.CurrentHead || newPostBody != plan.Global.CurrentPostBody {
				if err := webflow.UpdateGlobalCode(ctx, siteName, pToken, cookies, plan.Global.CleanedHead, newPostBody); err != nil {
					return result, fmt.Errorf("failed to update migrated global code: %w", err)
				}
				result.GlobalUpdated = true
			} else {
				result.GlobalPlan.Action = "up_to_date"
			}
		} else {
			globalPlan, err := PreviewGlobalPublish(ctx, siteName, cookies, pToken, globalScript, env)
			if err != nil {
				return result, fmt.Errorf("failed to preview global migration publish: %w", err)
			}
			result.GlobalPlan = globalPlan

			if globalPlan.Action == "update" {
				updated, _, err := PublishGlobalScript(ctx, siteName, cookies, pToken, globalScript, env)
				if err != nil {
					return result, fmt.Errorf("failed to update global script during migration: %w", err)
				}
				result.GlobalUpdated = updated
			}
		}
	}

	anyPageChanges := false
	for _, page := range plan.Pages {
		if !shouldWriteMigration(page) {
			continue
		}

		manifestPath, ok := pageEntries[page.FolderKey]
		if !ok {
			return result, fmt.Errorf("build manifest is missing page entry for %s (%s)", pageLabel(webflow.Page{ID: page.PageID, Title: page.Title, Slug: page.Slug}), page.FolderKey)
		}

		nextScript, err := resolvePageManagedScript(page.FolderKey, manifestPath, ghUser, repo, args)
		if err != nil {
			return result, fmt.Errorf("resolve migrated page bundle for %s: %w", pageLabel(webflow.Page{ID: page.PageID, Title: page.Title, Slug: page.Slug}), err)
		}
		newPostBody := updateScript(page.CleanedPostBody, nextScript, pageScriptID, env)
		if currentManagedScriptLabel(page.CurrentPostBody, pageScriptID) == managedScriptLabel(nextScript) && newPostBody == page.CurrentPostBody {
			continue
		}

		updatedPage := webflow.Page{
			ID:       page.PageID,
			Title:    page.Title,
			Slug:     page.Slug,
			PostBody: newPostBody,
		}

		if err := webflow.PutFullPageObject(ctx, baseURL, pToken, cookies, updatedPage); err != nil {
			return result, fmt.Errorf("failed to update migrated page %s: %w", pageLabel(updatedPage), err)
		}

		anyPageChanges = true
		result.UpdatedPages++
	}

	if anyPageChanges {
		if err := webflow.PublishSite(ctx, siteName, pToken, cookies); err != nil {
			return result, fmt.Errorf("failed to publish migrated pages: %w", err)
		}
		result.Published = true
		return result, nil
	}

	result.Published = result.GlobalUpdated
	return result, nil
}

func planGlobalMigration(globalData webflow.GlobalCode, globalEntry string, force bool) GlobalMigration {
	head := globalData.Meta["head"]
	postBody := globalData.Meta["postBody"]
	headScripts, cleanedHead := extractMigratableInlineScripts(head)
	postBodyScripts, cleanedPostBody := extractMigratableInlineScripts(postBody)

	plan := GlobalMigration{
		CurrentHead:     head,
		CurrentPostBody: postBody,
		CleanedHead:     cleanedHead,
		CleanedPostBody: cleanedPostBody,
		CurrentSrc:      extractScriptSrc(postBody, globalScriptID),
		HeadScripts:     headScripts,
		PostBodyScripts: postBodyScripts,
		EntryPath:       globalEntry,
	}

	if len(headScripts) == 0 && len(postBodyScripts) == 0 {
		plan.Action = "no_inline_scripts"
		plan.Message = "No migratable inline global scripts found"
		return plan
	}

	if strings.TrimSpace(globalEntry) == "" {
		plan.Action = "missing_global_entry"
		plan.Message = "Cannot migrate global code: global entry is not configured"
		return plan
	}
	if !fileExists(globalEntry) {
		plan.Action = "missing_global_entry"
		plan.Message = fmt.Sprintf("Cannot migrate global code: %s does not exist", globalEntry)
		return plan
	}

	plan.ModulePath = filepath.Join(filepath.Dir(globalEntry), "modules", "webflow.migrated.ts")
	plan.ModuleRelativePath = displayMigrationPath(plan.ModulePath)
	plan.ImportPath = moduleImportPath(globalEntry, plan.ModulePath)

	if fileExists(plan.ModulePath) && !force {
		plan.Action = "existing_file"
		plan.Message = fmt.Sprintf("Skipping global migration: %s already exists (use --force to overwrite)", plan.ModuleRelativePath)
		return plan
	}

	if fileExists(plan.ModulePath) {
		plan.Action = "overwrite"
		plan.Message = fmt.Sprintf("Will overwrite %s from Webflow global custom code", plan.ModuleRelativePath)
	} else {
		plan.Action = "write"
		plan.Message = fmt.Sprintf("Will create %s from Webflow global custom code", plan.ModuleRelativePath)
	}

	return plan
}

func extractMigratableInlineScripts(html string) ([]string, string) {
	if strings.TrimSpace(html) == "" {
		return nil, ""
	}

	var scripts []string
	var builder strings.Builder
	last := 0

	matches := inlineScriptPattern.FindAllStringSubmatchIndex(html, -1)
	for _, match := range matches {
		fullStart, fullEnd := match[0], match[1]
		attrStart, attrEnd := match[2], match[3]
		bodyStart, bodyEnd := match[4], match[5]

		attrs := html[attrStart:attrEnd]
		body := html[bodyStart:bodyEnd]
		if !isMigratableInlineScript(attrs, body) {
			continue
		}

		builder.WriteString(html[last:fullStart])
		last = fullEnd
		scripts = append(scripts, strings.TrimSpace(body))
	}

	builder.WriteString(html[last:])
	cleaned := strings.TrimSpace(builder.String())
	cleaned = excessNewlines.ReplaceAllString(cleaned, "\n\n")

	return scripts, cleaned
}

func isMigratableInlineScript(attrs, body string) bool {
	if strings.TrimSpace(body) == "" {
		return false
	}

	lowerAttrs := strings.ToLower(attrs)
	if strings.Contains(lowerAttrs, "src=") {
		return false
	}
	if strings.Contains(lowerAttrs, "data-script-id=") {
		return false
	}

	matches := scriptTypePattern.FindStringSubmatch(lowerAttrs)
	if len(matches) < 4 {
		return true
	}

	scriptType := matches[1]
	if scriptType == "" {
		scriptType = matches[2]
	}
	if scriptType == "" {
		scriptType = matches[3]
	}

	switch scriptType {
	case "module", "text/javascript", "application/javascript", "text/ecmascript", "application/ecmascript":
		return true
	default:
		return false
	}
}

func resolveMigrationTargetPath(pagesDir, folderKey string) (string, string, bool, string, string, string) {
	relativeBase := filepath.Join(pagesDir, filepath.FromSlash(folderKey))
	tsPath := filepath.Join(relativeBase, "index.ts")
	jsPath := filepath.Join(relativeBase, "index.js")
	modulePath := filepath.Join(relativeBase, "webflow.migrated.ts")

	switch {
	case fileExists(tsPath):
		return modulePath, modulePath, fileExists(modulePath), tsPath, tsPath, moduleImportPath(tsPath, modulePath)
	case fileExists(jsPath):
		return modulePath, modulePath, fileExists(modulePath), jsPath, jsPath, moduleImportPath(jsPath, modulePath)
	default:
		return modulePath, modulePath, fileExists(modulePath), tsPath, tsPath, moduleImportPath(tsPath, modulePath)
	}
}

func buildMigratedPageModule(page PageMigration) string {
	label := page.Title
	if strings.TrimSpace(label) == "" {
		label = page.Slug
	}
	if strings.TrimSpace(label) == "" {
		label = page.PageID
	}

	var builder strings.Builder
	builder.WriteString("import { definePage } from '@/utils/webflow'\n\n")
	builder.WriteString(fmt.Sprintf("// Migrated from Webflow page %q.\n", label))
	if page.Slug != "" {
		builder.WriteString(fmt.Sprintf("// Source slug: %s\n", page.Slug))
	}
	builder.WriteString("\n")
	builder.WriteString(fmt.Sprintf("definePage(%q, () => {\n", page.FolderKey))
	for index, script := range page.Scripts {
		builder.WriteString(fmt.Sprintf("  // Inline script %d\n", index+1))
		builder.WriteString("  ;(() => {\n")
		for _, line := range strings.Split(strings.TrimSpace(script), "\n") {
			builder.WriteString("    ")
			builder.WriteString(line)
			builder.WriteString("\n")
		}
		builder.WriteString("  })()\n")
		if index < len(page.Scripts)-1 {
			builder.WriteString("\n")
		}
	}
	builder.WriteString("})\n")

	return builder.String()
}

func buildMigratedGlobalModule(global GlobalMigration) string {
	var builder strings.Builder
	builder.WriteString("import { onWebflowReady } from '@/utils/webflow'\n\n")
	builder.WriteString("// Migrated from Webflow global custom code.\n\n")
	builder.WriteString("onWebflowReady(() => {\n")

	sectionCount := 0
	writeScripts := func(label string, scripts []string) {
		if len(scripts) == 0 {
			return
		}
		if sectionCount > 0 {
			builder.WriteString("\n")
		}
		sectionCount++
		builder.WriteString(fmt.Sprintf("  // %s\n", label))
		for index, script := range scripts {
			builder.WriteString(fmt.Sprintf("  // Inline script %d\n", index+1))
			builder.WriteString("  ;(() => {\n")
			for _, line := range strings.Split(strings.TrimSpace(script), "\n") {
				builder.WriteString("    ")
				builder.WriteString(line)
				builder.WriteString("\n")
			}
			builder.WriteString("  })()\n")
			if index < len(scripts)-1 {
				builder.WriteString("\n")
			}
		}
	}

	writeScripts("Migrated from global head", global.HeadScripts)
	writeScripts("Migrated from global postBody", global.PostBodyScripts)
	builder.WriteString("})\n")
	return builder.String()
}

func shouldWriteMigration(page PageMigration) bool {
	return page.Action == "write" || page.Action == "overwrite"
}

func shouldWriteGlobalMigration(global GlobalMigration) bool {
	return global.Action == "write" || global.Action == "overwrite"
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func moduleImportPath(entryPath, modulePath string) string {
	baseEntryPath := entryPath
	if !filepath.IsAbs(baseEntryPath) {
		if absPath, err := filepath.Abs(baseEntryPath); err == nil {
			baseEntryPath = absPath
		}
	}

	baseModulePath := modulePath
	if !filepath.IsAbs(baseModulePath) {
		if absPath, err := filepath.Abs(baseModulePath); err == nil {
			baseModulePath = absPath
		}
	}

	relativePath, err := filepath.Rel(filepath.Dir(baseEntryPath), baseModulePath)
	if err != nil {
		relativePath = modulePath
	}
	relativePath = filepath.ToSlash(relativePath)
	if !strings.HasPrefix(relativePath, ".") {
		relativePath = "./" + relativePath
	}
	return strings.TrimSuffix(relativePath, filepath.Ext(relativePath))
}

func ensureModuleImport(entryPath, importPath string) error {
	data, err := os.ReadFile(entryPath)
	if err != nil {
		return err
	}

	content := string(data)
	importLine := fmt.Sprintf("import %q\n", importPath)
	if strings.Contains(content, importLine) || strings.Contains(content, fmt.Sprintf("import '%s'", importPath)) || strings.Contains(content, fmt.Sprintf("import \"%s\"", importPath)) {
		return nil
	}

	lines := strings.Split(content, "\n")
	insertAt := 0
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "import ") {
			insertAt = index + 1
		}
	}

	lines = append(lines[:insertAt], append([]string{strings.TrimSpace(importLine)}, lines[insertAt:]...)...)
	updated := strings.Join(lines, "\n")
	if !strings.HasSuffix(updated, "\n") {
		updated += "\n"
	}

	return os.WriteFile(entryPath, []byte(updated), 0o644)
}

func ensurePageEntry(page PageMigration) error {
	if fileExists(page.EntryPath) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(page.EntryPath), 0o755); err != nil {
		return err
	}

	content := fmt.Sprintf("// Generated by wfkit migrate for %s.\nimport %q\n", pageLabel(webflow.Page{ID: page.PageID, Title: page.Title, Slug: page.Slug}), page.ImportPath)
	return os.WriteFile(page.EntryPath, []byte(content), 0o644)
}

func displayMigrationPath(path string) string {
	relativePath, err := filepath.Rel(".", path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(relativePath)
}
