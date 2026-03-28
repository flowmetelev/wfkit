package main

import (
	"wfkit/internal/version"

	"github.com/urfave/cli/v2"
)

func buildApp() *cli.App {
	return &cli.App{
		Name:    "wfkit",
		Usage:   "Webflow Publishing Tool",
		Version: version.Version,
		Action:  interactiveMode,
		Authors: []*cli.Author{
			{
				Name:  "Dmitry Metelev",
				Email: "mailmetelev@gmail.com",
			},
		},
		Commands:             buildCommands(),
		EnableBashCompletion: true,
		HideHelpCommand:      true,
		Before: func(c *cli.Context) error {
			return nil
		},
	}
}
