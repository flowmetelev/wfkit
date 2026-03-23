package steps

import (
	"wfkit/internal/initialize/utils"
)

// CreateToolingConfigs создает файлы конфигурации для линтеров, форматтеров и редакторов.
func CreateToolingConfigs() error {
	configs := map[string]string{
		"tsconfig.json.tmpl":  "tsconfig.json",
		"prettierrc.tmpl":     ".prettierrc",
		"prettierignore.tmpl": ".prettierignore",
		"eslintrc.tmpl":       ".eslintrc.json",
		"editorconfig.tmpl":   ".editorconfig",
	}

	for tmpl, out := range configs {
		if err := utils.RenderTemplateToFile(tmpl, nil, out); err != nil {
			return err
		}
	}

	return nil
}
