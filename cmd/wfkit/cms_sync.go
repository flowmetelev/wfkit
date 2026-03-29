package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"wfkit/internal/webflow"
)

type cmsDiffSummary struct {
	OutputDir     string                     `json:"outputDir"`
	Target        string                     `json:"target"`
	DeleteMissing bool                       `json:"deleteMissing"`
	Collections   []cmsCollectionDiffSummary `json:"collections"`
	Created       int                        `json:"created"`
	Updated       int                        `json:"updated"`
	Deleted       int                        `json:"deleted"`
	Unchanged     int                        `json:"unchanged"`
}

type cmsCollectionDiffSummary struct {
	ID          string               `json:"id"`
	Slug        string               `json:"slug"`
	Name        string               `json:"name"`
	LocalCount  int                  `json:"localCount"`
	RemoteCount int                  `json:"remoteCount"`
	Created     int                  `json:"created"`
	Updated     int                  `json:"updated"`
	Deleted     int                  `json:"deleted"`
	Unchanged   int                  `json:"unchanged"`
	Items       []cmsItemDiffSummary `json:"items"`
}

type cmsItemDiffSummary struct {
	Action string `json:"action"`
	ID     string `json:"id,omitempty"`
	Slug   string `json:"slug,omitempty"`
	Name   string `json:"name,omitempty"`
	Path   string `json:"path,omitempty"`
}

type cmsCollectionSyncPlan struct {
	Collection  webflow.CMSCollection
	LocalItems  []cmsLocalItem
	RemoteItems []map[string]interface{}
	Create      []cmsLocalItem
	Update      []cmsUpdatePlan
	Delete      []map[string]interface{}
	Unchanged   []cmsLocalItem
}

type cmsUpdatePlan struct {
	Local  cmsLocalItem
	Remote map[string]interface{}
}

type cmsLocalItem struct {
	Path   string
	Raw    map[string]interface{}
	Fields map[string]interface{}
	ID     string
	Slug   string
	Name   string
}

func (f *cmsFlow) buildSyncSummary(collections []webflow.CMSCollection, deleteMissing bool) (cmsDiffSummary, []cmsCollectionSyncPlan, error) {
	summary := cmsDiffSummary{
		OutputDir:     f.outputDir(),
		Target:        strings.TrimSpace(f.cliContext.String("target")),
		DeleteMissing: deleteMissing,
		Collections:   []cmsCollectionDiffSummary{},
	}
	if summary.Target == "" {
		summary.Target = "staging"
	}

	plans := make([]cmsCollectionSyncPlan, 0, len(collections))
	for _, collection := range collections {
		plan, err := f.buildCollectionPlan(collection, deleteMissing)
		if err != nil {
			return cmsDiffSummary{}, nil, err
		}
		plans = append(plans, plan)

		diff := cmsCollectionDiffSummary{
			ID:          collection.ID,
			Slug:        cmsCollectionSlug(collection),
			Name:        collection.Name,
			LocalCount:  len(plan.LocalItems),
			RemoteCount: len(plan.RemoteItems),
			Created:     len(plan.Create),
			Updated:     len(plan.Update),
			Deleted:     len(plan.Delete),
			Unchanged:   len(plan.Unchanged),
			Items:       summarizeCollectionPlan(plan),
		}
		summary.Collections = append(summary.Collections, diff)
		summary.Created += diff.Created
		summary.Updated += diff.Updated
		summary.Deleted += diff.Deleted
		summary.Unchanged += diff.Unchanged
	}

	return summary, plans, nil
}

func (f *cmsFlow) buildCollectionPlan(collection webflow.CMSCollection, deleteMissing bool) (cmsCollectionSyncPlan, error) {
	localItems, err := loadLocalCMSItems(f.outputDir(), collection)
	if err != nil {
		return cmsCollectionSyncPlan{}, err
	}
	remoteItems, err := webflow.GetCMSCollectionItems(f.cliContext.Context, f.config.AppName, collection.ID, f.cliContext.String("target"), f.token, f.cookies)
	if err != nil {
		return cmsCollectionSyncPlan{}, fmt.Errorf("failed to load CMS items for %s: %w", cmsCollectionSlug(collection), err)
	}

	return buildCMSCollectionPlan(collection, localItems, remoteItems.Items, deleteMissing), nil
}

func buildCMSCollectionPlan(collection webflow.CMSCollection, localItems []cmsLocalItem, remoteItems []map[string]interface{}, deleteMissing bool) cmsCollectionSyncPlan {
	plan := cmsCollectionSyncPlan{
		Collection:  collection,
		LocalItems:  append([]cmsLocalItem(nil), localItems...),
		RemoteItems: append([]map[string]interface{}(nil), remoteItems...),
	}

	remoteByID := make(map[string]map[string]interface{}, len(remoteItems))
	remoteBySlug := make(map[string]map[string]interface{}, len(remoteItems))
	for _, remote := range remoteItems {
		if id := cmsRemoteItemID(remote); id != "" {
			remoteByID[id] = remote
		}
		if slug := cmsRemoteItemSlug(remote); slug != "" {
			remoteBySlug[slug] = remote
		}
	}

	matchedRemoteIDs := map[string]struct{}{}
	for _, local := range localItems {
		var remote map[string]interface{}
		if local.ID != "" {
			remote = remoteByID[local.ID]
		}
		if remote == nil && local.Slug != "" {
			remote = remoteBySlug[local.Slug]
		}

		if remote == nil {
			plan.Create = append(plan.Create, local)
			continue
		}

		if id := cmsRemoteItemID(remote); id != "" {
			matchedRemoteIDs[id] = struct{}{}
		}

		remoteFields := cmsMutationFields(remote, collection)
		if cmsJSONEqual(local.Fields, remoteFields) {
			plan.Unchanged = append(plan.Unchanged, local)
			continue
		}

		plan.Update = append(plan.Update, cmsUpdatePlan{
			Local:  local,
			Remote: remote,
		})
	}

	if deleteMissing {
		for _, remote := range remoteItems {
			id := cmsRemoteItemID(remote)
			if id == "" {
				continue
			}
			if _, ok := matchedRemoteIDs[id]; ok {
				continue
			}
			plan.Delete = append(plan.Delete, remote)
		}
	}

	return plan
}

func summarizeCollectionPlan(plan cmsCollectionSyncPlan) []cmsItemDiffSummary {
	items := make([]cmsItemDiffSummary, 0, len(plan.Create)+len(plan.Update)+len(plan.Delete)+len(plan.Unchanged))
	for _, item := range plan.Create {
		items = append(items, cmsItemDiffSummary{Action: "create", ID: item.ID, Slug: item.Slug, Name: item.Name, Path: item.Path})
	}
	for _, item := range plan.Update {
		items = append(items, cmsItemDiffSummary{Action: "update", ID: cmsRemoteItemID(item.Remote), Slug: item.Local.Slug, Name: item.Local.Name, Path: item.Local.Path})
	}
	for _, item := range plan.Delete {
		items = append(items, cmsItemDiffSummary{Action: "delete", ID: cmsRemoteItemID(item), Slug: cmsRemoteItemSlug(item), Name: cmsRemoteItemName(item)})
	}
	for _, item := range plan.Unchanged {
		items = append(items, cmsItemDiffSummary{Action: "unchanged", ID: item.ID, Slug: item.Slug, Name: item.Name, Path: item.Path})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Action == items[j].Action {
			return items[i].Slug < items[j].Slug
		}
		return items[i].Action < items[j].Action
	})
	return items
}

func loadLocalCMSItems(outputDir string, collection webflow.CMSCollection) ([]cmsLocalItem, error) {
	itemsDir := filepath.Join(outputDir, cmsCollectionSlug(collection), "items")
	entries, err := os.ReadDir(itemsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("missing local CMS snapshot for %s in %s; run wfkit cms pull first", cmsCollectionSlug(collection), itemsDir)
		}
		return nil, fmt.Errorf("failed to read %s: %w", itemsDir, err)
	}

	items := make([]cmsLocalItem, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(itemsDir, entry.Name())
		raw, err := readCMSJSONFile(path)
		if err != nil {
			return nil, err
		}
		fields := cmsMutationFields(raw, collection)
		items = append(items, cmsLocalItem{
			Path:   path,
			Raw:    raw,
			Fields: fields,
			ID:     cmsRemoteItemID(raw),
			Slug:   cmsRemoteItemSlug(raw),
			Name:   cmsRemoteItemName(raw),
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Slug == items[j].Slug {
			return items[i].Path < items[j].Path
		}
		return items[i].Slug < items[j].Slug
	})
	return items, nil
}

func readCMSJSONFile(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil, fmt.Errorf("failed to decode %s: %w", path, err)
	}
	return decoded, nil
}

func cmsMutationFields(item map[string]interface{}, collection webflow.CMSCollection) map[string]interface{} {
	allowed := map[string]struct{}{}
	for _, field := range collection.Fields {
		slug := strings.TrimSpace(field.Slug)
		if slug != "" && field.Editable {
			allowed[slug] = struct{}{}
		}
	}
	if len(allowed) == 0 {
		allowed["name"] = struct{}{}
		allowed["slug"] = struct{}{}
	}

	fields := map[string]interface{}{}
	if nested, ok := item["fieldData"].(map[string]interface{}); ok {
		for key, value := range nested {
			if _, ok := allowed[key]; ok {
				fields[key] = value
			}
		}
	}
	for key, value := range item {
		if _, ok := allowed[key]; ok {
			fields[key] = value
		}
	}
	return fields
}

func cmsJSONEqual(left, right map[string]interface{}) bool {
	leftJSON, err := json.Marshal(left)
	if err != nil {
		return false
	}
	rightJSON, err := json.Marshal(right)
	if err != nil {
		return false
	}
	return string(leftJSON) == string(rightJSON)
}

func cmsRemoteItemID(item map[string]interface{}) string {
	return firstMapString(item, "_id", "id")
}

func cmsRemoteItemSlug(item map[string]interface{}) string {
	if slug := firstNestedString(item, "fieldData", "slug"); slug != "" {
		return normalizePageSlug(slug)
	}
	return normalizePageSlug(firstMapString(item, "slug", "name"))
}

func cmsRemoteItemName(item map[string]interface{}) string {
	if name := firstNestedString(item, "fieldData", "name"); name != "" {
		return name
	}
	return firstMapString(item, "name")
}
