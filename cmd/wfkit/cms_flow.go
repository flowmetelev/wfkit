package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"wfkit/internal/config"
	"wfkit/internal/utils"
	"wfkit/internal/webflow"

	"github.com/urfave/cli/v2"
)

type cmsFlow struct {
	cliContext  *cli.Context
	config      config.Config
	baseURL     string
	token       string
	cookies     string
	snapshot    webflow.CMSSnapshot
	collections []webflow.CMSCollection
}

type cmsCollectionSummary struct {
	ID        string `json:"id"`
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	ItemCount int    `json:"itemCount"`
}

type cmsPullSummary struct {
	OutputDir    string                     `json:"outputDir"`
	DatabaseID   string                     `json:"databaseId,omitempty"`
	DatabaseName string                     `json:"databaseName,omitempty"`
	Collections  []cmsPulledCollectionStats `json:"collections"`
}

type cmsPulledCollectionStats struct {
	ID         string `json:"id"`
	Slug       string `json:"slug"`
	Name       string `json:"name"`
	ItemCount  int    `json:"itemCount"`
	SchemaPath string `json:"schemaPath"`
	ItemsDir   string `json:"itemsDir"`
}

func newCMSFlow(c *cli.Context) *cmsFlow {
	return &cmsFlow{cliContext: c}
}

func (f *cmsFlow) runCollections() error {
	if err := f.loadConfig(); err != nil {
		return err
	}
	f.printHeader("CMS Collections")
	if err := f.authenticate(); err != nil {
		return err
	}
	if err := f.loadCollections(); err != nil {
		return err
	}

	if f.cliContext.Bool("json") {
		return printJSON(cmsCollectionSummaries(f.collections))
	}

	utils.PrintSection("Collections")
	if len(f.collections) == 0 {
		utils.PrintStatus("INFO", "No collections", "This Webflow site does not have CMS collections yet.")
		fmt.Println()
		return nil
	}

	for _, collection := range sortCMSCollections(f.collections) {
		utils.PrintStatus("INFO", displayValue(cmsCollectionSlug(collection)), displayValue(collection.Name))
		utils.PrintKeyValue("Collection ID", collection.ID)
		utils.PrintKeyValue("Items", fmt.Sprintf("%d", collection.TotalNumberOfItems))
		fmt.Println()
	}
	utils.PrintSummary(
		utils.SummaryMetric{Label: "Collections", Value: fmt.Sprintf("%d", len(f.collections)), Tone: "info"},
		utils.SummaryMetric{Label: "Database", Value: displayValue(cmsDatabaseName(f.snapshot.Database)), Tone: "info"},
	)
	return nil
}

func (f *cmsFlow) runPull() error {
	if err := f.loadConfig(); err != nil {
		return err
	}
	f.printHeader("CMS Pull")
	if err := f.authenticate(); err != nil {
		return err
	}
	if err := f.loadCollections(); err != nil {
		return err
	}

	selected, err := f.selectCollections()
	if err != nil {
		return err
	}
	if len(selected) == 0 {
		if f.cliContext.Bool("json") {
			return printJSON(cmsPullSummary{OutputDir: f.outputDir(), Collections: []cmsPulledCollectionStats{}})
		}
		utils.PrintStatus("INFO", "No collections", "Nothing to pull from Webflow.")
		fmt.Println()
		return nil
	}

	summary, err := f.writePullSnapshot(selected)
	if err != nil {
		return err
	}
	if f.cliContext.Bool("json") {
		return printJSON(summary)
	}

	utils.PrintSection("Pulled Collections")
	for _, collection := range summary.Collections {
		utils.PrintStatus("OK", displayValue(collection.Slug), displayValue(collection.Name))
		utils.PrintKeyValue("Items", fmt.Sprintf("%d", collection.ItemCount))
		utils.PrintKeyValue("Schema", collection.SchemaPath)
		utils.PrintKeyValue("Items dir", collection.ItemsDir)
		fmt.Println()
	}
	utils.PrintSuccessScreen(
		"CMS pulled",
		"The current Webflow CMS snapshot was written to local JSON files.",
		[]utils.SummaryMetric{
			{Label: "Collections", Value: fmt.Sprintf("%d", len(summary.Collections)), Tone: "success"},
			{Label: "Output", Value: summary.OutputDir, Tone: "info"},
		},
		"wfkit cms collections",
		fmt.Sprintf("wfkit cms pull --dir %s", summary.OutputDir),
	)
	return nil
}

func (f *cmsFlow) runDiff() error {
	if err := f.loadConfig(); err != nil {
		return err
	}
	f.printHeader("CMS Diff")
	if err := f.authenticate(); err != nil {
		return err
	}
	if err := f.loadCollections(); err != nil {
		return err
	}

	selected, err := f.selectCollections()
	if err != nil {
		return err
	}
	if len(selected) == 0 {
		if f.cliContext.Bool("json") {
			return printJSON(cmsDiffSummary{OutputDir: f.outputDir(), Target: strings.TrimSpace(f.cliContext.String("target")), Collections: []cmsCollectionDiffSummary{}})
		}
		utils.PrintStatus("INFO", "No collections", "Nothing to diff against Webflow.")
		fmt.Println()
		return nil
	}

	summary, _, err := f.buildSyncSummary(selected, f.cliContext.Bool("delete-missing"))
	if err != nil {
		return err
	}
	if f.cliContext.Bool("json") {
		return printJSON(summary)
	}
	f.printDiffSummary(summary)
	return nil
}

func (f *cmsFlow) runPush() error {
	if err := f.loadConfig(); err != nil {
		return err
	}
	f.printHeader("CMS Push")
	if err := f.authenticate(); err != nil {
		return err
	}
	if err := f.loadCollections(); err != nil {
		return err
	}

	selected, err := f.selectCollections()
	if err != nil {
		return err
	}
	if len(selected) == 0 {
		if f.cliContext.Bool("json") {
			return printJSON(cmsDiffSummary{OutputDir: f.outputDir(), Target: strings.TrimSpace(f.cliContext.String("target")), Collections: []cmsCollectionDiffSummary{}})
		}
		utils.PrintStatus("INFO", "No collections", "Nothing to push into Webflow.")
		fmt.Println()
		return nil
	}

	summary, plans, err := f.buildSyncSummary(selected, f.cliContext.Bool("delete-missing"))
	if err != nil {
		return err
	}
	if summary.Created == 0 && summary.Updated == 0 && summary.Deleted == 0 {
		if f.cliContext.Bool("json") {
			return printJSON(summary)
		}
		utils.PrintStatus("OK", "CMS already in sync", "No local JSON changes need to be pushed.")
		fmt.Println()
		return nil
	}

	if err := utils.RunTask("Push local CMS JSON to Webflow", func() error {
		return f.applySyncPlans(plans)
	}); err != nil {
		return err
	}

	if f.cliContext.Bool("json") {
		return printJSON(summary)
	}
	f.printDiffSummary(summary)
	utils.PrintSuccessScreen(
		"CMS synced",
		"Local CMS JSON changes were applied back to Webflow.",
		[]utils.SummaryMetric{
			{Label: "Created", Value: fmt.Sprintf("%d", summary.Created), Tone: "success"},
			{Label: "Updated", Value: fmt.Sprintf("%d", summary.Updated), Tone: "info"},
			{Label: "Deleted", Value: fmt.Sprintf("%d", summary.Deleted), Tone: "warn"},
		},
		"wfkit cms diff",
		"wfkit cms pull",
	)
	return nil
}

func (f *cmsFlow) loadConfig() error {
	cfg, err := config.ReadConfig()
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}
	if cfg.AppName == "" {
		return fmt.Errorf("missing appName configuration in wfkit.json")
	}
	f.config = cfg
	f.baseURL = cfg.EffectiveDesignURL()
	return nil
}

func (f *cmsFlow) printHeader(title string) {
	utils.PrintSection(title)
	utils.PrintKeyValue("Webflow", f.baseURL)
	fmt.Println()
}

func (f *cmsFlow) authenticate() error {
	return utils.RunTask("Authenticate with Webflow", func() error {
		token, cookies, err := webflow.GetCsrfTokenAndCookies(f.cliContext.Context, f.baseURL)
		if err != nil {
			return fmt.Errorf("failed to authenticate with Webflow: %w", err)
		}
		f.token = token
		f.cookies = cookies
		return nil
	})
}

func (f *cmsFlow) loadCollections() error {
	return utils.RunTask("Load CMS collections from Webflow", func() error {
		snapshot, collections, err := webflow.ListCMSCollections(f.cliContext.Context, f.config.AppName, f.token, f.cookies)
		if err != nil {
			return fmt.Errorf("failed to load CMS collections: %w", err)
		}
		f.snapshot = snapshot
		f.collections = collections
		return nil
	})
}

func (f *cmsFlow) selectCollections() ([]webflow.CMSCollection, error) {
	selector := strings.TrimSpace(f.cliContext.String("collection"))
	if selector == "" {
		return sortCMSCollections(f.collections), nil
	}

	selector = normalizePageSlug(selector)
	var selected []webflow.CMSCollection
	for _, collection := range f.collections {
		if cmsCollectionSlug(collection) == selector || normalizePageSlug(collection.Name) == selector || strings.TrimSpace(collection.ID) == selector {
			selected = append(selected, collection)
		}
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("no CMS collection found for %q", selector)
	}
	return sortCMSCollections(selected), nil
}

func (f *cmsFlow) outputDir() string {
	output := strings.TrimSpace(f.cliContext.String("dir"))
	if output == "" {
		output = "webflow/cms"
	}
	return output
}

func (f *cmsFlow) writePullSnapshot(collections []webflow.CMSCollection) (cmsPullSummary, error) {
	outputDir := f.outputDir()
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return cmsPullSummary{}, fmt.Errorf("failed to create %s: %w", outputDir, err)
	}

	metadata := map[string]interface{}{
		"siteName":       f.snapshot.SiteName,
		"pulledAt":       time.Now().UTC().Format(time.RFC3339),
		"maxCollections": f.snapshot.MaxCollections,
		"maxLocales":     f.snapshot.MaxLocales,
	}
	if f.snapshot.Database != nil {
		metadata["databaseId"] = f.snapshot.Database.ID
		metadata["databaseName"] = f.snapshot.Database.Name
		metadata["databaseDescription"] = f.snapshot.Database.Description
	}
	if err := writeJSONFile(filepath.Join(outputDir, "database.json"), metadata); err != nil {
		return cmsPullSummary{}, err
	}

	summary := cmsPullSummary{
		OutputDir: outputDir,
	}
	if f.snapshot.Database != nil {
		summary.DatabaseID = f.snapshot.Database.ID
		summary.DatabaseName = f.snapshot.Database.Name
	}

	for _, collection := range collections {
		collectionDir := filepath.Join(outputDir, cmsCollectionSlug(collection))
		itemsDir := filepath.Join(collectionDir, "items")
		if err := os.MkdirAll(collectionDir, 0o755); err != nil {
			return cmsPullSummary{}, fmt.Errorf("failed to create %s: %w", collectionDir, err)
		}
		if err := os.RemoveAll(itemsDir); err != nil {
			return cmsPullSummary{}, fmt.Errorf("failed to clean %s: %w", itemsDir, err)
		}
		if err := os.MkdirAll(itemsDir, 0o755); err != nil {
			return cmsPullSummary{}, fmt.Errorf("failed to create %s: %w", itemsDir, err)
		}

		if err := writeJSONFile(filepath.Join(collectionDir, "schema.json"), collection); err != nil {
			return cmsPullSummary{}, err
		}

		items, err := webflow.GetCMSCollectionItems(f.cliContext.Context, f.config.AppName, collection.ID, f.cliContext.String("target"), f.token, f.cookies)
		if err != nil {
			return cmsPullSummary{}, err
		}

		for _, item := range items.Items {
			itemPath := filepath.Join(itemsDir, cmsItemFileStem(item)+".json")
			if err := writeJSONFile(itemPath, item); err != nil {
				return cmsPullSummary{}, err
			}
		}

		summary.Collections = append(summary.Collections, cmsPulledCollectionStats{
			ID:         collection.ID,
			Slug:       cmsCollectionSlug(collection),
			Name:       collection.Name,
			ItemCount:  len(items.Items),
			SchemaPath: filepath.Join(collectionDir, "schema.json"),
			ItemsDir:   itemsDir,
		})
	}

	return summary, nil
}

func (f *cmsFlow) applySyncPlans(plans []cmsCollectionSyncPlan) error {
	for _, plan := range plans {
		for _, item := range plan.Create {
			if _, err := webflow.CreateCMSItem(f.cliContext.Context, f.config.AppName, plan.Collection.ID, f.token, f.cookies, item.Fields); err != nil {
				return fmt.Errorf("failed to create CMS item %s in %s: %w", displayValue(item.Slug), cmsCollectionSlug(plan.Collection), err)
			}
		}
		for _, update := range plan.Update {
			if _, err := webflow.UpdateCMSItem(f.cliContext.Context, f.config.AppName, plan.Collection.ID, cmsRemoteItemID(update.Remote), f.token, f.cookies, update.Local.Fields); err != nil {
				return fmt.Errorf("failed to update CMS item %s in %s: %w", displayValue(update.Local.Slug), cmsCollectionSlug(plan.Collection), err)
			}
		}
		for _, item := range plan.Delete {
			if err := webflow.DeleteCMSItem(f.cliContext.Context, f.config.AppName, plan.Collection.ID, cmsRemoteItemID(item), f.token, f.cookies); err != nil {
				return fmt.Errorf("failed to delete CMS item %s in %s: %w", displayValue(cmsRemoteItemSlug(item)), cmsCollectionSlug(plan.Collection), err)
			}
		}
	}
	return nil
}

func (f *cmsFlow) printDiffSummary(summary cmsDiffSummary) {
	utils.PrintSection("CMS Sync Plan")
	for _, collection := range summary.Collections {
		utils.PrintStatus("INFO", displayValue(collection.Slug), displayValue(collection.Name))
		utils.PrintKeyValue("Local items", fmt.Sprintf("%d", collection.LocalCount))
		utils.PrintKeyValue("Remote items", fmt.Sprintf("%d", collection.RemoteCount))
		utils.PrintKeyValue("Create", fmt.Sprintf("%d", collection.Created))
		utils.PrintKeyValue("Update", fmt.Sprintf("%d", collection.Updated))
		utils.PrintKeyValue("Delete", fmt.Sprintf("%d", collection.Deleted))
		utils.PrintKeyValue("Unchanged", fmt.Sprintf("%d", collection.Unchanged))
		fmt.Println()
	}
	utils.PrintSummary(
		utils.SummaryMetric{Label: "Created", Value: fmt.Sprintf("%d", summary.Created), Tone: "success"},
		utils.SummaryMetric{Label: "Updated", Value: fmt.Sprintf("%d", summary.Updated), Tone: "info"},
		utils.SummaryMetric{Label: "Deleted", Value: fmt.Sprintf("%d", summary.Deleted), Tone: "warn"},
		utils.SummaryMetric{Label: "Unchanged", Value: fmt.Sprintf("%d", summary.Unchanged), Tone: "info"},
	)
}

func cmsCollectionSummaries(collections []webflow.CMSCollection) []cmsCollectionSummary {
	collections = sortCMSCollections(collections)
	summaries := make([]cmsCollectionSummary, 0, len(collections))
	for _, collection := range collections {
		summaries = append(summaries, cmsCollectionSummary{
			ID:        collection.ID,
			Slug:      cmsCollectionSlug(collection),
			Name:      collection.Name,
			ItemCount: collection.TotalNumberOfItems,
		})
	}
	return summaries
}

func sortCMSCollections(collections []webflow.CMSCollection) []webflow.CMSCollection {
	sorted := append([]webflow.CMSCollection(nil), collections...)
	sort.Slice(sorted, func(i, j int) bool {
		left := cmsCollectionSlug(sorted[i])
		right := cmsCollectionSlug(sorted[j])
		if left == right {
			return strings.TrimSpace(sorted[i].Name) < strings.TrimSpace(sorted[j].Name)
		}
		return left < right
	})
	return sorted
}

func cmsCollectionSlug(collection webflow.CMSCollection) string {
	slug := normalizePageSlug(collection.Slug)
	if slug != "" {
		return slug
	}
	return normalizePageSlug(collection.Name)
}

func cmsDatabaseName(database *webflow.CMSDatabase) string {
	if database == nil {
		return ""
	}
	return strings.TrimSpace(database.Name)
}

func cmsItemFileStem(item map[string]interface{}) string {
	if slug := firstNestedString(item, "fieldData", "slug"); slug != "" {
		return normalizePageSlug(slug)
	}
	if slug := firstMapString(item, "slug", "name", "_id", "id"); slug != "" {
		return normalizePageSlug(slug)
	}
	return "item"
}

func writeJSONFile(path string, value interface{}) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create %s: %w", filepath.Dir(path), err)
	}
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode %s: %w", path, err)
	}
	encoded = append(encoded, '\n')
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	return nil
}

func firstNestedString(item map[string]interface{}, parent string, keys ...string) string {
	nested, ok := item[parent]
	if !ok {
		return ""
	}
	nestedMap, ok := nested.(map[string]interface{})
	if !ok {
		return ""
	}
	return firstMapString(nestedMap, keys...)
}

func firstMapString(item map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		value, ok := item[key]
		if !ok {
			continue
		}
		if str, ok := value.(string); ok {
			str = strings.TrimSpace(str)
			if str != "" {
				return str
			}
		}
	}
	return ""
}
