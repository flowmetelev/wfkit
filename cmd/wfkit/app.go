package main

import (
	"os"

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
		Commands: []*cli.Command{
			{
				Name:  "init",
				Usage: "Initialize a new Webflow project",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "name", Value: "my-project", Usage: "Project name"},
					&cli.StringFlag{Name: "pages-dir", Value: "src/pages", Usage: "Directory for pages"},
					&cli.StringFlag{Name: "global-entry", Value: "src/global/index.ts", Usage: "Global entry file"},
					&cli.StringFlag{Name: "global-var", Value: "WF", Usage: "Global variable name"},
					&cli.BoolFlag{Name: "init-git", Usage: "Initialize a local git repository inside the project"},
					&cli.BoolFlag{Name: "skip-install", Usage: "Skip dependency installation after generating the scaffold"},
					&cli.BoolFlag{Name: "force", Usage: "Allow writing scaffold files into an existing non-empty directory"},
					&cli.StringFlag{Name: "package-manager", Value: "bun", Usage: "Package manager (npm, yarn, pnpm, bun)"},
				},
				Action: initMode,
			},
			{
				Name:  "docs",
				Usage: "Render markdown and publish it to a dedicated Webflow docs page",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "file", Value: "docs/index.md", Usage: "Markdown entry file for the docs hub"},
					&cli.StringFlag{Name: "page-slug", Value: "docs", Usage: "Target Webflow page slug"},
					&cli.StringFlag{Name: "selector", Value: "[data-wf-docs-root], main", Usage: "Selector used to mount the rendered docs content"},
					&cli.BoolFlag{Name: "dry-run", Usage: "Show what would be changed without updating Webflow"},
					&cli.BoolFlag{Name: "publish", Value: true, Usage: "Publish the site after updating the docs page"},
					&cli.BoolFlag{Name: "notify", Usage: "Show desktop notification and play a sound when finished"},
				},
				Action: docsMode,
			},
			{
				Name:  "migrate",
				Usage: "Migrate inline Webflow page scripts into local page files and publish them via jsDelivr",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "pages-dir", Value: "src/pages", Usage: "Directory for generated page entry files"},
					&cli.BoolFlag{Name: "force", Usage: "Overwrite existing page entry files when they already exist"},
					&cli.BoolFlag{Name: "dry-run", Usage: "Show what would be migrated without writing files, pushing git, or updating Webflow"},
					&cli.StringFlag{Name: "custom-commit", Value: "Migrate Webflow page code via wfkit", Usage: "Custom commit message"},
					&cli.StringFlag{Name: "branch", Value: "main", Usage: "Git branch for CDN URLs"},
					&cli.StringFlag{Name: "build-dir", Value: "dist/assets", Usage: "Build directory"},
					&cli.BoolFlag{Name: "notify", Usage: "Show desktop notification and play a sound when finished"},
				},
				Action: migrateMode,
			},
			{
				Name:  "publish",
				Usage: "Publish code to Webflow",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "env", Value: "prod", Usage: "Environment mode: dev or prod"},
					&cli.BoolFlag{Name: "by-page", Usage: "Publish code for each page individually"},
					&cli.BoolFlag{Name: "dry-run", Usage: "Build and show what would change without pushing or updating Webflow"},
					&cli.StringFlag{Name: "script-url", Usage: "Custom script URL (overrides auto-generation)"},
					&cli.IntFlag{Name: "dev-port", Value: 5173, Usage: "Local dev server port (dev mode)"},
					&cli.StringFlag{Name: "dev-host", Value: "localhost", Usage: "Local dev server host (dev mode)"},
					&cli.StringFlag{Name: "custom-commit", Value: "Auto publish from wfkit tool", Usage: "Custom commit message"},
					&cli.StringFlag{Name: "branch", Value: "main", Usage: "Git branch for CDN URLs"},
					&cli.StringFlag{Name: "build-dir", Value: "dist/assets", Usage: "Build directory"},
					&cli.BoolFlag{Name: "notify", Usage: "Show desktop notification and play a sound when finished"},
					&cli.BoolFlag{Name: "update", Usage: "Check for updates before publishing"},
				},
				Action: publishMode,
			},
			{
				Name:  "proxy",
				Usage: "Proxy the published .webflow.io site and inject local dev scripts",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "site-url", Usage: "Published .webflow.io site URL (defaults to https://<name>.webflow.io)"},
					&cli.StringFlag{Name: "host", Usage: "Shared public host for proxy and injected dev URLs"},
					&cli.StringFlag{Name: "script-url", Usage: "Custom local script URL (overrides auto-generation)"},
					&cli.IntFlag{Name: "dev-port", Value: 5173, Usage: "Local Vite dev server port"},
					&cli.StringFlag{Name: "dev-host", Value: "localhost", Usage: "Local Vite dev server host"},
					&cli.IntFlag{Name: "proxy-port", Value: 3000, Usage: "Local proxy port"},
					&cli.StringFlag{Name: "proxy-host", Value: "localhost", Usage: "Local proxy host"},
					&cli.BoolFlag{Name: "open", Value: true, Usage: "Open the proxied site in your browser automatically"},
				},
				Action: proxyMode,
			},
			{
				Name:   "report-bug",
				Usage:  "Open the GitHub bug report form",
				Action: openBugReport,
			},
			{
				Name:   "request-feature",
				Usage:  "Open the GitHub feature request form",
				Action: openFeatureRequest,
			},
			{
				Name:  "doctor",
				Usage: "Check local configuration, tools, auth, and proxy readiness",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "skip-auth", Usage: "Skip the Webflow authentication check"},
				},
				Action: doctorMode,
			},
			{
				Name:  "update",
				Usage: "Check for and install updates",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "auto-update", Usage: "Automatically install updates if available"},
				},
				Action: updateMode,
			},
		},
		EnableBashCompletion: true,
		HideHelpCommand:      true,
		Before: func(c *cli.Context) error {
			return nil
		},
	}
}

func appArgs() []string {
	return os.Args
}
