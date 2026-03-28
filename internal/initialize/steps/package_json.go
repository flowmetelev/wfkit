// internal/initialize/steps/package_json.go
package steps

import (
	"regexp"
	"strings"

	"wfkit/internal/initialize/config"
	"wfkit/internal/initialize/utils"
)

func CreatePackageJSON(opts config.Options) error {
	data := struct {
		Name         string
		WFKitVersion string
	}{
		Name:         opts.Name,
		WFKitVersion: wfkitPackageVersion(opts.CLIValue),
	}

	templateName := "package.json.tmpl"

	return utils.RenderTemplateToFile(templateName, data, "package.json")
}

var releaseVersionPattern = regexp.MustCompile(`^\d+\.\d+\.\d+([-.][0-9A-Za-z.-]+)?$`)

func wfkitPackageVersion(version string) string {
	trimmed := strings.TrimSpace(strings.TrimPrefix(version, "v"))
	if trimmed == "" || !releaseVersionPattern.MatchString(trimmed) {
		return "latest"
	}
	return "^" + trimmed
}
