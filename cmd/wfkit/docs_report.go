package main

import (
	"fmt"

	"wfkit/internal/publish"
	"wfkit/internal/utils"
)

func printDocsTimeline(authed, planned, applied bool) {
	utils.PrintTimeline(
		"Docs Timeline",
		utils.TimelineStep{Label: "Authenticate", Status: timelineStatus(authed, false), Details: timelineDetails(authed, "Webflow session ready")},
		utils.TimelineStep{Label: "Plan docs hub", Status: timelineStatus(planned, false), Details: timelineDetails(planned, "markdown rendered and target page resolved")},
		utils.TimelineStep{Label: "Apply docs hub", Status: timelineStatus(applied, false), Details: timelineDetails(applied, "managed docs block updated")},
	)
}

func printDocsPlan(plan publish.DocsHubPlan) {
	utils.PrintSection("Docs Hub Plan")
	utils.PrintStatus(docsStatus(plan.Action), plan.PageSlug, plan.Message)
	utils.PrintKeyValue("Markdown", plan.EntryPath)
	utils.PrintKeyValue("Target page", displayValue(plan.PageTitle))
	utils.PrintKeyValue("Selector", plan.Selector)
	utils.PrintKeyValue("Title", plan.MarkdownTitle)
	fmt.Println()
}

func printDocsResult(result publish.DocsHubResult) {
	utils.PrintSection("Docs Result")
	utils.PrintSummary(
		utils.SummaryMetric{Label: "Updated", Value: map[bool]string{true: "yes", false: "no"}[result.Updated], Tone: "success"},
		utils.SummaryMetric{Label: "Published", Value: map[bool]string{true: "yes", false: "no"}[result.Published], Tone: "info"},
	)
	fmt.Println()
}

func docsStatus(action string) string {
	switch action {
	case "update":
		return "UPDATE"
	case "up_to_date":
		return "OK"
	default:
		return "SKIP"
	}
}
