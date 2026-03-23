package utils

import (
	"embed"
	"os"
	"text/template"
)

//go:embed templates/*
var FS embed.FS

// RenderTemplateToFile читает шаблон из embed.FS, компилирует его и записывает результат в файл.
func RenderTemplateToFile(templateName string, data interface{}, outputPath string) error {
	tmplData, err := FS.ReadFile("templates/" + templateName)
	if err != nil {
		return err
	}

	tmpl, err := template.New(templateName).Parse(string(tmplData))
	if err != nil {
		return err
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	return tmpl.Execute(file, data)
}
