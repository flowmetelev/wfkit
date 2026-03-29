package publish

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"wfkit/internal/webflow"
)

func TestUpdateScriptForDevReplacesManagedScriptAndInjectsViteClient(t *testing.T) {
	input := `<script type="module" src="http://localhost:5173/@vite/client"></script><script data-script-id="global-script" src="https://cdn.example/app.js"></script><div>content</div>`

	got := updateScript(input, ManagedScript{Delivery: deliveryCDN, URL: "http://localhost:5173/src/main.ts"}, globalScriptID, "dev")

	if strings.Contains(got, "https://cdn.example/app.js") {
		t.Fatalf("expected old managed script to be removed: %s", got)
	}
	if !strings.Contains(got, `src="http://localhost:5173/@vite/client"`) {
		t.Fatalf("expected vite client to be injected: %s", got)
	}
	if !strings.Contains(got, `data-script-id="global-script"`) {
		t.Fatalf("expected global script tag to be injected: %s", got)
	}
	if !strings.Contains(got, `<div>content</div>`) {
		t.Fatalf("expected existing HTML to be preserved: %s", got)
	}
}

func TestUpdateScriptForProdDoesNotInjectViteClient(t *testing.T) {
	got := updateScript(`<div>content</div>`, ManagedScript{Delivery: deliveryCDN, URL: "https://cdn.example/app.js"}, globalScriptID, "prod")

	if strings.Contains(got, "@vite/client") {
		t.Fatalf("did not expect vite client in prod output: %s", got)
	}
	if !strings.Contains(got, `src="https://cdn.example/app.js"`) {
		t.Fatalf("expected production script url in output: %s", got)
	}
}

func TestUpdateScriptForInlineEmbedsModuleCode(t *testing.T) {
	got := updateScript(`<div>content</div>`, ManagedScript{Delivery: deliveryInline, Code: `console.log("inline")`}, globalScriptID, "prod")

	if strings.Contains(got, `src="`) {
		t.Fatalf("did not expect src attribute for inline delivery: %s", got)
	}
	if !strings.Contains(got, `console.log("inline")`) {
		t.Fatalf("expected inline module code in output: %s", got)
	}
	if !strings.Contains(got, `data-script-id="global-script"`) {
		t.Fatalf("expected managed script id in output: %s", got)
	}
}

func TestExtractScriptSrcHandlesDifferentAttributeOrder(t *testing.T) {
	html := `<script src="https://cdn.example/app.js" data-script-id="global-script" defer></script>`

	got := extractScriptSrc(html, globalScriptID)
	if got != "https://cdn.example/app.js" {
		t.Fatalf("unexpected script src: %q", got)
	}
}

func TestCurrentManagedScriptLabelDetectsInlineModules(t *testing.T) {
	html := `<script data-script-id="global-script" type="module">console.log("inline")</script>`

	got := currentManagedScriptLabel(html, globalScriptID)
	if !strings.HasPrefix(got, "inline module (") {
		t.Fatalf("expected inline managed script label, got %q", got)
	}
}

func TestPageFolderKeyPrefersSlugAndNormalizesTitle(t *testing.T) {
	if got := pageFolderKey(webflow.Page{Slug: "Landing-Page"}); got != "landing-page" {
		t.Fatalf("expected slug-based folder key, got %q", got)
	}

	if got := pageFolderKey(webflow.Page{Title: "Pricing / Enterprise"}); got != "pricing-enterprise" {
		t.Fatalf("expected normalized title folder key, got %q", got)
	}
}

func TestExtractMigratableInlineScriptsSkipsManagedAndExternalScripts(t *testing.T) {
	html := strings.Join([]string{
		`<div>before</div>`,
		`<script>console.log("migrate me")</script>`,
		`<script src="https://cdn.example/app.js"></script>`,
		`<script data-script-id="page-script">console.log("managed")</script>`,
		`<script type="application/ld+json">{"name":"ignore"}</script>`,
		`<div>after</div>`,
	}, "\n")

	scripts, cleaned := extractMigratableInlineScripts(html)
	if len(scripts) != 1 {
		t.Fatalf("expected exactly one migrated script, got %d", len(scripts))
	}
	if scripts[0] != `console.log("migrate me")` {
		t.Fatalf("unexpected migrated script: %q", scripts[0])
	}
	if strings.Contains(cleaned, `console.log("migrate me")`) {
		t.Fatalf("expected migrated inline script to be removed from cleaned post body: %s", cleaned)
	}
	if !strings.Contains(cleaned, `src="https://cdn.example/app.js"`) {
		t.Fatalf("expected external script to be preserved: %s", cleaned)
	}
	if !strings.Contains(cleaned, `data-script-id="page-script"`) {
		t.Fatalf("expected managed script to be preserved for later cleanup: %s", cleaned)
	}
	if !strings.Contains(cleaned, `application/ld+json`) {
		t.Fatalf("expected non-JS script to be preserved: %s", cleaned)
	}
}

func TestPlanPageMigrationUsesSlugPathAndRespectsExistingFiles(t *testing.T) {
	tempDir := t.TempDir()
	pagesDir := filepath.Join(tempDir, "src", "pages")

	page := webflow.Page{
		ID:       "page_123",
		Title:    "Pricing",
		Slug:     "pricing",
		PostBody: `<script>console.log("pricing")</script>`,
	}

	plan, err := PlanPageMigration([]webflow.Page{page}, pagesDir, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plan.Pages) != 1 {
		t.Fatalf("expected one migration plan item, got %d", len(plan.Pages))
	}
	if got := filepath.ToSlash(plan.Pages[0].RelativePath); got != filepath.ToSlash(filepath.Join(pagesDir, "pricing", "webflow.migrated.ts")) {
		t.Fatalf("unexpected migration target path: %q", got)
	}
	if got := filepath.ToSlash(plan.Pages[0].EntryRelative); got != filepath.ToSlash(filepath.Join(pagesDir, "pricing", "index.ts")) {
		t.Fatalf("unexpected migration entry path: %q", got)
	}
	if plan.Pages[0].Action != "write" {
		t.Fatalf("expected write action for new file, got %q", plan.Pages[0].Action)
	}

	existingPath := filepath.Join(pagesDir, "pricing", "webflow.migrated.ts")
	if err := os.MkdirAll(filepath.Dir(existingPath), 0o755); err != nil {
		t.Fatalf("failed to create test page dir: %v", err)
	}
	if err := os.WriteFile(existingPath, []byte("export {}\n"), 0o644); err != nil {
		t.Fatalf("failed to seed existing page file: %v", err)
	}

	plan, err = PlanPageMigration([]webflow.Page{page}, pagesDir, false)
	if err != nil {
		t.Fatalf("unexpected error with existing file: %v", err)
	}
	if plan.Pages[0].Action != "existing_file" {
		t.Fatalf("expected existing_file action, got %q", plan.Pages[0].Action)
	}

	plan, err = PlanPageMigration([]webflow.Page{page}, pagesDir, true)
	if err != nil {
		t.Fatalf("unexpected error with force: %v", err)
	}
	if plan.Pages[0].Action != "overwrite" {
		t.Fatalf("expected overwrite action with force, got %q", plan.Pages[0].Action)
	}
	if filepath.ToSlash(plan.Pages[0].RelativePath) != filepath.ToSlash(existingPath) {
		t.Fatalf("expected existing migrated file to be reused, got %q", plan.Pages[0].RelativePath)
	}
}

func TestBuildMigratedPageModuleUsesDefinePageAndWrapsScriptsAsIIFEs(t *testing.T) {
	module := buildMigratedPageModule(PageMigration{
		Title:     "Home",
		Slug:      "home",
		FolderKey: "home",
		Scripts: []string{
			`console.log("first")`,
			`console.log("second")`,
		},
	})

	if !strings.Contains(module, `import { definePage } from '@/utils/webflow'`) {
		t.Fatalf("expected definePage import, got %s", module)
	}
	if !strings.Contains(module, `definePage("home", () => {`) {
		t.Fatalf("expected definePage wrapper, got %s", module)
	}
	if !strings.Contains(module, `// Source slug: home`) {
		t.Fatalf("expected source slug header, got %s", module)
	}
	if strings.Count(module, ";(() => {") != 2 {
		t.Fatalf("expected two IIFE wrappers, got %s", module)
	}
}

func TestPlanMigrationIncludesGlobalModuleAndImportPath(t *testing.T) {
	tempDir := t.TempDir()
	entryPath := filepath.Join(tempDir, "src", "global", "index.ts")
	if err := os.MkdirAll(filepath.Dir(entryPath), 0o755); err != nil {
		t.Fatalf("failed to create global entry dir: %v", err)
	}
	if err := os.WriteFile(entryPath, []byte("export const WF = {}\n"), 0o644); err != nil {
		t.Fatalf("failed to seed global entry: %v", err)
	}

	globalData := webflow.GlobalCode{
		Meta: map[string]string{
			"head":     `<script>console.log("head")</script>`,
			"postBody": `<div>footer</div><script>console.log("postBody")</script>`,
		},
	}

	plan, err := PlanMigration(globalData, nil, filepath.Join(tempDir, "src", "pages"), entryPath, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if plan.Global.Action != "write" {
		t.Fatalf("expected global write action, got %q", plan.Global.Action)
	}
	if !strings.HasSuffix(filepath.ToSlash(plan.Global.ModuleRelativePath), filepath.ToSlash(filepath.Join("src", "global", "modules", "webflow.migrated.ts"))) {
		t.Fatalf("unexpected global module path: %q", plan.Global.ModuleRelativePath)
	}
	if plan.Global.ImportPath != "./modules/webflow.migrated" {
		t.Fatalf("unexpected global import path: %q", plan.Global.ImportPath)
	}
	if len(plan.Global.HeadScripts) != 1 || len(plan.Global.PostBodyScripts) != 1 {
		t.Fatalf("expected one script from head and postBody, got %d and %d", len(plan.Global.HeadScripts), len(plan.Global.PostBodyScripts))
	}
}

func TestEnsureModuleImportAddsImportOnlyOnce(t *testing.T) {
	tempDir := t.TempDir()
	entryPath := filepath.Join(tempDir, "src", "global", "index.ts")
	if err := os.MkdirAll(filepath.Dir(entryPath), 0o755); err != nil {
		t.Fatalf("failed to create global entry dir: %v", err)
	}
	content := "import { mountSiteGlobal } from './modules/site.global'\n\nmountSiteGlobal()\n"
	if err := os.WriteFile(entryPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write entry file: %v", err)
	}

	if err := ensureModuleImport(entryPath, "./modules/webflow.migrated"); err != nil {
		t.Fatalf("failed to ensure import: %v", err)
	}
	if err := ensureModuleImport(entryPath, "./modules/webflow.migrated"); err != nil {
		t.Fatalf("failed to ensure import a second time: %v", err)
	}

	data, err := os.ReadFile(entryPath)
	if err != nil {
		t.Fatalf("failed to read updated entry file: %v", err)
	}
	if strings.Count(string(data), `import "./modules/webflow.migrated"`) != 1 {
		t.Fatalf("expected migrated import to be inserted exactly once, got %s", string(data))
	}
}

func TestWriteMigrationFilesCreatesPageEntryAndWiresModuleImport(t *testing.T) {
	tempDir := t.TempDir()
	pagesDir := filepath.Join(tempDir, "src", "pages")
	plan, err := PlanPageMigration([]webflow.Page{
		{
			ID:       "page_1",
			Title:    "Home",
			Slug:     "home",
			PostBody: `<script>console.log("home")</script>`,
		},
	}, pagesDir, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := WriteMigrationFiles(plan); err != nil {
		t.Fatalf("failed to write migration files: %v", err)
	}

	entryPath := filepath.Join(pagesDir, "home", "index.ts")
	entryData, err := os.ReadFile(entryPath)
	if err != nil {
		t.Fatalf("failed to read generated entry: %v", err)
	}
	if !strings.Contains(string(entryData), `import "./webflow.migrated"`) {
		t.Fatalf("expected generated entry to import migrated module, got %s", string(entryData))
	}

	modulePath := filepath.Join(pagesDir, "home", "webflow.migrated.ts")
	moduleData, err := os.ReadFile(modulePath)
	if err != nil {
		t.Fatalf("failed to read generated migrated module: %v", err)
	}
	if !strings.Contains(string(moduleData), `definePage("home", () => {`) {
		t.Fatalf("expected migrated module to initialize via definePage, got %s", string(moduleData))
	}
}
