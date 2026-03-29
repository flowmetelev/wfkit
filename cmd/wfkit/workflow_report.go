package main

import (
	"fmt"

	"wfkit/internal/utils"
)

func printPublishTimeline(env, delivery string, byPage, dryRun, authed, built, pushed, published bool) {
	mode := "global"
	if byPage {
		mode = "by-page"
	}

	buildLabel := "Build"
	if env == "dev" {
		buildLabel = "Dev server"
	}

	pushLabel := "Git"
	publishLabel := "Webflow"
	pushSkipped := dryRun || env != "prod" || delivery == "inline"
	if pushSkipped {
		pushLabel = "Git"
	}
	if dryRun {
		publishLabel = "Webflow"
	}

	utils.PrintTimeline("Workflow", []utils.TimelineStep{
		{Label: "Mode", Status: "READY", Details: fmt.Sprintf("%s %s", env, mode)},
		{Label: "Auth", Status: timelineStatus(authed, false), Details: timelineDetails(authed, "session ready")},
		{Label: buildLabel, Status: timelineStatus(built, false), Details: timelineDetails(built, publishBuildDetails(env))},
		{Label: pushLabel, Status: timelineStatus(pushed, pushSkipped), Details: timelineDetails(pushed, "assets synced")},
		{Label: publishLabel, Status: timelineStatus(published, dryRun), Details: timelineDetails(published, "site updated")},
	}...)
}

func printMigrateTimeline(dryRun, willBuild, willPush, willPublish, authed, loadedPages, loadedGlobal, plannedFiles, built, pushed, published bool) {
	buildSkipped := dryRun || !willBuild
	pushSkipped := dryRun || !willPush
	publishSkipped := dryRun || !willPublish

	utils.PrintTimeline("Workflow", []utils.TimelineStep{
		{Label: "Auth", Status: timelineStatus(authed, false), Details: timelineDetails(authed, "session ready")},
		{Label: "Pages", Status: timelineStatus(loadedPages, false), Details: timelineDetails(loadedPages, "loaded")},
		{Label: "Global", Status: timelineStatus(loadedGlobal, false), Details: timelineDetails(loadedGlobal, "loaded")},
		{Label: "Files", Status: timelineStatus(plannedFiles, dryRun), Details: timelineDetails(plannedFiles, "written")},
		{Label: "Build", Status: timelineStatus(built, buildSkipped), Details: timelineDetails(built, "bundles ready")},
		{Label: "Git", Status: timelineStatus(pushed, pushSkipped), Details: timelineDetails(pushed, "assets synced")},
		{Label: "Webflow", Status: timelineStatus(published, publishSkipped), Details: timelineDetails(published, "links updated")},
	}...)
}

func publishBuildDetails(env string) string {
	if env == "dev" {
		return "proxy ready"
	}
	return "assets ready"
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
