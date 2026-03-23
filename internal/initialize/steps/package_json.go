// internal/initialize/steps/package_json.go
package steps

import (
	"wfkit/internal/initialize/config"
	"wfkit/internal/initialize/utils"
)

func CreatePackageJSON(opts config.Options) error {
	data := struct {
		Name string
	}{
		Name: opts.Name,
	}

	templateName := "package.json.tmpl"

	return utils.RenderTemplateToFile(templateName, data, "package.json")
}
