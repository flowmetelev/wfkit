package main

import (
	"fmt"

	"wfkit/internal/publish"
	"wfkit/internal/utils"
)

func printMigrationPlan(plan publish.MigrationPlan) {
	utils.PrintSection("Migration Plan")

	migrateCount := 0
	skipCount := 0

	switch plan.Global.Action {
	case "write", "overwrite":
		migrateCount++
		utils.PrintStatus("MIGRATE", "global", fmt.Sprintf("%s (%d script block(s))", plan.Global.ModuleRelativePath, len(plan.Global.HeadScripts)+len(plan.Global.PostBodyScripts)))
	default:
		if plan.Global.Message != "" {
			skipCount++
			utils.PrintStatus("SKIP", "global", plan.Global.Message)
		}
	}

	for _, page := range plan.Pages {
		switch page.Action {
		case "write", "overwrite":
			migrateCount++
			utils.PrintStatus("MIGRATE", displayValue(page.Title), fmt.Sprintf("%s (%d script block(s))", page.RelativePath, len(page.Scripts)))
		default:
			skipCount++
			utils.PrintStatus("SKIP", displayValue(page.Title), page.Message)
		}
	}

	utils.PrintSummary(
		utils.SummaryMetric{Label: "To migrate", Value: fmt.Sprintf("%d", migrateCount), Tone: "warning"},
		utils.SummaryMetric{Label: "Skipped", Value: fmt.Sprintf("%d", skipCount), Tone: "info"},
	)
	fmt.Println()
}

func printMigrationPublishResult(result publish.MigrationPublishResult) {
	if result.GlobalPlan.NextSrc != "" {
		printGlobalPublishPlan(result.GlobalPlan)
	}

	utils.PrintSection("Migration Result")
	utils.PrintStatus("OK", "Migrated pages published", fmt.Sprintf("%d", result.UpdatedPages))
	if result.Published {
		utils.PrintStatus("OK", "Webflow", "Pages were updated and published")
	} else {
		utils.PrintStatus("SKIP", "Webflow", "No page changes were required after migration")
	}
	fmt.Println()
}

func hasPendingMigrations(plan publish.MigrationPlan) bool {
	if plan.Global.Action == "write" || plan.Global.Action == "overwrite" {
		return true
	}
	for _, page := range plan.Pages {
		if page.Action == "write" || page.Action == "overwrite" {
			return true
		}
	}
	return false
}

func countPageMigrations(plan publish.MigrationPlan) int {
	count := 0
	for _, page := range plan.Pages {
		if page.Action == "write" || page.Action == "overwrite" {
			count++
		}
	}
	return count
}
