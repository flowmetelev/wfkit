// internal/initialize/init.go
package initialize

import (
	"fmt"
	"os"
	"path/filepath"

	"wfkit/internal/initialize/config"
	"wfkit/internal/initialize/steps"
	"wfkit/internal/utils"
)

func InitProject(opts config.Options) error {
	return initProject(opts, steps.InstallDependencies, steps.InitializeGitRepository)
}

func initProject(opts config.Options, installDependencies func(string) error, initializeGitRepository func() error) error {
	// Установка значений по умолчанию
	opts.SetDefaultValues()

	restoreDir, err := prepareProjectDir(opts.Name)
	if err != nil {
		return err
	}
	defer func() {
		if restoreErr := restoreDir(); restoreErr != nil {
			utils.CPrint(fmt.Sprintf("Warning: failed to restore working directory: %v", restoreErr), "yellow")
		}
	}()

	// Создание директорий
	if err := steps.CreateDirectories(opts.PagesDir); err != nil {
		return err
	}

	// Создание package.json
	if err := steps.CreatePackageJSON(opts); err != nil {
		return err
	}

	// Создание wfkit.json
	if err := steps.CreateProjectConfig(opts); err != nil {
		return err
	}

	// Создание .gitignore
	if err := steps.CreateGitignore(); err != nil {
		return err
	}

	// Создание vite.config.js
	if err := steps.CreateViteConfig(opts); err != nil {
		return err
	}

	// Создание конфигурационных файлов для линтинга и форматирования
	if err := steps.CreateToolingConfigs(); err != nil {
		return err
	}

	// Создание глобальных файлов
	if err := steps.CreateGlobalFiles(opts); err != nil {
		return err
	}

	// Создание страниц
	if err := steps.CreatePageFiles(opts.PagesDir); err != nil {
		return err
	}

	// Установка зависимостей
	if err := installDependencies(opts.PackageManager); err != nil {
		return err
	}

	if opts.InitGit {
		if err := initializeGitRepository(); err != nil {
			return err
		}
	}

	utils.CPrint("Project initialized successfully! Update wfkit.json with your real Webflow and GitHub settings.", "green")
	return nil
}

func prepareProjectDir(name string) (func() error, error) {
	projectDir := filepath.Clean(name)
	if projectDir == "" || projectDir == "." {
		return nil, fmt.Errorf("project name must not be empty")
	}

	originalDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}

	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create project directory %s: %w", projectDir, err)
	}

	if err := os.Chdir(projectDir); err != nil {
		return nil, fmt.Errorf("failed to enter project directory %s: %w", projectDir, err)
	}

	return func() error {
		return os.Chdir(originalDir)
	}, nil
}
