// internal/initialize/config/opts.go
package config

type Options struct {
	PagesDir       string `default:"src/pages"`
	GlobalEntry    string `default:"src/global/index.ts"`
	GlobalVar      string `default:"WF"`
	Types          bool
	InitGit        bool
	PackageManager string `default:"bun"`
	Name           string
	GitHubUser     string
	RepositoryName string
	DocsEntry      string `default:"docs/index.md"`
	DocsPageSlug   string `default:"docs"`
}

// Метод для установки значений по умолчанию
func (o *Options) SetDefaultValues() {
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
