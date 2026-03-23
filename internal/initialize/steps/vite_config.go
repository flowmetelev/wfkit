package steps

import (
	"wfkit/internal/initialize/config"
	"wfkit/internal/initialize/utils"
)

func CreateViteConfig(opts config.Options) error {
	data := struct {
		PagesDir    string
		GlobalEntry string
	}{
		PagesDir:    opts.PagesDir,
		GlobalEntry: opts.GlobalEntry,
	}

	return utils.RenderTemplateToFile("vite.config.js.tmpl", data, "vite.config.js")
}
