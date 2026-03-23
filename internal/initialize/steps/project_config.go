package steps

import (
	initconfig "wfkit/internal/initialize/config"
	"wfkit/internal/initialize/utils"
)

func CreateProjectConfig(opts initconfig.Options) error {
	data := struct {
		AppName        string
		SiteURL        string
		GitHubUser     string
		RepositoryName string
		PackageManager string
		GlobalEntry    string
		DocsEntry      string
		DocsPageSlug   string
	}{
		AppName:        opts.Name,
		SiteURL:        "https://" + opts.Name + ".webflow.io",
		GitHubUser:     opts.GitHubUser,
		RepositoryName: opts.RepositoryName,
		PackageManager: opts.PackageManager,
		GlobalEntry:    opts.GlobalEntry,
		DocsEntry:      opts.DocsEntry,
		DocsPageSlug:   opts.DocsPageSlug,
	}

	return utils.RenderTemplateToFile("wfkit.json.tmpl", data, "wfkit.json")
}
