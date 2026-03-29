package webflow

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type CMSSnapshot struct {
	SiteName       string       `json:"siteName"`
	Database       *CMSDatabase `json:"database"`
	MaxCollections int          `json:"maxCollections"`
	MaxLocales     int          `json:"maxLocales"`
}

type CMSDatabase struct {
	ID                 string          `json:"_id"`
	Name               string          `json:"name"`
	Description        string          `json:"description"`
	Collections        []CMSCollection `json:"collections"`
	MaxFieldLimit      int             `json:"maxFieldLimit"`
	MaxItemLimit       int             `json:"maxItemLimit"`
	MaxCollectionLimit int             `json:"maxCollectionLimit"`
	MaxLocales         int             `json:"maxLocales"`
}

type CMSCollection struct {
	ID                           string         `json:"_id"`
	Name                         string         `json:"name"`
	Slug                         string         `json:"slug"`
	SingularName                 string         `json:"singularName"`
	Database                     string         `json:"database"`
	DetailPage                   string         `json:"detailPage"`
	TotalNumberOfItems           int            `json:"totalNumberOfItems"`
	TotalNumberOfItemsByLocaleID map[string]int `json:"totalNumberOfItemsByLocaleId"`
	Fields                       []CMSField     `json:"fields"`
	DeletedFields                []CMSField     `json:"deletedFields"`
	CreatedOn                    string         `json:"createdOn"`
	LastUpdated                  string         `json:"lastUpdated"`
	Plugin                       string         `json:"plugin"`
}

type CMSField struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Slug        string                 `json:"slug"`
	Type        string                 `json:"type"`
	Required    bool                   `json:"required"`
	Editable    bool                   `json:"editable"`
	Unique      bool                   `json:"unique"`
	Default     interface{}            `json:"default,omitempty"`
	Validations map[string]interface{} `json:"validations,omitempty"`
}

type CMSItemsResponse struct {
	URL     string                   `json:"url"`
	Object  string                   `json:"object"`
	Items   []map[string]interface{} `json:"items"`
	Count   int                      `json:"count"`
	Limit   int                      `json:"limit"`
	Offset  int                      `json:"offset"`
	Total   int                      `json:"total"`
	HasMore bool                     `json:"hasMore"`
}

type CMSItemMutationResponse struct {
	URL         string                   `json:"url"`
	Object      string                   `json:"object"`
	Items       []map[string]interface{} `json:"items,omitempty"`
	UpdatedItem map[string]interface{}   `json:"updated_item,omitempty"`
	Deleted     int                      `json:"deleted,omitempty"`
}

func GetCMSSnapshot(ctx context.Context, siteName, token, cookies string) (CMSSnapshot, error) {
	requestURL := siteAPIURL(siteName, fmt.Sprintf("dom?workflow=cms&t=%d", time.Now().UnixMilli()))
	return getCMSSnapshotFromURL(ctx, requestURL, token, cookies)
}

func ListCMSCollections(ctx context.Context, siteName, token, cookies string) (CMSSnapshot, []CMSCollection, error) {
	snapshot, err := GetCMSSnapshot(ctx, siteName, token, cookies)
	if err != nil {
		return CMSSnapshot{}, nil, err
	}
	if snapshot.Database == nil {
		return snapshot, nil, nil
	}
	return snapshot, append([]CMSCollection(nil), snapshot.Database.Collections...), nil
}

func GetCMSCollectionItems(ctx context.Context, siteName, collectionID, target, token, cookies string) (CMSItemsResponse, error) {
	return getCMSCollectionItemsPage(ctx, siteName, collectionID, target, token, cookies, 0, 100)
}

func getCMSCollectionItemsPage(ctx context.Context, siteName, collectionID, target, token, cookies string, offset, limit int) (CMSItemsResponse, error) {
	collectionID = strings.TrimSpace(collectionID)
	if collectionID == "" {
		return CMSItemsResponse{}, fmt.Errorf("missing CMS collection id")
	}
	siteName = strings.TrimSpace(siteName)
	if siteName == "" {
		return CMSItemsResponse{}, fmt.Errorf("missing Webflow site name")
	}

	target = strings.ToLower(strings.TrimSpace(target))
	if target == "" {
		target = "staging"
	}
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	query := url.Values{}
	query.Set("target", target)
	query.Set("offset", fmt.Sprintf("%d", offset))
	query.Set("limit", fmt.Sprintf("%d", limit))
	query.Add("sort[]", "-created-on")
	query.Set("format", "withoutHeavyFields")

	requestURL := fmt.Sprintf("https://%s.design.webflow.com/api/v1/collections/%s/items?%s", siteName, collectionID, query.Encode())
	firstPage, err := getCMSCollectionItemsFromURL(ctx, requestURL, token, cookies)
	if err != nil {
		return CMSItemsResponse{}, err
	}
	if !firstPage.HasMore && (firstPage.Total == 0 || len(firstPage.Items) >= firstPage.Total) {
		return firstPage, nil
	}

	allItems := append([]map[string]interface{}{}, firstPage.Items...)
	currentPage := firstPage
	for currentPage.HasMore || (currentPage.Total > 0 && len(allItems) < currentPage.Total) {
		nextOffset := len(allItems)
		nextQuery := url.Values{}
		nextQuery.Set("target", target)
		nextQuery.Set("offset", fmt.Sprintf("%d", nextOffset))
		nextQuery.Set("limit", fmt.Sprintf("%d", limit))
		nextQuery.Add("sort[]", "-created-on")
		nextQuery.Set("format", "withoutHeavyFields")
		nextURL := fmt.Sprintf("https://%s.design.webflow.com/api/v1/collections/%s/items?%s", siteName, collectionID, nextQuery.Encode())

		nextPage, err := getCMSCollectionItemsFromURL(ctx, nextURL, token, cookies)
		if err != nil {
			return CMSItemsResponse{}, err
		}
		if len(nextPage.Items) == 0 {
			break
		}
		allItems = append(allItems, nextPage.Items...)
		currentPage = nextPage
	}

	firstPage.Items = allItems
	firstPage.Count = len(allItems)
	firstPage.Offset = 0
	firstPage.Limit = len(allItems)
	firstPage.HasMore = false
	return firstPage, nil
}

func getCMSCollectionItemsFromURL(ctx context.Context, requestURL, token, cookies string) (CMSItemsResponse, error) {
	var payload CMSItemsResponse
	if err := getJSON(ctx, requestURL, cookies, token, &payload); err != nil {
		return CMSItemsResponse{}, fmt.Errorf("failed to load CMS items: %w", err)
	}
	return payload, nil
}

func getCMSSnapshotFromURL(ctx context.Context, requestURL, token, cookies string) (CMSSnapshot, error) {
	var snapshot CMSSnapshot
	if err := getJSON(ctx, requestURL, cookies, token, &snapshot); err != nil {
		return CMSSnapshot{}, fmt.Errorf("failed to load CMS snapshot: %w", err)
	}
	return snapshot, nil
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
	return firstString(nestedMap, keys...)
}

func CreateCMSItem(ctx context.Context, siteName, collectionID, token, cookies string, fields map[string]interface{}) (map[string]interface{}, error) {
	siteName = strings.TrimSpace(siteName)
	if siteName == "" {
		return nil, fmt.Errorf("missing Webflow site name")
	}
	collectionID = strings.TrimSpace(collectionID)
	if collectionID == "" {
		return nil, fmt.Errorf("missing CMS collection id")
	}
	requestURL := fmt.Sprintf("https://%s.design.webflow.com/api/v1/collections/%s/items", siteName, collectionID)
	return createCMSItemFromURL(ctx, requestURL, token, cookies, fields)
}

func UpdateCMSItem(ctx context.Context, siteName, collectionID, itemID, token, cookies string, fields map[string]interface{}) (map[string]interface{}, error) {
	siteName = strings.TrimSpace(siteName)
	if siteName == "" {
		return nil, fmt.Errorf("missing Webflow site name")
	}
	collectionID = strings.TrimSpace(collectionID)
	if collectionID == "" {
		return nil, fmt.Errorf("missing CMS collection id")
	}
	itemID = strings.TrimSpace(itemID)
	if itemID == "" {
		return nil, fmt.Errorf("missing CMS item id")
	}
	requestURL := fmt.Sprintf("https://%s.design.webflow.com/api/v1/collections/%s/items/%s", siteName, collectionID, itemID)
	return updateCMSItemFromURL(ctx, requestURL, token, cookies, fields)
}

func DeleteCMSItem(ctx context.Context, siteName, collectionID, itemID, token, cookies string) error {
	siteName = strings.TrimSpace(siteName)
	if siteName == "" {
		return fmt.Errorf("missing Webflow site name")
	}
	collectionID = strings.TrimSpace(collectionID)
	if collectionID == "" {
		return fmt.Errorf("missing CMS collection id")
	}
	itemID = strings.TrimSpace(itemID)
	if itemID == "" {
		return fmt.Errorf("missing CMS item id")
	}
	requestURL := fmt.Sprintf("https://%s.design.webflow.com/api/v1/collections/%s/items/%s", siteName, collectionID, itemID)
	return deleteCMSItemByURL(ctx, requestURL, token, cookies, itemID)
}

func createCMSItemFromURL(ctx context.Context, requestURL, token, cookies string, fields map[string]interface{}) (map[string]interface{}, error) {
	resp, err := doPost(ctx, requestURL, cookies, token, map[string]interface{}{"fields": fields})
	if err != nil {
		return nil, fmt.Errorf("failed to create CMS item: %w", err)
	}
	defer resp.Body.Close()

	var payload CMSItemMutationResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("failed to decode created CMS item: %w", err)
	}
	if len(payload.Items) == 0 {
		return nil, fmt.Errorf("failed to create CMS item: missing item in response")
	}
	return payload.Items[0], nil
}

func updateCMSItemFromURL(ctx context.Context, requestURL, token, cookies string, fields map[string]interface{}) (map[string]interface{}, error) {
	resp, err := doPatch(ctx, requestURL, cookies, token, map[string]interface{}{"fields": fields})
	if err != nil {
		return nil, fmt.Errorf("failed to update CMS item: %w", err)
	}
	defer resp.Body.Close()

	var payload CMSItemMutationResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("failed to decode updated CMS item: %w", err)
	}
	if len(payload.UpdatedItem) == 0 {
		return nil, fmt.Errorf("failed to update CMS item: missing updated item in response")
	}
	return payload.UpdatedItem, nil
}

func deleteCMSItemByURL(ctx context.Context, requestURL, token, cookies, itemID string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, requestURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create DELETE request: %w", err)
	}
	setupRequestHeaders(req, cookies)
	req.Header.Set("x-xsrf-token", token)

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute DELETE request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete CMS item %s: %w", itemID, parseAPIError(http.MethodDelete, resp.StatusCode, body))
	}

	return nil
}
