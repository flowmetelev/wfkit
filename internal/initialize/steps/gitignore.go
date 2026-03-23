package steps

import (
	"wfkit/internal/initialize/utils"
)

func CreateGitignore() error {
	return utils.RenderTemplateToFile("gitignore.tmpl", nil, ".gitignore")
}
