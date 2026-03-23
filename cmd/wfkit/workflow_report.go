package main

import (
	"fmt"

	"wfkit/internal/utils"
)

func printPublishTimeline(env string, byPage, dryRun, authed, built, pushed, published bool) {
	mode := "global"
	if byPage {
		mode = "by-page"
	}

	buildLabel := "Build assets"
	if env == "dev" {
		buildLabel = "Start dev server"
	}

	pushLabel := "Push to GitHub"
	publishLabel := "Update Webflow"
	if dryRun {
		pushLabel = "Push to GitHub (skipped)"
		publishLabel = "Update Webflow (skipped)"
	}

	utils.PrintTimeline("Workflow", []utils.TimelineStep{
		{Label: "Mode", Status: "READY", Details: fmt.Sprintf("%s %s", env, mode)},
		{Label: "Authenticate with Webflow", Status: timelineStatus(authed, false), Details: timelineDetails(authed, "session ready")},
		{Label: buildLabel, Status: timelineStatus(built, false), Details: timelineDetails(built, "assets ready")},
		{Label: pushLabel, Status: timelineStatus(pushed, dryRun), Details: timelineDetails(pushed, "git synced")},
		{Label: publishLabel, Status: timelineStatus(published, dryRun), Details: timelineDetails(published, "webflow updated")},
	}...)
}

func printMigrateTimeline(dryRun, authed, loadedPages, loadedGlobal, wroteFiles, built, pushed, published bool) {
	utils.PrintTimeline("Workflow", []utils.TimelineStep{
		{Label: "Authenticate with Webflow", Status: timelineStatus(authed, false), Details: timelineDetails(authed, "session ready")},
		{Label: "Load pages", Status: timelineStatus(loadedPages, false), Details: timelineDetails(loadedPages, "page metadata ready")},
		{Label: "Load global code", Status: timelineStatus(loadedGlobal, false), Details: timelineDetails(loadedGlobal, "global custom code ready")},
		{Label: "Write migration files", Status: timelineStatus(wroteFiles, dryRun), Details: timelineDetails(wroteFiles, "local entries generated")},
		{Label: "Build assets", Status: timelineStatus(built, dryRun), Details: timelineDetails(built, "manifest ready")},
		{Label: "Push to GitHub", Status: timelineStatus(pushed, dryRun), Details: timelineDetails(pushed, "git synced")},
		{Label: "Update Webflow", Status: timelineStatus(published, dryRun), Details: timelineDetails(published, "cdn links published")},
	}...)
}

func timelineStatus(done, skipped bool) string {
	switch {
	case skipped:
		return "SKIP"
	case done:
		return "OK"
	default:
		return "READY"
	}
}

func timelineDetails(done bool, details string) string {
	if done {
		return details
	}
	return ""
}
