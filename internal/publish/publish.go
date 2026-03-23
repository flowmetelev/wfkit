package publish

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"wfkit/internal/build"
	"wfkit/internal/utils"
	"wfkit/internal/webflow"
)

const (
	globalScriptID = "global-script"
	pageScriptID   = "page-script"
)

type GlobalPublishPlan struct {
	CurrentSrc string
	NextSrc    string
	Action     string
}

type PagePublishPlan struct {
	PageID          string
	Title           string
	Slug            string
	Folder          string
	ScriptFile      string
	CurrentPostBody string
	CurrentSrc      string
	NextSrc         string
	Action          string
	Message         string
}

type ByPagePlan struct {
	Global GlobalPublishPlan
	Pages  []PagePublishPlan
}

type ByPagePublishResult struct {
	Plan          ByPagePlan
	GlobalUpdated bool
	UpdatedPages  int
	Published     bool
}

// PublishGlobalScript публикует глобальный скрипт на сайт Webflow.
// Возвращает:
// - были ли внесены изменения
// - массив из head и body HTML (для справки)
// - любую возникшую ошибку
func PublishGlobalScript(ctx context.Context, siteName, cookies, pToken, scriptUrl, env string) (bool, [2]string, error) {
	globalData, err := webflow.GetGlobalCode(ctx, siteName, pToken, cookies)
	if err != nil {
		return false, [2]string{}, fmt.Errorf("failed to get global code: %w", err)
	}

	head := globalData.Meta["head"]
	postBody := globalData.Meta["postBody"]

	oldSrc := extractScriptSrc(postBody, globalScriptID)
	if oldSrc == scriptUrl {
		return false, [2]string{head, postBody}, nil
	}

	newPostBody := updateScript(postBody, scriptUrl, globalScriptID, env)
	if newPostBody == postBody {
		return false, [2]string{head, postBody}, nil
	}

	if err := webflow.UpdateGlobalCode(ctx, siteName, pToken, cookies, head, newPostBody); err != nil {
		return false, [2]string{}, fmt.Errorf("failed to update global code: %w", err)
	}

	if err := webflow.PublishSite(ctx, siteName, pToken, cookies); err != nil {
		return true, [2]string{head, postBody}, fmt.Errorf("code updated but failed to publish site: %w", err)
	}

	return true, [2]string{head, postBody}, nil
}

func PreviewGlobalPublish(ctx context.Context, siteName, cookies, pToken, scriptURL, env string) (GlobalPublishPlan, error) {
	globalData, err := webflow.GetGlobalCode(ctx, siteName, pToken, cookies)
	if err != nil {
		return GlobalPublishPlan{}, fmt.Errorf("failed to get global code: %w", err)
	}

	postBody := globalData.Meta["postBody"]
	currentSrc := extractScriptSrc(postBody, globalScriptID)
	nextBody := updateScript(postBody, scriptURL, globalScriptID, env)
	action := "update"
	if currentSrc == scriptURL || nextBody == postBody {
		action = "up_to_date"
	}

	return GlobalPublishPlan{
		CurrentSrc: currentSrc,
		NextSrc:    scriptURL,
		Action:     action,
	}, nil
}

// PublishByPage публикует скрипты для каждой страницы на сайте Webflow.
// Обрабатывает глобальные и специфичные для страницы скрипты на основе содержимого директории сборки.
func PublishByPage(ctx context.Context, siteName, baseUrl, cookies, pToken, ghUser, repo string, args map[string]interface{}) (ByPagePublishResult, error) {
	plan, err := PlanByPagePublish(ctx, siteName, cookies, pToken, ghUser, repo, args)
	if err != nil {
		return ByPagePublishResult{}, err
	}

	env, ok := args["env"].(string)
	if !ok {
		env = "prod" // default to production if not specified
	}

	result := ByPagePublishResult{Plan: plan}

	if plan.Global.NextSrc != "" && plan.Global.Action == "update" {
		updated, _, err := PublishGlobalScript(ctx, siteName, cookies, pToken, plan.Global.NextSrc, env)
		if err != nil {
			return result, fmt.Errorf("failed to update global script: %w", err)
		}
		if updated {
			utils.CPrint("Global script updated", "green")
			result.GlobalUpdated = true
		} else {
			utils.CPrint("Global script is already up to date", "green")
		}
	}

	anyChanges := false

	for _, pagePlan := range plan.Pages {
		if pagePlan.Action != "update" {
			if pagePlan.Message != "" && pagePlan.Action != "up_to_date" {
				utils.CPrint(pagePlan.Message, "yellow")
			}
			continue
		}

		newPostBody := updateScript(pagePlan.CurrentPostBody, pagePlan.NextSrc, pageScriptID, env)
		updatedPage := webflow.Page{
			ID:       pagePlan.PageID,
			Title:    pagePlan.Title,
			PostBody: newPostBody,
			Slug:     pagePlan.Slug,
		}

		if err := webflow.PutFullPageObject(ctx, baseUrl, pToken, cookies, updatedPage); err != nil {
			utils.CPrint(fmt.Sprintf("Failed to update page %s: %v", pagePlan.Title, err), "red")
			continue
		}

		utils.CPrint(fmt.Sprintf("Page %s updated", pagePlan.Title), "green")
		anyChanges = true
		result.UpdatedPages++
	}

	if anyChanges {
		if err := webflow.PublishSite(ctx, siteName, pToken, cookies); err != nil {
			return result, fmt.Errorf("failed to publish site changes: %w", err)
		}
		utils.CPrint("Site published with all changes", "green")
		result.Published = true
	} else if result.GlobalUpdated {
		result.Published = true
	} else {
		utils.CPrint("No changes were needed, skipping publish", "blue")
	}

	return result, nil
}

func PlanByPagePublish(ctx context.Context, siteName, cookies, pToken, ghUser, repo string, args map[string]interface{}) (ByPagePlan, error) {
	buildDir, ok := args["build-dir"].(string)
	if !ok {
		return ByPagePlan{}, fmt.Errorf("missing or invalid 'build-dir' argument")
	}

	branch, ok := args["branch"].(string)
	if !ok {
		return ByPagePlan{}, fmt.Errorf("missing or invalid 'branch' argument")
	}

	env, ok := args["env"].(string)
	if !ok {
		env = "prod"
	}

	plan := ByPagePlan{}
	pageEntries := map[string]string{}

	if manifestPages, err := build.ResolvePageEntries(buildDir); err == nil {
		pageEntries = manifestPages
	}

	if globalPath, err := build.ResolveGlobalEntry(buildDir); err == nil && globalPath != "" {
		globalURL := buildCDNUrl(ghUser, repo, branch, buildDir, globalPath, env, args)
		globalPlan, err := PreviewGlobalPublish(ctx, siteName, cookies, pToken, globalURL, env)
		if err != nil {
			return ByPagePlan{}, err
		}
		plan.Global = globalPlan
	}

	pages, err := webflow.GetPagesListFromDom(ctx, siteName, pToken, cookies)
	if err != nil {
		return ByPagePlan{}, fmt.Errorf("failed to get pages list: %w", err)
	}

	for _, page := range pages {
		pagePlan := PagePublishPlan{
			PageID: page.ID,
			Title:  page.Title,
			Slug:   page.Slug,
		}

		folderKey := pageFolderKey(page)
		if folderKey == "" {
			pagePlan.Action = "missing_title"
			pagePlan.Message = "Page with missing title and slug, skipping"
			plan.Pages = append(plan.Pages, pagePlan)
			continue
		}

		if manifestPath, ok := pageEntries[folderKey]; ok {
			pagePlan.Folder = filepath.Join(buildDir, filepath.Dir(manifestPath))
			pagePlan.ScriptFile = filepath.Base(manifestPath)
		} else {
			pagePlan.Folder = filepath.Join(buildDir, folderKey)
			if _, err := os.Stat(pagePlan.Folder); os.IsNotExist(err) {
				pagePlan.Action = "missing_directory"
				pagePlan.Message = fmt.Sprintf("Skipping %s: directory not found (%s)", pageLabel(page), pagePlan.Folder)
				plan.Pages = append(plan.Pages, pagePlan)
				continue
			}

			scriptFile, err := findFile(pagePlan.Folder, "index-", ".js")
			if err != nil {
				pagePlan.Action = "error"
				pagePlan.Message = fmt.Sprintf("Error accessing %s: %v", pagePlan.Folder, err)
				plan.Pages = append(plan.Pages, pagePlan)
				continue
			}
			if scriptFile == "" {
				pagePlan.Action = "missing_script"
				pagePlan.Message = fmt.Sprintf("No index script found for %s in %s", pageLabel(page), pagePlan.Folder)
				plan.Pages = append(plan.Pages, pagePlan)
				continue
			}
			pagePlan.ScriptFile = scriptFile
		}

		pagePlan.CurrentPostBody = page.PostBody
		pagePlan.CurrentSrc = extractScriptSrc(page.PostBody, pageScriptID)
		pagePath := filepath.ToSlash(filepath.Join(folderKey, pagePlan.ScriptFile))
		if manifestPath, ok := pageEntries[folderKey]; ok {
			pagePath = manifestPath
		}
		pagePlan.NextSrc = buildCDNUrl(ghUser, repo, branch, buildDir, pagePath, env, args)

		if page.PostBody == "" {
			pagePlan.Action = "missing_postbody"
			pagePlan.Message = fmt.Sprintf("Missing postBody for page %s", pageLabel(page))
			plan.Pages = append(plan.Pages, pagePlan)
			continue
		}

		if pagePlan.CurrentSrc == pagePlan.NextSrc {
			pagePlan.Action = "up_to_date"
			pagePlan.Message = fmt.Sprintf("Page %s is already up to date", pageLabel(page))
			plan.Pages = append(plan.Pages, pagePlan)
			continue
		}

		pagePlan.Action = "update"
		pagePlan.Message = fmt.Sprintf("Page %s will be updated from %s to %s", pageLabel(page), valueOrDash(pagePlan.CurrentSrc), pagePlan.NextSrc)
		plan.Pages = append(plan.Pages, pagePlan)
	}

	return plan, nil
}

// extractScriptSrc находит и возвращает атрибут src из тега script с указанным ID

func extractScriptSrc(html, scriptID string) string {
	re := regexp.MustCompile(fmt.Sprintf(`<script[^>]*data-script-id=["']%s["'][^>]*src=["']([^"']+)["'][^>]*>`, scriptID))
	matches := re.FindStringSubmatch(html)
	if len(matches) > 1 {
		return matches[1]
	}

	// Try alternate format where src comes before data-script-id
	re = regexp.MustCompile(fmt.Sprintf(`<script[^>]*src=["']([^"']+)["'][^>]*data-script-id=["']%s["'][^>]*>`, scriptID))
	matches = re.FindStringSubmatch(html)
	if len(matches) > 1 {
		return matches[1]
	}

	return ""
}

// updateScript удаляет существующий скрипт с указанным ID и добавляет новый
func updateScript(html, scriptUrl, scriptID, env string) string {
	// Remove existing script with this ID
	re := regexp.MustCompile(fmt.Sprintf(`<script[^>]*data-script-id=["']%s["'][^>]*>.*?</script>`, scriptID))
	cleaned := re.ReplaceAllString(html, "")
	cleaned = stripViteClientScript(cleaned)

	// Create new script tag
	scriptTag := fmt.Sprintf(`<script data-script-id="%s" type="module" defer src="%s"></script>`, scriptID, scriptUrl)

	// Add Vite client for development environments
	if env != "prod" {
		viteClientUrl := buildViteClientURL(scriptUrl)
		scriptTag = fmt.Sprintf(`<script type="module" src="%s"></script>%s`, viteClientUrl, scriptTag)
	}

	return scriptTag + "\n" + cleaned
}

func stripViteClientScript(html string) string {
	re := regexp.MustCompile(`<script[^>]*src=["'][^"']*/@vite/client["'][^>]*>.*?</script>`)
	return re.ReplaceAllString(html, "")
}

func buildViteClientURL(scriptURL string) string {
	u, err := url.Parse(scriptURL)
	if err != nil {
		return strings.TrimRight(scriptURL, "/") + "/@vite/client"
	}
	u.Path = "/@vite/client"
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

// findFile ищет файл с заданным префиксом и суффиксом в указанной директории
func findFile(directory, prefix, suffix string) (string, error) {
	files, err := os.ReadDir(directory)
	if err != nil {
		return "", err
	}

	for _, f := range files {
		if f.IsDir() {
			continue
		}

		name := f.Name()
		if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, suffix) {
			return name, nil
		}
	}

	return "", nil
}

// buildCDNUrl конструирует URL к скрипту на основе окружения
func buildCDNUrl(user, repo, branch, baseDir, path, env string, args map[string]interface{}) string {
	if env != "prod" {
		port := 5173
		if p, ok := args["dev-port"].(int); ok {
			port = p
		}
		host := "localhost"
		if h, ok := args["dev-host"].(string); ok {
			host = h
		}
		return fmt.Sprintf("http://%s:%d/%s", host, port, path)
	}
	return fmt.Sprintf("https://cdn.jsdelivr.net/gh/%s/%s@%s/%s/%s", user, repo, branch, baseDir, path)
}

func pageFolderKey(page webflow.Page) string {
	if page.Slug != "" {
		return strings.ToLower(strings.TrimSpace(page.Slug))
	}

	title := strings.TrimSpace(page.Title)
	if title == "" {
		return ""
	}

	title = strings.ToLower(title)
	replacer := strings.NewReplacer(" ", "-", "_", "-", "/", "-", "\\", "-", ".", "-")
	title = replacer.Replace(title)
	re := regexp.MustCompile(`[^a-z0-9-]+`)
	title = re.ReplaceAllString(title, "")
	title = regexp.MustCompile(`-+`).ReplaceAllString(title, "-")
	title = strings.Trim(title, "-")
	return title
}

func pageLabel(page webflow.Page) string {
	if page.Title != "" {
		return page.Title
	}
	if page.Slug != "" {
		return page.Slug
	}
	return page.ID
}

func valueOrDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}
