package steps

import (
	"wfkit/internal/initialize/config"
	"wfkit/internal/initialize/utils"
)

func CreateProjectReadme(opts config.Options) error {
	data := struct {
		ProjectName    string
		DocsPageSlug   string
		InstallCommand string
		RunScript      string
	}{
		ProjectName:    opts.Name,
		DocsPageSlug:   opts.DocsPageSlug,
		InstallCommand: installCommand(opts.PackageManager),
		RunScript:      runScriptCommand(opts.PackageManager),
	}

	return utils.RenderTemplateToFile("project-readme.md.tmpl", data, "README.md")
}

func installCommand(packageManager string) string {
	switch packageManager {
	case "bun":
		return "bun install"
	case "yarn":
		return "yarn"
	case "pnpm":
		return "pnpm install"
	default:
		return "npm install"
	}
}

func runScriptCommand(packageManager string) string {
	switch packageManager {
	case "bun":
		return "bun run"
	case "yarn":
		return "yarn"
	case "pnpm":
		return "pnpm"
	default:
		return "npm run"
	}
}
