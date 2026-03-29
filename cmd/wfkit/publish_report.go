package main

import (
	"fmt"

	"wfkit/internal/publish"
	"wfkit/internal/utils"
)

func printGlobalPublishPlan(plan publish.GlobalPublishPlan) {
	statusText := "UP_TO_DATE"
	if plan.Action == "update" {
		statusText = "UPDATE"
	}

	utils.PrintSection("Global Publish")
	utils.PrintStatus(statusText, "Bundle", fmt.Sprintf("current %s  next %s", displayValue(plan.CurrentSrc), displayValue(plan.NextSrc)))
	fmt.Println()
}

func printByPagePlan(plan publish.ByPagePlan) {
	if plan.Global.NextSrc != "" {
		printGlobalPublishPlan(plan.Global)
	}

	utils.PrintSection("Page Publish")
	updateCount := 0
	warnCount := 0

	for _, page := range plan.Pages {
		switch page.Action {
		case "update":
			updateCount++
			utils.PrintStatus("UPDATE", displayValue(page.Title), page.NextSrc)
		case "up_to_date":
			utils.PrintStatus("OK", displayValue(page.Title), "")
		default:
			warnCount++
			utils.PrintStatus("SKIP", displayValue(page.Title), page.Message)
		}
	}

	utils.PrintSummary(
		utils.SummaryMetric{Label: "Updates", Value: fmt.Sprintf("%d", updateCount), Tone: "warning"},
		utils.SummaryMetric{Label: "Warnings", Value: fmt.Sprintf("%d", warnCount), Tone: "info"},
	)
	fmt.Println()
}

func displayValue(value string) string {
	if value == "" {
		return "-"
	}
	return value
}
