package main

import (
	stderrors "errors"
	"strings"

	"github.com/urfave/cli/v2"

	"wfkit/internal/globalconfig"
	desktopnotify "wfkit/internal/notify"
	"wfkit/internal/sound"
	"wfkit/internal/utils"
)

func notifySuccess(enabled bool, title, message string) {
	if !enabled {
		return
	}

	if err := desktopnotify.Success(title, message); err != nil && !stderrors.Is(err, desktopnotify.ErrUnsupported) {
		utils.CPrint("Failed to show desktop notification", "yellow")
	}

	if err := sound.PlaySuccessSound(); err != nil {
		utils.CPrint("Failed to play notification sound", "yellow")
	}
}

func notifyFailure(enabled bool, title, message string) {
	if !enabled {
		return
	}

	if err := desktopnotify.Failure(title, message); err != nil && !stderrors.Is(err, desktopnotify.ErrUnsupported) {
		utils.CPrint("Failed to show desktop notification", "yellow")
	}
}

func notificationsEnabled(args []string) bool {
	for _, arg := range args {
		if arg == "--notify" {
			return true
		}
	}

	conf, err := globalconfig.LoadConfig()
	if err != nil || conf == nil {
		return false
	}

	return conf.Notify
}

func resolveNotifyFlag(c *cli.Context) bool {
	if c.IsSet("notify") {
		return c.Bool("notify")
	}

	conf, err := globalconfig.LoadConfig()
	if err != nil || conf == nil {
		return c.Bool("notify")
	}

	return conf.Notify
}

func notificationBody(err error, fallback string) string {
	if err == nil {
		return strings.TrimSpace(fallback)
	}

	msg := strings.TrimSpace(err.Error())
	if msg == "" {
		return strings.TrimSpace(fallback)
	}

	lines := strings.Split(msg, "\n")
	return strings.TrimSpace(lines[0])
}

func errorHints(err error, version string, args []string) []string {
	reportHint := "Report a bug: " + bugReportURL(err, version, args)
	defaultHints := []string{
		"Run `wfkit doctor` to inspect local configuration and auth.",
		"Retry with `--dry-run` if you want to preview changes first.",
		reportHint,
	}

	if err == nil {
		return defaultHints
	}

	message := strings.ToLower(err.Error())
	if strings.Contains(message, "webflow designer session is outdated") {
		return []string{
			"Refresh your open Webflow Designer tab.",
			"Return to the terminal and rerun the same command.",
			reportHint,
		}
	}

	return defaultHints
}

func bugReportURL(err error, version string, args []string) string {
	title := ""
	if len(args) > 0 {
		title = "[Bug]: wfkit " + strings.Join(args, " ")
	} else if strings.TrimSpace(version) != "" || err != nil {
		title = "[Bug]: wfkit command failed"
	}
	return issueFormURL("bug_report.yml", title)
}
