package main

import (
	"fmt"

	"wfkit/internal/globalconfig"
	"wfkit/internal/utils"

	"github.com/charmbracelet/huh"
)

type configFlow struct {
	config *globalconfig.Config
}

func newConfigFlow() *configFlow {
	return &configFlow{
		config: &globalconfig.Config{PackageManager: "bun"},
	}
}

func (f *configFlow) run() error {
	if err := f.load(); err != nil {
		return err
	}

	if err := f.collectInput(); err != nil {
		return err
	}

	if err := globalconfig.SaveConfig(f.config); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	utils.CPrint("Global configuration saved successfully!", "green")
	return nil
}

func (f *configFlow) load() error {
	conf, err := globalconfig.LoadConfig()
	if err != nil {
		return nil
	}

	f.config = conf
	return nil
}

func (f *configFlow) collectInput() error {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Default GitHub Username").
				Description("Used for CDN links (e.g. yndmitry)").
				Value(&f.config.GitHubUser),
			huh.NewSelect[string]().
				Title("Default Package Manager").
				Options(packageManagerSelectOptions()...).
				Value(&f.config.PackageManager),
			huh.NewConfirm().
				Title("Desktop notifications by default?").
				Description("Used by `publish --notify` and `migrate --notify` when you don't pass the flag explicitly.").
				Value(&f.config.Notify),
		),
	).Run()
}
