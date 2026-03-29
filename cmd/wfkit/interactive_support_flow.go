package main

import (
	"fmt"
	"os"

	"wfkit/internal/utils"

	"github.com/charmbracelet/huh"
	"github.com/urfave/cli/v2"
)

type interactiveSupportFlow struct {
	parent *cli.Context
	action string
}

func newInteractiveSupportFlow(parent *cli.Context, action string) *interactiveSupportFlow {
	return &interactiveSupportFlow{parent: parent, action: action}
}

func (f *interactiveSupportFlow) run() error {
	for {
		title, description, targetURL := f.metadata()
		utils.PrintSection(title)
		utils.PrintKeyValue("Host", issueFormBaseURL)
		fmt.Println("URL:")
		fmt.Println(targetURL)
		fmt.Println(description)
		fmt.Println()

		var next string
		if err := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title(title).
					Description("Open the GitHub form in your browser or go back.").
					Options(
						huh.NewOption("Open in browser", "open"),
						huh.NewOption("Back to main menu", "back"),
					).
					Value(&next),
			),
		).Run(); err != nil {
			return err
		}

		switch next {
		case "open":
			if err := openURL(targetURL); err != nil {
				return fmt.Errorf("failed to open %s: %w", targetURL, err)
			}
			utils.PrintStatus("OK", "Opened", targetURL)
			fmt.Println()
			return nil
		case "back":
			return nil
		}
	}
}

func (f *interactiveSupportFlow) metadata() (title, description, targetURL string) {
	switch f.action {
	case "request_feature":
		return "Request a feature", "Open the GitHub feature request form with the wfkit template preselected.", featureRequestURL()
	case "report_bug":
		return "Report a bug", "Open the GitHub bug report form with the wfkit template preselected.", bugReportURL(nil, f.parent.App.Version, os.Args[1:])
	default:
		return "Support", "Open the wfkit GitHub support flow.", issueFormBaseURL
	}
}
