// internal/initialize/config/opts.go
package config

import "path/filepath"

type Options struct {
	ProjectDir     string
	PagesDir       string `default:"src/pages"`
	GlobalEntry    string `default:"src/global/index.ts"`
	GlobalVar      string `default:"WF"`
	InitGit        bool
	Force          bool
	SkipInstall    bool
	PackageManager string `default:"bun"`
	CLIValue       string
	Name           string
	GitHubUser     string
	RepositoryName string
	DocsEntry      string `default:"docs/index.md"`
	DocsPageSlug   string `default:"docs"`
}

// Метод для установки значений по умолчанию
func (o *Options) SetDefaultValues() {
	if o.ProjectDir == "" {
		o.ProjectDir = o.Name
	}
	if o.Name == "" && o.ProjectDir != "" {
		o.Name = filepath.Base(filepath.Clean(o.ProjectDir))
	}
	if o.PagesDir == "" {
		o.PagesDir = "src/pages"
	}
	if o.GlobalEntry == "" {
		o.GlobalEntry = "src/global/index.ts"
	}
	if o.GlobalVar == "" {
		o.GlobalVar = "WF"
	}
	if o.PackageManager == "" {
		o.PackageManager = "bun"
	}
	if o.DocsEntry == "" {
		o.DocsEntry = "docs/index.md"
	}
	if o.DocsPageSlug == "" {
		o.DocsPageSlug = "docs"
	}
}
