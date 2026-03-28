package steps

import (
	"fmt"
	"path/filepath"
	"strings"

	"wfkit/internal/initialize/config"
	"wfkit/internal/initialize/utils"
)

func CreateGlobalFiles(opts config.Options) error {
	files := []struct {
		template string
		output   string
		data     interface{}
	}{
		{
			template: "global-entry.ts.tmpl",
			output:   opts.GlobalEntry,
			data: struct {
				GlobalVar string
			}{
				GlobalVar: opts.GlobalVar,
			},
		},
		{
			template: "global-module.ts.tmpl",
			output:   filepath.Join("src", "global", "modules", "site.global.ts"),
		},
		{
			template: "dom-utils.ts.tmpl",
			output:   filepath.Join("src", "utils", "dom.ts"),
		},
		{
			template: "webflow-utils.ts.tmpl",
			output:   filepath.Join("src", "utils", "webflow.ts"),
		},
		{
			template: "site-status-feature.ts.tmpl",
			output:   filepath.Join("src", "features", "site-status.ts"),
		},
		{
			template: "webflow-vite-plugin.js.tmpl",
			output:   filepath.Join("build", "webflow-vite-plugin.js"),
		},
		{
			template: "docs-index.md.tmpl",
			output:   opts.DocsEntry,
			data: struct {
				ProjectName  string
				DocsPageSlug string
			}{
				ProjectName:  opts.Name,
				DocsPageSlug: opts.DocsPageSlug,
			},
		},
	}

	for _, file := range files {
		if err := utils.RenderTemplateToFile(file.template, file.data, file.output); err != nil {
			return fmt.Errorf("failed to create %s: %w", file.output, err)
		}
	}

	typeData := struct {
		GlobalVar         string
		GlobalEntryImport string
	}{
		GlobalVar:         opts.GlobalVar,
		GlobalEntryImport: globalEntryImportPath(opts.GlobalEntry),
	}
	if err := utils.RenderTemplateToFile("global.d.ts.tmpl", typeData, "src/global.d.ts"); err != nil {
		return err
	}

	return nil
}

func globalEntryImportPath(globalEntry string) string {
	trimmed := strings.TrimPrefix(filepath.ToSlash(globalEntry), "src/")
	trimmed = strings.TrimSuffix(trimmed, filepath.Ext(trimmed))
	if trimmed == "" {
		return "./global"
	}
	return "./" + trimmed
}
