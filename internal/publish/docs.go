package publish

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"regexp"
	"strings"

	"wfkit/internal/webflow"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
)

const docsHubScriptID = "docs-hub"

var docsHubScriptPattern = regexp.MustCompile(`(?is)<script\b[^>]*data-script-id=["']docs-hub["'][^>]*>.*?</script>`)

type DocsHubOptions struct {
	EntryPath string
	PageSlug  string
	Publish   bool
	Selector  string
}

type DocsHubPlan struct {
	PageID          string
	PageTitle       string
	PageSlug        string
	CreatePage      bool
	EntryPath       string
	Selector        string
	MarkdownTitle   string
	CurrentPostBody string
	NextPostBody    string
	Action          string
	Message         string
}

type DocsHubResult struct {
	Plan      DocsHubPlan
	Created   bool
	Updated   bool
	Published bool
}

type docsHeading struct {
	Level int
	ID    string
	Text  string
}

func PlanDocsHubSync(pages []webflow.Page, opts DocsHubOptions) (DocsHubPlan, error) {
	entryPath := strings.TrimSpace(opts.EntryPath)
	if entryPath == "" {
		entryPath = "docs/index.md"
	}
	pageSlug := strings.TrimSpace(opts.PageSlug)
	if pageSlug == "" {
		pageSlug = "docs"
	}
	selector := strings.TrimSpace(opts.Selector)
	if selector == "" {
		selector = `[data-wf-docs-root], main`
	}

	markdownBytes, err := os.ReadFile(entryPath)
	if err != nil {
		return DocsHubPlan{}, fmt.Errorf("failed to read %s: %w", entryPath, err)
	}

	title, renderedHTML, err := renderDocsHubHTML(markdownBytes)
	if err != nil {
		return DocsHubPlan{}, fmt.Errorf("failed to render docs hub HTML: %w", err)
	}

	targetPage, ok := findPageBySlug(pages, pageSlug)
	currentPostBody := ""
	pageID := ""
	pageTitle := title
	createPage := !ok
	action := "create"
	message := fmt.Sprintf("Will create docs hub page %q (%s) from %s", pageTitle, pageSlug, entryPath)
	if ok {
		pageID = targetPage.ID
		pageTitle = targetPage.Title
		currentPostBody = targetPage.PostBody
		action = "update"
		message = fmt.Sprintf("Will update docs hub page %s from %s", pageLabel(targetPage), entryPath)
	}

	nextPostBody := upsertInlineManagedScript(currentPostBody, docsHubScriptID, buildDocsHubBootstrap(title, renderedHTML, selector))
	if ok && currentPostBody == nextPostBody {
		action = "up_to_date"
		message = fmt.Sprintf("Docs hub page %s is already up to date", pageLabel(targetPage))
	}

	return DocsHubPlan{
		PageID:          pageID,
		PageTitle:       pageTitle,
		PageSlug:        pageSlug,
		CreatePage:      createPage,
		EntryPath:       entryPath,
		Selector:        selector,
		MarkdownTitle:   title,
		CurrentPostBody: currentPostBody,
		NextPostBody:    nextPostBody,
		Action:          action,
		Message:         message,
	}, nil
}

func ApplyDocsHubSync(ctx context.Context, siteName, baseURL, token, cookies string, plan DocsHubPlan, publishSite bool) (DocsHubResult, error) {
	result := DocsHubResult{Plan: plan}
	if plan.Action == "up_to_date" {
		return result, nil
	}

	page := webflow.Page{
		ID:       plan.PageID,
		Title:    plan.PageTitle,
		Slug:     plan.PageSlug,
		PostBody: plan.NextPostBody,
	}
	if plan.CreatePage {
		createdPage, err := webflow.CreateStaticPage(ctx, siteName, baseURL, token, cookies, plan.PageTitle, plan.PageSlug)
		if err != nil {
			return result, fmt.Errorf("failed to create docs hub page %q: %w", plan.PageSlug, err)
		}
		page.ID = createdPage.ID
		page.Title = createdPage.Title
		result.Plan.PageID = createdPage.ID
		result.Plan.PageTitle = createdPage.Title
		result.Created = true
	}

	if err := webflow.PutFullPageObject(ctx, baseURL, token, cookies, page); err != nil {
		return result, fmt.Errorf("failed to update docs hub page %s: %w", pageLabel(page), err)
	}
	result.Updated = true

	if publishSite {
		if err := webflow.PublishSite(ctx, siteName, token, cookies); err != nil {
			return result, fmt.Errorf("failed to publish docs hub page: %w", err)
		}
		result.Published = true
	}

	return result, nil
}

func renderDocsHubHTML(markdown []byte) (string, string, error) {
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithRendererOptions(html.WithUnsafe()),
	)

	context := parser.NewContext()
	document := md.Parser().Parse(text.NewReader(markdown), parser.WithContext(context))
	headings := collectDocsHeadings(document, markdown)

	var rendered bytes.Buffer
	if err := md.Renderer().Render(&rendered, markdown, document); err != nil {
		return "", "", err
	}

	title := firstHeadingTitle(headings)
	if title == "" {
		title = "Documentation"
	}

	layout := buildDocsHubLayout(rendered.String(), headings)
	return title, layout, nil
}

func collectDocsHeadings(document ast.Node, source []byte) []docsHeading {
	var headings []docsHeading
	ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		heading, ok := node.(*ast.Heading)
		if !ok {
			return ast.WalkContinue, nil
		}

		text := strings.TrimSpace(extractNodeText(heading, source))
		if text == "" {
			return ast.WalkContinue, nil
		}

		id := ""
		if idValue, ok := heading.AttributeString("id"); ok {
			switch typed := idValue.(type) {
			case []byte:
				id = strings.TrimSpace(string(typed))
			case string:
				id = strings.TrimSpace(typed)
			}
		}
		headings = append(headings, docsHeading{
			Level: heading.Level,
			ID:    id,
			Text:  text,
		})
		return ast.WalkContinue, nil
	})
	return headings
}

func extractNodeText(node ast.Node, source []byte) string {
	var builder strings.Builder
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		switch typed := child.(type) {
		case *ast.Text:
			builder.Write(typed.Segment.Value(source))
		default:
			builder.WriteString(extractNodeText(child, source))
		}
	}
	return builder.String()
}

func firstHeadingTitle(headings []docsHeading) string {
	for _, heading := range headings {
		if heading.Level == 1 {
			return heading.Text
		}
	}
	if len(headings) > 0 {
		return headings[0].Text
	}
	return ""
}

func buildDocsHubLayout(contentHTML string, headings []docsHeading) string {
	title := firstHeadingTitle(headings)
	if title == "" {
		title = "Documentation"
	}

	var toc strings.Builder
	toc.WriteString(`<nav class="wf-docs-toc" aria-label="Table of contents">`)
	for _, heading := range headings {
		if heading.ID == "" {
			continue
		}
		levelClass := fmt.Sprintf("wf-docs-toc-level-%d", heading.Level)
		toc.WriteString(fmt.Sprintf(`<a class="%s" href="#%s">%s</a>`, levelClass, heading.ID, escapeDocsHTML(heading.Text)))
	}
	toc.WriteString(`</nav>`)

	return fmt.Sprintf(
		`<div class="wf-docs-shell"><aside class="wf-docs-sidebar"><div class="wf-docs-sidebar-inner"><div class="wf-docs-eyebrow">Documentation</div><h1 class="wf-docs-sidebar-title">%s</h1>%s</div></aside><article class="wf-docs-content">%s</article></div>`,
		escapeDocsHTML(title),
		toc.String(),
		contentHTML,
	)
}

func buildDocsHubBootstrap(title, docsHTML, selector string) string {
	htmlPayload := base64.StdEncoding.EncodeToString([]byte(docsHTML))
	cssPayload := base64.StdEncoding.EncodeToString([]byte(docsHubCSS))

	return fmt.Sprintf(
		`(function(){const selector=%q;const title=%q;const html=atob(%q);const css=atob(%q);const root=document.querySelector(selector)||document.createElement('main');if(!root.parentNode){document.body.appendChild(root)}let style=document.getElementById('wf-docs-style');if(!style){style=document.createElement('style');style.id='wf-docs-style';style.textContent=css;document.head.appendChild(style)}root.innerHTML=html;root.setAttribute('data-wf-docs-rendered','true');if(title){document.title=title}})();`,
		selector,
		title,
		htmlPayload,
		cssPayload,
	)
}

func upsertInlineManagedScript(html, scriptID, scriptContent string) string {
	cleaned := removeInlineManagedScript(html, scriptID)
	scriptTag := fmt.Sprintf(`<script data-script-id="%s">%s</script>`, scriptID, scriptContent)
	cleaned = strings.TrimSpace(cleaned)
	if cleaned == "" {
		return scriptTag
	}
	return scriptTag + "\n" + cleaned
}

func removeInlineManagedScript(html, scriptID string) string {
	re := regexp.MustCompile(fmt.Sprintf(`(?is)<script\b[^>]*data-script-id=["']%s["'][^>]*>.*?</script>`, scriptID))
	cleaned := re.ReplaceAllString(html, "")
	return strings.TrimSpace(cleaned)
}

func findPageBySlug(pages []webflow.Page, slug string) (webflow.Page, bool) {
	target := strings.TrimSpace(strings.ToLower(slug))
	for _, page := range pages {
		if strings.TrimSpace(strings.ToLower(page.Slug)) == target {
			return page, true
		}
	}
	return webflow.Page{}, false
}

func escapeDocsHTML(value string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&#39;",
	)
	return replacer.Replace(value)
}

const docsHubCSS = `
.wf-docs-shell{max-width:1200px;margin:0 auto;padding:48px 24px 96px;display:grid;grid-template-columns:minmax(220px,280px) minmax(0,1fr);gap:40px;font-family:ui-sans-serif,system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;color:#e2e8f0}
.wf-docs-sidebar{position:sticky;top:32px;align-self:start}
.wf-docs-sidebar-inner{padding:24px;border:1px solid rgba(148,163,184,.2);border-radius:24px;background:linear-gradient(180deg,rgba(15,23,42,.96),rgba(15,23,42,.8));box-shadow:0 20px 60px rgba(15,23,42,.18)}
.wf-docs-eyebrow{font-size:12px;letter-spacing:.12em;text-transform:uppercase;color:#67e8f9;margin-bottom:12px}
.wf-docs-sidebar-title{margin:0 0 20px;font-size:28px;line-height:1.1;color:#f8fafc}
.wf-docs-toc{display:flex;flex-direction:column;gap:10px}
.wf-docs-toc a{color:#cbd5e1;text-decoration:none;font-size:14px;line-height:1.4}
.wf-docs-toc a:hover{color:#67e8f9}
.wf-docs-toc-level-2{padding-left:12px}
.wf-docs-toc-level-3,.wf-docs-toc-level-4,.wf-docs-toc-level-5,.wf-docs-toc-level-6{padding-left:24px}
.wf-docs-content{min-width:0;padding:32px;border:1px solid rgba(148,163,184,.16);border-radius:28px;background:rgba(15,23,42,.7);box-shadow:0 20px 60px rgba(15,23,42,.16)}
.wf-docs-content>*:first-child{margin-top:0}
.wf-docs-content>*:last-child{margin-bottom:0}
.wf-docs-content h1,.wf-docs-content h2,.wf-docs-content h3,.wf-docs-content h4,.wf-docs-content h5,.wf-docs-content h6{margin:1.75em 0 .5em;color:#f8fafc;line-height:1.15}
.wf-docs-content p,.wf-docs-content li{color:#cbd5e1;font-size:16px;line-height:1.75}
.wf-docs-content a{color:#67e8f9}
.wf-docs-content code{padding:.15em .4em;border-radius:6px;background:rgba(30,41,59,.9);font-size:.92em;color:#e2e8f0}
.wf-docs-content pre{overflow:auto;padding:18px 20px;border-radius:18px;background:#020617}
.wf-docs-content pre code{padding:0;background:transparent}
.wf-docs-content blockquote{margin:24px 0;padding-left:18px;border-left:3px solid #22d3ee;color:#94a3b8}
.wf-docs-content table{width:100%;border-collapse:collapse;margin:24px 0}
.wf-docs-content th,.wf-docs-content td{padding:12px 14px;border:1px solid rgba(148,163,184,.2);text-align:left}
.wf-docs-content img{max-width:100%;height:auto;border-radius:16px}
@media (max-width:960px){.wf-docs-shell{grid-template-columns:1fr}.wf-docs-sidebar{position:static}.wf-docs-content{padding:24px}}
`
